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
// 分享页只需要「分组 + 参数名 / 值 / 单位」；参数项 id 对展示毫无用处，
// 不该暴露给外部访客。表 id 则体现在公开 URL（/s/:id）上，不在这里重复返回。
type PublicSpec struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Industry    string        `json:"industry"`
	Tags        []string      `json:"tags"`
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
	ValueType string `json:"value_type"`
	Remark    string `json:"remark,omitempty"`
}

// SitemapEntry 已公开（分享开启中）的参数表，用于生成 sitemap.xml。
type SitemapEntry struct {
	ID        string    `gorm:"column:id"`
	Title     string    `gorm:"column:title"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// PublicTableSummary 公开列表页的一行。
//
// 与 SitemapEntry 一样只走公开边界（share_links.enabled），字段面向访客展示，
// 不含 user_id / visibility 等内部信息。
type PublicTableSummary struct {
	ID          string    `gorm:"column:id" json:"id"`
	Title       string    `gorm:"column:title" json:"title"`
	Description string    `gorm:"column:description" json:"description"`
	Industry    string    `gorm:"column:industry" json:"industry"`
	Tags        []string  `gorm:"column:tags;serializer:json" json:"tags"`
	ItemCount   int64     `gorm:"column:item_count" json:"item_count"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}
