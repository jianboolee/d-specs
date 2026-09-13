// Package followups 根据一轮助手回复，生成 1~3 个「下一步可以怎么说」的建议动作。
//
// 用途：聊天界面在每条回复下方展示这些动作，点击即当作一条用户消息发出去，
// 省去用户自己组织语言。生成是尽力而为的：任何失败都退化成「没有动作」，
// 绝不能因为这一步出错而影响本该正常返回的回复。
package followups

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jianboolee/d-specs/internal/ai/provider"
)

const (
	// maxActions 单轮最多给出的动作数
	maxActions = 3
	// maxLabelLen label 是按钮上的短文案，超长会撑坏排版
	maxLabelLen = 12
	// maxMessageLen message 是要原样发出去的指令，给足空间但要有上限
	maxMessageLen = 200
	// maxReplyRunes 送进 prompt 的回复截断长度，避免长回复抬高这一步的成本
	maxReplyRunes = 1200
	// maxTokens 生成动作本身的预算，动作很短，给多反而是浪费
	maxTokens = 400
)

// Action 一个建议动作。
type Action struct {
	// Label 按钮上的短文案，例如「带上年款」
	Label string `json:"label"`
	// Message 点击后发送的完整指令，例如「把表标题带上 2026 款」
	Message string `json:"message"`
}

// Generator 动作生成器
type Generator struct {
	llm provider.Provider
}

func New(llm provider.Provider) *Generator {
	return &Generator{llm: llm}
}

const systemPrompt = `你是「参数配置表」编辑助手的交互设计器。根据助手刚刚给出的回复，推测用户接下来最可能想说的话，输出 1 到 3 个建议动作。这些动作会作为按钮展示在回复下方，用户点击后该动作的 message 会作为一条新消息原样发送给助手。

只输出一个 JSON 对象，不要输出任何解释文字或 Markdown 代码块。格式：

{"actions":[{"label":"带上年款","message":"把表标题带上 2026 款"}]}

规则：
- label：按钮上的短文案，2 到 6 个汉字，不要带标点、不要带「请」等客套词。例如「带上年款」「使用简称」「补上型号」「按推荐来」。
- message：用户视角的完整指令，必须独立表意，不要用「这个」「它」「上面说的」这类指代，也不要复述整段回复。例如「把表标题改成带 2026 款的写法」「用简称『旅行家V90』作为表标题」。
- 只依据助手回复的内容做建议：回复里提到可选方案就各给一个动作，回复指出还缺某参数就给「补上 X」，回复给出了推荐做法就给「按推荐来」。
- 如果回复是纯告知、没有有意义的下一步（例如只是回答一个问题），返回空数组 {"actions":[]}。
- 宁缺毋滥：不要为了凑数编造用户根本没打算做的事。`

// Suggest 生成建议动作。
//
// 任何失败（未配置 LLM、超时、返回不可解析）都返回 nil，不返回 error：
// 调用方在已经拿到正常回复之后再调它，这里不应该有「失败」的上层语义。
func (g *Generator) Suggest(ctx context.Context, tableTitle, userMessage, reply string) []Action {
	if g == nil || g.llm == nil {
		return nil
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}

	resp, err := g.llm.Chat(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: systemPrompt},
			{Role: provider.RoleUser, Content: buildPrompt(tableTitle, userMessage, reply)},
		},
		// 要的是稳定可解析的结果，温度压低
		Temperature: 0.2,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil
	}

	out, err := parse(strings.TrimSpace(resp.Message.Content))
	if err != nil {
		return nil
	}
	return sanitize(out.Actions)
}

func buildPrompt(tableTitle, userMessage, reply string) string {
	var b strings.Builder
	if title := strings.TrimSpace(tableTitle); title != "" {
		fmt.Fprintf(&b, "当前参数表标题：%s\n", title)
	}
	if msg := strings.TrimSpace(userMessage); msg != "" {
		fmt.Fprintf(&b, "用户上一条消息：%s\n", truncateRunes(msg, maxReplyRunes))
	}
	b.WriteString("助手刚给出的回复：\n")
	b.WriteString(truncateRunes(reply, maxReplyRunes))
	b.WriteString("\n\n请据此给出 1 到 3 个建议动作，只输出 JSON。")
	return b.String()
}

type llmOutput struct {
	Actions []Action `json:"actions"`
}

// parse 容错解析模型返回的 JSON：剥掉 ``` 代码块围栏，
// 再退化到「取第一个 { 到最后一个 }」，容忍前后夹带的说明文字。
func parse(raw string) (*llmOutput, error) {
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
		return nil, fmt.Errorf("无法解析建议动作：%w", err)
	}
	return &out, nil
}

func stripCodeFence(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
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

// sanitize 清洗模型输出：丢弃缺字段的、截断超长的、按 label 去重、限制条数。
func sanitize(raw []Action) []Action {
	out := make([]Action, 0, maxActions)
	seen := make(map[string]struct{}, len(raw))
	for _, a := range raw {
		label := strings.TrimSpace(a.Label)
		message := strings.TrimSpace(a.Message)
		// label 与 message 缺一不可：只有 label 没法发送，只有 message 没有按钮文案
		if label == "" || message == "" {
			continue
		}
		label = truncateRunes(label, maxLabelLen)
		message = truncateRunes(message, maxMessageLen)

		key := strings.ToLower(label)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		out = append(out, Action{Label: label, Message: message})
		if len(out) == maxActions {
			break
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
