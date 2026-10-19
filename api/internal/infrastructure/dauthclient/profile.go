package dauthclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dauth "github.com/jianboolee/d-auth/packages/gosdk"
)

// ErrProfileUnavailable 表示向 d-auth 查资料暂时失败（网络、5xx、密钥未配）。
// 展示类接口据此降级（昵称/头像为空，只保留 ID）——登录判定本身靠本地验签，
// 不依赖这里。
var ErrProfileUnavailable = errors.New("dauth: profile unavailable")

// Profile 是 d-specs 展示所需的用户资料（来自 d-auth /v1/users/:id，手机号已脱敏）。
// 只有展示字段——授权用的 ID 来自本地验签的 JWT sub，不取这里。
type Profile struct {
	ID        string
	Nickname  string
	AvatarURL string
	Phone     string // 掩码形态，如 138****8000
	IsAdmin   bool
}

// ProfileFetcher 抽象「按用户 ID 取展示资料」，便于测试注入桩；生产用 *ProfileClient。
type ProfileFetcher interface {
	Profile(ctx context.Context, id string) (*Profile, error)
}

// ProfileClient 用 API Key 调 d-auth 的脱敏资料接口（/v1/users/:id，scope users:read）。
//
// 为什么用 API Key 而不是转发 cookie：access token 是纯本地验签的 JWT，读路径
// 不查库；JWT 里没有昵称/头像，展示资料需另取。用 API Key 按 sub 查是服务端标准
// 取法，且不依赖调用方携带 cookie。
type ProfileClient struct {
	client *dauth.Client
}

// NewProfileClient 构造资料客户端。apiKey 为空时返回的实例一律
// 返回 ErrProfileUnavailable（展示降级），不影响登录（授权不依赖它）。
func NewProfileClient(baseURL, apiKey string) *ProfileClient {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil
	}
	return &ProfileClient{client: dauth.New(dauth.Options{
		BaseURL: base,
		APIKey:  strings.TrimSpace(apiKey),
	})}
}

// Profile 按用户 ID 取脱敏资料。d-auth 侧密钥缺失或网络失败时返回 ErrProfileUnavailable。
func (c *ProfileClient) Profile(ctx context.Context, id string) (*Profile, error) {
	if c == nil || c.client == nil || id == "" {
		return nil, ErrProfileUnavailable
	}
	p, err := c.client.UserProfile(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProfileUnavailable, err)
	}
	return &Profile{
		ID:        p.ID,
		Nickname:  p.Nickname,
		AvatarURL: p.AvatarURL,
		Phone:     p.Phone,
		IsAdmin:   p.IsAdmin,
	}, nil
}
