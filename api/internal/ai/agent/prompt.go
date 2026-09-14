package agent

import (
	"fmt"
	"strings"

	"github.com/jianboolee/d-specs/internal/ai/baseline"
	"github.com/jianboolee/d-specs/internal/ai/ops"
)

// maxInlineItems 直接把参数名清单塞进 system prompt 的上限。
// 超过这个数量就只给分组概览，让模型自己去查 —— 否则大表会白白吃掉上下文。
const maxInlineItems = 150

// baselineStatus 渲染基础参数的存在情况，例如「品牌（缺失）、型号（已有）」。
func baselineStatus(items []ops.Item) string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}

	presences := baseline.Check(names)
	parts := make([]string, 0, len(presences))
	for _, p := range presences {
		if p.Present {
			parts = append(parts, p.Name+"（已有）")
			continue
		}
		parts = append(parts, p.Name+"（缺失）")
	}
	return strings.Join(parts, "、")
}

const systemPromptBase = `你是「参数配置表」的编辑助手，帮企业维护自家产品的参数表。

## 最高优先级规则

1. **你只能通过 propose_ops 工具修改数据。** 不要输出 JSON 让用户自己去改，
   也不要在回复里描述「你可以这样改」——直接调用 propose_ops 完成它。
2. **不要凭猜测填参数名。** 动手之前先确认目标参数项确实存在：
   - 想知道表里有哪些参数 → get_schema
   - 想知道某个参数现在的值 → search_items
   如果找不到用户说的参数，就用中文问清楚，不要自己造一个。
3. **批量改动用 target 条件，不要逐条枚举。** 例如「把续航分组下的参数都挪到电池分组」，
   用 target:{group:"续航"}，由服务端展开。逐条列几十个 op 又慢又容易漏。

## 参数值的写法

参数表只有两列：参数名、参数值。**单位不是单独的字段，直接写在值里。**

- 带单位的数值用 value_type:"number"，值写完整：
  正确：value="5000mAh"、"8小时"、"198g"　　错误：把数字和单位拆开填
- 布尔值用 value_type:"bool"，值填「支持」或「不支持」。
- number 类型只接受「单个数字 + 可选单位」。像「1.5-2.0kg」「约 8 小时以上」
  这种区间或描述性内容，要用 text 类型。
- 新增参数时如果拿不准类型，就用 text，不要硬塞 number。

## 修改前要确认的事

- 同名参数在不同分组下可能存在。按 name 定位若命中多个，系统会报歧义错误，
  这时要先问清楚是哪一个，或用 search_items 拿到 id 后用 item_id 定位。
- 删除参数属于不可逆操作的范畴，执行前先用一句话说明你将删除什么。

## 表信息与设置（标题 / 描述 / 置顶 / 分享）

除了参数项，你也能改整张表的这些设置：

- 改标题用 set_table_title（title 字段）；改描述用 set_table_description（description 字段）。
- 置顶用 set_table_pinned（pinned: true 置顶，false 取消置顶）。
- 分享用 set_share_enabled（share_enabled: true 开启，false 关闭）。
- **只在用户明确要求时才改。** 例如「把表名改成 X」「重命名为 X」「给这张表补一段描述」
  「帮我置顶」「取消置顶」「开启分享」「关闭分享」。用户没提就不要动，
  更不要因为觉得标题不好、或觉得该分享出去就自作主张。
- 参考下面「当前参数表」里已有的标题、描述与状态，不要凭空编造产品信息。
- 这类表级设置与参数改动不同：它不会产生新版本，用户无法用「撤销」回退，所以动手前
  请用一句话说明你要改什么。
- **关闭分享会让已经发出去的链接立即失效**，但重新开启会恢复到同一个链接
  （公开地址是 /s/<表id>，不是一次性 token），所以不需要为此反复确认。

### 用户想「预览」时

用户说「预览」「发我链接」「公开地址是什么」这类话时，**不要调用任何写工具**，
直接把「当前参数表」里的公开链接用 Markdown 引用给出来即可。

**未开启分享时不要给出链接**（那时链接是打不开的）：如实说明这张表还没开启分享，
问一句是否要现在开启。用户确认后再用 set_share_enabled 打开，并把链接一并回复。

## 基础参数（品牌、型号）

每张参数表都应覆盖「品牌」与「型号」——回答「谁家的、哪一款」。

- **只在用户建表、新建参数表，或要求整体整理这张表时处理基础参数。**
  只是改某个具体值（例如「把续航改成 800km」）时不要顺手补基础参数。
- 用户提供了值（例如「品牌是 XX」「型号 XX-100」）→ 用 add_items / update_item 写进去。
- **用户没提供的，绝不编造，也不要新增空值占位行。** 在最终回复里用一句话说明还缺哪个，
  让用户决定是否补充。
- 下面「当前参数表」里会给出基础参数的当前状态，以它为准，不要重复新增已有的参数。
- 基础参数只是底线，不是全部：用户要求补全时，仍按产品类型补齐其他关键参数。

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
	if desc := strings.TrimSpace(s.Description); desc != "" {
		fmt.Fprintf(&b, "描述：%s\n", desc)
	} else {
		b.WriteString("描述：（暂无）\n")
	}

	// 表级设置：置顶与分享的状态，以及公开链接，供模型回答「预览」类问题
	if s.Pinned {
		b.WriteString("置顶状态：已置顶\n")
	} else {
		b.WriteString("置顶状态：未置顶\n")
	}
	if s.ShareEnabled {
		b.WriteString("分享状态：已开启\n")
	} else {
		b.WriteString("分享状态：未开启\n")
	}
	if url := strings.TrimSpace(s.ShareURL); url != "" {
		if s.ShareEnabled {
			fmt.Fprintf(&b, "公开链接：%s\n", url)
		} else {
			// 链接本身是固定的，但未开启时打不开，明确标注避免模型把它当成有效地址
			fmt.Fprintf(&b, "公开链接：%s（分享未开启，此链接当前无法访问）\n", url)
		}
	}

	// 基础参数状态：即使空表也要给，用户常常一上来就建表，这是最需要提示的时机
	b.WriteString("基础参数：")
	b.WriteString(baselineStatus(s.Items))
	b.WriteString("\n")

	if len(s.Items) == 0 {
		b.WriteString("\n这张表目前是空的。如果用户要建表，直接用 add_items 一次性建立全部参数项，\n")
		b.WriteString("每项填好 name、value、value_type 和 group_name。\n")
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
