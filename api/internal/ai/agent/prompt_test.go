package agent

import (
	"strings"
	"testing"

	"github.com/jianboolee/d-specs/internal/ai/ops"
)

func TestBuildSystemPromptBaselineStatus(t *testing.T) {
	tests := []struct {
		name      string
		items     []ops.Item
		wantParts []string
	}{
		{
			name:      "空表时两个基础参数都标缺失",
			items:     nil,
			wantParts: []string{"基础参数：品牌（缺失）、型号（缺失）", "这张表目前是空的"},
		},
		{
			name:      "已有品牌、缺型号",
			items:     []ops.Item{{Name: "品牌", Value: "XX"}},
			wantParts: []string{"基础参数：品牌（已有）、型号（缺失）"},
		},
		{
			name:      "齐全",
			items:     []ops.Item{{Name: "型号", Value: "X1"}, {Name: "品牌", Value: "XX"}},
			wantParts: []string{"基础参数：品牌（已有）、型号（已有）"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSystemPrompt(ops.Snapshot{Title: "测试表", Items: tt.items})
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("system prompt 缺少 %q\n---\n%s", part, got)
				}
			}
		})
	}
}

// 行为规则必须随 prompt 一起下发，否则模型不知道缺失时该怎么处理
func TestBuildSystemPromptHasBaselineRules(t *testing.T) {
	got := BuildSystemPrompt(ops.Snapshot{Title: "测试表"})
	for _, part := range []string{"绝不编造", "不要新增空值占位行", "整体整理这张表时处理基础参数"} {
		if !strings.Contains(got, part) {
			t.Errorf("system prompt 缺少基础参数规则 %q", part)
		}
	}
}
