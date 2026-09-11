package spec

import (
	"context"
	"fmt"
	"strings"

	"github.com/donktech/d-specs/internal/ai/ops"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ==================== 配置表 ====================

func (s *Service) List(ctx context.Context, userID, keyword string, page, size int) (*ListResult, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	items, total, err := s.repo.ListTables(ctx, userID, keyword, (page-1)*size, size)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []TableSummary{}
	}

	return &ListResult{Items: items, Total: total, Page: page, Size: size}, nil
}

func (s *Service) Create(ctx context.Context, userID string, req CreateRequest) (*DetailResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}

	table, err := s.repo.CreateTable(ctx, userID, title, req.Description, req.Industry, nil)
	if err != nil {
		return nil, err
	}

	return &DetailResult{
		Table:  toTableDetail(table),
		Items:  []ops.Item{},
		Groups: []string{},
	}, nil
}

func (s *Service) Detail(ctx context.Context, userID, tableID string) (*DetailResult, error) {
	table, items, err := s.repo.GetTableWithItems(ctx, userID, tableID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ops.Item{}
	}

	groups := ops.Snapshot{Items: items}.Groups()
	if groups == nil {
		groups = []string{}
	}

	return &DetailResult{Table: toTableDetail(table), Items: items, Groups: groups}, nil
}

func (s *Service) UpdateMeta(ctx context.Context, userID, tableID string, req UpdateRequest) (*TableDetail, error) {
	table, err := s.repo.UpdateTableMeta(ctx, userID, tableID, TablePatch{
		Title:       req.Title,
		Description: req.Description,
		Industry:    req.Industry,
	})
	if err != nil {
		return nil, err
	}
	detail := toTableDetail(table)
	return &detail, nil
}

func (s *Service) Delete(ctx context.Context, userID, tableID string) error {
	return s.repo.SoftDeleteTable(ctx, userID, tableID)
}

// SetVisibility 更新可见性，供分享模块使用
func (s *Service) SetVisibility(ctx context.Context, userID, tableID, visibility string) error {
	if visibility != VisibilityLink {
		visibility = VisibilityPrivate
	}
	return s.repo.SetVisibility(ctx, userID, tableID, visibility)
}

// Duplicate 复制一张表，换产品改几个值的高频操作
func (s *Service) Duplicate(ctx context.Context, userID, tableID string, req DuplicateRequest) (*DetailResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}

	table, err := s.repo.DuplicateTable(ctx, userID, tableID, title)
	if err != nil {
		return nil, err
	}
	return s.Detail(ctx, userID, table.ID)
}

// ==================== 变更 ====================

// Apply 统一变更入口。手工编辑与 AI 变更都走这里，因此校验、版本、
// 审计只有一套实现，不会出现「AI 改的能撤销、手改的不能」。
func (s *Service) Apply(ctx context.Context, userID, tableID string, req ApplyRequest) (*ApplyResult, error) {
	if len(req.Ops) == 0 {
		return nil, ErrNothingToApply
	}

	table, items, err := s.repo.GetTableWithItems(ctx, userID, tableID)
	if err != nil {
		return nil, err
	}

	// 客户端带了过期版本号时快速失败，省掉一次没意义的校验与写入
	if req.BaseRevision != nil && *req.BaseRevision != table.CurrentRevision {
		return nil, ErrRevisionConflict
	}

	next, changes, applyErr := ops.Apply(ops.Snapshot{Title: table.Title, Items: items}, req.Ops)
	if applyErr != nil {
		return nil, fmt.Errorf("%w：%s", ErrOpsRejected, applyErr.Error())
	}
	if len(changes) == 0 {
		return nil, ErrNothingToApply
	}

	summary := strings.TrimSpace(req.Summary)
	updated, err := s.repo.ApplySnapshot(ctx, ApplyParams{
		UserID:       userID,
		TableID:      tableID,
		BaseRevision: table.CurrentRevision,
		Next:         next,
		Source:       normalizeSource(req.Source),
		Summary:      summary,
		Ops:          req.Ops,
	})
	if err != nil {
		return nil, err
	}

	return &ApplyResult{
		Table:    toTableDetail(updated),
		Revision: updated.CurrentRevision,
		Changes:  changes,
		Summary:  summary,
	}, nil
}

// Restore 把参数内容恢复到指定版本。
//
// 实现方式是把目标版本的快照写成**新版本**，而不是把版本号回退：
// 历史因此保持线性且不可变，撤销本身也是一次可追溯、可再撤销的变更。
// 撤销/重做的栈由前端维护 —— 它才知道用户实际走过哪些版本。
func (s *Service) Restore(ctx context.Context, userID, tableID string, revision int) (*ApplyResult, error) {
	if revision < 0 {
		return nil, ErrRevisionNotFound
	}

	table, err := s.repo.GetTable(ctx, userID, tableID)
	if err != nil {
		return nil, err
	}

	if revision == table.CurrentRevision {
		return nil, ErrNothingToApply
	}

	target, err := s.repo.GetRevisionItems(ctx, userID, tableID, revision)
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.ApplySnapshot(ctx, ApplyParams{
		UserID:       userID,
		TableID:      tableID,
		BaseRevision: table.CurrentRevision,
		Next:         ops.Snapshot{Title: table.Title, Items: target},
		Source:       SourceRestore,
		Summary:      fmt.Sprintf("恢复到版本 r%d", revision),
	})
	if err != nil {
		return nil, err
	}

	return &ApplyResult{
		Table:    toTableDetail(updated),
		Revision: updated.CurrentRevision,
		Changes:  []ops.Change{},
		Summary:  fmt.Sprintf("已恢复到版本 r%d", revision),
	}, nil
}

func (s *Service) Revisions(ctx context.Context, userID, tableID string, limit int) ([]RevisionMeta, error) {
	rows, err := s.repo.ListRevisions(ctx, userID, tableID, limit)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []RevisionMeta{}
	}
	return rows, nil
}

func normalizeSource(source string) string {
	switch strings.TrimSpace(source) {
	case SourceAI:
		return SourceAI
	case SourceTemplate:
		return SourceTemplate
	case SourceImport:
		return SourceImport
	default:
		return SourceUser
	}
}
