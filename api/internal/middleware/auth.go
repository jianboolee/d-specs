package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jianboolee/d-specs/internal/contextx"
	"github.com/jianboolee/d-specs/internal/infrastructure/dauthclient"
	"github.com/jianboolee/d-specs/internal/response"

	"github.com/gin-gonic/gin"
	dauth "github.com/jianboolee/d-auth/packages/gosdk"
)

// AuthRequired 强制登录。支持两种凭证，二者都是纯本地验签、零网络：
//
//  1. 浏览器：access JWT cookie（`.x-rv.com` 共享，值就是 access JWT）。
//  2. token 客户端（小程序 / 其它）：Authorization: Bearer <access JWT>。
//
// 两者都是 d-auth 签发的同一枚 RS256 access JWT，验签路径完全一致；
// 验签失败一律 401；公钥取不到（d-auth 暂时不可达）返回 503，
// 避免把用户误判成未登录踢下线。
func AuthRequired(verifier dauthclient.TokenVerifier, cookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := rawCredential(c, cookieName)
		if raw == "" {
			response.Error(c, http.StatusUnauthorized, 401, "请先登录")
			c.Abort()
			return
		}

		claims, err := verifier.VerifyToken(c.Request.Context(), raw)
		if err != nil {
			if errors.Is(err, dauth.ErrKeySourceUnavailable) {
				// d-auth 暂时不可达：返回 503，避免把用户误判成未登录踢下线。
				response.Error(c, http.StatusServiceUnavailable, 503, "认证服务暂不可用，请稍后重试")
				c.Abort()
				return
			}
			// 伪造 / 过期 / 算法降级（含旧 HS256 token）。
			response.Error(c, http.StatusUnauthorized, 401, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		ctx := contextx.WithUserID(c.Request.Context(), claims.Subject)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// VerifyLocal 本地验签当前请求的 access token（Bearer 优先，其次 cookie），
// 不写任何响应。供公开探针（/auth/session）复用中间件的判定逻辑：
// 未登录/无效/公钥取不到都返回 ok=false，调用方据此静默返回 {user:null}。
func VerifyLocal(c *gin.Context, verifier dauthclient.TokenVerifier, cookieName string) (string, bool) {
	raw := rawCredential(c, cookieName)
	if raw == "" {
		return "", false
	}
	claims, err := verifier.VerifyToken(c.Request.Context(), raw)
	if err != nil {
		return "", false
	}
	return claims.Subject, true
}

// defaultCookieName 是承载 access JWT 的 cookie 兜底名（与 d-auth 的
// DAUTH_COOKIE_NAME 一致）。生产用 __Secure- 前缀；配置层已给默认值，
// 这里只防 cookieName 传入空串。
const defaultCookieName = "dauth_access"

// rawCredential 从请求里取 access token：优先 Authorization: Bearer，其次 cookie。
// 命中其一即返回；都没有返回空串。cookie 的值就是 access JWT（d-auth 约定）。
func rawCredential(c *gin.Context, cookieName string) string {
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if raw := strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")); raw != "" {
			return raw
		}
	}
	if cookieName == "" {
		cookieName = defaultCookieName
	}
	if cookie, err := c.Cookie(cookieName); err == nil {
		return strings.TrimSpace(cookie)
	}
	return ""
}
