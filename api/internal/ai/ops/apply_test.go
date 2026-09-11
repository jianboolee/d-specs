package ops

import (
	"strings"
	"testing"
)

// 测试用配置表：刻意包含「不同分组下同名」的参数项，用于验证歧义处理
func snapshot() Snapshot {
	return Snapshot{
		Title: "示例手机 A1",
		Items: []Item{
			{ID: "a1", Name: "机身颜色", Value: "钛金属", ValueType: TypeText, GroupName: "外观", SortOrder: 0},
			{ID: "a2", Name: "重量", Value: "198g", ValueType: TypeNumber, GroupName: "外观", SortOrder: 1},
			{ID: "a3", Name: "电池容量", Value: "4800mAh", ValueType: TypeNumber, GroupName: "续航", SortOrder: 2},
			{ID: "a4", Name: "充电功率", Value: "30W", ValueType: TypeNumber, GroupName: "续航", SortOrder: 3},
			{ID: "a5", Name: "重量", Value: "1.2kg", ValueType: TypeNumber, GroupName: "包装", SortOrder: 4},
			{ID: "a6", Name: "无线充电", Value: "不支持", ValueType: TypeBool, GroupName: "续航", SortOrder: 5},
			{ID: "a7", Name: "充电接口", Value: "USB-C", ValueType: TypeEnum, EnumOptions: []string{"USB-C", "Lightning"}, GroupName: "续航", SortOrder: 6},
		},
	}
}

func str(s string) *string { return &s }

// ==================== add ====================

func TestAddItem(t *testing.T) {
	t.Run("新增成功且排在末尾", func(t *testing.T) {
		next, changes, err := Apply(snapshot(), []Op{{
			Op:   OpAddItem,
			Item: &Draft{Name: "处理器", Value: "A18", ValueType: TypeText, GroupName: "性能"},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(changes) != 1 || changes[0].Kind != ChangeAdd {
			t.Fatalf("期望 1 条 add 变更，实际 %+v", changes)
		}
		if len(next.Items) != 8 {
			t.Fatalf("期望 8 项，实际 %d", len(next.Items))
		}
		last := next.Items[7]
		if last.Name != "处理器" || last.SortOrder != 7 {
			t.Errorf("新项未正确排到末尾: %+v", last)
		}
	})

	t.Run("同分组重名被拒", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:   OpAddItem,
			Item: &Draft{Name: "重量", Value: "200", GroupName: "外观"},
		}})
		if err == nil {
			t.Fatal("期望重名报错，实际通过")
		}
		if !strings.Contains(err.Error(), "已存在参数") {
			t.Errorf("错误信息不够明确: %v", err)
		}
	})

	t.Run("不同分组允许重名", func(t *testing.T) {
		// 「重量」在 外观 与 包装 下各有一条，属于合理业务场景
		_, _, err := Apply(snapshot(), []Op{{
			Op:   OpAddItem,
			Item: &Draft{Name: "重量", Value: "2.0kg", ValueType: TypeNumber, GroupName: "毛重"},
		}})
		if err != nil {
			t.Fatalf("跨分组重名不应报错: %v", err)
		}
	})

	t.Run("参数名为空被拒", func(t *testing.T) {
		if _, _, err := Apply(snapshot(), []Op{{Op: OpAddItem, Item: &Draft{Name: "  "}}}); err == nil {
			t.Fatal("期望空名称报错")
		}
	})

	t.Run("非法 value_type 被拒", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:   OpAddItem,
			Item: &Draft{Name: "新参数", ValueType: "color"},
		}})
		if err == nil || !strings.Contains(err.Error(), "不合法") {
			t.Fatalf("期望 value_type 报错，实际: %v", err)
		}
	})

	t.Run("批量新增", func(t *testing.T) {
		next, changes, err := Apply(snapshot(), []Op{{
			Op: OpAddItems,
			Items: []Draft{
				{Name: "蓝牙版本", Value: "5.3", ValueType: TypeText, GroupName: "连接"},
				{Name: "NFC", Value: "支持", ValueType: TypeBool, GroupName: "连接"},
				{Name: "扬声器数量", Value: "2个", ValueType: TypeNumber, GroupName: "连接"},
			},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(changes) != 3 || len(next.Items) != 10 {
			t.Fatalf("期望新增 3 项共 10 项，实际 changes=%d items=%d", len(changes), len(next.Items))
		}
	})
}

// ==================== 数值 + 单位 ====================

func TestNumberValueNormalization(t *testing.T) {
	t.Run("值与单位是一个整体，不该被拆开", func(t *testing.T) {
		next, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a3",
			Patch:  &Patch{Value: str("5000mAh")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		item, _ := next.FindByID("a3")
		if item.Value != "5000mAh" {
			t.Fatalf("期望 value=5000mAh，实际 value=%q", item.Value)
		}
	})

	t.Run("带单位的数值原样保留，不做单位归一化", func(t *testing.T) {
		// 配置表是给客户看的，用户写的「毫安时」不该被悄悄改成「mAh」
		next, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a3",
			Patch:  &Patch{Value: str("5000 毫安时")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		item, _ := next.FindByID("a3")
		if item.Value != "5000 毫安时" {
			t.Fatalf("期望原文保留，实际 %q", item.Value)
		}
	})

	t.Run("中文数值单位混写也能通过校验", func(t *testing.T) {
		next, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a3",
			Patch:  &Patch{Value: str("8小时")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		item, _ := next.FindByID("a3")
		if item.Value != "8小时" {
			t.Fatalf("期望 8小时，实际 %q", item.Value)
		}
	})

	t.Run("无法解析为数值时被拒并提示改用 text", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a3",
			Patch:  &Patch{Value: str("1.5-2.0kg")},
		}})
		if err == nil || !strings.Contains(err.Error(), "text") {
			t.Fatalf("期望报错并提示改用 text，实际: %v", err)
		}
	})
}

func TestBoolAndEnum(t *testing.T) {
	t.Run("布尔值归一化", func(t *testing.T) {
		for _, raw := range []string{"true", "有", "yes", "1"} {
			next, _, err := Apply(snapshot(), []Op{{
				Op:     OpUpdateItem,
				ItemID: "a6",
				Patch:  &Patch{Value: str(raw)},
			}})
			if err != nil {
				t.Fatalf("值 %q 意外报错: %v", raw, err)
			}
			item, _ := next.FindByID("a6")
			if item.Value != "支持" {
				t.Errorf("值 %q 期望归一为「支持」，实际 %q", raw, item.Value)
			}
		}
	})

	t.Run("非布尔值被拒", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a6",
			Patch:  &Patch{Value: str("部分支持")},
		}})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("枚举值大小写不敏感并归一为候选值", func(t *testing.T) {
		next, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a7",
			Patch:  &Patch{Value: str("usb-c")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		item, _ := next.FindByID("a7")
		if item.Value != "USB-C" {
			t.Fatalf("期望归一为 USB-C，实际 %q", item.Value)
		}
	})

	t.Run("枚举取值不在候选中被拒", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a7",
			Patch:  &Patch{Value: str("Micro-USB")},
		}})
		if err == nil || !strings.Contains(err.Error(), "候选值") {
			t.Fatalf("期望候选值报错，实际: %v", err)
		}
	})
}

// ==================== 定位 ====================

func TestTargeting(t *testing.T) {
	t.Run("按 item_id 定位", func(t *testing.T) {
		next, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a3",
			Patch:  &Patch{Value: str("5000")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		item, _ := next.FindByID("a3")
		if item.Value != "5000" {
			t.Fatalf("期望 5000，实际 %q", item.Value)
		}
	})

	t.Run("按 name 定位，重名时报歧义", func(t *testing.T) {
		// 「重量」在 外观 和 包装 下各有一条
		_, _, err := Apply(snapshot(), []Op{{
			Op:    OpUpdateItem,
			Name:  "重量",
			Patch: &Patch{Value: str("200")},
		}})
		if err == nil {
			t.Fatal("期望歧义报错")
		}
		if !strings.Contains(err.Error(), "歧义") || !strings.Contains(err.Error(), "item_id") {
			t.Errorf("歧义错误应提示改用 item_id: %v", err)
		}
	})

	t.Run("按 name 定位唯一项成功", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:    OpUpdateItem,
			Name:  "电池容量",
			Patch: &Patch{Value: str("5000")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
	})

	t.Run("名称不存在时提示先查询", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:    OpUpdateItem,
			Name:  "电池毫安数",
			Patch: &Patch{Value: str("5000")},
		}})
		if err == nil || !strings.Contains(err.Error(), "search_items") {
			t.Fatalf("错误信息应引导模型先查询: %v", err)
		}
	})

	t.Run("缺少任何定位信息被拒", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{Op: OpUpdateItem, Patch: &Patch{Value: str("1")}}})
		if err == nil || !strings.Contains(err.Error(), "item_id") {
			t.Fatalf("期望定位报错，实际: %v", err)
		}
	})
}

// ==================== 条件式展开 ====================

func TestTargetExpansion(t *testing.T) {
	t.Run("按分组展开", func(t *testing.T) {
		// 续航组有 a3 电池容量、a4 充电功率、a6 无线充电、a7 充电接口
		next, changes, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			Target: &Target{Group: "续航"},
			Patch:  &Patch{GroupName: str("电池与充电")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(changes) != 4 {
			t.Fatalf("期望展开出 4 条变更，实际 %d", len(changes))
		}
		for _, id := range []string{"a3", "a4", "a6", "a7"} {
			item, _ := next.FindByID(id)
			if item.GroupName != "电池与充电" {
				t.Errorf("参数项 %s 分组未更新: %q", id, item.GroupName)
			}
		}
		// 其他分组不受影响
		other, _ := next.FindByID("a2")
		if other.GroupName != "外观" {
			t.Errorf("外观组不应被改动，实际 %q", other.GroupName)
		}
	})

	t.Run("分组不存在时列出可用分组", func(t *testing.T) {
		_, _, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			Target: &Target{Group: "续航能力"},
			Patch:  &Patch{Value: str("1")},
		}})
		if err == nil {
			t.Fatal("期望报错")
		}
		if !strings.Contains(err.Error(), "续航") {
			t.Errorf("错误信息应列出可用分组: %v", err)
		}
	})

	t.Run("全部展开", func(t *testing.T) {
		_, changes, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			Target: &Target{All: true},
			Patch:  &Patch{Remark: str("已核对")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(changes) != 7 {
			t.Fatalf("期望 7 条变更，实际 %d", len(changes))
		}
	})

	t.Run("按名称集合展开", func(t *testing.T) {
		_, changes, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			Target: &Target{NameIn: []string{"电池容量", "充电功率"}},
			Patch:  &Patch{Remark: str("续航核心")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(changes) != 2 {
			t.Fatalf("期望 2 条变更，实际 %d", len(changes))
		}
	})

	t.Run("item_id 优先于 target", func(t *testing.T) {
		_, changes, err := Apply(snapshot(), []Op{{
			Op:     OpUpdateItem,
			ItemID: "a3",
			Target: &Target{All: true},
			Patch:  &Patch{Value: str("5000")},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(changes) != 1 || changes[0].ItemID != "a3" {
			t.Fatalf("期望只改 a3，实际 %+v", changes)
		}
	})
}

// ==================== delete / reorder / set_group ====================

func TestDelete(t *testing.T) {
	t.Run("按 id 删除", func(t *testing.T) {
		next, changes, err := Apply(snapshot(), []Op{{Op: OpDeleteItem, ItemID: "a6"}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(next.Items) != 6 {
			t.Fatalf("期望剩 6 项，实际 %d", len(next.Items))
		}
		if _, ok := next.FindByID("a6"); ok {
			t.Error("a6 应已被删除")
		}
		if changes[0].Kind != ChangeDelete || changes[0].Before == nil {
			t.Errorf("删除变更应保留删除前的值用于撤销: %+v", changes[0])
		}
	})

	t.Run("按条件批量删除", func(t *testing.T) {
		next, _, err := Apply(snapshot(), []Op{{
			Op:     OpDeleteItem,
			Target: &Target{Group: "续航"},
		}})
		if err != nil {
			t.Fatalf("意外报错: %v", err)
		}
		if len(next.Items) != 3 {
			t.Fatalf("期望剩 3 项，实际 %d", len(next.Items))
		}
	})
}

func TestReorder(t *testing.T) {
	next, changes, err := Apply(snapshot(), []Op{{
		Op:    OpReorderItems,
		Order: []string{"a6", "a3"},
	}})
	if err != nil {
		t.Fatalf("意外报错: %v", err)
	}
	if next.Items[0].ID != "a6" || next.Items[1].ID != "a3" {
		t.Fatalf("期望 a6, a3 排在最前，实际 %s, %s", next.Items[0].ID, next.Items[1].ID)
	}
	// 未列出的项保持原有相对顺序
	if next.Items[2].ID != "a1" || next.Items[3].ID != "a2" {
		t.Errorf("未列出的项相对顺序被破坏: %s, %s", next.Items[2].ID, next.Items[3].ID)
	}
	// SortOrder 应被重排为连续序号
	for i, item := range next.Items {
		if item.SortOrder != i {
			t.Errorf("第 %d 项 SortOrder = %d，期望 %d", i, item.SortOrder, i)
		}
	}
	if len(changes) != 2 {
		t.Errorf("期望 2 条 move 变更，实际 %d", len(changes))
	}
}

func TestReorderRejectsUnknownID(t *testing.T) {
	if _, _, err := Apply(snapshot(), []Op{{Op: OpReorderItems, Order: []string{"nope"}}}); err == nil {
		t.Fatal("期望未知 id 报错")
	}
}

func TestSetGroup(t *testing.T) {
	next, _, err := Apply(snapshot(), []Op{{
		Op:        OpSetGroup,
		ItemIDs:   []string{"a1"},
		GroupName: "整机外观",
	}})
	if err != nil {
		t.Fatalf("意外报错: %v", err)
	}
	item, _ := next.FindByID("a1")
	if item.GroupName != "整机外观" {
		t.Fatalf("分组未更新: %q", item.GroupName)
	}
}

// ==================== 原子性与错误收集 ====================

func TestAtomicity(t *testing.T) {
	original := snapshot()

	// 第一条合法、第二条非法 —— 整批都不应生效
	next, changes, err := Apply(original, []Op{
		{Op: OpUpdateItem, ItemID: "a3", Patch: &Patch{Value: str("5000")}},
		{Op: OpUpdateItem, ItemID: "does-not-exist", Patch: &Patch{Value: str("1")}},
	})
	if err == nil {
		t.Fatal("期望报错")
	}
	if changes != nil {
		t.Errorf("校验失败时不应返回变更，实际 %+v", changes)
	}
	if next.Items[2].Value != "4800mAh" {
		t.Errorf("校验失败时不应改动任何数据，a3 变成 %q", next.Items[2].Value)
	}
}

func TestValidateIsDryRun(t *testing.T) {
	original := snapshot()
	if errs := Validate(original, []Op{{Op: OpUpdateItem, ItemID: "a3", Patch: &Patch{Value: str("5000")}}}); len(errs) != 0 {
		t.Fatalf("期望校验通过，实际 %v", errs)
	}
	if original.Items[2].Value != "4800mAh" {
		t.Error("Validate 不应改动输入快照")
	}
}

func TestCollectsAllErrors(t *testing.T) {
	// 三条都错，应一次性全部返回，而不是只报第一条
	_, _, err := Apply(snapshot(), []Op{
		{Op: OpUpdateItem, ItemID: "nope1", Patch: &Patch{Value: str("1")}},
		{Op: OpUpdateItem, ItemID: "nope2", Patch: &Patch{Value: str("1")}},
		{Op: "unknown_op"},
	})
	if err == nil {
		t.Fatal("期望报错")
	}
	ve, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("期望 ValidationErrors，实际 %T", err)
	}
	if len(ve) != 3 {
		t.Fatalf("期望收集到 3 条错误，实际 %d: %v", len(ve), ve)
	}
	if !strings.Contains(err.Error(), "未知操作类型") {
		t.Errorf("应包含未知操作类型的提示: %v", err)
	}
}

func TestInputGuards(t *testing.T) {
	t.Run("空 ops 被拒", func(t *testing.T) {
		if _, _, err := Apply(snapshot(), nil); err == nil {
			t.Fatal("期望空 ops 报错")
		}
	})

	t.Run("超过单次条数上限被拒", func(t *testing.T) {
		opsList := make([]Op, MaxOpsPerRequest+1)
		for i := range opsList {
			opsList[i] = Op{Op: OpUpdateItem, ItemID: "a3", Patch: &Patch{Value: str("1")}}
		}
		if _, _, err := Apply(snapshot(), opsList); err == nil {
			t.Fatal("期望超出上限报错")
		}
	})

	t.Run("空 patch 被拒", func(t *testing.T) {
		if _, _, err := Apply(snapshot(), []Op{{Op: OpUpdateItem, ItemID: "a3", Patch: &Patch{}}}); err == nil {
			t.Fatal("期望空 patch 报错")
		}
	})

	t.Run("add_item 缺少 item 被拒", func(t *testing.T) {
		if _, _, err := Apply(snapshot(), []Op{{Op: OpAddItem}}); err == nil {
			t.Fatal("期望缺少 item 报错")
		}
	})

	t.Run("add_items 空数组被拒", func(t *testing.T) {
		if _, _, err := Apply(snapshot(), []Op{{Op: OpAddItems}}); err == nil {
			t.Fatal("期望空 items 报错")
		}
	})
}

func TestRenameCollision(t *testing.T) {
	// 把 a1（机身颜色，外观组）改名为「重量」，会与同组的 a2 冲突
	_, _, err := Apply(snapshot(), []Op{{
		Op:     OpUpdateItem,
		ItemID: "a1",
		Patch:  &Patch{Name: str("重量")},
	}})
	if err == nil || !strings.Contains(err.Error(), "重名") {
		t.Fatalf("期望重名报错，实际: %v", err)
	}
}

func TestSnapshotHelpers(t *testing.T) {
	s := snapshot()

	t.Run("FindByName 忽略空格与大小写", func(t *testing.T) {
		if _, ok := s.FindByName(" 电池容量 "); !ok {
			t.Error("应忽略首尾空格")
		}
	})

	t.Run("FindByNameAll 返回全部同名项", func(t *testing.T) {
		if got := s.FindByNameAll("重量"); len(got) != 2 {
			t.Errorf("期望 2 个「重量」，实际 %d", len(got))
		}
	})

	t.Run("Groups 去重且保持出现顺序", func(t *testing.T) {
		groups := s.Groups()
		want := []string{"外观", "续航", "包装"}
		if len(groups) != len(want) {
			t.Fatalf("期望 %v，实际 %v", want, groups)
		}
		for i := range want {
			if groups[i] != want[i] {
				t.Fatalf("期望 %v，实际 %v", want, groups)
			}
		}
	})

	t.Run("Clone 是深拷贝", func(t *testing.T) {
		c := s.Clone()
		c.Items[0].Name = "改过了"
		if s.Items[0].Name == "改过了" {
			t.Error("Clone 不应共享底层数组")
		}
		c.Items[6].EnumOptions[0] = "改过了"
		if s.Items[6].EnumOptions[0] == "改过了" {
			t.Error("Clone 应深拷贝 enum_options")
		}
	})
}
