// Package provider 定义 LLM 供应商抽象。
//
// 当前只接 DeepSeek，但保留接口是为了让「换模型 / 加档位」不影响 ai 编排层。
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// 消息角色
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// ToolCallTypeFunction 目前只有函数调用一种工具类型
const ToolCallTypeFunction = "function"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	// ReasoningContent 思考过程。
	//
	// deepseek-flash 是推理模型，会把思考单独放在这个字段里，且它**与 content
	// 共享 max_tokens 预算** —— max_tokens 太小会导致正文为空。
	// 不展示给用户，也不回传给模型（agent 组装历史消息时不会带上它）。
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ParseArguments 解析工具调用参数。
//
// 模型偶尔会返回空串或非对象 JSON，这里统一兜底成空 map，
// 让上层校验器去报「缺少必需参数」，而不是在解析阶段炸掉。
func (f ToolCallFunction) ParseArguments() (map[string]any, error) {
	raw := f.Arguments
	if raw == "" || raw == "null" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析 %s 参数失败: %w", f.Name, err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// NewFunctionTool 构造一个 function 类型工具定义
func NewFunctionTool(name, description string, parameters map[string]any) Tool {
	return Tool{
		Type: ToolCallTypeFunction,
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

type ChatRequest struct {
	Messages    []Message
	Tools       []Tool
	Temperature float64
	MaxTokens   int
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// CachedTokens 命中前缀缓存的 prompt token 数。
	// agent loop 每轮都会重发增长中的前缀，缓存命中率直接决定成本。
	CachedTokens int `json:"cached_tokens"`
	// ReasoningTokens 思考消耗的 token，已计入 CompletionTokens。
	// 推理模型下这部分可能占大头。
	ReasoningTokens int `json:"reasoning_tokens"`
}

func (u Usage) Add(other Usage) Usage {
	return Usage{
		PromptTokens:     u.PromptTokens + other.PromptTokens,
		CompletionTokens: u.CompletionTokens + other.CompletionTokens,
		TotalTokens:      u.TotalTokens + other.TotalTokens,
		CachedTokens:     u.CachedTokens + other.CachedTokens,
		ReasoningTokens:  u.ReasoningTokens + other.ReasoningTokens,
	}
}

type ChatResponse struct {
	Message      Message
	FinishReason string
	Usage        Usage
}

// ToolCalls 本次响应要求调用的工具
func (r *ChatResponse) ToolCalls() []ToolCall {
	return r.Message.ToolCalls
}

// StreamDelta 流式响应中的一段增量。
//
// Content 是正文增量，直接按顺序拼接即为完整正文；Reasoning 是思考增量，
// 不展示给用户，仅用于观测。两者是增量而不是累计值。
type StreamDelta struct {
	Content   string
	Reasoning string
}

type Provider interface {
	Name() string
	Model() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// ChatStream 以流式方式执行，每收到一段正文/思考增量就回调一次 onDelta，
	// 最终返回与 Chat 同构的完整响应（含累计正文、工具调用与用量）。
	//
	// onDelta 可以为 nil。回调在调用方 goroutine 内同步执行，不应阻塞过久。
	ChatStream(ctx context.Context, req ChatRequest, onDelta func(StreamDelta)) (*ChatResponse, error)
	// ListModels 拉取账号可用模型列表，用于部署前确认模型 id 是否正确
	ListModels(ctx context.Context) ([]string, error)
}

// APIError 供应商返回的结构化错误
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("llm api error (http %d, %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("llm api error (http %d): %s", e.StatusCode, e.Message)
}

// IsRetryable 是否值得重试（限流与服务端错误）
func (e *APIError) IsRetryable() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500
}

// IsAPIError 判断并取出结构化 API 错误
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
