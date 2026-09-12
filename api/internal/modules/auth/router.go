package auth

import (
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册认证相关路由
func Register(rg *gin.RouterGroup, d *deps.Deps) {
	svc := NewService(d.DB, d.JWT)
	h := NewHandler(svc)

	// 公开
	rg.POST("/auth/register", h.Register)
	rg.POST("/auth/login", h.Login)

	// 需登录
	authed := rg.Group("")
	authed.Use(middleware.AuthRequired(d.JWT))
	authed.GET("/me", h.Me)
}
