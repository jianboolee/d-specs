// Package agent 实现 AI 编排循环。
//
// 循环形态：模型读工具取数 → 产出 propose_ops → 服务端校验 →
// 校验失败则把错误回灌让模型自我修正 → 通过则应用并产生新快照。
//
// 只接 Flash 档位时，这个「服务端校验 + 错误回灌」的闭环是可靠性的主要来源，
// 不能指望模型一次就把 ops 写对。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jianboolee/d-specs/internal/ai/ops"
	"github.com/jianboolee/d-specs/internal/ai/provider"
	"github.com/jianboolee/d-specs/internal/ai/tools"
)

var (
	// ErrNoLLM LLM 未配置
	ErrNoLLM = errors.New("LLM 未配置，无法进行对话")
)

type EventType string

const (
	EventText       EventType = "text"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventApplied    EventType = "applied"
	EventPending    EventType = "pending"
)

// Event 编排过程中的事件，用于驱动 SSE 与前端实时展示
type Event struct {
	Type       EventType    `json:"type"`
	Text       string       `json:"text,omitempty"`
	ToolName   string       `json:"tool_name,omitempty"`
	ToolArgs   string       `json:"tool_args,omitempty"`
	ToolResult any          `json:"tool_result,omitempty"`
	Summary    string       `json:"summary,omitempty"`
	Changes    []ops.Change `json:"changes,omitempty"`
	Message    string       `json:"message,omitempty"`
}

type EmitFunc func(Event)

// ApplyHook 让调用方接管「ops 生效」这一步。
//
// 不设置时 agent 直接在自己的内存快照上生效（`cmd/aicheck` 就是这么用的）。
// 设置后由调用方负责通过数据库落地并回传最新快照 —— 这样对话过程中模型
// 后续的读操作看到的都是真实数据，并发冲突之类的错误也能作为工具结果
// 回灌给模型让它重试，而不是只在最后悄悄失败一次。
type ApplyHook func(
	ctx context.Context,
	operations []ops.Op,
	summary string,
) (ops.Snapshot, []ops.Change, error)

type Config struct {
	MaxToolRounds   int
	AutoApplyMaxOps int
	MaxTokens       int
	Temperature     float64
	ApplyHook       ApplyHook
}

type Agent struct {
	llm       provider.Provider
	cfg       Config
	applyHook ApplyHook
}

func New(llm provider.Provider, cfg Config) *Agent {
	if cfg.MaxToolRounds <= 0 {
		cfg.MaxToolRounds = 6
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	return &Agent{llm: llm, cfg: cfg, applyHook: cfg.ApplyHook}
}

// RunResult 一轮对话的结果
type RunResult struct {
	// Reply 模型的最终文字回复
	Reply string
	// Snapshot 应用变更后的快照；未应用时与输入一致
	Snapshot ops.Snapshot
	// Changes 本次实际生效的变更
	Changes []ops.Change
	// Summary 模型给的变更说明
	Summary string
	// Applied 是否真的改了数据
	Applied bool
	// Pending 被安全阀拦下、等待用户确认的 ops
	Pending       []ops.Op
	PendingReason string
	Usage         provider.Usage
	Rounds        int
	// Truncated 是否因达到轮数上限而中断
	Truncated bool
}

// Run 执行一轮对话。
//
// history 是不含 system prompt 的历史消息；snapshot 是当前表状态。
// emit 可以为 nil。
func (a *Agent) Run(
	ctx context.Context,
	history []provider.Message,
	snapshot ops.Snapshot,
	emit EmitFunc,
) (*RunResult, error) {
	if a.llm == nil {
		return nil, ErrNoLLM
	}

	messages := make([]provider.Message, 0, len(history)+8)
	messages = append(messages, provider.Message{
		Role:    provider.RoleSystem,
		Content: BuildSystemPrompt(snapshot),
	})
	messages = append(messages, history...)

	res := &RunResult{Snapshot: snapshot}
	registry := tools.NewRegistry(snapshot)
	toolDefs := tools.Definitions()

	for round := 0; round < a.cfg.MaxToolRounds; round++ {
		res.Rounds = round + 1

		resp, err := a.llm.Chat(ctx, provider.ChatRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Temperature: a.cfg.Temperature,
			MaxTokens:   a.cfg.MaxTokens,
		})
		if err != nil {
			return nil, err
		}
		res.Usage = res.Usage.Add(resp.Usage)

		calls := resp.ToolCalls()
		if len(calls) == 0 {
			res.Reply = strings.TrimSpace(resp.Message.Content)
			res.Snapshot = registry.Snapshot()
			// 推理模型的思考与正文共享 max_tokens 预算，预算太小时正文会为空。
			// 必须显式告知，否则前端只会显示一条空白回复。
			if res.Reply == "" && resp.FinishReason == "length" {
				res.Truncated = true
				res.Reply = "模型输出被截断，未产生正文。请调大 AI_MAX_TOKENS 后重试。"
			}
			return res, nil
		}

		// 模型在调用工具前可能先说了半句话，一并透出
		if text := strings.TrimSpace(resp.Message.Content); text != "" {
			emitEvent(emit, Event{Type: EventText, Text: text})
		}

		messages = append(messages, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Message.Content,
			ToolCalls: calls,
		})

		for _, call := range calls {
			name := call.Function.Name
			emitEvent(emit, Event{
				Type:     EventToolCall,
				ToolName: name,
				ToolArgs: call.Function.Arguments,
			})

			result, callErr := a.executeTool(ctx, call, registry, res, emit)

			messages = append(messages, provider.Message{
				Role:       provider.RoleTool,
				ToolCallID: call.ID,
				Content:    encodeToolResult(result, callErr),
			})

			emitEvent(emit, Event{
				Type:       EventToolResult,
				ToolName:   name,
				ToolResult: result,
				Message:    errorText(callErr),
			})
		}
	}

	// 轮数用尽：把已有结果交回去，同时标记中断，由上层决定如何提示用户
	res.Truncated = true
	res.Snapshot = registry.Snapshot()
	return res, nil
}

func (a *Agent) executeTool(
	ctx context.Context,
	call provider.ToolCall,
	registry *tools.Registry,
	res *RunResult,
	emit EmitFunc,
) (any, error) {
	name := call.Function.Name

	args, err := call.Function.ParseArguments()
	if err != nil {
		return nil, err
	}

	if tools.IsReadTool(name) {
		return registry.Execute(name, args)
	}

	if name != tools.ToolProposeOps {
		return nil, fmt.Errorf("未知工具 %s", name)
	}

	return a.handleProposeOps(ctx, args, registry, res, emit)
}

// handleProposeOps 处理唯一的写工具。
//
// 校验失败时返回结构化的错误对象（而不是 error），这样它会作为 tool result
// 正常回灌给模型，让模型在下一轮自我修正 —— 这是整个可靠性的关键回路。
func (a *Agent) handleProposeOps(
	ctx context.Context,
	args map[string]any,
	registry *tools.Registry,
	res *RunResult,
	emit EmitFunc,
) (any, error) {
	summary := strings.TrimSpace(stringValue(args["summary"]))

	operations, err := decodeOps(args["ops"])
	if err != nil {
		return rejected(err.Error()), nil
	}

	// 先在校验一遍：无论最终是本地生效还是交给 ApplyHook 落库，
	// 这一步都能给出即时、统一格式的错误，也用于安全阀的 diff 预览
	current := registry.Snapshot()
	next, changes, applyErr := ops.Apply(current, operations)
	if applyErr != nil {
		return rejected(applyErr.Error()), nil
	}

	// 安全阀：规模过大的变更不自动落地，交由用户确认
	if a.cfg.AutoApplyMaxOps > 0 && len(operations) > a.cfg.AutoApplyMaxOps {
		reason := fmt.Sprintf("本次包含 %d 条操作，超过自动应用上限 %d 条，需要你确认后再应用",
			len(operations), a.cfg.AutoApplyMaxOps)
		res.Pending = operations
		res.PendingReason = reason
		emitEvent(emit, Event{
			Type:    EventPending,
			Summary: summary,
			Changes: changes,
			Message: reason,
		})
		return map[string]any{
			"ok":      true,
			"applied": false,
			"reason":  reason,
			"hint":    "变更已提交等待用户确认，请不要再重复调用 propose_ops，直接用中文向用户说明这次的改动内容并请其确认",
			"diff":    changes,
		}, nil
	}

	// 落地。有 ApplyHook 时以数据库为准，否则在内存快照上生效
	applied := next
	appliedChanges := changes
	if a.applyHook != nil {
		persisted, persistedChanges, hookErr := a.applyHook(ctx, operations, summary)
		if hookErr != nil {
			return rejected(hookErr.Error()), nil
		}
		applied = persisted
		appliedChanges = persistedChanges
	}

	registry.SetSnapshot(applied)
	res.Changes = append(res.Changes, appliedChanges...)
	res.Applied = true
	if summary != "" {
		res.Summary = summary
	}

	emitEvent(emit, Event{
		Type:    EventApplied,
		Summary: summary,
		Changes: appliedChanges,
	})

	return map[string]any{
		"ok":      true,
		"applied": true,
		"summary": summary,
		"changes": appliedChanges,
		"hint":    "变更已生效。请用中文简要说明改了什么，不要重复列出全部数据",
	}, nil
}

// rejected 构造回灌给模型的校验失败结果
func rejected(message string) map[string]any {
	return map[string]any{
		"ok":    false,
		"error": message,
		"hint":  "请根据错误信息修正后重新调用 propose_ops；如果是因为参数名不对，先用 search_items 确认实际参数名",
	}
}

func decodeOps(raw any) ([]ops.Op, error) {
	if raw == nil {
		return nil, errors.New("缺少 ops 字段")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("ops 无法解析：%w", err)
	}
	var out []ops.Op
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("ops 格式不正确：%w", err)
	}
	if len(out) == 0 {
		return nil, errors.New("ops 为空数组")
	}
	return out, nil
}

func encodeToolResult(result any, err error) string {
	if err != nil {
		payload := map[string]any{"ok": false, "error": err.Error()}
		data, mErr := json.Marshal(payload)
		if mErr != nil {
			return `{"ok":false,"error":"工具执行失败"}`
		}
		return string(data)
	}

	data, mErr := json.Marshal(result)
	if mErr != nil {
		return `{"ok":false,"error":"结果序列化失败"}`
	}
	return string(data)
}

func emitEvent(emit EmitFunc, ev Event) {
	if emit == nil {
		return
	}
	emit(ev)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
