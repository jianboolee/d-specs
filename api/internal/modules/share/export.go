package share

import (
	"bytes"
	"net/http"

	"github.com/jianboolee/d-specs/internal/export"

	"github.com/gin-gonic/gin"
)

// PublicExport 公开导出单张参数表为 xlsx，无需登录。
//
// 与其它公开接口共用同一访问边界（share_links.enabled）：分享关闭即 404，
// 不因为「导出」多开了口子。
func (h *Handler) PublicExport(c *gin.Context) {
	res, err := h.resolvePublic(c, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}

	// 先渲染到内存再写响应，渲染失败还能返回 JSON 错误
	var buf bytes.Buffer
	if err := export.Write(&buf, export.Meta{
		Title:       res.Title,
		Description: res.Description,
		Industry:    res.Industry,
		UpdatedAt:   res.UpdatedAt,
	}, publicGroups(res)); err != nil {
		writeError(c, err)
		return
	}

	c.Header("Content-Type", export.ContentTypeXLSX)
	c.Header("Content-Disposition", export.ContentDisposition(res.Title, ".xlsx"))
	// 与分享页 HTML 同款短缓存：关闭分享后一分钟内失效
	c.Header("Cache-Control", "public, max-age=60")
	c.Data(http.StatusOK, export.ContentTypeXLSX, buf.Bytes())
}

// publicGroups 把公开视图的分组映射成导出结构。
//
// groupRows 已把空分组名保留成 Name 为空的组，这里原样透传，
// 渲染时不会写分组标题。
func publicGroups(res *PublicSpec) []export.Group {
	groups := make([]export.Group, 0, len(res.Groups))
	for _, g := range res.Groups {
		rows := make([]export.Row, 0, len(g.Items))
		for _, it := range g.Items {
			rows = append(rows, export.Row{Name: it.Name, Value: it.Value, Remark: it.Remark})
		}
		groups = append(groups, export.Group{Name: g.Name, Rows: rows})
	}
	return groups
}
