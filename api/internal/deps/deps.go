package deps

import (
	"log/slog"

	"github.com/jianboolee/d-specs/internal/ai/provider"
	"github.com/jianboolee/d-specs/internal/config"
	infraauth "github.com/jianboolee/d-specs/internal/infrastructure/auth"

	"gorm.io/gorm"
)

type Deps struct {
	Config *config.Config
	DB     *gorm.DB
	JWT    *infraauth.JWTManager
	LLM    provider.Provider
	Logger *slog.Logger
}
