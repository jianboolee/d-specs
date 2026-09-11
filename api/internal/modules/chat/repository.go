package chat

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/donktech/d-specs/internal/infrastructure/idgen"

	"gorm.io/gorm"
)

var (
	// ErrSessionNotFound 会话不存在或不属于当前用户
	ErrSessionNotFound = errors.New("会话不存在")
)

// Repository 数据访问层。
//
// 与 spec 模块同样的约定：需要鉴权的方法都把 userID 作为第一个业务参数。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetOrCreateSession sessionID 为空时新建会话；否则校验归属后返回
func (r *Repository) GetOrCreateSession(ctx context.Context, userID, tableID, sessionID, title string) (*ChatSession, error) {
	sessionID = strings.TrimSpace(sessionID)

	if sessionID != "" {
		var session ChatSession
		err := r.db.WithContext(ctx).
			Where("id = ? AND user_id = ? AND table_id = ?", sessionID, userID, tableID).
			Take(&session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		if err != nil {
			return nil, err
		}
		return &session, nil
	}

	session := &ChatSession{
		ID:      idgen.New(),
		TableID: tableID,
		UserID:  userID,
		Title:   truncateRunes(title, 60),
	}
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

func (r *Repository) GetSession(ctx context.Context, userID, sessionID string) (*ChatSession, error) {
	var session ChatSession
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// SessionSummary 会话列表项，带消息数
type SessionSummary struct {
	ID           string    `gorm:"column:id"`
	TableID      string    `gorm:"column:table_id"`
	Title        string    `gorm:"column:title"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
	MessageCount int       `gorm:"column:message_count"`
}

func (r *Repository) ListSessions(ctx context.Context, userID, tableID string) ([]SessionSummary, error) {
	var rows []SessionSummary
	// 消息数用相关子查询一次带出，避免前端为了显示「N 条消息」再逐条请求
	err := r.db.WithContext(ctx).
		Table("chat_sessions").
		Select("id, table_id, title, created_at, updated_at, "+
			"(SELECT COUNT(*) FROM chat_messages WHERE chat_messages.session_id = chat_sessions.id) AS message_count").
		Where("user_id = ? AND table_id = ?", userID, tableID).
		Order("updated_at DESC").
		Limit(50).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// TouchSession 更新会话的 updated_at，让最近用过的会话排在前面
func (r *Repository) TouchSession(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).
		Model(&ChatSession{}).
		Where("id = ?", sessionID).
		Update("updated_at", gorm.Expr("now()")).Error
}

func (r *Repository) SaveMessage(ctx context.Context, msg *ChatMessage) error {
	if msg.ID == "" {
		msg.ID = idgen.New()
	}
	return r.db.WithContext(ctx).Create(msg).Error
}

// ListMessages 按时间正序返回最近的 limit 条
func (r *Repository) ListMessages(ctx context.Context, userID, sessionID string, limit int) ([]ChatMessage, error) {
	if _, err := r.GetSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// 先按倒序取最近 N 条，再翻回正序，保证拿到的是「最近」而不是「最早」
	var rows []ChatMessage
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

func (r *Repository) SaveAIOperation(ctx context.Context, op *AIOperation) error {
	return r.db.WithContext(ctx).Create(op).Error
}

func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
