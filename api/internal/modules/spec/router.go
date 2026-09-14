package spec

import (
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册参数表相关路由。全部需要登录。
//
// 返回 Service 供其它模块复用（对话模块的 AI 变更必须走同一个 service，
// 否则 AI 与手工变更的校验和版本语义会各走一套）。
func Register(rg *gin.RouterGroup, d *deps.Deps) *Service {
	repo := NewRepository(d.DB)
	svc := NewService(repo)
	h := NewHandler(svc)

	authed := rg.Group("")
	authed.Use(middleware.AuthRequired(d.JWT))

	tables := authed.Group("/tables")
	tables.GET("", h.List)
	tables.POST("", h.Create)
	tables.GET("/:id", h.Detail)
	tables.PATCH("/:id", h.Update)
	tables.DELETE("/:id", h.Delete)

	// 换产品改几个值，是最高频的操作
	tables.POST("/:id/duplicate", h.Duplicate)

	// 统一变更入口：手工编辑与 AI 变更共用
	tables.POST("/:id/apply", h.Apply)

	// 版本：撤销/重做由前端基于 restore 实现
	tables.GET("/:id/revisions", h.Revisions)
	tables.POST("/:id/restore", h.Restore)

	// 导出为 xlsx（需登录，只能导出自己的表）
	tables.GET("/:id/export", h.Export)

	return svc
}
