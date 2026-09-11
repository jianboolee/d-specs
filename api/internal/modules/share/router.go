package share

import (
	"github.com/donktech/d-specs/internal/deps"
	"github.com/donktech/d-specs/internal/middleware"
	"github.com/donktech/d-specs/internal/modules/spec"

	"github.com/gin-gonic/gin"
)

// Register 注册分享相关路由。
//
// 管理分享需要登录；读取分享内容不需要 —— 那正是「分享」的意义。
func Register(rg *gin.RouterGroup, d *deps.Deps, specSvc *spec.Service) {
	repo := NewRepository(d.DB)
	svc := NewService(repo, specSvc)
	h := NewHandler(svc)

	authed := rg.Group("")
	authed.Use(middleware.AuthRequired(d.JWT))

	authed.GET("/tables/:id/share", h.Status)
	authed.POST("/tables/:id/share", h.Enable)
	authed.DELETE("/tables/:id/share", h.Disable)

	// 公开只读：安全边界由 token 承担
	rg.GET("/public/specs/:token", h.PublicSpec)
}
