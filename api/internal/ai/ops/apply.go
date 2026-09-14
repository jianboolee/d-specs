package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jianboolee/d-specs/internal/infrastructure/idgen"
	"github.com/jianboolee/d-specs/pkg/quantity"
)

// ValidationErrors 一次性收集全部校验问题。
//
// 不要遇到第一个错误就返回：这些错误会被回灌给模型让它自我修正，
// 一次给全比让它来回试三轮更省 token 也更可靠。
type ValidationErrors []error

func (e ValidationErrors) Error() string {
	if len(e) == 1 {
		return e[0].Error()
	}
	parts := make([]string, len(e))
	for i, err := range e {
		parts[i] = fmt.Sprintf("(%d) %s", i+1, err.Error())
	}
	return "操作校验未通过：" + strings.Join(parts, "；")
}

// Apply 校验并应用 operations。
//
// 原子语义：任何一条 op 校验不通过，整批都不生效，返回原始快照与全部错误。
func Apply(s Snapshot, operations []Op) (Snapshot, []Change, error) {
	next := s.Clone()
	changes, errs := applyAll(&next, operations)
	if len(errs) > 0 {
		return s, nil, ValidationErrors(errs)
	}
	return next, changes, nil
}

// Validate 只校验不应用
func Validate(s Snapshot, operations []Op) []error {
	_, _, err := Apply(s, operations)
	if err == nil {
		return nil
	}
	var ve ValidationErrors
	if errors.As(err, &ve) {
		return ve
	}
	return []error{err}
}

var knownOps = []string{
	OpAddItem, OpAddItems, OpUpdateItem, OpDeleteItem, OpReorderItems, OpSetGroup,
	OpSetTableTitle, OpSetTableDescription, OpSetTablePinned, OpSetShareEnabled,
}

// 表级元信息的上限，与 spec 模块 DTO 的 binding 保持一致
const (
	maxTitleLen       = 120
	maxDescriptionLen = 2000
)

func applyAll(s *Snapshot, operations []Op) ([]Change, []error) {
	var changes []Change
	var errs []error

	if len(operations) == 0 {
		return nil, []error{errors.New("operations 为空：没有需要执行的变更")}
	}
	if len(operations) > MaxOpsPerRequest {
		return nil, []error{fmt.Errorf("单次操作条数 %d 超过上限 %d，请拆分为多次调用", len(operations), MaxOpsPerRequest)}
	}

	for i, op := range operations {
		label := fmt.Sprintf("第 %d 条操作", i+1)
		cs, opErrs := applyOne(s, op, label)
		changes = append(changes, cs...)
		errs = append(errs, opErrs...)
	}

	return changes, errs
}

func applyOne(s *Snapshot, op Op, label string) ([]Change, []error) {
	label = fmt.Sprintf("%s(%s)", label, op.Op)

	switch op.Op {
	case OpAddItem:
		return applyAddItem(s, op, label)
	case OpAddItems:
		return applyAddItems(s, op, label)
	case OpUpdateItem:
		return applyUpdateItem(s, op, label)
	case OpDeleteItem:
		return applyDeleteItem(s, op, label)
	case OpReorderItems:
		return applyReorder(s, op, label)
	case OpSetGroup:
		return applySetGroup(s, op, label)
	case OpSetTableTitle:
		return applySetTableTitle(s, op, label)
	case OpSetTableDescription:
		return applySetTableDescription(s, op, label)
	case OpSetTablePinned:
		return applySetTablePinned(s, op, label)
	case OpSetShareEnabled:
		return applySetShareEnabled(s, op, label)
	default:
		return nil, []error{fmt.Errorf("%s：未知操作类型 %q（可用：%s）", label, op.Op, strings.Join(knownOps, "、"))}
	}
}

// ==================== 表级元信息 ====================

// applySetTableTitle 重命名整张表。
//
// 只改 Snapshot 里的标题；真正的落库由调用方走元信息路径完成
// （不产生 revision，所以不可通过撤销回退）。
func applySetTableTitle(s *Snapshot, op Op, label string) ([]Change, []error) {
	title := strings.TrimSpace(op.Title)
	if title == "" {
		return nil, []error{fmt.Errorf("%s：表标题不能为空", label)}
	}
	if len([]rune(title)) > maxTitleLen {
		return nil, []error{fmt.Errorf("%s：表标题过长（%d 字），上限 %d 字", label, len([]rune(title)), maxTitleLen)}
	}
	if title == s.Title {
		return nil, []error{fmt.Errorf("%s：新标题与当前标题相同，无需修改", label)}
	}

	old := s.Title
	s.Title = title
	return []Change{{
		Kind:     ChangeTable,
		Name:     "表标题",
		OldValue: old,
		NewValue: title,
	}}, nil
}

// applySetTableDescription 修改表描述；传空字符串表示清空。
func applySetTableDescription(s *Snapshot, op Op, label string) ([]Change, []error) {
	if op.Description == nil {
		return nil, []error{fmt.Errorf("%s：缺少 description 字段", label)}
	}
	desc := strings.TrimSpace(*op.Description)
	if len([]rune(desc)) > maxDescriptionLen {
		return nil, []error{fmt.Errorf("%s：表描述过长（%d 字），上限 %d 字", label, len([]rune(desc)), maxDescriptionLen)}
	}
	if desc == s.Description {
		return nil, []error{fmt.Errorf("%s：新描述与当前描述相同，无需修改", label)}
	}

	old := s.Description
	s.Description = desc
	return []Change{{
		Kind:     ChangeTable,
		Name:     "表描述",
		OldValue: old,
		NewValue: desc,
	}}, nil
}

// applySetTablePinned 置顶 / 取消置顶。
//
// 与标题、描述一样只改 Snapshot，落库走元信息路径：置顶是用户级偏好，
// 不产生 revision，因此也没有「撤销」可回退。
func applySetTablePinned(s *Snapshot, op Op, label string) ([]Change, []error) {
	if op.Pinned == nil {
		return nil, []error{fmt.Errorf("%s：缺少 pinned 字段", label)}
	}
	if *op.Pinned == s.Pinned {
		if *op.Pinned {
			return nil, []error{fmt.Errorf("%s：这张表已经是置顶状态，无需重复操作", label)}
		}
		return nil, []error{fmt.Errorf("%s：这张表当前未置顶，无需取消", label)}
	}

	old := yesNo(s.Pinned)
	s.Pinned = *op.Pinned
	return []Change{{
		Kind:     ChangeTable,
		Name:     "置顶",
		OldValue: old,
		NewValue: yesNo(s.Pinned),
	}}, nil
}

// applySetShareEnabled 开启 / 关闭公开分享。
//
// 关闭分享会让已发出的链接立即失效，但重新开启会复用同一个链接
// （公开地址是 /s/<表id>，不是一次性 token），因此这里不做二次确认。
func applySetShareEnabled(s *Snapshot, op Op, label string) ([]Change, []error) {
	if op.ShareEnabled == nil {
		return nil, []error{fmt.Errorf("%s：缺少 share_enabled 字段", label)}
	}
	if *op.ShareEnabled == s.ShareEnabled {
		if *op.ShareEnabled {
			return nil, []error{fmt.Errorf("%s：这张表已经开启分享，无需重复操作", label)}
		}
		return nil, []error{fmt.Errorf("%s：这张表当前未开启分享，无需关闭", label)}
	}

	old := enabledText(s.ShareEnabled)
	s.ShareEnabled = *op.ShareEnabled
	return []Change{{
		Kind:     ChangeTable,
		Name:     "分享",
		OldValue: old,
		NewValue: enabledText(s.ShareEnabled),
	}}, nil
}

func yesNo(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

func enabledText(v bool) string {
	if v {
		return "已开启"
	}
	return "已关闭"
}

// ==================== add ====================

func applyAddItem(s *Snapshot, op Op, label string) ([]Change, []error) {
	if op.Item == nil {
		return nil, []error{fmt.Errorf("%s：缺少 item 字段", label)}
	}
	change, errs := addItem(s, *op.Item, label)
	if len(errs) > 0 {
		return nil, errs
	}
	return []Change{change}, nil
}

func applyAddItems(s *Snapshot, op Op, label string) ([]Change, []error) {
	if len(op.Items) == 0 {
		return nil, []error{fmt.Errorf("%s：items 为空，请用 add_item 或提供 items 数组", label)}
	}

	var changes []Change
	var errs []error
	for i, draft := range op.Items {
		itemLabel := fmt.Sprintf("%s 的第 %d 项", label, i+1)
		change, itemErrs := addItem(s, draft, itemLabel)
		if len(itemErrs) > 0 {
			errs = append(errs, itemErrs...)
			continue
		}
		changes = append(changes, change)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return changes, nil
}

func addItem(s *Snapshot, d Draft, label string) (Change, []error) {
	name := strings.TrimSpace(d.Name)
	if name == "" {
		return Change{}, []error{fmt.Errorf("%s：参数名不能为空", label)}
	}
	if len(s.Items) >= MaxItemsPerTable {
		return Change{}, []error{fmt.Errorf("%s：参数项数量已达上限 %d", label, MaxItemsPerTable)}
	}

	group := strings.TrimSpace(d.GroupName)

	// 同分组下重名才算冲突；不同分组重名是合理业务场景
	for _, existing := range s.FindByNameAll(name) {
		if existing.GroupName == group {
			where := "未分组"
			if group != "" {
				where = "分组「" + group + "」"
			}
			return Change{}, []error{fmt.Errorf("%s：%s下已存在参数「%s」，若要改值请用 update_item", label, where, name)}
		}
	}

	valueType := strings.TrimSpace(d.ValueType)
	if valueType == "" {
		valueType = TypeText
	}
	if !validValueTypes[valueType] {
		return Change{}, []error{fmt.Errorf("%s：value_type=%q 不合法（可用：text、number、bool、enum）", label, d.ValueType)}
	}

	value, errs := normalizeValue(valueType, d.Value, d.EnumOptions, label)
	if len(errs) > 0 {
		return Change{}, errs
	}

	item := Item{
		ID:          idgen.New(),
		Name:        name,
		Value:       value,
		ValueType:   valueType,
		EnumOptions: append([]string(nil), d.EnumOptions...),
		GroupName:   group,
		Remark:      strings.TrimSpace(d.Remark),
		SortOrder:   nextSortOrder(s),
	}
	s.Items = append(s.Items, item)

	added := item
	return Change{Kind: ChangeAdd, ItemID: item.ID, Name: item.Name, After: &added}, nil
}

// ==================== update ====================

func applyUpdateItem(s *Snapshot, op Op, label string) ([]Change, []error) {
	if op.Patch == nil {
		return nil, []error{fmt.Errorf("%s：缺少 patch 字段", label)}
	}
	if op.Patch.isEmpty() {
		return nil, []error{fmt.Errorf("%s：patch 没有任何要修改的字段", label)}
	}

	ids, err := resolveTargetIDs(s, op, label)
	if err != nil {
		return nil, []error{err}
	}

	var changes []Change
	var errs []error
	for _, id := range ids {
		change, itemErrs := updateItem(s, id, *op.Patch, label)
		if len(itemErrs) > 0 {
			errs = append(errs, itemErrs...)
			continue
		}
		changes = append(changes, change)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return changes, nil
}

func updateItem(s *Snapshot, id string, patch Patch, label string) (Change, []error) {
	// 每次都按 id 重新取，避免持有切片指针
	item, ok := s.FindByID(id)
	if !ok {
		return Change{}, []error{fmt.Errorf("%s：找不到 item_id=%s 的参数项", label, id)}
	}

	before := *item
	updated := *item

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return Change{}, []error{fmt.Errorf("%s：参数名不能改为空（原参数「%s」）", label, before.Name)}
		}
		targetGroup := updated.GroupName
		if patch.GroupName != nil {
			targetGroup = strings.TrimSpace(*patch.GroupName)
		}
		for _, other := range s.FindByNameAll(name) {
			if other.ID != id && other.GroupName == targetGroup {
				return Change{}, []error{fmt.Errorf("%s：改为「%s」会与同分组下已有参数重名", label, name)}
			}
		}
		updated.Name = name
	}

	if patch.GroupName != nil {
		updated.GroupName = strings.TrimSpace(*patch.GroupName)
	}
	if patch.Remark != nil {
		updated.Remark = strings.TrimSpace(*patch.Remark)
	}
	if patch.ValueType != nil {
		vt := strings.TrimSpace(*patch.ValueType)
		if !validValueTypes[vt] {
			return Change{}, []error{fmt.Errorf("%s：value_type=%q 不合法（可用：text、number、bool、enum）", label, *patch.ValueType)}
		}
		updated.ValueType = vt
	}
	if patch.EnumOptions != nil {
		updated.EnumOptions = append([]string(nil), (*patch.EnumOptions)...)
	}

	// 值可能随 patch 一起改，也可能因类型变化需要重新归一化
	newValue := updated.Value
	if patch.Value != nil {
		newValue = *patch.Value
	}

	value, errs := normalizeValue(updated.ValueType, newValue, updated.EnumOptions, label)
	if len(errs) > 0 {
		return Change{}, errs
	}
	updated.Value = value

	// 写回
	*item = updated

	after := updated
	return Change{Kind: ChangeUpdate, ItemID: id, Name: updated.Name, Before: &before, After: &after}, nil
}

// ==================== delete ====================

func applyDeleteItem(s *Snapshot, op Op, label string) ([]Change, []error) {
	ids, err := resolveTargetIDs(s, op, label)
	if err != nil {
		return nil, []error{err}
	}

	var changes []Change
	removed := make(map[string]bool, len(ids))
	for _, id := range ids {
		item, ok := s.FindByID(id)
		if !ok {
			continue
		}
		before := *item
		removed[id] = true
		changes = append(changes, Change{Kind: ChangeDelete, ItemID: id, Name: item.Name, Before: &before})
	}

	kept := make([]Item, 0, len(s.Items))
	for _, it := range s.Items {
		if removed[it.ID] {
			continue
		}
		kept = append(kept, it)
	}
	s.Items = kept

	return changes, nil
}

// ==================== reorder ====================

func applyReorder(s *Snapshot, op Op, label string) ([]Change, []error) {
	if len(op.Order) == 0 {
		return nil, []error{fmt.Errorf("%s：order 为空", label)}
	}

	rank := make(map[string]int, len(op.Order))
	for i, id := range op.Order {
		if _, ok := s.FindByID(id); !ok {
			return nil, []error{fmt.Errorf("%s：order 中的 item_id=%s 不存在", label, id)}
		}
		if _, dup := rank[id]; dup {
			return nil, []error{fmt.Errorf("%s：order 中 item_id=%s 重复", label, id)}
		}
		rank[id] = i
	}

	// 列出的项按给定顺序排到前面，未列出的保持原有相对顺序
	listed := make([]Item, 0, len(op.Order))
	rest := make([]Item, 0, len(s.Items))
	for _, it := range s.Items {
		if _, ok := rank[it.ID]; ok {
			listed = append(listed, it)
			continue
		}
		rest = append(rest, it)
	}

	sortByRank(listed, rank)
	ordered := append(listed, rest...)
	for i := range ordered {
		ordered[i].SortOrder = i
	}
	s.Items = ordered

	changes := make([]Change, 0, len(listed))
	for _, it := range listed {
		cur := it
		changes = append(changes, Change{Kind: ChangeMove, ItemID: it.ID, Name: it.Name, After: &cur})
	}
	return changes, nil
}

func sortByRank(items []Item, rank map[string]int) {
	// 规模很小（单表参数项数量级），插入排序即可，避免引入 sort 的比较函数开销
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && rank[items[j].ID] < rank[items[j-1].ID]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// ==================== set_group ====================

func applySetGroup(s *Snapshot, op Op, label string) ([]Change, []error) {
	group := strings.TrimSpace(op.GroupName)

	var ids []string
	if len(op.ItemIDs) > 0 {
		for _, id := range op.ItemIDs {
			if _, ok := s.FindByID(id); !ok {
				return nil, []error{fmt.Errorf("%s：item_ids 中的 %s 不存在", label, id)}
			}
			ids = append(ids, id)
		}
	} else {
		resolved, err := resolveTargetIDs(s, op, label)
		if err != nil {
			return nil, []error{err}
		}
		ids = resolved
	}

	if len(ids) == 0 {
		return nil, []error{fmt.Errorf("%s：没有指定要调整分组的参数项", label)}
	}

	var changes []Change
	for _, id := range ids {
		item, ok := s.FindByID(id)
		if !ok {
			continue
		}
		if item.GroupName == group {
			continue
		}
		before := *item
		item.GroupName = group
		after := *item
		changes = append(changes, Change{Kind: ChangeMove, ItemID: id, Name: item.Name, Before: &before, After: &after})
	}
	return changes, nil
}

// ==================== 定位 ====================

// resolveTargetIDs 把一条 op 的定位信息解析成参数项 ID 列表。
// 优先级：item_id > target 条件 > name
func resolveTargetIDs(s *Snapshot, op Op, label string) ([]string, error) {
	if op.ItemID != "" {
		if _, ok := s.FindByID(op.ItemID); !ok {
			return nil, fmt.Errorf("%s：找不到 item_id=%s 的参数项，请先用 search_items 确认", label, op.ItemID)
		}
		return []string{op.ItemID}, nil
	}

	if !op.Target.IsEmpty() {
		return expandTarget(s, op.Target, label)
	}

	if name := strings.TrimSpace(op.Name); name != "" {
		matches := s.FindByNameAll(name)
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("%s：找不到名为「%s」的参数项，请先用 search_items 确认参数名", label, name)
		case 1:
			return []string{matches[0].ID}, nil
		default:
			groups := make([]string, 0, len(matches))
			for _, m := range matches {
				if m.GroupName == "" {
					groups = append(groups, "未分组")
					continue
				}
				groups = append(groups, m.GroupName)
			}
			return nil, fmt.Errorf("%s：名为「%s」的参数项有 %d 个（分别位于 %s），存在歧义，请改用 item_id 指定",
				label, name, len(matches), strings.Join(groups, "、"))
		}
	}

	return nil, fmt.Errorf("%s：必须提供 item_id、name 或 target 之一来定位参数项", label)
}

// expandTarget 展开条件式目标。
//
// 这是「服务端兜底」的落点：模型说「续航分组下的全部参数」，
// 由这里展开成具体项，而不是让模型逐条枚举。
func expandTarget(s *Snapshot, t *Target, label string) ([]string, error) {
	var ids []string

	switch {
	case t.All:
		for _, it := range s.Items {
			ids = append(ids, it.ID)
		}
	case t.Group != "":
		for _, it := range s.Items {
			if it.GroupName == t.Group {
				ids = append(ids, it.ID)
			}
		}
		if len(ids) == 0 {
			groups := s.Groups()
			available := "（当前没有任何分组）"
			if len(groups) > 0 {
				available = "（现有分组：" + strings.Join(groups, "、") + "）"
			}
			return nil, fmt.Errorf("%s：分组「%s」下没有参数项 %s", label, t.Group, available)
		}
	case len(t.NameIn) > 0:
		for _, n := range t.NameIn {
			matches := s.FindByNameAll(n)
			if len(matches) == 0 {
				return nil, fmt.Errorf("%s：条件中的参数「%s」不存在", label, n)
			}
			for _, m := range matches {
				ids = append(ids, m.ID)
			}
		}
	default:
		return nil, fmt.Errorf("%s：target 条件为空", label)
	}

	if len(ids) > MaxExpandedOps {
		return nil, fmt.Errorf("%s：条件「%s」展开出 %d 条操作，超过单次上限 %d，请缩小范围",
			label, t.Describe(), len(ids), MaxExpandedOps)
	}
	return ids, nil
}

// ==================== 值归一化 ====================

func (p Patch) isEmpty() bool {
	return p.Name == nil && p.Value == nil && p.ValueType == nil &&
		p.EnumOptions == nil && p.GroupName == nil && p.Remark == nil
}

var boolTrueValues = map[string]bool{
	"true": true, "1": true, "是": true, "支持": true, "有": true, "yes": true, "y": true, "√": true,
}

var boolFalseValues = map[string]bool{
	"false": true, "0": true, "否": true, "不支持": true, "无": true, "no": true, "n": true, "×": true,
}

// normalizeValue 按 value_type 归一化值，并把「5000mAh」拆成 value=5000 / unit=mAh
// normalizeValue 按 value_type 归一化参数值。
//
// 单位不再是独立字段，直接写在值里（5000mAh、8小时、198g），所以这里
// 只做校验与布尔/枚举的规范化。
//
// number 类型**只校验不改写**：用户写的「8小时」就原样保留，
// 不该被悄悄改成「8h」—— 参数表是要给客户看的，措辞由用户决定。
func normalizeValue(valueType, rawValue string, enumOptions []string, label string) (string, []error) {
	value := strings.TrimSpace(rawValue)

	if value == "" {
		// 允许先建占位项（模板套用后待填）
		return "", nil
	}

	switch valueType {
	case TypeNumber:
		if _, err := quantity.Parse(value); err != nil {
			return value, []error{fmt.Errorf(
				"%s：value=%q 不是数值（number 类型只接受单个数字，可带单位，例如 5000mAh）；区间或描述性内容请改用 text 类型",
				label, rawValue)}
		}
		return value, nil

	case TypeBool:
		key := strings.ToLower(value)
		switch {
		case boolTrueValues[key]:
			return "支持", nil
		case boolFalseValues[key]:
			return "不支持", nil
		default:
			return value, []error{fmt.Errorf(
				"%s：value=%q 不是布尔值（bool 类型请用「支持」/「不支持」）", label, rawValue)}
		}

	case TypeEnum:
		if len(enumOptions) == 0 {
			return value, []error{fmt.Errorf(
				"%s：value_type=enum 但未提供 enum_options，请补充候选值或改用 text", label)}
		}
		for _, opt := range enumOptions {
			if strings.EqualFold(strings.TrimSpace(opt), value) {
				return strings.TrimSpace(opt), nil
			}
		}
		return value, []error{fmt.Errorf(
			"%s：value=%q 不在候选值 %v 内", label, rawValue, enumOptions)}

	default: // text
		return value, nil
	}
}

func nextSortOrder(s *Snapshot) int {
	max := -1
	for _, it := range s.Items {
		if it.SortOrder > max {
			max = it.SortOrder
		}
	}
	return max + 1
}
