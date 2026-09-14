// Package ops 定义 AI 与手工编辑共用的参数表变更协议。
//
// 设计铁律：AI 只能产出结构化操作（Op），绝不整表覆盖。
// 所有变更 —— 无论来自 AI、用户手工操作还是模板套用 —— 都经过同一套
// Validate / Apply，保证校验、版本、撤销、审计只有一份实现。
package ops

import "fmt"

// 操作类型
const (
	OpAddItem      = "add_item"
	OpAddItems     = "add_items"
	OpUpdateItem   = "update_item"
	OpDeleteItem   = "delete_item"
	OpReorderItems = "reorder_items"
	OpSetGroup     = "set_group"

	// 表级元信息操作。与参数项操作不同，它们不改动参数内容，
	// 落库时走元信息路径（不产生 revision），因此不可通过画布撤销。
	OpSetTableTitle       = "set_table_title"
	OpSetTableDescription = "set_table_description"
	// 置顶与分享同样是表级设置：不产生新版本，也不进撤销栈。
	OpSetTablePinned  = "set_table_pinned"
	OpSetShareEnabled = "set_share_enabled"
)

// IsTableOp 判断是否为表级元信息操作。
//
// 落库时据此分流：参数 op 走版本化 Apply，表级 op 走元信息更新。
func IsTableOp(op string) bool {
	switch op {
	case OpSetTableTitle, OpSetTableDescription, OpSetTablePinned, OpSetShareEnabled:
		return true
	default:
		return false
	}
}

// 参数值类型
const (
	TypeText   = "text"
	TypeNumber = "number"
	TypeBool   = "bool"
	TypeEnum   = "enum"
)

// 规模上限，防止模型失控生成海量操作
const (
	// MaxOpsPerRequest 单次请求的 op 条数上限
	MaxOpsPerRequest = 200
	// MaxItemsPerTable 单张参数表的参数项上限
	MaxItemsPerTable = 2000
	// MaxExpandedOps 条件式 op 展开后的条数上限
	MaxExpandedOps = 500
)

var validValueTypes = map[string]bool{
	TypeText:   true,
	TypeNumber: true,
	TypeBool:   true,
	TypeEnum:   true,
}

// Item 参数项
//
// 单位不是独立字段：参数值本身写完整（5000mAh、8小时、198g），
// 这样既贴近真实规格表的写法，也不会出现「值有、单位空」的半截数据。
type Item struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Value       string   `json:"value"`
	ValueType   string   `json:"value_type"`
	EnumOptions []string `json:"enum_options,omitempty"`
	GroupName   string   `json:"group_name"`
	Remark      string   `json:"remark"`
	SortOrder   int      `json:"sort_order"`
}

// Snapshot 一张参数表的完整状态
type Snapshot struct {
	Title string `json:"title"`
	// Description 表描述。表级 op 需要它来计算变更前后的值。
	Description string `json:"description"`
	// Pinned 与 ShareEnabled 是表级设置：表级 op 需要它们做「无需重复操作」判定。
	Pinned       bool `json:"pinned"`
	ShareEnabled bool `json:"share_enabled"`
	// ShareURL 公开分享链接，仅供 prompt 展示（模型据此回答「预览」）。
	// 它是确定性的 /s/<表id>，与是否开启分享无关。
	ShareURL string `json:"share_url,omitempty"`
	Items    []Item `json:"items"`
}

// Clone 深拷贝，避免 Apply 过程中污染调用方持有的快照
func (s Snapshot) Clone() Snapshot {
	out := Snapshot{
		Title:        s.Title,
		Description:  s.Description,
		Pinned:       s.Pinned,
		ShareEnabled: s.ShareEnabled,
		ShareURL:     s.ShareURL,
		Items:        make([]Item, len(s.Items)),
	}
	copy(out.Items, s.Items)
	for i := range out.Items {
		if s.Items[i].EnumOptions != nil {
			out.Items[i].EnumOptions = append([]string(nil), s.Items[i].EnumOptions...)
		}
	}
	return out
}

// FindByID 按 id 定位参数项
func (s Snapshot) FindByID(id string) (*Item, bool) {
	for i := range s.Items {
		if s.Items[i].ID == id {
			return &s.Items[i], true
		}
	}
	return nil, false
}

// FindByName 按名称定位参数项（去空格、忽略大小写）
func (s Snapshot) FindByName(name string) (*Item, bool) {
	key := normalizeName(name)
	if key == "" {
		return nil, false
	}
	for i := range s.Items {
		if normalizeName(s.Items[i].Name) == key {
			return &s.Items[i], true
		}
	}
	return nil, false
}

// FindByNameAll 按名称找出全部匹配（用于重名歧义检测）
func (s Snapshot) FindByNameAll(name string) []*Item {
	key := normalizeName(name)
	if key == "" {
		return nil
	}
	var out []*Item
	for i := range s.Items {
		if normalizeName(s.Items[i].Name) == key {
			out = append(out, &s.Items[i])
		}
	}
	return out
}

// Groups 按出现顺序返回全部分组名
func (s Snapshot) Groups() []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range s.Items {
		if it.GroupName == "" || seen[it.GroupName] {
			continue
		}
		seen[it.GroupName] = true
		out = append(out, it.GroupName)
	}
	return out
}

// Target 条件式目标。
//
// 这是「服务端兜底」的关键：模型不必逐条枚举要改哪几项，
// 只需表达「续航分组下的全部参数」，由 Apply 展开。
// Flash 档位下这一点尤其重要 —— 逐条枚举极易漏项。
type Target struct {
	Group  string   `json:"group,omitempty"`
	NameIn []string `json:"name_in,omitempty"`
	All    bool     `json:"all,omitempty"`
}

func (t *Target) IsEmpty() bool {
	return t == nil || (t.Group == "" && len(t.NameIn) == 0 && !t.All)
}

// Describe 用于错误提示
func (t *Target) Describe() string {
	if t == nil {
		return "空条件"
	}
	switch {
	case t.All:
		return "全部参数项"
	case t.Group != "":
		return fmt.Sprintf("分组「%s」", t.Group)
	case len(t.NameIn) > 0:
		return fmt.Sprintf("名称匹配 %v", t.NameIn)
	default:
		return "空条件"
	}
}

// Draft 新建参数项
type Draft struct {
	Name        string   `json:"name"`
	Value       string   `json:"value"`
	ValueType   string   `json:"value_type,omitempty"`
	EnumOptions []string `json:"enum_options,omitempty"`
	GroupName   string   `json:"group_name,omitempty"`
	Remark      string   `json:"remark,omitempty"`
}

// Patch 修改参数项；字段为 nil 表示不改
type Patch struct {
	Name        *string   `json:"name,omitempty"`
	Value       *string   `json:"value,omitempty"`
	ValueType   *string   `json:"value_type,omitempty"`
	EnumOptions *[]string `json:"enum_options,omitempty"`
	GroupName   *string   `json:"group_name,omitempty"`
	Remark      *string   `json:"remark,omitempty"`
}

// Op 一条操作
type Op struct {
	Op string `json:"op"`

	// 定位方式，优先级：ItemID > Name > Target
	ItemID string  `json:"item_id,omitempty"`
	Name   string  `json:"name,omitempty"`
	Target *Target `json:"target,omitempty"`

	// 新建
	Item  *Draft  `json:"item,omitempty"`
	Items []Draft `json:"items,omitempty"`

	// 修改
	Patch *Patch `json:"patch,omitempty"`

	// 重排
	Order []string `json:"order,omitempty"`

	// 分组
	ItemIDs   []string `json:"item_ids,omitempty"`
	GroupName string   `json:"group_name,omitempty"`

	// 表级元信息：set_table_title / set_table_description 使用。
	// Title 直接取字符串；Description 用指针以区分「不传」与「传空清空描述」。
	Title       string  `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`

	// 表级设置：set_table_pinned / set_share_enabled 使用。
	// 同样用指针：取消置顶、关闭分享都要传 false，不能让「零值」与「未传」混淆。
	Pinned       *bool `json:"pinned,omitempty"`
	ShareEnabled *bool `json:"share_enabled,omitempty"`
}

// ChangeKind 变更类型
type ChangeKind string

const (
	ChangeAdd    ChangeKind = "add"
	ChangeUpdate ChangeKind = "update"
	ChangeDelete ChangeKind = "delete"
	ChangeMove   ChangeKind = "move"
	// ChangeTable 表级元信息变更（标题/描述），不代表参数内容变化
	ChangeTable ChangeKind = "table"
)

// Change 单条变更记录，用于前端高亮与审计
type Change struct {
	Kind   ChangeKind `json:"kind"`
	ItemID string     `json:"item_id"`
	Name   string     `json:"name"`
	Before *Item      `json:"before,omitempty"`
	After  *Item      `json:"after,omitempty"`

	// OldValue / NewValue 承载非参数项变更的前后值（目前仅表级元信息）
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
}

func normalizeName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == ' ' || r == '\t' || r == '\n' || r == '\u3000' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		out = append(out, r)
	}
	return string(out)
}
