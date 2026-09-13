package baseline

import (
	"reflect"
	"testing"
)

func TestMissing(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		want     []string
	}{
		{name: "全缺", existing: nil, want: []string{"品牌", "型号"}},
		{name: "只有品牌", existing: []string{"品牌"}, want: []string{"型号"}},
		{name: "齐全", existing: []string{"品牌", "型号"}, want: nil},
		{
			name:     "忽略空格与大小写差异",
			existing: []string{" 品牌 ", "型号\u3000"},
			want:     nil,
		},
		{
			name:     "无关参数不影响判断",
			existing: []string{"续航", "电池容量"},
			want:     []string{"品牌", "型号"},
		},
		{
			name:     "缺失顺序遵循 Names",
			existing: []string{"续航", "型号"},
			want:     []string{"品牌"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Missing(tt.existing); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Missing(%v) = %v, want %v", tt.existing, got, tt.want)
			}
		})
	}
}

func TestCheckOrderAndFlag(t *testing.T) {
	got := Check([]string{"型号"})
	want := []Presence{
		{Name: "品牌", Present: false},
		{Name: "型号", Present: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Check() = %+v, want %+v", got, want)
	}
}
