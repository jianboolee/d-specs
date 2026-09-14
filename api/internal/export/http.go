package export

import (
	"fmt"
	"net/url"
	"strings"
)

// ContentTypeXLSX xlsx 的 MIME 类型
const ContentTypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// ContentDisposition 拼出 attachment 响应头。
//
// 中文文件名不能直接放进 filename=（HTTP 头按 latin-1 解读，会变成乱码或被拒），
// 因此按 RFC 5987 用 filename* 传 UTF-8 百分号编码，同时给一个 ASCII 兜底
// filename= 供不认识 filename* 的旧客户端使用。
func ContentDisposition(title, ext string) string {
	name := Filename(title)
	ascii := asciiFallback(name)

	return fmt.Sprintf(
		`attachment; filename="%s%s"; filename*=UTF-8''%s%s`,
		ascii, ext, url.PathEscape(name), url.PathEscape(ext),
	)
}

// asciiFallback 把非 ASCII 字符替换成下划线，得到可安全放进引号的兜底名。
func asciiFallback(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "spec"
	}
	return out
}
