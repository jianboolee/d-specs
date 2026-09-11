package agent

import (
	"fmt"
	"strings"

	"github.com/donktech/d-specs/internal/ai/ops"
)

// maxInlineItems 直接把参数名清单塞进 system prompt 的上限。
// 超过这个数量就只给分组概览，让模型自己去查 —— 否则大表会白白吃掉上下文。
const maxInlineItems = 150

const systemPromptBase = `你是「参数配置表」的编辑助手，帮助企业维护产品的参数表。

## 最高优先级规则

1. **你只能通过 propose_ops 工具修改数据。** 不要输出 JSON 让用户自己去改，
   也不要在回复里描述「你可以这样改」——直接调用 propose_ops 完成它。
2. **不要凭猜测填参数名。** 动手之前先确认目标参数项确实存在：
   - 想知道表里有哪些参数 → get_schema
   - 想知道某个参数现在的值 → search_items
   如果找不到用户说的参数，就用中文问清楚，不要自己造一个。
3. **批量改动用 target 条件，不要逐条枚举。** 例如「把续航分组下所有参数加个单位」，
   用 target:{group:"续航"}，由服务端展开。逐条列几十个 op 又慢又容易漏。

## 参数值的写法

- 带单位的数值用 value_type:"number"，**value 只填数字，单位填在 unit 里**。
  正确：value="5000", unit="mAh"　　错误：value="5000mAh"
- 布尔值用 value_type:"bool"，值填「支持」或「不支持」。
- 只有 text 类型才可以把单位和描述一起写在 value 里（例如「1.5-2.0kg」这种区间）。
- 新增参数时如果拿不准类型，就用 text，不要硬塞 number。

## 修改前要确认的事

- 同名参数在不同分组下可能存在。按 name 定位若命中多个，系统会报歧义错误，
  这时要先问清楚是哪一个，或用 search_items 拿到 id 后用 item_id 定位。
- 删除参数属于不可逆操作的范畴，执行前先用一句话说明你将删除什么。

## 回复风格

- 用中文，简洁，不要复述工具返回的原始数据。
- 改完后用一两句话说明改了什么。
- 没有实际改动时（比如只是回答问题），不要调用 propose_ops。`

// BuildSystemPrompt 组装 system prompt。
//
// 刻意只带「参数名 + 分组」而不带值：参数表可能有几百条，
// 全量值塞进去必然撑爆窗口。需要值时让模型自己用 search_items 拉。
func BuildSystemPrompt(s ops.Snapshot) string {
	var b strings.Builder
	b.WriteString(systemPromptBase)
	b.WriteString("\n\n## 当前参数表\n\n")

	title := s.Title
	if strings.TrimSpace(title) == "" {
		title = "（未命名）"
	}
	fmt.Fprintf(&b, "标题：%s\n参数项数量：%d\n", title, len(s.Items))

	if len(s.Items) == 0 {
		b.WriteString("\n这张表目前是空的。如果用户要建表，直接用 add_items 一次性建立全部参数项，\n")
		b.WriteString("每项填好 name、value、unit、value_type 和 group_name。\n")
		return b.String()
	}

	groups := s.Groups()
	if len(groups) > 0 {
		b.WriteString("\n分组：")
		parts := make([]string, 0, len(groups))
		counts := map[string]int{}
		for _, it := range s.Items {
			counts[it.GroupName]++
		}
		for _, g := range groups {
			parts = append(parts, fmt.Sprintf("%s(%d项)", g, counts[g]))
		}
		if counts[""] > 0 {
			parts = append(parts, fmt.Sprintf("未分组(%d项)", counts[""]))
		}
		b.WriteString(strings.Join(parts, "、"))
		b.WriteString("\n")
	}

	if len(s.Items) > maxInlineItems {
		fmt.Fprintf(&b, "\n参数项较多（%d 项），此处不列出全部名称，请用 get_schema 或 search_items 查询具体参数。\n", len(s.Items))
		return b.String()
	}

	b.WriteString("\n参数项清单（只有名称与类型，值请用 search_items 查询）：\n")
	for _, it := range s.Items {
		line := "- " + it.Name
		if it.ValueType != "" && it.ValueType != ops.TypeText {
			line += "（" + it.ValueType + "）"
		}
		if it.GroupName != "" {
			line += " [分组：" + it.GroupName + "]"
		}
		line += " id=" + it.ID
		b.WriteString(line + "\n")
	}

	return b.String()
}
