// Package quantity 解析「数值 + 单位」形式的参数值。
//
// 配置表里大量数值带单位（5000mAh、45W、198g、6.7英寸）。AI 返回时单位可能
// 写在值里、也可能单独给出，还可能写成中文别名。这里负责把它们归一化，
// 让 spec_items.value 与 spec_items.unit 始终分开存放。
package quantity

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrEmpty 空值
	ErrEmpty = errors.New("值为空")
	// ErrNotNumeric 无法解析为数值
	ErrNotNumeric = errors.New("无法解析为数值")
)

// Value 解析结果
type Value struct {
	Number float64
	Unit   string // 归一化后的单位；原文未带单位时为空
}

// 允许的前缀修饰词，例如「约 5000mAh」
var trimPrefixes = []string{"约", "大约", "大概", "≈", "~", "～", ">", "≥", "<", "≤"}

// 匹配「数值 + 可选单位」。
//
// 单位首字符不允许是数字、正负号或小数点：否则 "1.5-2.0kg" 这类区间会被
// 误解析成 value=1.5 / unit="-2.0kg"，静默写坏数据。区间应走 text 类型。
var numberPattern = regexp.MustCompile(`^([+-]?\d+(?:\.\d+)?)\s*([^\d\s+\-.].*)?$`)

// 千分位逗号，例如 "1,200"
var thousandsSeparator = regexp.MustCompile(`\d,\d`)

// Parse 解析形如 "5000mAh" / "45 W" / "6.7英寸" / "198g" / "5000" 的字符串。
//
// 不做四则运算，不支持区间（"1.5-2.0kg" 会返回 ErrNotNumeric），
// 区间值应作为 text 存储，不要强行塞进 number 类型。
func Parse(raw string) (Value, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Value{}, ErrEmpty
	}

	for _, p := range trimPrefixes {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
		}
	}

	// 去掉千分位逗号，否则 "1,200mAh" 会被解析成 value=1 / unit=",200mAh"
	for thousandsSeparator.MatchString(s) {
		s = strings.ReplaceAll(s, ",", "")
	}

	m := numberPattern.FindStringSubmatch(s)
	if m == nil {
		return Value{}, ErrNotNumeric
	}

	num, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return Value{}, ErrNotNumeric
	}

	return Value{Number: num, Unit: NormalizeUnit(m[2])}, nil
}

// IsNumeric 判断字符串是否为单个数值（可带单位）
func IsNumeric(raw string) bool {
	_, err := Parse(raw)
	return err == nil
}

// unitAliases 单位别名表。key 为归一化后单位，value 为可接受的写法。
var unitAliases = map[string][]string{
	// 电学
	"mAh": {"mah", "毫安时", "毫安", "ma·h"},
	"Ah":  {"ah", "安时"},
	"Wh":  {"wh", "瓦时"},
	"kWh": {"kwh", "千瓦时", "度"},
	"W":   {"w", "瓦", "瓦特"},
	"kW":  {"kw", "千瓦"},
	"V":   {"v", "伏", "伏特"},
	"mV":  {"mv", "毫伏"},
	"A":   {"a", "安", "安培"},
	"mA":  {"ma", "毫安"},

	// 长度与尺寸
	"mm":   {"mm", "毫米", "公厘"},
	"cm":   {"cm", "厘米", "公分"},
	"m":    {"m", "米"},
	"inch": {"inch", "in", "英寸", "吋", "\""},
	"ft":   {"ft", "英尺", "呎"},

	// 重量
	"mg": {"mg", "毫克"},
	"g":  {"g", "克", "gram", "grams"},
	"kg": {"kg", "千克", "公斤"},
	"t":  {"t", "吨"},
	"lb": {"lb", "lbs", "磅"},

	// 容量与存储
	"MB": {"mb", "兆字节"},
	"GB": {"gb", "吉字节"},
	"TB": {"tb", "太字节"},
	"L":  {"l", "升"},
	"mL": {"ml", "毫升"},

	// 频率与算力
	"Hz":   {"hz", "赫兹"},
	"kHz":  {"khz", "千赫兹"},
	"MHz":  {"mhz", "兆赫兹"},
	"GHz":  {"ghz", "吉赫兹"},
	"TOPS": {"tops", "万亿次每秒"},

	// 显示与影像
	"nits": {"nit", "nits", "尼特", "cd/m2", "cd/m²"},
	"dpi":  {"dpi"},
	"PPI":  {"ppi"},
	"MP":   {"mp", "万像素", "百万像素"},
	"fps":  {"fps", "帧每秒"},

	// 时间
	"s":   {"s", "秒", "sec"},
	"min": {"min", "分钟", "分钟时"},
	"h":   {"h", "小时", "hr", "hrs"},

	// 其它
	"°C":  {"c", "℃", "°c", "摄氏度", "摄氏"},
	"dB":  {"db", "分贝"},
	"Pa":  {"pa", "帕"},
	"kPa": {"kpa", "千帕"},
	"rpm": {"rpm", "转每分"},
}

// normalizedLookup 反查表：小写别名 -> 归一化单位
var normalizedLookup = func() map[string]string {
	m := make(map[string]string, 128)
	for canonical, aliases := range unitAliases {
		m[strings.ToLower(canonical)] = canonical
		for _, alias := range aliases {
			m[strings.ToLower(alias)] = canonical
		}
	}
	return m
}()

// NormalizeUnit 归一化单位别名；无法识别时原样返回（去掉首尾空格）。
//
// 宁可保留未知单位也不要强行归一到错误的单位 —— 配置表里单位错了比不统一更糟。
func NormalizeUnit(unit string) string {
	u := strings.TrimSpace(unit)
	if u == "" {
		return ""
	}
	if canonical, ok := normalizedLookup[strings.ToLower(u)]; ok {
		return canonical
	}
	return u
}

// SameUnit 判断两个单位是否等价
func SameUnit(a, b string) bool {
	return NormalizeUnit(a) == NormalizeUnit(b)
}
