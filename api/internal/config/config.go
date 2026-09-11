package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DeepSeekConfig DeepSeek 接入配置（OpenAI 兼容协议）
type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// AIConfig AI 编排相关配置
type AIConfig struct {
	DeepSeek DeepSeekConfig

	// MaxToolRounds 单次对话内 agent loop 的最大工具调用轮数，
	// 同时也是校验失败后回灌模型自我修正的次数上限
	MaxToolRounds int

	// AutoApplyMaxOps 安全阀：单次 ops 数超过该值时不自动应用，转为待确认。
	// 设为 0 表示关闭该安全阀（始终自动应用）
	AutoApplyMaxOps int

	MaxTokens   int
	Temperature float64
}

type Config struct {
	AppEnv        string
	HTTPPort      string
	PublicURLBase string
	DatabaseURL   string
	RedisURL      string
	JWTSecret     string
	JWTExpire     time.Duration
	AI            AIConfig
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("HTTP_PORT", "8300")
	viper.SetDefault("PUBLIC_URL_BASE", "")
	viper.SetDefault("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/dspecs?sslmode=disable")
	viper.SetDefault("REDIS_URL", "redis://localhost:6379/0")
	viper.SetDefault("JWT_SECRET", "change-me-in-production")
	viper.SetDefault("JWT_EXPIRE", "720h")

	viper.SetDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
	// 首版只接 Flash 档位。模型 id 以 GET /models 返回为准，用 `make ai-check` 核对
	viper.SetDefault("DEEPSEEK_MODEL", "deepseek-flash")
	viper.SetDefault("DEEPSEEK_TIMEOUT", "120s")

	viper.SetDefault("AI_MAX_TOOL_ROUNDS", 6)
	viper.SetDefault("AI_AUTO_APPLY_MAX_OPS", 30)
	// Flash 是推理模型，思考与正文共享这个预算。一次 add_items 建 30 项参数
	// 大约要 8000 token，留足余量避免正文被截断成空。
	viper.SetDefault("AI_MAX_TOKENS", 8192)
	viper.SetDefault("AI_TEMPERATURE", 0.2)

	return &Config{
		AppEnv:        strings.TrimSpace(viper.GetString("APP_ENV")),
		HTTPPort:      strings.TrimSpace(viper.GetString("HTTP_PORT")),
		PublicURLBase: strings.TrimRight(strings.TrimSpace(viper.GetString("PUBLIC_URL_BASE")), "/"),
		DatabaseURL:   strings.TrimSpace(viper.GetString("DATABASE_URL")),
		RedisURL:      strings.TrimSpace(viper.GetString("REDIS_URL")),
		JWTSecret:     viper.GetString("JWT_SECRET"),
		JWTExpire:     duration(viper.GetString("JWT_EXPIRE"), 30*24*time.Hour),
		AI: AIConfig{
			DeepSeek: DeepSeekConfig{
				APIKey:  strings.TrimSpace(viper.GetString("DEEPSEEK_API_KEY")),
				BaseURL: strings.TrimRight(strings.TrimSpace(viper.GetString("DEEPSEEK_BASE_URL")), "/"),
				Model:   strings.TrimSpace(viper.GetString("DEEPSEEK_MODEL")),
				Timeout: duration(viper.GetString("DEEPSEEK_TIMEOUT"), 120*time.Second),
			},
			MaxToolRounds:   viper.GetInt("AI_MAX_TOOL_ROUNDS"),
			AutoApplyMaxOps: viper.GetInt("AI_AUTO_APPLY_MAX_OPS"),
			MaxTokens:       viper.GetInt("AI_MAX_TOKENS"),
			Temperature:     viper.GetFloat64("AI_TEMPERATURE"),
		},
	}, nil
}

func (c *Config) IsDevelopment() bool {
	return strings.EqualFold(c.AppEnv, "development")
}

func duration(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
