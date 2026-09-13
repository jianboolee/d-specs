package share

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jianboolee/d-specs/internal/contextx"
	"github.com/jianboolee/d-specs/internal/modules/spec"
	"github.com/jianboolee/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc  *Service
	page *PageRenderer
}

func NewHandler(svc *Service, page *PageRenderer) *Handler {
	return &Handler{svc: svc, page: page}
}

func (h *Handler) Status(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Status(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Enable(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Enable(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Disable(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Disable(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

// PublicSpec 公开只读内容的 JSON 接口，无需登录。
//
// key 既可以是表 id（当前规范地址），也可以是历史分享 token，
// 便于老客户端继续工作。
func (h *Handler) PublicSpec(c *gin.Context) {
	res, err := h.resolvePublic(c, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

// PublicTableList 公开参数表列表 / 搜索，无需登录。
//
// 只返回分享开启中的表；边界与 PublicSpec 完全一致，不因为多一个列表接口就放宽。
func (h *Handler) PublicTableList(c *gin.Context) {
	// 简单搜索不做分页，给个上限防止一次拉全表
	const maxResults = 200

	entries, err := h.svc.SearchPublicTables(c.Request.Context(), c.Query("keyword"), maxResults)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务器内部错误")
		return
	}
	response.OK(c, gin.H{"items": entries})
}

// PublicPage 分享页的 HTML 渲染。
//
// 这是全部公开页面里唯一返给搜索引擎的入口：爬虫（尤其是不执行 JS 的百度）
// 必须能在这里直接读到标题、描述与参数正文，否则收录无从谈起。
func (h *Handler) PublicPage(c *gin.Context) {
	key := c.Param("id")

	res, err := h.svc.PublicSpec(c.Request.Context(), key)
	if errors.Is(err, ErrShareNotFound) {
		// 历史 /s/<token> 永久重定向到规范的 /s/<表id>，
		// 让已经发出去的旧链接继续可用，并把权重归并到新地址
		if tableID, tokenErr := h.svc.TableIDByToken(c.Request.Context(), key); tokenErr == nil {
			c.Redirect(http.StatusMovedPermanently, "/s/"+tableID)
			return
		}
	}
	if err != nil {
		// 关闭分享后必须及时 404，搜索引擎才会把过期页面从索引里摘掉
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", h.page.RenderNotFound(c))
		return
	}

	// 短缓存：既减轻爬虫回访压力，又保证关闭分享后一分钟内生效
	c.Header("Cache-Control", "public, max-age=60")
	c.Data(http.StatusOK, "text/html; charset=utf-8", h.page.Render(c, key, res))
}

// Sitemap 动态列出所有分享开启中的表，便于搜索引擎发现与回访。
func (h *Handler) Sitemap(c *gin.Context) {
	entries, err := h.svc.ListPublicTables(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "sitemap unavailable")
		return
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString("\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	b.WriteString("\n")
	for _, e := range entries {
		b.WriteString("  <url><loc>")
		b.WriteString(xmlEscape(h.page.absURL(c, "/s/"+e.ID)))
		b.WriteString("</loc>")
		if !e.UpdatedAt.IsZero() {
			b.WriteString("<lastmod>" + e.UpdatedAt.Format("2006-01-02") + "</lastmod>")
		}
		b.WriteString("</url>\n")
	}
	b.WriteString("</urlset>\n")

	c.Header("Cache-Control", "public, max-age=600")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(b.String()))
}

// Robots 只放行公开分享页，登录后的应用路由与接口不对外索引。
func (h *Handler) Robots(c *gin.Context) {
	lines := []string{
		"User-agent: *",
		"Allow: /s/",
		"Disallow: /tables",
		"Disallow: /login",
		"Disallow: /api/",
	}
	if origin := h.page.absURL(c, ""); origin != "" {
		lines = append(lines, "", "Sitemap: "+origin+"/sitemap.xml")
	}

	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strings.Join(lines, "\n")+"\n"))
}

// ==================== 辅助 ====================

// resolvePublic 按表 id 取公开内容；取不到时回退到历史 token。
func (h *Handler) resolvePublic(c *gin.Context, key string) (*PublicSpec, error) {
	res, err := h.svc.PublicSpec(c.Request.Context(), key)
	if err == nil || !errors.Is(err, ErrShareNotFound) {
		return res, err
	}

	tableID, tokenErr := h.svc.TableIDByToken(c.Request.Context(), key)
	if tokenErr != nil {
		// 两种解析都失败，按「分享不存在」返回 404
		return nil, err
	}
	return h.svc.PublicSpec(c.Request.Context(), tableID)
}

func requireUser(c *gin.Context) (string, bool) {
	userID, err := contextx.RequireUserID(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusUnauthorized, 401, "请先登录")
		return "", false
	}
	return userID, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrShareNotFound), errors.Is(err, ErrNotShared):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, spec.ErrTableNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, 500, "服务器内部错误")
	}
}

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

func xmlEscape(s string) string { return xmlEscaper.Replace(s) }
