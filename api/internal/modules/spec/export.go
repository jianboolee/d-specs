package spec

import (
	"bytes"
	"net/http"

	"github.com/jianboolee/d-specs/internal/export"

	"github.com/gin-gonic/gin"
)

// Export 导出单张参数表为 xlsx。
//
// 用 Detail 取数而不是另写查询：越权校验（user_id）与参数项顺序都复用既有实现。
func (h *Handler) Export(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	detail, err := h.svc.Detail(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}

	// 先渲染到内存再写响应：这样渲染失败还能正常返回 JSON 错误，
	// 而不是把半个二进制文件配上 200 状态码发出去。
	var buf bytes.Buffer
	if err := export.Write(&buf, export.Meta{
		Title:       detail.Table.Title,
		Description: detail.Table.Description,
		Industry:    detail.Table.Industry,
		UpdatedAt:   detail.Table.UpdatedAt,
	}, groupForExport(detail)); err != nil {
		writeError(c, err)
		return
	}

	c.Header("Content-Type", export.ContentTypeXLSX)
	c.Header("Content-Disposition", export.ContentDisposition(detail.Table.Title, ".xlsx"))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, export.ContentTypeXLSX, buf.Bytes())
}

// groupForExport 把平铺的参数项按 group_name 归组。
//
// 按首次出现顺序建组、未命名分组也自成一组的规则，与画布展示和公开导出
// 保持一致：同一张表无论从哪条路径导出，分块顺序都相同。
func groupForExport(detail *DetailResult) []export.Group {
	var groups []export.Group
	index := map[string]int{}

	for _, it := range detail.Items {
		idx, ok := index[it.GroupName]
		if !ok {
			idx = len(groups)
			index[it.GroupName] = idx
			groups = append(groups, export.Group{Name: it.GroupName})
		}
		groups[idx].Rows = append(groups[idx].Rows, export.Row{
			Name:   it.Name,
			Value:  it.Value,
			Remark: it.Remark,
		})
	}
	return groups
}
