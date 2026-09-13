package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jianboolee/d-specs/internal/ai/agent"
	"github.com/jianboolee/d-specs/internal/ai/ops"
	"github.com/jianboolee/d-specs/internal/ai/provider"
	"github.com/jianboolee/d-specs/internal/config"
	"github.com/jianboolee/d-specs/internal/modules/spec"
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
		Title:       detail.Table.Title,
		Description: detail.Table.Description,
		Items:       detail.Items,
	}, func(ev agent.Event) {
		if emitErr != nil {
			return
		}
		event := Event{SessionID: session.ID}
		switch ev.Type {
		case agent.EventText:
			event.Type = EventText
			// 流式下 text 是增量（delta），原样透传由前端增量拼接
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
//
// 参数项 op 与表级元信息 op 分开落库：前者走版本化的 Apply（产生 revision、
// 可撤销），后者走 UpdateMeta（不产生 revision、不可撤销）。只有参数项改动
// 才会推进版本号。
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

	itemOps, tableOps := splitTableOps(operations)

	// 表级改动不在版本历史里，撤销无意义；若这批同时含参数改动，
	// 只对参数部分给出「可撤销」的信号（表级变更本身无法回退）。
	revision, changes, applyErr := s.applySplitOps(ctx, userID, tableID, itemOps, tableOps, summary)
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

	rev := revision
	_ = s.repo.SaveAIOperation(ctx, &AIOperation{
		TableID:   tableID,
		SessionID: sessionID,
		Revision:  &rev,
		Ops:       opsJSON,
		Summary:   summary,
		Status:    StatusApplied,
	})

	// 取回最新快照，让模型后续的读操作看到的是真实数据
	detail, err := s.specSvc.Detail(ctx, userID, tableID)
	if err != nil {
		return ops.Snapshot{}, nil, revision, err
	}

	return ops.Snapshot{
		Title:       detail.Table.Title,
		Description: detail.Table.Description,
		Items:       detail.Items,
	}, changes, revision, nil
}

// applySplitOps 先应用参数项 op（版本化），再应用表级 op（元信息）。
//
// 顺序有讲究：先落参数再改元信息，这样即便元信息失败，参数改动也已生效，
// 不会出现「表名改了但参数没改」这种半截状态。
func (s *Service) applySplitOps(
	ctx context.Context,
	userID, tableID string,
	itemOps, tableOps []ops.Op,
	summary string,
) (int, []ops.Change, error) {
	// 变更前后值取自进入时的真实状态，供前端展示
	before, err := s.specSvc.Detail(ctx, userID, tableID)
	if err != nil {
		return 0, nil, err
	}
	revision := before.Table.CurrentRevision

	var changes []ops.Change

	if len(itemOps) > 0 {
		res, err := s.specSvc.Apply(ctx, userID, tableID, spec.ApplyRequest{
			Source:  spec.SourceAI,
			Summary: summary,
			Ops:     itemOps,
		})
		if err != nil {
			return 0, nil, err
		}
		revision = res.Revision
		changes = append(changes, res.Changes...)
	}

	if len(tableOps) > 0 {
		tableChanges, err := s.applyTableOps(ctx, userID, tableID, before.Table, tableOps)
		if err != nil {
			return 0, nil, fmt.Errorf("%w：%s", spec.ErrOpsRejected, err.Error())
		}
		changes = append(changes, tableChanges...)
	}

	return revision, changes, nil
}

// applyTableOps 应用表级元信息 op（标题/描述）。
//
// 复用 ops.Apply 做校验与变更预览：传一份最小快照即可，只取其中的表级变更。
func (s *Service) applyTableOps(
	ctx context.Context,
	userID, tableID string,
	table spec.TableDetail,
	tableOps []ops.Op,
) ([]ops.Change, error) {
	base := ops.Snapshot{Title: table.Title, Description: table.Description}
	_, changes, err := ops.Apply(base, tableOps)
	if err != nil {
		return nil, err
	}

	patch := spec.UpdateRequest{}
	for _, c := range changes {
		text := c.NewValue
		switch c.Name {
		case "表标题":
			patch.Title = &text
		case "表描述":
			patch.Description = &text
		}
	}

	if _, err := s.specSvc.UpdateMeta(ctx, userID, tableID, patch); err != nil {
		return nil, err
	}
	return changes, nil
}

// splitTableOps 按「是否为表级元信息操作」拆分，保持原有先后顺序。
func splitTableOps(operations []ops.Op) (itemOps, tableOps []ops.Op) {
	for _, op := range operations {
		if ops.IsTableOp(op.Op) {
			tableOps = append(tableOps, op)
			continue
		}
		itemOps = append(itemOps, op)
	}
	return itemOps, tableOps
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
		// 表级变更（标题/描述）不走 Item 前后值，用 OldValue/NewValue 承载
		if c.Kind == ops.ChangeTable {
			v.Before = c.OldValue
			v.After = c.NewValue
		}
		out = append(out, v)
	}
	return out
}

func formatItemValue(item ops.Item) string {
	if item.Value == "" {
		return "—"
	}
	// 值本身已包含单位，直接展示
	return item.Value
}
