// Package analysis 提供「产品类型识别 + 自动打标签 + 下一步建议」能力。
//
// 与 chat 模块的区别：这里不做多轮工具循环，只根据当前表内容做一次
// 结构化的单次调用。因此不需要 agent / tools，直接问模型要一段 JSON。
package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jianboolee/d-specs/internal/ai/provider"
	"github.com/jianboolee/d-specs/internal/config"
	"github.com/jianboolee/d-specs/internal/modules/spec"
)

var (
	// ErrLLMUnavailable 未配置 LLM
	ErrLLMUnavailable = errors.New("AI 未配置，无法使用分析功能")
	// ErrNothingToAnalyze 空表没有可分析的内容
	ErrNothingToAnalyze = errors.New("这张表还没有参数，先添加一些内容再分析")
)

const (
	// maxInlineItems 直接把参数名清单塞进 prompt 的上限，与 agent 的取值一致。
	// 超过这个数量就只给分组概览，避免大表把 prompt 撑爆。
	maxInlineItems = 150
	// maxTags 自动写入的标签个数上限
	maxTags = 5
	// maxTagLen 单个标签的最大长度
	maxTagLen = 16
	// maxSuggestions 建议条数上限
	maxSuggestions = 6
)

type Service struct {
	specSvc *spec.Service
	llm     provider.Provider
	cfg     *config.Config
}

func NewService(specSvc *spec.Service, llm provider.Provider, cfg *config.Config) *Service {
	return &Service{specSvc: specSvc, llm: llm, cfg: cfg}
}

const systemPrompt = `你是「参数配置表」的产品分析助手。根据用户给的参数表内容，判断这张表描述的是哪一类产品，并给出还可以补充哪些关键参数的建议。

只输出一个 JSON 对象，不要输出任何解释文字或 Markdown 代码块。格式：

{"product_type":"房车","tags":["房车","越野"],"suggestions":[{"name":"品牌","reason":"选型与报价必备"}]}

规则：
- product_type：用最贴切的中文短语概括产品类型，例如「房车」「汽车配件」「电子消费品」「家用电器」「工业设备」。
- tags：2 到 5 个中文短标签，用于归类与展示。保留大小写与专有名词写法，不要带标点、不要重复。
- suggestions：0 到 6 条，列出这类产品通常必备、但当前表里还没有的参数。每条给参数名和一句话理由。
  只建议真正缺失的关键参数（例如品牌、型号、指导价、上市时间），不要把表里已有的参数再列一遍。
- 如果参数太少、无法判断，就给出你能判断的最接近的类型，并尽量给出建议。`

// Analyze 分析一张表：识别产品类型、自动打标签、给出补充建议。
func (s *Service) Analyze(ctx context.Context, userID, tableID string) (*Result, error) {
	if s.llm == nil {
		return nil, ErrLLMUnavailable
	}

	// 越权在这里被挡掉，同时拿到当前表内容
	detail, err := s.specSvc.Detail(ctx, userID, tableID)
	if err != nil {
		return nil, err
	}
	if len(detail.Items) == 0 {
		return nil, ErrNothingToAnalyze
	}

	out, err := s.callLLM(ctx, detail)
	if err != nil {
		return nil, err
	}

	tags := sanitizeTags(out.Tags)
	if tags == nil {
		tags = []string{}
	}

	// 标签自动落库。走元信息更新，不产生版本 —— 与改标题同类。
	// 只写 tags，标题/描述/行业保持不变。
	if len(tags) > 0 {
		if _, err := s.specSvc.UpdateMeta(ctx, userID, tableID, spec.UpdateRequest{Tags: &tags}); err != nil {
			return nil, err
		}
	}

	return &Result{
		ProductType: strings.TrimSpace(out.ProductType),
		Tags:        tags,
		Suggestions: sanitizeSuggestions(out.Suggestions),
	}, nil
}

func (s *Service) callLLM(ctx context.Context, detail *spec.DetailResult) (*llmOutput, error) {
	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: systemPrompt},
		{Role: provider.RoleUser, Content: buildUserPrompt(detail)},
	}

	maxTokens := s.cfg.AI.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	resp, err := s.llm.Chat(ctx, provider.ChatRequest{
		Messages: messages,
		// 分析要的是稳定、可解析的结果，温度压低
		Temperature: 0.2,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(resp.Message.Content)
	if raw == "" {
		return nil, fmt.Errorf("AI 未返回内容，请稍后重试")
	}

	out, err := parseLLMOutput(raw)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// buildUserPrompt 组装用户消息。只给参数名不给值：值对判断产品类型帮助有限，
// 却会显著推高 token。
func buildUserPrompt(detail *spec.DetailResult) string {
	var b strings.Builder
	title := strings.TrimSpace(detail.Table.Title)
	if title == "" {
		title = "（未命名）"
	}
	fmt.Fprintf(&b, "标题：%s\n", title)
	if desc := strings.TrimSpace(detail.Table.Description); desc != "" {
		fmt.Fprintf(&b, "描述：%s\n", desc)
	}
	if industry := strings.TrimSpace(detail.Table.Industry); industry != "" {
		fmt.Fprintf(&b, "已有行业标注：%s\n", industry)
	}
	if len(detail.Table.Tags) > 0 {
		fmt.Fprintf(&b, "已有标签：%s\n", strings.Join(detail.Table.Tags, "、"))
	}
	fmt.Fprintf(&b, "参数项数量：%d\n", len(detail.Items))

	counts := map[string]int{}
	for _, it := range detail.Items {
		counts[it.GroupName]++
	}
	groups := make([]string, 0, len(counts))
	for name, c := range counts {
		if name == "" {
			continue
		}
		groups = append(groups, fmt.Sprintf("%s(%d项)", name, c))
	}
	if len(groups) > 0 {
		fmt.Fprintf(&b, "分组：%s\n", strings.Join(groups, "、"))
	}

	b.WriteString("\n参数名清单：\n")
	if len(detail.Items) > maxInlineItems {
		// 大表只给分组概览，避免 prompt 过大
		b.WriteString("（参数项较多，这里不逐条列出，请结合标题与分组判断）\n")
	} else {
		for _, it := range detail.Items {
			b.WriteString("- ")
			b.WriteString(it.Name)
			if it.GroupName != "" {
				b.WriteString(" [分组：" + it.GroupName + "]")
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n请判断产品类型、给出标签与建议，只输出 JSON。")
	return b.String()
}

// parseLLMOutput 容错解析模型返回的 JSON。
//
// 模型可能把 JSON 包在 ```json 代码块里，或在前后夹带说明文字，
// 所以先剥掉代码块围栏，再退化到「取第一个 { 到最后一个 }」。
func parseLLMOutput(raw string) (*llmOutput, error) {
	cleaned := stripCodeFence(raw)
	if !strings.HasPrefix(cleaned, "{") {
		if start := strings.Index(cleaned, "{"); start >= 0 {
			if end := strings.LastIndex(cleaned, "}"); end > start {
				cleaned = cleaned[start : end+1]
			}
		}
	}

	var out llmOutput
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return nil, fmt.Errorf("无法解析 AI 返回的分析结果：%w", err)
	}
	return &out, nil
}

func stripCodeFence(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// 去掉第一行（``` 或 ```json）
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[idx+1:]
	} else {
		return s
	}
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func sanitizeTags(raw []string) []string {
	out := make([]string, 0, maxTags)
	seen := map[string]struct{}{}
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len([]rune(t)) > maxTagLen {
			t = string([]rune(t)[:maxTagLen])
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) == maxTags {
			break
		}
	}
	return out
}

func sanitizeSuggestions(raw []Suggestion) []Suggestion {
	out := make([]Suggestion, 0, maxSuggestions)
	seen := map[string]struct{}{}
	for _, s := range raw {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, Suggestion{Name: name, Reason: strings.TrimSpace(s.Reason)})
		if len(out) == maxSuggestions {
			break
		}
	}
	return out
}
