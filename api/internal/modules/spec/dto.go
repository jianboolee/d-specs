package spec

import (
	"time"

	"github.com/donktech/d-specs/internal/ai/ops"
)

// ==================== 请求 ====================

type CreateRequest struct {
	Title       string `json:"title" binding:"required,max=120"`
	Description string `json:"description" binding:"max=2000"`
	Industry    string `json:"industry" binding:"max=64"`
	// TemplateID 预留：套用模板建表（P3）
	TemplateID string `json:"template_id" binding:"max=32"`
}

type UpdateRequest struct {
	Title       *string `json:"title" binding:"omitempty,max=120"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
	Industry    *string `json:"industry" binding:"omitempty,max=64"`
}

type DuplicateRequest struct {
	Title string `json:"title" binding:"required,max=120"`
}

// ApplyRequest 统一变更入口：手工编辑与 AI 变更走同一个接口。
//
// BaseRevision 是乐观锁依据，必须由客户端带上。
type ApplyRequest struct {
	BaseRevision *int     `json:"base_revision"`
	Source       string   `json:"source" binding:"omitempty,oneof=user ai template import"`
	Summary      string   `json:"summary" binding:"max=500"`
	Ops          []ops.Op `json:"ops" binding:"required,min=1"`
}

type RestoreRequest struct {
	Revision *int `json:"revision" binding:"required"`
}

// ==================== 响应 ====================

// TableSummary 列表项，带参数项数量
type TableSummary struct {
	ID              string    `gorm:"column:id" json:"id"`
	Title           string    `gorm:"column:title" json:"title"`
	Description     string    `gorm:"column:description" json:"description"`
	Industry        string    `gorm:"column:industry" json:"industry"`
	Visibility      string    `gorm:"column:visibility" json:"visibility"`
	CurrentRevision int       `gorm:"column:current_revision" json:"current_revision"`
	ItemCount       int       `gorm:"column:item_count" json:"item_count"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

type ListResult struct {
	Items []TableSummary `json:"items"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

// TableDetail 表元信息（不含参数项）
type TableDetail struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Industry        string    `json:"industry"`
	Visibility      string    `json:"visibility"`
	CurrentRevision int       `json:"current_revision"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toTableDetail(t *SpecTable) TableDetail {
	return TableDetail{
		ID:              t.ID,
		Title:           t.Title,
		Description:     t.Description,
		Industry:        t.Industry,
		Visibility:      t.Visibility,
		CurrentRevision: t.CurrentRevision,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

// DetailResult 表详情：元信息 + 全部参数项 + 分组清单
type DetailResult struct {
	Table  TableDetail `json:"table"`
	Items  []ops.Item  `json:"items"`
	Groups []string    `json:"groups"`
}

// ApplyResult 一次变更的结果
type ApplyResult struct {
	Table    TableDetail  `json:"table"`
	Revision int          `json:"revision"`
	Changes  []ops.Change `json:"changes"`
	Summary  string       `json:"summary"`
}
