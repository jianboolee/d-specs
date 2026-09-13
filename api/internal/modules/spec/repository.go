package spec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jianboolee/d-specs/internal/ai/ops"
	"github.com/jianboolee/d-specs/internal/infrastructure/idgen"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 数据访问层。
//
// 隔离约定：**每个方法的第一个业务参数都是 userID，且必须出现在 WHERE 里**。
// 这是有意为之 —— 租户隔离不能依赖 handler 或 service 自觉，把签名定死之后，
// 漏加过滤条件会直接编译不过，而不是变成一次越权查询。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// RevisionMeta 版本列表项（不含完整快照，避免把大 JSON 拖出来）
type RevisionMeta struct {
	Revision  int       `gorm:"column:revision" json:"revision"`
	Source    string    `gorm:"column:source" json:"source"`
	Summary   string    `gorm:"column:summary" json:"summary"`
	ItemCount int       `gorm:"column:item_count" json:"item_count"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// ==================== 参数表 ====================

func (r *Repository) ListTables(ctx context.Context, userID, keyword string, offset, limit int) ([]TableSummary, int64, error) {
	query := r.db.WithContext(ctx).Model(&SpecTable{}).
		Where("user_id = ? AND deleted_at IS NULL", userID)

	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + strings.ToLower(kw) + "%"
		query = query.Where("lower(title) LIKE ? OR lower(description) LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 参数项数量用相关子查询一次带出，避免 N+1
	var rows []TableSummary
	err := query.
		Select("id, title, description, industry, tags, visibility, pinned, current_revision, created_at, updated_at, " +
			"(SELECT COUNT(*) FROM spec_items WHERE spec_items.table_id = spec_tables.id) AS item_count").
		// 置顶的排最前，其内与其余表一样按最近更新
		Order("pinned DESC, updated_at DESC").Offset(offset).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// CreateTable 建表，并在同一事务里写入 revision 0 的初始快照。
//
// 之所以连空表也要写 revision 0：让「恢复到任意版本」有一个统一的起点，
// 不必为 revision 0 特判「那时还没有快照」。
func (r *Repository) CreateTable(
	ctx context.Context,
	userID, title, description, industry string,
	initialItems []ops.Item,
) (*SpecTable, error) {
	table := &SpecTable{
		ID:          idgen.New(),
		UserID:      userID,
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(description),
		Industry:    strings.TrimSpace(industry),
		Visibility:  VisibilityPrivate,
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(table).Error; err != nil {
			return err
		}
		if err := replaceItems(tx, table.ID, initialItems); err != nil {
			return err
		}
		return insertRevision(tx, table.ID, 0, initialItems, SourceUser, "创建参数表", nil)
	})
	if err != nil {
		return nil, err
	}
	return table, nil
}

func (r *Repository) GetTable(ctx context.Context, userID, tableID string) (*SpecTable, error) {
	var table SpecTable
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", tableID, userID).
		Take(&table).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTableNotFound
	}
	if err != nil {
		return nil, err
	}
	return &table, nil
}

// GetTableWithItems 取表与全部参数项，按 sort_order 排序
func (r *Repository) GetTableWithItems(ctx context.Context, userID, tableID string) (*SpecTable, []ops.Item, error) {
	table, err := r.GetTable(ctx, userID, tableID)
	if err != nil {
		return nil, nil, err
	}

	var rows []SpecItem
	if err := r.db.WithContext(ctx).
		Where("table_id = ?", table.ID).
		Order("sort_order ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	return table, modelsToItems(rows), nil
}

// TablePatch 可修改的表元信息，nil 表示不改
type TablePatch struct {
	Title       *string
	Description *string
	Industry    *string
	Tags        *[]string
	Pinned      *bool
}

// normalizeTags 去掉空白项、去重并保留顺序，最多 8 个。
//
// 落库前统一处理，避免 AI 或接口传进来的脏数据（空串、重复）进库。
func normalizeTags(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) == 8 {
			break
		}
	}
	return out
}

// UpdateTableMeta 修改表标题/描述/行业/标签。
//
// 这类元信息变更不产生 revision —— 版本用于参数内容的撤销，
// 把改标题也塞进去会让撤销语义变混乱。
func (r *Repository) UpdateTableMeta(ctx context.Context, userID, tableID string, patch TablePatch) (*SpecTable, error) {
	updates := map[string]any{}
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		updates["title"] = title
	}
	if patch.Description != nil {
		updates["description"] = strings.TrimSpace(*patch.Description)
	}
	if patch.Industry != nil {
		updates["industry"] = strings.TrimSpace(*patch.Industry)
	}
	if patch.Tags != nil {
		// 走 map 更新不会触发 BeforeSave，序列化 serializer:json 也未必生效，
		// 这里显式序列化并加 ::jsonb 转型，避免「text 赋给 jsonb」的类型错误
		raw, err := json.Marshal(normalizeTags(*patch.Tags))
		if err != nil {
			return nil, err
		}
		updates["tags"] = gorm.Expr("?::jsonb", string(raw))
	}
	if patch.Pinned != nil {
		updates["pinned"] = *patch.Pinned
	}
	if len(updates) == 0 {
		return r.GetTable(ctx, userID, tableID)
	}

	result := r.db.WithContext(ctx).Model(&SpecTable{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", tableID, userID).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		// 可能存在但值未变，这里再确认一次存在性
		return r.GetTable(ctx, userID, tableID)
	}
	return r.GetTable(ctx, userID, tableID)
}

// SetVisibility 更新表的可见性（private / link）。
//
// 由分享模块调用；写 spec_tables 的动作留在 spec 模块内，避免别的模块直接改这张表。
func (r *Repository) SetVisibility(ctx context.Context, userID, tableID, visibility string) error {
	result := r.db.WithContext(ctx).Model(&SpecTable{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", tableID, userID).
		Update("visibility", visibility)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTableNotFound
	}
	return nil
}

func (r *Repository) SoftDeleteTable(ctx context.Context, userID, tableID string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&SpecTable{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", tableID, userID).
		Update("deleted_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTableNotFound
	}
	return nil
}

// DuplicateTable 复制一张表（含参数项），换产品改几个值的最高频操作。
//
// 新表从 revision 0 开始，不继承源表的历史 —— 副本是一张新表，
// 把别人的历史带过来只会让撤销变得难以理解。
func (r *Repository) DuplicateTable(ctx context.Context, userID, srcTableID, newTitle string) (*SpecTable, error) {
	_, items, err := r.GetTableWithItems(ctx, userID, srcTableID)
	if err != nil {
		return nil, err
	}

	// 重新生成 id，避免两表共享参数项主键
	copied := make([]ops.Item, 0, len(items))
	for _, it := range items {
		it.ID = idgen.New()
		copied = append(copied, it)
	}

	return r.CreateTable(ctx, userID, newTitle, "", "", copied)
}

// ==================== 变更 ====================

// ApplyParams 一次变更落库所需的全部信息
type ApplyParams struct {
	UserID       string
	TableID      string
	BaseRevision int
	Next         ops.Snapshot
	Source       string
	Summary      string
	Ops          []ops.Op
}

// ApplySnapshot 把校验通过的新快照落库。
//
// 三件事必须原子完成：占用版本号、同步参数项、写版本快照。
// 版本号用乐观锁抢占，抢不到说明期间有别人改过，直接让客户端刷新重来。
func (r *Repository) ApplySnapshot(ctx context.Context, p ApplyParams) (*SpecTable, error) {
	var table SpecTable

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 乐观锁：抢占版本号
		result := tx.Model(&SpecTable{}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL AND current_revision = ?",
				p.TableID, p.UserID, p.BaseRevision).
			Update("current_revision", p.BaseRevision+1)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 要么表不存在/不归当前用户，要么版本已过期，需要区分
			var count int64
			if err := tx.Model(&SpecTable{}).
				Where("id = ? AND user_id = ? AND deleted_at IS NULL", p.TableID, p.UserID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrTableNotFound
			}
			return ErrRevisionConflict
		}

		// 2. 同步参数项
		if err := replaceItems(tx, p.TableID, p.Next.Items); err != nil {
			return err
		}

		// 3. 写版本快照
		if err := insertRevision(tx, p.TableID, p.BaseRevision+1, p.Next.Items, p.Source, p.Summary, p.Ops); err != nil {
			return err
		}

		return tx.Where("id = ?", p.TableID).Take(&table).Error
	})
	if err != nil {
		return nil, err
	}
	return &table, nil
}

// ==================== 版本 ====================

func (r *Repository) ListRevisions(ctx context.Context, userID, tableID string, limit int) ([]RevisionMeta, error) {
	if _, err := r.GetTable(ctx, userID, tableID); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var out []RevisionMeta
	// item_count 直接由 JSONB 算出来，避免把整份快照拖出来再在 Go 里数。
	// 用 CASE 兜住历史坏数据（items 为 null 时 jsonb_array_length 会报错）
	err := r.db.WithContext(ctx).Model(&SpecRevision{}).
		Select(`revision, source, summary, created_at,
			CASE WHEN jsonb_typeof(snapshot->'items') = 'array'
			     THEN jsonb_array_length(snapshot->'items') ELSE 0 END AS item_count`).
		Where("table_id = ?", tableID).
		Order("revision DESC").
		Limit(limit).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetRevisionItems 取某个版本的参数项快照
func (r *Repository) GetRevisionItems(ctx context.Context, userID, tableID string, revision int) ([]ops.Item, error) {
	if _, err := r.GetTable(ctx, userID, tableID); err != nil {
		return nil, err
	}

	var row SpecRevision
	err := r.db.WithContext(ctx).
		Where("table_id = ? AND revision = ?", tableID, revision).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRevisionNotFound
	}
	if err != nil {
		return nil, err
	}

	var payload SnapshotPayload
	if err := json.Unmarshal(row.Snapshot, &payload); err != nil {
		return nil, fmt.Errorf("解析版本快照失败: %w", err)
	}
	return payload.Items, nil
}

// ==================== 内部 ====================

// replaceItems 让库里的参数项与目标快照完全一致。
//
// 用「upsert + 删除多余」而不是「全删重插」：前者能保住已有项的 created_at，
// 也让 id 在多次变更之间保持稳定。
func replaceItems(tx *gorm.DB, tableID string, items []ops.Item) error {
	rows := make([]SpecItem, 0, len(items))
	for _, it := range items {
		rows = append(rows, itemToModel(tableID, it))
	}

	if len(rows) > 0 {
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			// 不含 updated_at / created_at：前者由触发器维护，后者只在插入时取默认值
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "value", "value_type", "value_num",
				"enum_options", "group_name", "remark", "sort_order",
			}),
		}).Create(&rows).Error
		if err != nil {
			return err
		}
	}

	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}

	del := tx.Where("table_id = ?", tableID)
	if len(ids) > 0 {
		del = del.Where("id NOT IN ?", ids)
	}
	return del.Delete(&SpecItem{}).Error
}

func insertRevision(
	tx *gorm.DB,
	tableID string,
	revision int,
	items []ops.Item,
	source, summary string,
	applied []ops.Op,
) error {
	// nil 必须归一成空切片：json.Marshal(nil 切片) 会输出 null 而不是 []，
	// 之后 jsonb_array_length(snapshot->'items') 会直接报错
	if items == nil {
		items = []ops.Item{}
	}

	snapshot, err := json.Marshal(SnapshotPayload{Items: items})
	if err != nil {
		return fmt.Errorf("序列化版本快照失败: %w", err)
	}

	var opsJSON []byte
	if len(applied) > 0 {
		opsJSON, err = json.Marshal(applied)
		if err != nil {
			return fmt.Errorf("序列化 ops 失败: %w", err)
		}
	}

	if source == "" {
		source = SourceUser
	}

	return tx.Create(&SpecRevision{
		TableID:  tableID,
		Revision: revision,
		Snapshot: snapshot,
		Source:   source,
		Summary:  summary,
		Ops:      opsJSON,
	}).Error
}
