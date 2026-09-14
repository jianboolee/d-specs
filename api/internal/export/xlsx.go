// Package export 把参数表渲染成可下载的文件。
//
// 刻意与 spec / share 模块解耦：这里只认「标题 + 分组 + 行」这套中立结构，
// 由调用方（需登录的导出、公开分享页的导出）各自把数据映射进来。
// 这样渲染逻辑只有一份，而两条数据来源的差异不会渗进来。
package export

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// Row 一行参数项。刻意不含 id / value_type：导出是给人看与拿去再编辑的，
// 参数项 id 属内部信息，类型对读者也无意义（单位已经写在值里）。
type Row struct {
	Name   string
	Value  string
	Remark string
}

// Group 一个分组。Name 为空表示「未分组」。
type Group struct {
	Name string
	Rows []Row
}

// Meta 表级信息，渲染在文件头部。
type Meta struct {
	Title       string
	Description string
	Industry    string
	UpdatedAt   time.Time
}

const (
	sheetName = "参数表"
	// 参数表的列：参数名 / 参数值 / 备注
	colCount = 3
)

// Write 把参数表写成 xlsx 并写入 w。
func Write(w io.Writer, meta Meta, groups []Group) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	// 默认工作表改名为「参数表」，并另建一份用不到的 Sheet1 会在保存时报空表错误，
	// 因此直接复用默认表而不是新建
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.SetActiveSheet(index)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}

	row := 1

	// 标题：合并三列，字号加大
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = "参数表"
	}
	if err := f.MergeCell(sheetName, "A1", "C1"); err != nil {
		return err
	}
	titleStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "left", WrapText: true},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellValue(sheetName, "A1", title); err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetName, "A1", "C1", titleStyle); err != nil {
		return err
	}
	row = 2

	// 描述 / 行业 / 更新时间：有多少写多少，不留空行
	if desc := strings.TrimSpace(meta.Description); desc != "" {
		if err := setMergedText(f, row, desc); err != nil {
			return err
		}
		row++
	}
	if metaInfo := metaLine(meta); metaInfo != "" {
		if err := setMergedText(f, row, metaInfo); err != nil {
			return err
		}
		row++
	}
	row++ // 与正文之间空一行

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F2F2F2"}},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "left"},
	})
	if err != nil {
		return err
	}
	groupStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return err
	}
	wrapStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	if err != nil {
		return err
	}

	for _, g := range groups {
		// 分组名：未命名分组不写标题，直接铺行，避免出现空标题
		if name := strings.TrimSpace(g.Name); name != "" {
			if err := f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row)); err != nil {
				return err
			}
			if err := f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), name); err != nil {
				return err
			}
			if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), groupStyle); err != nil {
				return err
			}
			row++
		}

		// 每个分组都自成一张小表，自带表头，便于单独复制走
		for i, h := range []string{"参数名", "参数值", "备注"} {
			cell, err := excelize.CoordinatesToCellName(i+1, row)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheetName, cell, h); err != nil {
				return err
			}
		}
		if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), headerStyle); err != nil {
			return err
		}
		row++

		for _, r := range g.Rows {
			values := []string{r.Name, r.Value, r.Remark}
			for i, v := range values {
				cell, err := excelize.CoordinatesToCellName(i+1, row)
				if err != nil {
					return err
				}
				if err := f.SetCellValue(sheetName, cell, v); err != nil {
					return err
				}
			}
			if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), wrapStyle); err != nil {
				return err
			}
			row++
		}
		row++ // 组间空一行
	}

	// 列宽：参数名与参数值留足，备注窄一些
	if err := f.SetColWidth(sheetName, "A", "A", 24); err != nil {
		return err
	}
	if err := f.SetColWidth(sheetName, "B", "B", 36); err != nil {
		return err
	}
	if err := f.SetColWidth(sheetName, "C", "C", 24); err != nil {
		return err
	}

	return f.Write(w)
}

func setMergedText(f *excelize.File, row int, text string) error {
	if err := f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row)); err != nil {
		return err
	}
	return f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), text)
}

// metaLine 拼出「行业 · 更新于 …」这一行，两者都没有时返回空串。
func metaLine(meta Meta) string {
	var parts []string
	if industry := strings.TrimSpace(meta.Industry); industry != "" {
		parts = append(parts, industry)
	}
	if !meta.UpdatedAt.IsZero() {
		parts = append(parts, "更新于 "+meta.UpdatedAt.In(time.Local).Format("2006年1月2日"))
	}
	return strings.Join(parts, " · ")
}

const (
	// fallbackFilename 表名为空时的兜底文件名
	fallbackFilename = "参数表"
	// maxFilenameRunes 文件名长度上限（不含扩展名），防止超长表名撑爆响应头
	maxFilenameRunes = 60
)

// Filename 由表名生成安全的下载文件名（不含扩展名）。
//
// 去掉路径分隔符与控制字符：表名是用户输入，直接进 Content-Disposition
// 可能构成响应头注入；同时避免导出到本地时被当成路径。
func Filename(title string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == 0:
			return -1
		case unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, title)

	cleaned = strings.TrimSpace(cleaned)
	// Windows 不允许文件名以点或空格结尾
	cleaned = strings.TrimRight(cleaned, ". ")

	if cleaned == "" {
		return fallbackFilename
	}

	runes := []rune(cleaned)
	if len(runes) > maxFilenameRunes {
		cleaned = string(runes[:maxFilenameRunes])
	}
	return cleaned
}
