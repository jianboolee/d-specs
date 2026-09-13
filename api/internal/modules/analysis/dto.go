package analysis

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

// llmOutput 模型应返回的严格 JSON 结构。
type llmOutput struct {
	ProductType string       `json:"product_type"`
	Tags        []string     `json:"tags"`
	Suggestions []Suggestion `json:"suggestions"`
}
