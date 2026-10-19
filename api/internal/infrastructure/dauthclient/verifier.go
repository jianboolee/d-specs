// Package dauthclient 是对 d-auth 账户中心 Go SDK 的项目内封装。
//
// 接入形态是跨子域 SSO：登录态由 d-auth 下发的共享 cookie 承载
// （dauth_access = RS256 access JWT），本后端本地验签即认人，不查库、
// 不请求 d-auth。JWT 里只有 sub / roles，没有昵称/头像——展示资料另用
// API Key 按 ID 向 d-auth 的脱敏接口取（见 profile.go）。
package dauthclient

import (
	"context"

	dauth "github.com/jianboolee/d-auth/packages/gosdk"
)

// TokenVerifier 校验 d-auth 的 access token（RS256 + JWKS）。
//
// 抽成接口只为便于测试注入桩实现；生产用 NewTokenVerifier 返回的
// *dauth.Verifier —— 它并发安全，进程内建一个全局实例复用即可。
type TokenVerifier interface {
	VerifyToken(ctx context.Context, raw string) (*dauth.TokenClaims, error)
}

// NewTokenVerifier 构造 d-auth 验签器。
//
// baseURL 是 d-auth origin（不带 /v1）；issuer 非空时校验 token 的 iss。
// 公钥从 baseURL + /v1/.well-known/jwks.json 自动拉取并按 kid 缓存（默认 5 分钟节流），
// d-auth 轮换密钥后按 kid 自动重拉 —— 不需要、也不要自己造 JWKS 缓存。
func NewTokenVerifier(baseURL, issuer string) *dauth.Verifier {
	return dauth.NewVerifier(dauth.VerifierOptions{
		BaseURL: baseURL,
		Issuer:  issuer,
	})
}
