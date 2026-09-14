package export

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func sampleMeta() Meta {
	return Meta{
		Title:       "野营房车 A1",
		Description: "2026 款房车参数",
		Industry:    "房车",
		UpdatedAt:   time.Date(2026, 9, 14, 10, 0, 0, 0, time.Local),
	}
}

func sampleGroups() []Group {
	return []Group{
		{Name: "电力", Rows: []Row{
			{Name: "电池容量", Value: "5000mAh", Remark: "标称值"},
			{Name: "充电功率", Value: "45W"},
		}},
		{Name: "", Rows: []Row{
			{Name: "品牌", Value: "小野驴"},
		}},
	}
}

// 打开生成的 xlsx，返回按行拼好的单元格文本，便于断言顺序与内容。
func readSheet(t *testing.T, data []byte) (string, []string) {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("生成的文件无法打开: %v", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) != 1 || sheets[0] != sheetName {
		t.Fatalf("应只有一个名为 %q 的工作表，实际 %v", sheetName, sheets)
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		t.Fatalf("读取行失败: %v", err)
	}
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, strings.Join(r, "|"))
	}
	return sheets[0], lines
}

func TestWriteXLSX(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sampleMeta(), sampleGroups()); err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("生成内容为空")
	}

	_, lines := readSheet(t, buf.Bytes())
	joined := strings.Join(lines, "\n")

	for _, want := range []string{
		"野营房车 A1",
		"2026 款房车参数",
		"房车 · 更新于 2026年9月14日",
		"电力",
		"参数名|参数值|备注",
		"电池容量|5000mAh|标称值",
		"充电功率|45W",
		"品牌|小野驴",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("xlsx 缺少 %q\n---\n%s", want, joined)
		}
	}
}

// 未命名分组不写分组标题，但仍要有自己的表头与内容
func TestWriteXLSXUngrouped(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Meta{Title: "无分组表"}, []Group{
		{Name: "", Rows: []Row{{Name: "品牌", Value: "小野驴"}}},
	}); err != nil {
		t.Fatalf("生成失败: %v", err)
	}

	_, lines := readSheet(t, buf.Bytes())
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "品牌|小野驴") {
		t.Fatalf("未分组内容缺失\n---\n%s", joined)
	}
	// 表头出现即说明铺开了行；不能出现空的分组标题行
	for _, line := range lines {
		if line == "|" || line == "||" {
			t.Errorf("出现了空行: %q\n---\n%s", line, joined)
		}
	}
}

func TestWriteXLSXEmptyTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Meta{}, nil); err != nil {
		t.Fatalf("空表也应能生成: %v", err)
	}
	_, lines := readSheet(t, buf.Bytes())
	if len(lines) == 0 {
		t.Fatal("空表也应至少有标题行")
	}
	if lines[0] != fallbackFilename {
		t.Fatalf("表名为空时应回退为 %q，实际 %q", fallbackFilename, lines[0])
	}
}

func TestFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"普通中文", "野营房车 A1", "野营房车 A1"},
		{"去掉路径分隔符", "a/b\\c", "abc"},
		{"去掉控制字符", "a\x00b\x1fc", "abc"},
		{"去掉首尾空白", "  参数表  ", "参数表"},
		{"去掉结尾的点", "参数表...", "参数表"},
		{"全为空时兜底", "   ", fallbackFilename},
		{"空串兜底", "", fallbackFilename},
		{"超长截断", strings.Repeat("长", maxFilenameRunes+20), strings.Repeat("长", maxFilenameRunes)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filename(tt.input); got != tt.want {
				t.Errorf("Filename(%q) = %q，期望 %q", tt.input, got, tt.want)
			}
		})
	}
}

// 取出 Content-Disposition 里 ASCII 兜底的 filename= 值
func asciiFilename(t *testing.T, header string) string {
	t.Helper()
	const marker = `filename="`
	start := strings.Index(header, marker)
	if start < 0 {
		t.Fatalf("缺少 ASCII 兜底 filename，实际 %q", header)
	}
	rest := header[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("filename 的引号未闭合，实际 %q", header)
	}
	return rest[:end]
}

func TestContentDisposition(t *testing.T) {
	got := ContentDisposition("野营房车 A1", ".xlsx")

	if !strings.HasPrefix(got, "attachment;") {
		t.Errorf("应为 attachment，实际 %q", got)
	}
	// 中文必须走 filename*（百分号编码），不能出现在 ASCII 兜底里
	if !strings.Contains(got, "filename*=UTF-8''") {
		t.Errorf("缺少 RFC 5987 的 filename*，实际 %q", got)
	}
	if !strings.Contains(got, "%E9%87%8E") {
		t.Errorf("中文未做百分号编码，实际 %q", got)
	}

	// 兜底名必须是纯 ASCII，否则部分客户端会截断
	fallback := asciiFilename(t, got)
	for _, r := range fallback {
		if r > 0x7e {
			t.Errorf("兜底文件名含非 ASCII 字符 %q: %q", r, fallback)
		}
	}
}

// 表名里的引号不能逃逸出 filename= 的引号
func TestContentDispositionQuotesSanitized(t *testing.T) {
	fallback := asciiFilename(t, ContentDisposition(`恶意"name`, ".xlsx"))
	if strings.Contains(fallback, `"`) {
		t.Fatalf("引号未清理，可能逃逸响应头: %q", fallback)
	}
}
