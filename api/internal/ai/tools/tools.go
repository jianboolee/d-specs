// Package tools 定义暴露给模型的工具集，并实现其中的只读工具。
//
// 分工：
//   - 只读工具（get_schema / search_items / get_items / count_items）
//     让模型按需取数，而不是把整张表预灌进上下文 —— 参数表大了会直接撑爆窗口。
//   - 唯一写工具 propose_ops
//     模型改变数据的唯一通道，产出结构化操作交给 ops 包校验后应用。
package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/donktech/d-specs/internal/ai/ops"
	"github.com/donktech/d-specs/internal/ai/provider"
)

// 工具名
const (
	ToolGetSchema   = "get_schema"
	ToolSearchItems = "search_items"
	ToolGetItems    = "get_items"
	ToolCountItems  = "count_items"
	ToolProposeOps  = "propose_ops"
)

// 默认与上限
const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
)

// IsReadTool 判断是否为只读工具
func IsReadTool(name string) bool {
	switch name {
	case ToolGetSchema, ToolSearchItems, ToolGetItems, ToolCountItems:
		return true
	default:
		return false
	}
}

// Definitions 返回提供给模型的完整工具定义。
//
// 工具描述是模型行为的主要约束手段：写清楚「什么时候用哪个」比在 system prompt
// 里反复叮嘱更有效，尤其是能力较弱的档位。
func Definitions() []provider.Tool {
	return []provider.Tool{
		provider.NewFunctionTool(
			ToolGetSchema,
			"获取当前参数表的结构概览：表标题、分组列表及每组参数项数量、全部参数项的名称与类型。不返回具体参数值。改动之前应先调用它来确认实际存在哪些参数名。",
			objectSchema(nil, nil),
		),
		provider.NewFunctionTool(
			ToolSearchItems,
			"按关键词或分组检索参数项，返回匹配项的 id、名称、值、单位、分组。当需要知道某个参数当前的值时使用。",
			objectSchema(map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "按参数名模糊匹配，例如「电池」「重量」。不传则不过滤",
				},
				"group": map[string]any{
					"type":        "string",
					"description": "限定分组，例如「续航」。不传则不过滤",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("返回条数上限，默认 %d，最大 %d", defaultSearchLimit, maxSearchLimit),
				},
			}, nil),
		),
		provider.NewFunctionTool(
			ToolGetItems,
			"按 id 批量获取参数项的完整信息。当已经从 get_schema 或 search_items 拿到 id、需要确认具体值时使用。",
			objectSchema(map[string]any{
				"item_ids": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "参数项 id 列表",
				},
			}, []string{"item_ids"}),
		),
		provider.NewFunctionTool(
			ToolCountItems,
			"统计参数项数量，可按分组细分。在做批量改动前用它判断影响面大小。",
			objectSchema(map[string]any{
				"group": map[string]any{
					"type":        "string",
					"description": "只统计某个分组。不传则统计全部",
				},
			}, nil),
		),
		provider.NewFunctionTool(
			ToolProposeOps,
			"提交对参数表的修改。这是唯一能改动数据的方式，所有变更都必须通过它提交。修改前请先确认目标参数项确实存在（用 get_schema 或 search_items 查过），不要凭猜测填参数名。",
			objectSchema(map[string]any{
				"summary": map[string]any{
					"type":        "string",
					"description": "用一句中文说明这次改了什么，会展示给用户",
				},
				"ops": map[string]any{
					"type":        "array",
					"description": "要执行的变更列表，按顺序执行",
					"items":       opSchema(),
				},
			}, []string{"summary", "ops"}),
		),
	}
}

func opSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"op": map[string]any{
				"type": "string",
				"enum": []string{
					ops.OpAddItem, ops.OpAddItems, ops.OpUpdateItem,
					ops.OpDeleteItem, ops.OpReorderItems, ops.OpSetGroup,
				},
				"description": "操作类型。add_item=新增一条；add_items=批量新增；update_item=修改；delete_item=删除；reorder_items=调整顺序；set_group=调整分组",
			},
			"item_id": map[string]any{
				"type":        "string",
				"description": "目标参数项 id。最精确的定位方式，能拿到 id 时优先用它",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "按参数名定位目标。仅在拿不到 id 时使用；若该名称对应多个参数项会报歧义错误",
			},
			"target": map[string]any{
				"type":        "object",
				"description": "条件式定位，由服务端展开成具体参数项。做批量修改时用它，不要逐条枚举，例如「把续航分组下所有参数的单位补上」",
				"properties": map[string]any{
					"group": map[string]any{
						"type":        "string",
						"description": "匹配该分组下的全部参数项",
					},
					"name_in": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "匹配这些参数名",
					},
					"all": map[string]any{
						"type":        "boolean",
						"description": "true 表示表中的全部参数项",
					},
				},
			},
			"item":  draftSchema("新增单条参数项时填写"),
			"items": map[string]any{"type": "array", "items": draftSchema(""), "description": "批量新增时填写"},
			"patch": map[string]any{
				"type":        "object",
				"description": "修改内容，只填需要改的字段，未填的字段保持原样",
				"properties": map[string]any{
					"name":         map[string]any{"type": "string", "description": "新的参数名"},
					"value":        map[string]any{"type": "string", "description": "新的参数值。单位直接写在值里，例如 5000mAh、8小时、198g"},
					"value_type":   map[string]any{"type": "string", "enum": []string{ops.TypeText, ops.TypeNumber, ops.TypeBool, ops.TypeEnum}},
					"enum_options": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "value_type=enum 时的候选值"},
					"group_name":   map[string]any{"type": "string", "description": "新的分组名；传空字符串表示移出分组"},
					"remark":       map[string]any{"type": "string", "description": "备注"},
				},
			},
			"order": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "reorder_items 使用：这些 id 会按给定顺序排到最前面，未列出的保持原有相对顺序",
			},
			"item_ids": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "set_group 使用：要调整分组的参数项 id 列表",
			},
			"group_name": map[string]any{
				"type":        "string",
				"description": "set_group 使用：目标分组名，传空字符串表示移出分组",
			},
		},
		"required": []string{"op"},
	}
}

func draftSchema(description string) map[string]any {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":       map[string]any{"type": "string", "description": "参数名，例如「电池容量」"},
			"value":      map[string]any{"type": "string", "description": "参数值。单位直接写在值里，例如 5000mAh、8小时、198g"},
			"value_type": map[string]any{"type": "string", "enum": []string{ops.TypeText, ops.TypeNumber, ops.TypeBool, ops.TypeEnum}, "description": "默认 text；带单位的数值用 number"},
			"enum_options": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "value_type=enum 时的候选值",
			},
			"group_name": map[string]any{"type": "string", "description": "所属分组，例如「续航」"},
			"remark":     map[string]any{"type": "string", "description": "备注说明"},
		},
		"required": []string{"name"},
	}
	if description != "" {
		schema["description"] = description
	}
	return schema
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// ==================== 只读工具执行 ====================

// Registry 只读工具的执行器，作用于一份内存快照
type Registry struct {
	snapshot ops.Snapshot
}

func NewRegistry(s ops.Snapshot) *Registry {
	return &Registry{snapshot: s}
}

// Snapshot 返回当前快照
func (r *Registry) Snapshot() ops.Snapshot {
	return r.snapshot
}

// SetSnapshot 在 ops 生效后同步最新状态，
// 使同一轮对话里后续的工具调用能看到刚改完的结果
func (r *Registry) SetSnapshot(s ops.Snapshot) {
	r.snapshot = s
}

// Execute 执行只读工具并返回可 JSON 序列化的结果
func (r *Registry) Execute(name string, args map[string]any) (any, error) {
	switch name {
	case ToolGetSchema:
		return r.getSchema(), nil
	case ToolSearchItems:
		return r.searchItems(args), nil
	case ToolGetItems:
		return r.getItems(args), nil
	case ToolCountItems:
		return r.countItems(args), nil
	default:
		return nil, fmt.Errorf("工具 %s 不是可执行的只读工具", name)
	}
}

type groupStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type schemaItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ValueType string `json:"value_type"`
	GroupName string `json:"group_name,omitempty"`
}

type schemaResult struct {
	Title     string       `json:"title"`
	ItemCount int          `json:"item_count"`
	Groups    []groupStat  `json:"groups"`
	Items     []schemaItem `json:"items"`
	Note      string       `json:"note,omitempty"`
}

func (r *Registry) getSchema() schemaResult {
	out := schemaResult{
		Title:     r.snapshot.Title,
		ItemCount: len(r.snapshot.Items),
		Groups:    r.groupStats(""),
		Items:     make([]schemaItem, 0, len(r.snapshot.Items)),
	}

	for _, it := range r.snapshot.Items {
		out.Items = append(out.Items, schemaItem{
			ID:        it.ID,
			Name:      it.Name,
			ValueType: it.ValueType,
			GroupName: it.GroupName,
		})
	}

	if len(out.Items) == 0 {
		out.Note = "当前参数表还是空的，可以直接用 add_items 一次性建立参数项"
	}
	return out
}

type searchResult struct {
	Total     int        `json:"total"`
	Matched   int        `json:"matched"`
	Items     []ops.Item `json:"items"`
	Truncated bool       `json:"truncated,omitempty"`
}

func (r *Registry) searchItems(args map[string]any) searchResult {
	keyword := strings.ToLower(strings.TrimSpace(stringArg(args, "keyword")))
	group := strings.TrimSpace(stringArg(args, "group"))
	limit := intArg(args, "limit", defaultSearchLimit)
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	var matched []ops.Item
	for _, it := range r.snapshot.Items {
		if group != "" && it.GroupName != group {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(it.Name), keyword) {
			continue
		}
		matched = append(matched, it)
	}

	res := searchResult{Total: len(r.snapshot.Items), Matched: len(matched)}
	if len(matched) > limit {
		res.Truncated = true
		matched = matched[:limit]
	}
	if matched == nil {
		matched = []ops.Item{}
	}
	res.Items = matched
	return res
}

type getItemsResult struct {
	Items   []ops.Item `json:"items"`
	Missing []string   `json:"missing,omitempty"`
}

func (r *Registry) getItems(args map[string]any) getItemsResult {
	ids := stringSliceArg(args, "item_ids")
	res := getItemsResult{Items: make([]ops.Item, 0, len(ids))}
	for _, id := range ids {
		item, ok := r.snapshot.FindByID(id)
		if !ok {
			res.Missing = append(res.Missing, id)
			continue
		}
		res.Items = append(res.Items, *item)
	}
	return res
}

type countResult struct {
	Total  int         `json:"total"`
	Groups []groupStat `json:"groups"`
	Filter string      `json:"filter,omitempty"`
}

func (r *Registry) countItems(args map[string]any) countResult {
	group := strings.TrimSpace(stringArg(args, "group"))
	if group == "" {
		return countResult{Total: len(r.snapshot.Items), Groups: r.groupStats("")}
	}
	return countResult{Total: r.groupStats(group)[0].Count, Filter: group}
}

func (r *Registry) groupStats(group string) []groupStat {
	counts := map[string]int{}
	for _, it := range r.snapshot.Items {
		counts[it.GroupName]++
	}
	if group != "" {
		return []groupStat{{Name: group, Count: counts[group]}}
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]groupStat, 0, len(names)+1)
	for _, name := range names {
		out = append(out, groupStat{Name: name, Count: counts[name]})
	}
	if counts[""] > 0 {
		out = append(out, groupStat{Name: "", Count: counts[""]})
	}
	return out
}

// ==================== 参数取值辅助 ====================

func stringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func intArg(args map[string]any, key string, fallback int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return fallback
	}
}

func stringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
