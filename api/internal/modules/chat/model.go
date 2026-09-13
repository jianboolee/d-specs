package chat

import (
	"time"

	"github.com/jianboolee/d-specs/internal/ai/followups"
	"github.com/jianboolee/d-specs/internal/ai/provider"
)

// 消息角色
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// AI 操作状态
const (
	StatusApplied  = "applied"
	StatusRejected = "rejected"
	StatusFailed   = "failed"
)

// ChatSession 会话
type ChatSession struct {
	ID        string    `gorm:"column:id;primaryKey"`
	TableID   string    `gorm:"column:table_id"`
	UserID    string    `gorm:"column:user_id"`
	Title     string    `gorm:"column:title"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (ChatSession) TableName() string { return "chat_sessions" }

// ChatMessage 消息。
//
// 完整保留 tool 回合，是为了「AI 到底做了什么」可追溯；
// 但回放给模型时只取 user/assistant 的文本（见 Service.buildHistory）。
type ChatMessage struct {
	ID         string    `gorm:"column:id;primaryKey"`
	SessionID  string    `gorm:"column:session_id"`
	Role       string    `gorm:"column:role"`
	Content    string    `gorm:"column:content"`
	ToolCalls  []byte    `gorm:"column:tool_calls;type:jsonb"`
	ToolCallID string    `gorm:"column:tool_call_id"`
	Usage      []byte    `gorm:"column:usage;type:jsonb"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

// AIOperation AI 操作审计
type AIOperation struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	TableID   string    `gorm:"column:table_id"`
	SessionID string    `gorm:"column:session_id"`
	Revision  *int      `gorm:"column:revision"`
	Ops       []byte    `gorm:"column:ops;type:jsonb"`
	Summary   string    `gorm:"column:summary"`
	Status    string    `gorm:"column:status"`
	Error     string    `gorm:"column:error"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (AIOperation) TableName() string { return "ai_operations" }

// ==================== 事件 ====================

// 事件类型，与前端 SSE 事件名一一对应
const (
	EventStart      = "start"
	EventText       = "text"
	EventToolCall   = "tool_call"
	EventToolResult = "tool_result"
	EventApplied    = "applied"
	EventPending    = "pending"
	EventDone       = "done"
	EventError      = "error"
)

// Event 推送给前端的一个事件
type Event struct {
	Type      string       `json:"type"`
	Text      string       `json:"text,omitempty"`
	ToolName  string       `json:"tool_name,omitempty"`
	ToolArgs  string       `json:"tool_args,omitempty"`
	Summary   string       `json:"summary,omitempty"`
	Changes   []ChangeView `json:"changes,omitempty"`
	Revision  int          `json:"revision,omitempty"`
	Message   string       `json:"message,omitempty"`
	SessionID string       `json:"session_id,omitempty"`
	MessageID string       `json:"message_id,omitempty"`
	Reply     string       `json:"reply,omitempty"`
	Usage     *UsageView   `json:"usage,omitempty"`
	// Actions 基于本轮回复生成的建议动作，随 done 事件下发。
	// 刻意不落库：它们只服务于当前这一轮交互，刷新或切换会话后即消失。
	Actions []followups.Action `json:"actions,omitempty"`
}

// ChangeView 变更摘要，前端据此高亮
type ChangeView struct {
	Kind   string `json:"kind"`
	ItemID string `json:"item_id"`
	Name   string `json:"name"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type UsageView struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens"`
	CachedTokens     int `json:"cached_tokens"`
}

func toUsageView(u provider.Usage) *UsageView {
	return &UsageView{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		ReasoningTokens:  u.ReasoningTokens,
		CachedTokens:     u.CachedTokens,
	}
}

// Emit 事件回调。返回 error 会中止对话（用于客户端断开连接时及时收手）
type Emit func(Event) error

// ==================== 请求 / 响应 ====================

type SendRequest struct {
	// SessionID 为空时自动新建会话。
	// 上限 36 是 UUID v7 的字符串长度（见 internal/infrastructure/idgen）
	SessionID string `json:"session_id" binding:"max=36"`
	Message   string `json:"message" binding:"required,max=4000"`
}

// AnalyzeRequest 触发一轮「完善参数表」分析。
//
// 没有 message 字段：这一轮不是用户发言，而是系统发起的产品分析，
// 因此只带会话号，分析本身由服务端取当前表内容完成。
type AnalyzeRequest struct {
	// SessionID 为空时自动新建会话
	SessionID string `json:"session_id" binding:"max=36"`
}

type SessionView struct {
	ID        string    `json:"id"`
	TableID   string    `json:"table_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// MessageCount 会话内的消息条数，供前端切换器展示
	MessageCount int `json:"message_count"`
}

type MessageView struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
