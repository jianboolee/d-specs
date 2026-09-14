package share

import (
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/middleware"
	"github.com/jianboolee/d-specs/internal/modules/spec"

	"github.com/gin-gonic/gin"
)

// Register 注册分享相关路由。
//
// 管理分享需要登录；读取分享内容不需要 —— 那正是「分享」的意义。
//
// engine 用于挂载根路径下的公开页面（/s/:id 等）。它们不能放在 /api/v1 下：
// 搜索引擎收录的是面向人类的 /s/<id>，而不是带 /api 前缀的接口地址。
//
// 返回 svc 供 chat 模块复用：AI 的开启/关闭分享必须与手工操作走同一套实现，
// 否则两边的分享边界会各走一套。
func Register(engine *gin.Engine, rg *gin.RouterGroup, d *deps.Deps, specSvc *spec.Service) *Service {
	repo := NewRepository(d.DB)
	svc := NewService(repo, specSvc)
	page := NewPageRenderer(d.Config.WebDistDir, d.Config.SiteName, d.Config.SiteURL)
	h := NewHandler(svc, page)

	authed := rg.Group("")
	authed.Use(middleware.AuthRequired(d.JWT))

	authed.GET("/tables/:id/share", h.Status)
	authed.POST("/tables/:id/share", h.Enable)
	authed.DELETE("/tables/:id/share", h.Disable)

	// 公开只读：访问边界由 share_links.enabled 承担。
	// key 兼容表 id（规范）与历史 token。
	rg.GET("/public/specs/:id", h.PublicSpec)
	// 公开列表 / 搜索：与公开分享同一访问边界，供「发现」页使用
	rg.GET("/public/specs", h.PublicTableList)

	// 分享页 HTML：搜索引擎抓取与链接预览的实际入口
	engine.GET("/s/:id", h.PublicPage)
	engine.GET("/sitemap.xml", h.Sitemap)
	engine.GET("/robots.txt", h.Robots)

	return svc
}
