package auth

import (
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册认证相关路由。
//
// 登录只有一种方式，由 d-auth 账户中心完成：前端未登录时整页跳转 d-auth
// 的中央登录页（SSO），登录成功后 d-auth 下发共享 cookie 并回跳。本系统
// 不提供 password-login / sms-login，也不代理登出——refresh token 在
// `dauth_refresh` cookie（Path 限死 /v1/auth）里，后端收不到、无法吊销，
// 登出由前端直接调 d-auth 的 `POST /v1/auth/logout`。
func Register(rg *gin.RouterGroup, d *deps.Deps) {
	h := NewHandler(d)

	// 公开
	rg.GET("/auth/config", h.Config)
	rg.GET("/auth/session", h.Session) // 探针不挂鉴权：未登录返回空，供前端启动判登录态

	// 需登录
	authed := rg.Group("")
	authed.Use(middleware.AuthRequired(d.Verifier, d.Config.DAuth.CookieName))
	authed.GET("/me", h.Me)
}
