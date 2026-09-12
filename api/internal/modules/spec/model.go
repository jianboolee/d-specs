package spec

import (
	"time"

	"github.com/jianboolee/d-specs/internal/ai/ops"
	"github.com/jianboolee/d-specs/pkg/quantity"
)

// SpecTable 参数表。一张表 = 一个产品的参数配置表。
type SpecTable struct {
	ID          string `gorm:"column:id;primaryKey"`
	UserID      string `gorm:"column:user_id"`
	Title       string `gorm:"column:title"`
	Description string `gorm:"column:description"`
	Industry    string `gorm:"column:industry"`
	Visibility  string `gorm:"column:visibility"`
	// CurrentRevision 乐观锁 + 版本号；写操作必须带上期望值比对
	CurrentRevision int        `gorm:"column:current_revision"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
}

func (SpecTable) TableName() string { return "spec_tables" }

// SpecItem 参数项。表内每行 = 一条参数。
type SpecItem struct {
	ID        string `gorm:"column:id;primaryKey"`
	TableID   string `gorm:"column:table_id"`
	Name      string `gorm:"column:name"`
	Value     string `gorm:"column:value"`
	ValueType string `gorm:"column:value_type"`
	// ValueNum value_type=number 时冗余存数值，支撑排序与区间筛选
	ValueNum    *float64  `gorm:"column:value_num"`
	EnumOptions []string  `gorm:"column:enum_options;serializer:json"`
	GroupName   string    `gorm:"column:group_name"`
	Remark      string    `gorm:"column:remark"`
	SortOrder   int       `gorm:"column:sort_order"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (SpecItem) TableName() string { return "spec_items" }

// SpecRevision 版本快照。撤销/重做的唯一依据。
type SpecRevision struct {
	ID       int64  `gorm:"column:id;primaryKey"`
	TableID  string `gorm:"column:table_id"`
	Revision int    `gorm:"column:revision"`
	// Snapshot { items: [...] } 的 JSON
	Snapshot []byte `gorm:"column:snapshot;type:jsonb"`
	Source   string `gorm:"column:source"`
	Summary  string `gorm:"column:summary"`
	// Ops 本次变更的 ops 明细，便于审计「AI 到底改了什么」
	Ops       []byte    `gorm:"column:ops;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (SpecRevision) TableName() string { return "spec_revisions" }

// SnapshotPayload 序列化到 spec_revisions.snapshot 的结构
type SnapshotPayload struct {
	Items []ops.Item `json:"items"`
}

// 变更来源，写入 spec_revisions.source
const (
	SourceUser     = "user"
	SourceAI       = "ai"
	SourceTemplate = "template"
	SourceImport   = "import"
	SourceRestore  = "restore"
)

// 可见性
const (
	VisibilityPrivate = "private"
	VisibilityLink    = "link"
)

// ==================== 转换 ====================

func itemToModel(tableID string, it ops.Item) SpecItem {
	m := SpecItem{
		ID:          it.ID,
		TableID:     tableID,
		Name:        it.Name,
		Value:       it.Value,
		ValueType:   it.ValueType,
		EnumOptions: it.EnumOptions,
		GroupName:   it.GroupName,
		Remark:      it.Remark,
		SortOrder:   it.SortOrder,
	}
	if m.ValueType == "" {
		m.ValueType = ops.TypeText
	}
	if m.ValueType == ops.TypeNumber {
		// 用 quantity 而不是 strconv：参数值现在带着单位（5000mAh、8小时），
		// 直接 ParseFloat 会一律失败，value_num 永远为空
		if parsed, err := quantity.Parse(it.Value); err == nil {
			num := parsed.Number
			m.ValueNum = &num
		}
	}
	return m
}

func modelToItem(m SpecItem) ops.Item {
	return ops.Item{
		ID:          m.ID,
		Name:        m.Name,
		Value:       m.Value,
		ValueType:   m.ValueType,
		EnumOptions: m.EnumOptions,
		GroupName:   m.GroupName,
		Remark:      m.Remark,
		SortOrder:   m.SortOrder,
	}
}

func modelsToItems(rows []SpecItem) []ops.Item {
	items := make([]ops.Item, 0, len(rows))
	for _, m := range rows {
		items = append(items, modelToItem(m))
	}
	return items
}
