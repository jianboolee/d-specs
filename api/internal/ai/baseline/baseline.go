// Package baseline 定义每张参数表都应有的基础参数，并提供缺失检测。
//
// agent 与 analysis 共用这一份定义：两处若各写一份常量，规则迟早会漂移。
package baseline

import (
	"strings"
	"unicode"
)

// Names 基础参数清单（最小集）。
//
// 只收「谁家的、哪一款」这两个标识类参数：它们对任何产品类型都成立，
// 且缺失时最影响表的可用性。其余必备项交给按产品类型的分析去建议。
var Names = []string{"品牌", "型号"}

// Presence 单个基础参数的存在情况
type Presence struct {
	Name    string
	Present bool
}

// Check 按 Names 的顺序返回每个基础参数是否已存在。
//
// 比较时忽略首尾空白（含全角空格 U+3000）与大小写，
// 以免「品牌 」「型号 」这类录入差异被误判成缺失。
func Check(existing []string) []Presence {
	seen := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		if key := normalize(name); key != "" {
			seen[key] = struct{}{}
		}
	}

	out := make([]Presence, 0, len(Names))
	for _, name := range Names {
		_, ok := seen[normalize(name)]
		out = append(out, Presence{Name: name, Present: ok})
	}
	return out
}

// Missing 返回缺失的基础参数名，顺序与 Names 一致。
func Missing(existing []string) []string {
	var out []string
	for _, p := range Check(existing) {
		if !p.Present {
			out = append(out, p.Name)
		}
	}
	return out
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimFunc(name, unicode.IsSpace))
}
