package auth

import (
	"net/http"
	"strings"

	"github.com/jianboolee/d-specs/internal/contextx"
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/middleware"
	"github.com/jianboolee/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler 暴露前端所需的认证面。
//
// 登录已交给 d-auth 的中央登录页（SSO）：前端未登录时直接跳
// `{DAUTH_SSO_BASE_URL}/login?client=…&return_to=…`，登录成功后 d-auth
// 下发共享 cookie 并回跳。本系统不代理登录、不签发自己的会话。
//
// 登录态由 middleware.AuthRequired 本地验签 cookie/Bearer 判定
// （不查库、不请求 d-auth）。昵称/头像不在 JWT 里，本处理器用 API Key
// 按用户 ID 向 d-auth 另取（脱敏资料）。
type Handler struct {
	d *deps.Deps
}

func NewHandler(d *deps.Deps) *Handler {
	return &Handler{d: d}
}

// ---------- 登录态探针 ----------

// Session GET /auth/session
// 登录态探针：本地验签 cookie/Bearer 判定是否登录，登录则按 ID 取展示资料。
// 未登录返回 {user:null}（不报 401，适合启动时判断登录态）。
func (h *Handler) Session(c *gin.Context) {
	userID, loggedIn := middleware.VerifyLocal(c, h.d.Verifier, h.d.Config.DAuth.CookieName)
	if !loggedIn {
		response.OK(c, gin.H{"user": nil})
		return
	}
	response.OK(c, gin.H{"user": h.view(c, userID)})
}

// Me GET /me（需登录）
// 登录主体由中间件注入（本地验签）；展示资料按 ID 向 d-auth 另取。
func (h *Handler) Me(c *gin.Context) {
	userID, err := contextx.RequireUserID(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusUnauthorized, 401, "未登录")
		return
	}
	response.OK(c, h.view(c, userID))
}

// Config GET /auth/config
// 公开：把 SSO 登录所需的 d-auth origin 与应用标识下发给前端，前端据此拼
// `{sso_base_url}/login?client=…&return_to=…`。避免前端硬编码 d-auth 地址。
func (h *Handler) Config(c *gin.Context) {
	response.OK(c, gin.H{
		"sso_base_url": h.d.Config.DAuth.SSOBaseURL,
		"client_key":   h.d.Config.DAuth.ClientKey,
	})
}

// view 把登录主体拼成资料视图；昵称/头像是尽力而为的展示字段，
// 取不到时为空串（登录判定与授权不受影响）。
func (h *Handler) view(c *gin.Context, userID string) UserProfile {
	v := UserProfile{ID: userID}
	if h.d.Profiles == nil {
		return v
	}
	p, err := h.d.Profiles.Profile(c.Request.Context(), userID)
	if err != nil || p == nil {
		// 资料暂不可用：降级为只返回 ID，不阻断登录判定。
		return v
	}
	v.Name = p.Nickname
	v.AvatarURL = p.AvatarURL
	v.Phone = strings.TrimSpace(p.Phone)
	return v
}
