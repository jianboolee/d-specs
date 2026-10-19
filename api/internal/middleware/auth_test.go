package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	dauth "github.com/jianboolee/d-auth/packages/gosdk"
)

// stubVerifier 是 TokenVerifier 的桩：按 raw token 决定验签结果，
// 让中间件的 401/503/成功三条分支都可独立验证，无需真实 JWKS。
type stubVerifier struct {
	claims map[string]*dauth.TokenClaims
	errs   map[string]error
}

func (v *stubVerifier) VerifyToken(_ context.Context, raw string) (*dauth.TokenClaims, error) {
	if err, ok := v.errs[raw]; ok {
		return nil, err
	}
	if c, ok := v.claims[raw]; ok {
		return c, nil
	}
	return nil, dauth.ErrInvalidToken
}

const testCookieName = "dauth_access"

func runAuthedRequest(t *testing.T, verifier *stubVerifier, token, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("")
	group.Use(AuthRequired(verifier, testCookieName))
	group.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: testCookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// 验签失败 → 401；公钥取不到（JWKS 抖动）→ 503；成功 → 放行并注入用户 ID。
func TestAuthRequired_Outcomes(t *testing.T) {
	verifier := &stubVerifier{
		claims: map[string]*dauth.TokenClaims{
			"good": {Subject: "user-1"},
		},
		errs: map[string]error{
			"expired":   dauth.ErrInvalidToken,
			"jwks-down": dauth.ErrKeySourceUnavailable,
		},
	}

	rec := runAuthedRequest(t, verifier, "expired", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token: status = %d, want 401", rec.Code)
	}

	rec = runAuthedRequest(t, verifier, "jwks-down", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("jwks unavailable: status = %d, want 503", rec.Code)
	}

	rec = runAuthedRequest(t, verifier, "good", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("valid token: status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

// 缺凭证（既无 Bearer 也无 cookie）→ 401。
func TestAuthRequired_MissingCredential(t *testing.T) {
	rec := runAuthedRequest(t, &stubVerifier{}, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credential: status = %d, want 401", rec.Code)
	}
}

// cookie 的值就是 access JWT：无 Bearer 时也应验签通过。
func TestAuthRequired_CookieCredential(t *testing.T) {
	verifier := &stubVerifier{claims: map[string]*dauth.TokenClaims{
		"cookie-jwt": {Subject: "user-2"},
	}}
	rec := runAuthedRequest(t, verifier, "", "cookie-jwt")
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie token: status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

// VerifyLocal 是公开探针用的无副作用版本：未登录/无效返回 false，不写响应。
func TestVerifyLocal(t *testing.T) {
	verifier := &stubVerifier{
		claims: map[string]*dauth.TokenClaims{"good": {Subject: "user-3"}},
		errs:   map[string]error{"jwks-down": dauth.ErrKeySourceUnavailable},
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/probe", func(c *gin.Context) {
		id, ok := VerifyLocal(c, verifier, testCookieName)
		if !ok {
			c.String(http.StatusOK, "anon")
			return
		}
		c.String(http.StatusOK, id)
	})

	cases := []struct {
		name   string
		header string
		cookie string
		want   string
	}{
		{"no credential", "", "", "anon"},
		{"valid bearer", "Bearer good", "", "user-3"},
		{"invalid token", "Bearer bad", "", "anon"},
		{"jwks down treated as anon", "Bearer jwks-down", "", "anon"},
		{"cookie", "", "good", "user-3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/probe", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: testCookieName, Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if got := rec.Body.String(); got != tc.want {
				t.Fatalf("VerifyLocal = %q, want %q", got, tc.want)
			}
		})
	}
}
