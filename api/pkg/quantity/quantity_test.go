package quantity

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantNum   float64
		wantUnit  string
		wantError bool
	}{
		{name: "数值带单位", raw: "5000mAh", wantNum: 5000, wantUnit: "mAh"},
		{name: "数值与单位间有空格", raw: "45 W", wantNum: 45, wantUnit: "W"},
		{name: "中文单位", raw: "6.7英寸", wantNum: 6.7, wantUnit: "inch"},
		{name: "中文别名归一", raw: "198克", wantNum: 198, wantUnit: "g"},
		{name: "修饰前缀", raw: "约 5000mAh", wantNum: 5000, wantUnit: "mAh"},
		{name: "波浪号前缀", raw: "~120Hz", wantNum: 120, wantUnit: "Hz"},
		{name: "纯数值无单位", raw: "5000", wantNum: 5000, wantUnit: ""},
		{name: "小数", raw: "7.8", wantNum: 7.8, wantUnit: ""},
		{name: "负数", raw: "-20°C", wantNum: -20, wantUnit: "°C"},
		{name: "未知单位原样保留", raw: "12 foo", wantNum: 12, wantUnit: "foo"},
		{name: "千分位逗号", raw: "1,200mAh", wantNum: 1200, wantUnit: "mAh"},

		// 区间不支持，应作为 text 存储
		{name: "区间拒绝", raw: "1.5-2.0kg", wantError: true},
		{name: "区间拒绝（无单位）", raw: "1.5-2.0", wantError: true},
		{name: "描述性文本拒绝", raw: "支持", wantError: true},
		{name: "空值拒绝", raw: "", wantError: true},
		{name: "只有单位拒绝", raw: "mAh", wantError: true},
		{name: "两个数字拒绝", raw: "10 20", wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if tc.wantError {
				if err == nil {
					t.Fatalf("Parse(%q) 期望报错，实际得到 %+v", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) 意外报错: %v", tc.raw, err)
			}
			if got.Number != tc.wantNum {
				t.Errorf("Parse(%q).Number = %v, 期望 %v", tc.raw, got.Number, tc.wantNum)
			}
			if got.Unit != tc.wantUnit {
				t.Errorf("Parse(%q).Unit = %q, 期望 %q", tc.raw, got.Unit, tc.wantUnit)
			}
		})
	}
}

func TestNormalizeUnit(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"mAh", "mAh"},
		{"mah", "mAh"},
		{"毫安时", "mAh"},
		{"MAH", "mAh"},
		{"英寸", "inch"},
		{"吋", "inch"},
		{"w", "W"},
		{"瓦特", "W"},
		{"公斤", "kg"},
		{"吉赫兹", "GHz"},
		// 未知单位不应被强行归一，否则会造成单位错误
		{"whatever", "whatever"},
		{"", ""},
	}

	for _, tc := range cases {
		if got := NormalizeUnit(tc.raw); got != tc.want {
			t.Errorf("NormalizeUnit(%q) = %q, 期望 %q", tc.raw, got, tc.want)
		}
	}
}

func TestSameUnit(t *testing.T) {
	if !SameUnit("mAh", "毫安时") {
		t.Error("SameUnit(mAh, 毫安时) 应为 true")
	}
	if SameUnit("mAh", "Ah") {
		t.Error("SameUnit(mAh, Ah) 应为 false")
	}
}
