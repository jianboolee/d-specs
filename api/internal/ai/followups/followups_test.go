package followups

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jianboolee/d-specs/internal/ai/provider"
)

// fakeProvider 返回预设内容，用于验证解析与失败兜底
type fakeProvider struct {
	content string
	err     error
	calls   int
}

func (f *fakeProvider) Name() string  { return "fake" }
func (f *fakeProvider) Model() string { return "fake-model" }
func (f *fakeProvider) ListModels(context.Context) ([]string, error) {
	return []string{"fake-model"}, nil
}

func (f *fakeProvider) Chat(context.Context, provider.ChatRequest) (*provider.ChatResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &provider.ChatResponse{Message: provider.Message{Content: f.content}}, nil
}

func (f *fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(provider.StreamDelta)) (*provider.ChatResponse, error) {
	return f.Chat(context.Background(), provider.ChatRequest{})
}

func TestParseToleratesFencesAndProse(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "裸 JSON", raw: `{"actions":[{"label":"带上年款","message":"带上 2026 款"}]}`, want: 1},
		{name: "代码块围栏", raw: "```json\n{\"actions\":[{\"label\":\"使用简称\",\"message\":\"用简称\"}]}\n```", want: 1},
		{name: "前后夹带说明", raw: "好的，这是建议：\n{\"actions\":[{\"label\":\"补上型号\",\"message\":\"补上型号\"}]}\n希望有帮助", want: 1},
		{name: "空数组", raw: `{"actions":[]}`, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := parse(tt.raw)
			if err != nil {
				t.Fatalf("parse(%q) 返回错误: %v", tt.raw, err)
			}
			if len(out.Actions) != tt.want {
				t.Errorf("解析出 %d 个动作，期望 %d", len(out.Actions), tt.want)
			}
		})
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := parse("完全不是 JSON"); err == nil {
		t.Error("不可解析的内容应返回错误")
	}
}

func TestSanitize(t *testing.T) {
	longLabel := strings.Repeat("字", maxLabelLen+5)
	longMessage := strings.Repeat("很", maxMessageLen+50)

	tests := []struct {
		name string
		raw  []Action
		want []Action
	}{
		{
			name: "丢弃缺字段的条目",
			raw: []Action{
				{Label: "  ", Message: "有消息没标签"},
				{Label: "有标签没消息", Message: " "},
				{Label: "带上年款", Message: "带上 2026 款"},
			},
			want: []Action{{Label: "带上年款", Message: "带上 2026 款"}},
		},
		{
			name: "按 label 去重",
			raw: []Action{
				{Label: "使用简称", Message: "第一次"},
				{Label: "使用简称", Message: "第二次"},
			},
			want: []Action{{Label: "使用简称", Message: "第一次"}},
		},
		{
			name: "超过 3 条被截断",
			raw: []Action{
				{Label: "一", Message: "1"},
				{Label: "二", Message: "2"},
				{Label: "三", Message: "3"},
				{Label: "四", Message: "4"},
			},
			want: []Action{
				{Label: "一", Message: "1"},
				{Label: "二", Message: "2"},
				{Label: "三", Message: "3"},
			},
		},
		{
			name: "超长的 label 与 message 被截断",
			raw:  []Action{{Label: longLabel, Message: longMessage}},
			want: []Action{{
				Label:   string([]rune(longLabel)[:maxLabelLen]),
				Message: string([]rune(longMessage)[:maxMessageLen]),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitize(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sanitize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSuggestReturnsNilOnFailure(t *testing.T) {
	// 未配置 LLM
	if got := New(nil).Suggest(context.Background(), "表", "问题", "回复"); got != nil {
		t.Errorf("llm 为 nil 时应返回 nil，实际 %+v", got)
	}

	// 调用出错
	bad := &fakeProvider{err: context.DeadlineExceeded}
	if got := New(bad).Suggest(context.Background(), "表", "问题", "回复"); got != nil {
		t.Errorf("调用失败时应返回 nil，实际 %+v", got)
	}

	// 空回复不触发调用
	skip := &fakeProvider{content: `{"actions":[{"label":"a","message":"b"}]}`}
	if got := New(skip).Suggest(context.Background(), "表", "问题", "   "); got != nil {
		t.Errorf("空回复应返回 nil，实际 %+v", got)
	}
	if skip.calls != 0 {
		t.Errorf("空回复不应发起调用，实际 %d 次", skip.calls)
	}
}

func TestSuggestHappyPath(t *testing.T) {
	fp := &fakeProvider{content: `{"actions":[{"label":"带上年款","message":"把表标题带上 2026 款"},{"label":"使用简称","message":"用简称『旅行家V90』作为标题"}]}`}
	got := New(fp).Suggest(context.Background(), "大通房车", "改成十周年纪念版", "已改为「大通房车旅行家V90十周年纪念版」。")
	if len(got) != 2 {
		t.Fatalf("期望 2 个动作，实际 %d: %+v", len(got), got)
	}
	if got[0].Label != "带上年款" || got[1].Message == "" {
		t.Errorf("动作内容不正确: %+v", got)
	}
}
