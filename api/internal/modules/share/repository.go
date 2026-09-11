package share

import (
	"context"
	"errors"
	"time"

	"github.com/donktech/d-specs/internal/infrastructure/idgen"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ==================== 需登录 ====================

// GetByTable 取某张表当前的分享链接（含已关闭的）
func (r *Repository) GetByTable(ctx context.Context, userID, tableID string) (*ShareLink, error) {
	// 先确认表归属，再查链接 —— 链接表本身没有 user_id
	var count int64
	if err := r.db.WithContext(ctx).
		Table("spec_tables").
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", tableID, userID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrNotShared
	}

	var link ShareLink
	err := r.db.WithContext(ctx).
		Where("table_id = ?", tableID).
		Order("created_at DESC").
		Take(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotShared
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// UpsertEnabled 开启分享：已有链接就复用并重新启用，没有就新建。
//
// 复用而不是每次生成新 token，是为了让已经发出去的链接在「关闭再打开」后仍然有效。
func (r *Repository) UpsertEnabled(ctx context.Context, userID, tableID string) (*ShareLink, error) {
	link, err := r.GetByTable(ctx, userID, tableID)
	if err == nil {
		if !link.Enabled {
			if err := r.db.WithContext(ctx).
				Model(&ShareLink{}).
				Where("id = ?", link.ID).
				Update("enabled", true).Error; err != nil {
				return nil, err
			}
			link.Enabled = true
		}
		return link, nil
	}
	if !errors.Is(err, ErrNotShared) {
		return nil, err
	}

	created := &ShareLink{
		ID:      idgen.New(),
		TableID: tableID,
		// 32 位随机串，熵足够高以防被枚举爬取
		Token:   idgen.Token(),
		Enabled: true,
	}
	if err := r.db.WithContext(ctx).Create(created).Error; err != nil {
		return nil, err
	}
	return created, nil
}

func (r *Repository) SetEnabled(ctx context.Context, userID, tableID string, enabled bool) error {
	link, err := r.GetByTable(ctx, userID, tableID)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&ShareLink{}).
		Where("id = ?", link.ID).
		Update("enabled", enabled).Error
}

// ==================== 公开访问 ====================

// resolveByToken 按 token 找到启用中的分享，并返回它指向的表。
//
// ⚠️ 这是本项目里唯一不带 user_id 的读取路径 —— 公开分享页天然没有登录用户。
// 安全边界完全由 token 承担：token 由 idgen.Token() 生成（32 位随机），
// 且必须是 enabled=true。调用方只能通过这个函数拿到数据，不允许绕过它直查。
func (r *Repository) resolveByToken(ctx context.Context, token string) (*ShareLink, error) {
	if token == "" {
		return nil, ErrShareNotFound
	}

	var link ShareLink
	err := r.db.WithContext(ctx).
		Where("token = ? AND enabled = true", token).
		Take(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrShareNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

type publicRow struct {
	Title       string    `gorm:"column:title"`
	Description string    `gorm:"column:description"`
	Industry    string    `gorm:"column:industry"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (r *Repository) loadPublicTable(ctx context.Context, tableID string) (*publicRow, error) {
	var row publicRow
	err := r.db.WithContext(ctx).
		Table("spec_tables").
		Select("title, description, industry, updated_at").
		Where("id = ? AND deleted_at IS NULL", tableID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrShareNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// publicItemRow 公开页用的参数项行。比 PublicItem 多一个 group_name，
// 用于在 service 里分组。
type publicItemRow struct {
	Name      string `gorm:"column:name"`
	Value     string `gorm:"column:value"`
	ValueType string `gorm:"column:value_type"`
	Remark    string `gorm:"column:remark"`
	GroupName string `gorm:"column:group_name"`
}

func (r *Repository) loadPublicItems(ctx context.Context, tableID string) ([]publicItemRow, error) {
	var rows []publicItemRow
	err := r.db.WithContext(ctx).
		Table("spec_items").
		Select("name, value, value_type, remark, group_name").
		Where("table_id = ?", tableID).
		Order("sort_order ASC, created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// incrementViewCount 访问计数。失败不影响页面展示，因此调用方可以忽略错误。
func (r *Repository) incrementViewCount(ctx context.Context, linkID string) error {
	return r.db.WithContext(ctx).
		Model(&ShareLink{}).
		Where("id = ?", linkID).
		Update("view_count", gorm.Expr("view_count + 1")).Error
}
