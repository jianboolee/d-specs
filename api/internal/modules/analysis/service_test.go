package analysis

import (
	"strings"
	"testing"

	"github.com/jianboolee/d-specs/internal/ai/ops"
	"github.com/jianboolee/d-specs/internal/modules/spec"
)

func promptDetail(names ...string) *spec.DetailResult {
	items := make([]ops.Item, 0, len(names))
	for _, n := range names {
		items = append(items, ops.Item{Name: n})
	}
	return &spec.DetailResult{
		Table: spec.TableDetail{Title: "测试表"},
		Items: items,
	}
}

func TestBuildUserPromptBaselineHint(t *testing.T) {
	tests := []struct {
		name          string
		detail        *spec.DetailResult
		wantContains  string
		wantAbsentSub string
	}{
		{
			name:         "缺品牌与型号",
			detail:       promptDetail("续航", "电池容量"),
			wantContains: "缺少的基础参数：品牌、型号",
		},
		{
			name:         "只缺型号",
			detail:       promptDetail("品牌", "续航"),
			wantContains: "缺少的基础参数：型号",
		},
		{
			name:          "齐全时不给提示行",
			detail:        promptDetail("品牌", "型号"),
			wantAbsentSub: "缺少的基础参数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildUserPrompt(tt.detail)
			if tt.wantContains != "" && !strings.Contains(got, tt.wantContains) {
				t.Errorf("buildUserPrompt 缺少 %q\n---\n%s", tt.wantContains, got)
			}
			if tt.wantAbsentSub != "" && strings.Contains(got, tt.wantAbsentSub) {
				t.Errorf("buildUserPrompt 不应包含 %q\n---\n%s", tt.wantAbsentSub, got)
			}
		})
	}
}
