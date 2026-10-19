package middleware

import (
	"net/http"

	"github.com/jianboolee/d-specs/internal/config"

	"github.com/gin-gonic/gin"
)

// CORS 放行跨源请求。
//
// 登录态走 cookie 后，浏览器只有在响应带 `Access-Control-Allow-Credentials: true`
// 时才肯把 cookie 发到跨源接口；而该头与 `Allow-Origin: *` 互斥，必须回显
// 具体 Origin。白名单取配置的 ALLOWED_ORIGINS：
//   - 生产同源部署（前后端同域）本就不产生跨源请求，留空即可；
//   - 本地多端口联调（Vite :8402 → API :8401）时留空，回退为「回显任意来源」，
//     仅开发环境宽松，生产必须显式配置或同源。
func CORS(cfg *config.Config) gin.HandlerFunc {
	allowAll := cfg == nil || cfg.IsDevelopment()
	var allowed []string
	if cfg != nil {
		allowed = cfg.AllowedOrigins
	}

	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" && originAllowed(origin, allowed, allowAll) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// originAllowed 判断请求来源是否可放行：显式配置了白名单就以它为准；
// 未配置时仅在宽松模式（开发）下回显任意来源。
func originAllowed(origin string, allowed []string, allowAll bool) bool {
	if len(allowed) > 0 {
		for _, a := range allowed {
			if a == origin {
				return true
			}
		}
		return false
	}
	return allowAll
}
