package analysis

import (
	"strings"
	"testing"
)

func TestResultMarkdown(t *testing.T) {
	tests := []struct {
		name       string
		result     Result
		wantParts  []string
		absentPart string
	}{
		{
			name: "带类型、标签与建议",
			result: Result{
				ProductType: "房车",
				Tags:        []string{"房车", "越野"},
				Suggestions: []Suggestion{
					{Name: "品牌", Reason: "选型与报价必备"},
					{Name: "指导价"},
				},
			},
			wantParts: []string{
				"**产品类型**：房车",
				"**自动标签**：房车、越野",
				"- **品牌**：选型与报价必备",
				"- **指导价**",
				"需要补哪几项，直接告诉我",
			},
		},
		{
			name:       "没有标签时不出标签行",
			result:     Result{ProductType: "电子消费品", Suggestions: []Suggestion{{Name: "型号"}}},
			wantParts:  []string{"**产品类型**：电子消费品", "- **型号**"},
			absentPart: "**自动标签**",
		},
		{
			name:       "没有建议时给出完整结论",
			result:     Result{ProductType: "房车"},
			wantParts:  []string{"看起来已经比较完整", "**产品类型**：房车"},
			absentPart: "- **",
		},
		{
			name:      "类型缺失时用兜底文案",
			result:    Result{Suggestions: []Suggestion{{Name: "品牌"}}},
			wantParts: []string{"**产品类型**：暂无法判断"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Markdown()
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("Markdown() 缺少 %q\n---\n%s", part, got)
				}
			}
			if tt.absentPart != "" && strings.Contains(got, tt.absentPart) {
				t.Errorf("Markdown() 不应包含 %q\n---\n%s", tt.absentPart, got)
			}
		})
	}
}

// 建议项里的空名字应被跳过，而不是渲染成空条目
func TestResultMarkdownSkipsEmptySuggestionName(t *testing.T) {
	res := Result{
		ProductType: "配件",
		Suggestions: []Suggestion{{Name: "  "}, {Name: "材质", Reason: "影响选型"}},
	}

	got := res.Markdown()
	if strings.Contains(got, "- **") && strings.Contains(got, "- **\n") {
		t.Errorf("空名字建议未被跳过：\n%s", got)
	}
	if !strings.Contains(got, "- **材质**：影响选型") {
		t.Errorf("有效建议丢失：\n%s", got)
	}
}
