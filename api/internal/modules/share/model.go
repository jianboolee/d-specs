package share

import (
	"errors"
	"time"
)

var (
	// ErrShareNotFound 分享链接不存在或已关闭
	ErrShareNotFound = errors.New("分享链接不存在或已关闭")
	// ErrNotShared 该参数表尚未开启分享
	ErrNotShared = errors.New("该参数表尚未开启分享")
)

// ShareLink 只读分享链接
type ShareLink struct {
	ID        string    `gorm:"column:id;primaryKey"`
	TableID   string    `gorm:"column:table_id"`
	Token     string    `gorm:"column:token"`
	Enabled   bool      `gorm:"column:enabled"`
	ViewCount int64     `gorm:"column:view_count"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (ShareLink) TableName() string { return "share_links" }

// ==================== 响应 ====================

// StatusView 分享状态（需登录）
type StatusView struct {
	Enabled   bool   `json:"enabled"`
	Token     string `json:"token,omitempty"`
	URL       string `json:"url,omitempty"`
	ViewCount int64  `json:"view_count"`
}

// PublicSpec 公开只读页的数据。
//
// 刻意用一套独立的精简结构，而不是复用 spec 的详情响应：
// 分享页只需要「分组 + 参数名 / 值 / 单位」，表 id 与参数项 id 都不该暴露给
// 拿到链接的外部访客 —— 它们对展示毫无用处。
type PublicSpec struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Industry    string        `json:"industry"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Groups      []PublicGroup `json:"groups"`
}

type PublicGroup struct {
	Name  string       `json:"name"`
	Items []PublicItem `json:"items"`
}

type PublicItem struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Unit      string `json:"unit"`
	ValueType string `json:"value_type"`
	Remark    string `json:"remark,omitempty"`
}
