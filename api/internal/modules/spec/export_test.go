package spec

import (
	"testing"

	"github.com/jianboolee/d-specs/internal/ai/ops"
)

// 分组顺序按首次出现，未命名分组也自成一组，与画布展示一致。
func TestGroupForExport(t *testing.T) {
	detail := &DetailResult{
		Items: []ops.Item{
			{Name: "品牌", Value: "小野驴"},
			{Name: "电池容量", Value: "5000mAh", GroupName: "电力"},
			{Name: "充电功率", Value: "45W", GroupName: "电力"},
			{Name: "车长", Value: "6m", GroupName: "尺寸"},
			{Name: "型号", Value: "A1"},
		},
	}

	groups := groupForExport(detail)

	if len(groups) != 3 {
		t.Fatalf("应有 3 组（未分组/电力/尺寸），实际 %d: %+v", len(groups), groups)
	}
	// 顺序＝首次出现：第一条是未命名，随后是电力，再是尺寸
	if groups[0].Name != "" {
		t.Errorf("第一组应为未命名分组，实际 %q", groups[0].Name)
	}
	if groups[1].Name != "电力" || groups[2].Name != "尺寸" {
		t.Errorf("分组顺序不对: %q, %q", groups[1].Name, groups[2].Name)
	}

	if len(groups[0].Rows) != 2 {
		t.Errorf("未分组应有 2 行，实际 %d", len(groups[0].Rows))
	}
	if len(groups[1].Rows) != 2 {
		t.Errorf("电力组应有 2 行，实际 %d", len(groups[1].Rows))
	}
	if groups[1].Rows[0].Name != "电池容量" || groups[1].Rows[0].Value != "5000mAh" {
		t.Errorf("电力组内容不对: %+v", groups[1].Rows)
	}
}

func TestGroupForExportEmpty(t *testing.T) {
	if groups := groupForExport(&DetailResult{}); len(groups) != 0 {
		t.Fatalf("空表不应产生分组，实际 %+v", groups)
	}
}
