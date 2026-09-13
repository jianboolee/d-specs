package analysis

import "strings"

// Suggestion 一条「下一步该补充什么参数」的建议。
type Suggestion struct {
	// Name 建议补充的参数名，例如「品牌」「型号」「指导价」
	Name string `json:"name"`
	// Reason 为什么建议补充，一句话说明
	Reason string `json:"reason"`
}

// Result 一次 AI 分析的结果。
//
// Tags 是已经落库的标签（服务端已清洗去重）；Suggestions 只用于展示，
// 不会自动写入参数表 —— 是否补齐由用户决定。
type Result struct {
	// ProductType 模型判断出的产品类型，例如「房车」「配件」「电子消费品」
	ProductType string       `json:"product_type"`
	Tags        []string     `json:"tags"`
	Suggestions []Suggestion `json:"suggestions"`
}

// Markdown 把结构化分析渲染成一段中文 Markdown，作为一轮对话的助手回复。
//
// 之所以在服务端渲染而不是把 JSON 丢给前端：分析结果会作为 assistant 消息
// 落库，后续对话直接回放这段文本，两端只需要一份格式。
func (r Result) Markdown() string {
	var b strings.Builder

	productType := strings.TrimSpace(r.ProductType)
	if productType == "" {
		productType = "暂无法判断"
	}
	b.WriteString("我通读了一遍这张参数表。\n\n**产品类型**：")
	b.WriteString(productType)
	b.WriteString("\n")

	if len(r.Tags) > 0 {
		b.WriteString("**自动标签**：")
		b.WriteString(strings.Join(r.Tags, "、"))
		b.WriteString("（已保存到这张表）\n")
	}

	b.WriteString("\n")
	if len(r.Suggestions) == 0 {
		b.WriteString("常见的必备参数都齐了，这张表看起来已经比较完整。\n")
		return b.String()
	}

	b.WriteString("这类产品通常还会用到以下参数，目前表里还没有：\n\n")
	for _, s := range r.Suggestions {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		b.WriteString("- **")
		b.WriteString(name)
		b.WriteString("**")
		if reason := strings.TrimSpace(s.Reason); reason != "" {
			b.WriteString("：")
			b.WriteString(reason)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n需要补哪几项，直接告诉我，我来帮你加上。\n")
	return b.String()
}

// llmOutput 模型应返回的严格 JSON 结构。
type llmOutput struct {
	ProductType string       `json:"product_type"`
	Tags        []string     `json:"tags"`
	Suggestions []Suggestion `json:"suggestions"`
}
