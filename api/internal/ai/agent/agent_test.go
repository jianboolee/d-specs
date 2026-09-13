package agent

import (
	"context"
	"testing"

	"github.com/jianboolee/d-specs/internal/ai/ops"
	"github.com/jianboolee/d-specs/internal/ai/provider"
)

// fakeProvider 按调用次数返回预设响应，用于复现「正文被思考挤空」这条路径。
type fakeProvider struct {
	responses []*provider.ChatResponse
	calls     int
	// lastReq 记录最后一次请求，用于断言重试请求是否带上了工具定义
	lastReq provider.ChatRequest
}

func (f *fakeProvider) Name() string  { return "fake" }
func (f *fakeProvider) Model() string { return "fake-model" }
func (f *fakeProvider) ListModels(context.Context) ([]string, error) {
	return []string{"fake-model"}, nil
}

func (f *fakeProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	f.lastReq = req
	idx := f.calls
	f.calls++
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	return f.responses[idx], nil
}

// 思考吃光预算时，正文为空且 finish_reason=length；此时应追问一次拿回正文。
func TestRunRetriesWhenContentEmpty(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatResponse{
		{
			Message:      provider.Message{Role: provider.RoleAssistant, Content: ""},
			FinishReason: "length",
		},
		{
			Message:      provider.Message{Role: provider.RoleAssistant, Content: "  已完成后备说明。  "},
			FinishReason: "stop",
			Usage:        provider.Usage{CompletionTokens: 12},
		},
	}}

	ag := New(fp, Config{MaxToolRounds: 3, MaxTokens: 8192})
	res, err := ag.Run(context.Background(), nil, emptySnapshot(), nil)
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}

	if fp.calls != 2 {
		t.Fatalf("期望追问一次（共 2 次调用），实际 %d 次", fp.calls)
	}
	if res.Truncated {
		t.Fatalf("追问已拿到正文，不应标记 Truncated")
	}
	if res.Reply != "已完成后备说明。" {
		t.Fatalf("正文应为追问结果并去除首尾空白，实际 %q", res.Reply)
	}
	// 追问不再需要工具，否则模型可能又去调工具
	if len(fp.lastReq.Tools) != 0 {
		t.Fatalf("追问请求不应带工具定义，实际 %d 个", len(fp.lastReq.Tools))
	}
	// 追问的用量要计入总计
	if res.Usage.CompletionTokens != 12 {
		t.Fatalf("追问用量应累加，completion_tokens 实际 %d", res.Usage.CompletionTokens)
	}
}

// 追问仍然拿不到正文时，给出面向用户的提示，而不是让前端显示空白。
func TestRunFallsBackWhenRetryAlsoEmpty(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatResponse{
		{
			Message:      provider.Message{Role: provider.RoleAssistant, Content: ""},
			FinishReason: "length",
		},
		{
			Message:      provider.Message{Role: provider.RoleAssistant, Content: "   "},
			FinishReason: "length",
		},
	}}

	ag := New(fp, Config{MaxToolRounds: 3, MaxTokens: 8192})
	res, err := ag.Run(context.Background(), nil, emptySnapshot(), nil)
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}

	if !res.Truncated {
		t.Fatalf("追问仍为空时应标记 Truncated")
	}
	if res.Reply == "" {
		t.Fatalf("应给出面向用户的兜底文案，不能留空")
	}
}

// 正文正常时不应触发追问。
func TestRunNoRetryWhenContentPresent(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatResponse{
		{
			Message:      provider.Message{Role: provider.RoleAssistant, Content: "正常回复"},
			FinishReason: "stop",
		},
	}}

	ag := New(fp, Config{MaxToolRounds: 3, MaxTokens: 8192})
	res, err := ag.Run(context.Background(), nil, emptySnapshot(), nil)
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}

	if fp.calls != 1 {
		t.Fatalf("正文正常时不应追问，实际调用 %d 次", fp.calls)
	}
	if res.Reply != "正常回复" {
		t.Fatalf("回复不正确：%q", res.Reply)
	}
	if res.Truncated {
		t.Fatalf("不应标记 Truncated")
	}
}

func emptySnapshot() ops.Snapshot {
	return ops.Snapshot{Title: "测试表", Items: []ops.Item{}}
}
