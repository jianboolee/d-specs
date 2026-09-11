package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/donktech/d-specs/internal/ai/agent"
	"github.com/donktech/d-specs/internal/ai/ops"
	"github.com/donktech/d-specs/internal/ai/provider"
	"github.com/donktech/d-specs/internal/config"
	"github.com/donktech/d-specs/internal/modules/spec"
)

// historyLimit 回放给模型的历史消息条数
const historyLimit = 20

var (
	// ErrLLMUnavailable 未配置 LLM
	ErrLLMUnavailable = errors.New("AI 未配置，无法使用对话功能")
	// ErrEmptyMessage 空消息
	ErrEmptyMessage = errors.New("消息不能为空")
)

type Service struct {
	repo    *Repository
	specSvc *spec.Service
	llm     provider.Provider
	cfg     *config.Config
}

func NewService(repo *Repository, specSvc *spec.Service, llm provider.Provider, cfg *config.Config) *Service {
	return &Service{repo: repo, specSvc: specSvc, llm: llm, cfg: cfg}
}

// Prepare 在建立 SSE 流之前做前置校验。
//
// 必须在写 SSE 响应头之前调用：一旦开始流式输出，HTTP 状态码就固定是 200，
// 「表不存在」「消息为空」「AI 未配置」这类错误只能变成流里的 error 事件，
// 前端就失去了用状态码区分错误的能力。
func (s *Service) Prepare(ctx context.Context, userID, tableID, message string) error {
	if s.llm == nil {
		return ErrLLMUnavailable
	}
	// binding:"required" 只挡得住「字段缺失」，全是空白的字符串要靠这里
	if strings.TrimSpace(message) == "" {
		return ErrEmptyMessage
	}
	_, err := s.specSvc.Detail(ctx, userID, tableID)
	return err
}

// Stream 处理一轮对话，并通过 emit 把事件推给调用方（SSE handler）。
func (s *Service) Stream(ctx context.Context, userID, tableID string, req SendRequest, emit Emit) error {
	if s.llm == nil {
		return ErrLLMUnavailable
	}

	text := strings.TrimSpace(req.Message)
	if text == "" {
		return ErrEmptyMessage
	}

	// 先确认表归属并取当前快照；越权会在这一步被挡掉
	detail, err := s.specSvc.Detail(ctx, userID, tableID)
	if err != nil {
		return err
	}

	session, err := s.repo.GetOrCreateSession(ctx, userID, tableID, req.SessionID, text)
	if err != nil {
		return err
	}

	if err := emit(Event{Type: EventStart, SessionID: session.ID}); err != nil {
		return err
	}

	// 用户消息先落库，这样即使后面模型调用失败，这轮提问也不会丢
	if err := s.repo.SaveMessage(ctx, &ChatMessage{
		SessionID: session.ID,
		Role:      RoleUser,
		Content:   text,
	}); err != nil {
		return err
	}

	history, err := s.buildHistory(ctx, userID, session.ID)
	if err != nil {
		return err
	}

	// 每次对话单独构造 agent：ApplyHook 需要绑定到具体的 user / table / session
	var lastRevision int
	ag := agent.New(s.llm, agent.Config{
		MaxToolRounds:   s.cfg.AI.MaxToolRounds,
		AutoApplyMaxOps: s.cfg.AI.AutoApplyMaxOps,
		MaxTokens:       s.cfg.AI.MaxTokens,
		Temperature:     s.cfg.AI.Temperature,
		ApplyHook: func(
			hookCtx context.Context,
			operations []ops.Op,
			summary string,
		) (ops.Snapshot, []ops.Change, error) {
			snapshot, changes, revision, hookErr := s.applyOps(hookCtx, userID, tableID, session.ID, operations, summary)
			if hookErr == nil {
				lastRevision = revision
			}
			return snapshot, changes, hookErr
		},
	})

	// agent 的 emit 回调没有返回值，客户端断开的信号先记下来，Run 结束后统一处理
	var emitErr error
	res, runErr := ag.Run(ctx, history, ops.Snapshot{
		Title: detail.Table.Title,
		Items: detail.Items,
	}, func(ev agent.Event) {
		if emitErr != nil {
			return
		}
		event := Event{SessionID: session.ID}
		switch ev.Type {
		case agent.EventText:
			event.Type = EventText
			event.Text = ev.Text
		case agent.EventToolCall:
			event.Type = EventToolCall
			event.ToolName = ev.ToolName
			event.ToolArgs = ev.ToolArgs
		case agent.EventToolResult:
			event.Type = EventToolResult
			event.ToolName = ev.ToolName
		case agent.EventApplied:
			event.Type = EventApplied
			event.Summary = ev.Summary
			event.Changes = toChangeViews(ev.Changes)
			event.Revision = lastRevision
		case agent.EventPending:
			event.Type = EventPending
			event.Summary = ev.Summary
			event.Changes = toChangeViews(ev.Changes)
			event.Message = ev.Message
		default:
			return
		}
		emitErr = emit(event)
	})

	if runErr != nil {
		return runErr
	}
	if emitErr != nil {
		// 客户端已断开，不必再写回复
		return emitErr
	}

	// 助手回复落库
	assistant := &ChatMessage{
		SessionID: session.ID,
		Role:      RoleAssistant,
		Content:   res.Reply,
	}
	if usageJSON, mErr := json.Marshal(res.Usage); mErr == nil {
		assistant.Usage = usageJSON
	}
	if err := s.repo.SaveMessage(ctx, assistant); err != nil {
		return err
	}
	_ = s.repo.TouchSession(ctx, session.ID)

	return emit(Event{
		Type:      EventDone,
		SessionID: session.ID,
		MessageID: assistant.ID,
		Reply:     res.Reply,
		Usage:     toUsageView(res.Usage),
	})
}

// applyOps 把 AI 产出的 ops 落到数据库。
//
// 这里是「校验 → 落库 → 审计」的汇合点：ops 仍然走 spec 模块的统一执行器，
// 所以 AI 变更与手工变更共享同一套校验、乐观锁与版本快照。
func (s *Service) applyOps(
	ctx context.Context,
	userID, tableID, sessionID string,
	operations []ops.Op,
	summary string,
) (ops.Snapshot, []ops.Change, int, error) {
	opsJSON, err := json.Marshal(operations)
	if err != nil {
		return ops.Snapshot{}, nil, 0, err
	}

	res, applyErr := s.specSvc.Apply(ctx, userID, tableID, spec.ApplyRequest{
		Source:  spec.SourceAI,
		Summary: summary,
		Ops:     operations,
	})
	if applyErr != nil {
		status := StatusFailed
		if errors.Is(applyErr, spec.ErrOpsRejected) {
			status = StatusRejected
		}
		// 失败也要留痕：「AI 想改但没改成」是排查线上问题的第一手材料
		_ = s.repo.SaveAIOperation(ctx, &AIOperation{
			TableID:   tableID,
			SessionID: sessionID,
			Ops:       opsJSON,
			Summary:   summary,
			Status:    status,
			Error:     applyErr.Error(),
		})
		return ops.Snapshot{}, nil, 0, applyErr
	}

	revision := res.Revision
	_ = s.repo.SaveAIOperation(ctx, &AIOperation{
		TableID:   tableID,
		SessionID: sessionID,
		Revision:  &revision,
		Ops:       opsJSON,
		Summary:   summary,
		Status:    StatusApplied,
	})

	// 取回最新快照，让模型后续的读操作看到的是真实数据
	detail, err := s.specSvc.Detail(ctx, userID, tableID)
	if err != nil {
		return ops.Snapshot{}, nil, revision, err
	}

	return ops.Snapshot{Title: detail.Table.Title, Items: detail.Items}, res.Changes, revision, nil
}

// buildHistory 只回放 user / assistant 的文本，不回放 tool 回合。
//
// 工具返回的是「当时那张表」的数据，塞回上下文既占 token、又可能与当前
// 真实状态冲突。模型判断当前状态靠的是 system prompt 里的表结构，
// 需要具体值时随时用只读工具查。
//
// 这也让上下文规模与对话轮数近似线性，而不是被工具输出撑爆。
func (s *Service) buildHistory(ctx context.Context, userID, sessionID string) ([]provider.Message, error) {
	rows, err := s.repo.ListMessages(ctx, userID, sessionID, historyLimit)
	if err != nil {
		return nil, err
	}

	out := make([]provider.Message, 0, len(rows))
	for _, m := range rows {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		switch m.Role {
		case RoleUser:
			out = append(out, provider.Message{Role: provider.RoleUser, Content: content})
		case RoleAssistant:
			out = append(out, provider.Message{Role: provider.RoleAssistant, Content: content})
		}
	}
	return out, nil
}

// ==================== 查询 ====================

func (s *Service) Sessions(ctx context.Context, userID, tableID string) ([]SessionView, error) {
	if _, err := s.specSvc.Detail(ctx, userID, tableID); err != nil {
		return nil, err
	}

	rows, err := s.repo.ListSessions(ctx, userID, tableID)
	if err != nil {
		return nil, err
	}

	out := make([]SessionView, 0, len(rows))
	for _, r := range rows {
		out = append(out, SessionView{
			ID:           r.ID,
			TableID:      r.TableID,
			Title:        r.Title,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
			MessageCount: r.MessageCount,
		})
	}
	return out, nil
}

func (s *Service) Messages(ctx context.Context, userID, sessionID string, limit int) ([]MessageView, error) {
	rows, err := s.repo.ListMessages(ctx, userID, sessionID, limit)
	if err != nil {
		return nil, err
	}

	out := make([]MessageView, 0, len(rows))
	for _, r := range rows {
		if r.Role == RoleTool || strings.TrimSpace(r.Content) == "" {
			continue
		}
		out = append(out, MessageView{
			ID:        r.ID,
			Role:      r.Role,
			Content:   r.Content,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// ==================== 转换 ====================

func toChangeViews(changes []ops.Change) []ChangeView {
	out := make([]ChangeView, 0, len(changes))
	for _, c := range changes {
		v := ChangeView{Kind: string(c.Kind), ItemID: c.ItemID, Name: c.Name}
		if c.Before != nil {
			v.Before = formatItemValue(*c.Before)
		}
		if c.After != nil {
			v.After = formatItemValue(*c.After)
		}
		out = append(out, v)
	}
	return out
}

func formatItemValue(item ops.Item) string {
	value := item.Value
	if value == "" {
		value = "—"
	}
	if item.Unit != "" {
		return value + " " + item.Unit
	}
	return value
}
