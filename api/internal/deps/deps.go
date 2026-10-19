package deps

import (
	"log/slog"

	"github.com/jianboolee/d-specs/internal/ai/provider"
	"github.com/jianboolee/d-specs/internal/config"
	"github.com/jianboolee/d-specs/internal/infrastructure/dauthclient"

	"gorm.io/gorm"
)

type Deps struct {
	Config *config.Config
	DB     *gorm.DB
	LLM    provider.Provider
	Logger *slog.Logger

	// Verifier 本地验签 d-auth 的 access token（RS256 + JWKS）。登录态的唯一判定依据。
	Verifier dauthclient.TokenVerifier
	// Profiles 用 API Key 按用户 ID 取脱敏展示资料（昵称/头像）；缺失时展示降级。
	Profiles dauthclient.ProfileFetcher
}
