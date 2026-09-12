package deps

import (
	"log/slog"

	"github.com/jianboolee/d-specs/internal/ai/provider"
	"github.com/jianboolee/d-specs/internal/config"
	infraauth "github.com/jianboolee/d-specs/internal/infrastructure/auth"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Deps struct {
	Config *config.Config
	DB     *gorm.DB
	Redis  *redis.Client
	JWT    *infraauth.JWTManager
	LLM    provider.Provider
	Logger *slog.Logger
}
