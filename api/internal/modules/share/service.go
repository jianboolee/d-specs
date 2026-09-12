package share

import (
	"context"
	"errors"
	"strings"

	"github.com/jianboolee/d-specs/internal/modules/spec"
)

type Service struct {
	repo    *Repository
	specSvc *spec.Service
}

func NewService(repo *Repository, specSvc *spec.Service) *Service {
	return &Service{repo: repo, specSvc: specSvc}
}

// ==================== 需登录 ====================

func (s *Service) Status(ctx context.Context, userID, tableID string) (*StatusView, error) {
	// 先确认表归属，越权在这里被挡掉
	if _, err := s.specSvc.Detail(ctx, userID, tableID); err != nil {
		return nil, err
	}

	link, err := s.repo.GetByTable(ctx, userID, tableID)
	if errors.Is(err, ErrNotShared) {
		// 「还没开过分享」是正常状态，不该当成错误：
		// 否则前端每次打开一张新表都要处理一次 404
		return &StatusView{Enabled: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return toStatusView(link), nil
}

// Enable 开启分享。
//
// 复用已有 token 而不是每次新建：否则「关闭再打开」会让已经发出去的链接失效。
func (s *Service) Enable(ctx context.Context, userID, tableID string) (*StatusView, error) {
	if _, err := s.specSvc.Detail(ctx, userID, tableID); err != nil {
		return nil, err
	}

	link, err := s.repo.UpsertEnabled(ctx, userID, tableID)
	if err != nil {
		return nil, err
	}
	if err := s.specSvc.SetVisibility(ctx, userID, tableID, spec.VisibilityLink); err != nil {
		return nil, err
	}
	return toStatusView(link), nil
}

func (s *Service) Disable(ctx context.Context, userID, tableID string) (*StatusView, error) {
	if _, err := s.specSvc.Detail(ctx, userID, tableID); err != nil {
		return nil, err
	}

	if err := s.repo.SetEnabled(ctx, userID, tableID, false); err != nil {
		return nil, err
	}
	if err := s.specSvc.SetVisibility(ctx, userID, tableID, spec.VisibilityPrivate); err != nil {
		return nil, err
	}

	link, err := s.repo.GetByTable(ctx, userID, tableID)
	if err != nil {
		return nil, err
	}
	return toStatusView(link), nil
}

// ==================== 公开访问 ====================

// PublicSpec 按 token 取只读内容。无需登录。
func (s *Service) PublicSpec(ctx context.Context, token string) (*PublicSpec, error) {
	link, err := s.repo.resolveByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	table, err := s.repo.loadPublicTable(ctx, link.TableID)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.loadPublicItems(ctx, link.TableID)
	if err != nil {
		return nil, err
	}

	// 计数失败不该影响访客看到内容
	_ = s.repo.incrementViewCount(ctx, link.ID)

	return &PublicSpec{
		Title:       table.Title,
		Description: table.Description,
		Industry:    table.Industry,
		UpdatedAt:   table.UpdatedAt,
		Groups:      groupRows(rows),
	}, nil
}

func groupRows(rows []publicItemRow) []PublicGroup {
	index := map[string]int{}
	groups := make([]PublicGroup, 0, 4)

	for _, r := range rows {
		name := strings.TrimSpace(r.GroupName)
		idx, ok := index[name]
		if !ok {
			index[name] = len(groups)
			groups = append(groups, PublicGroup{Name: name, Items: []PublicItem{}})
			idx = len(groups) - 1
		}
		groups[idx].Items = append(groups[idx].Items, PublicItem{
			Name:      r.Name,
			Value:     r.Value,
			ValueType: r.ValueType,
			Remark:    r.Remark,
		})
	}
	return groups
}

func toStatusView(link *ShareLink) *StatusView {
	view := &StatusView{
		Enabled:   link.Enabled,
		ViewCount: link.ViewCount,
	}
	if link.Enabled {
		view.Token = link.Token
		// 前端用 location.origin 拼成完整地址
		view.URL = "/s/" + link.Token
	}
	return view
}
