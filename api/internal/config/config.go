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

// DAuthConfig d-auth 账户中心接入配置（跨子域 SSO）。
//
// 登录态由 d-auth 下发的共享 cookie 承载（dauth_access = RS256 access JWT），
// 本后端本地验签即认人，不查库、不请求 d-auth：
//   - BaseURL 是账户中心 origin（如 https://auth.x-rv.com），不带 /v1，
//     用于拉 JWKS 公钥与按 ID 取展示资料；
//   - SSOBaseURL 是浏览器登录跳转/登出用的 origin，留空回落 BaseURL；
//   - ClientKey 是子系统应用标识，拼进登录跳转的 client= 参数；
//   - CookieName 是承载 access JWT 的 cookie 名，须与 d-auth 的 DAUTH_COOKIE_NAME 一致；
//   - APIKey 是 sk-... 密钥（scope 含 users:read），只用于按 ID 取脱敏展示资料，
//     缺失时展示降级（昵称/头像为空），不影响登录判定；
//   - JWTIssuer 校验 token 的 iss，留空回落 BaseURL（只是比对字符串，不是密钥）。
type DAuthConfig struct {
	BaseURL    string
	SSOBaseURL string
	ClientKey  string
	CookieName string
	APIKey     string
	JWTIssuer  string
}

type Config struct {
	AppEnv        string
	HTTPPort      string
	PublicURLBase string
	// SiteName 站点名，用于分享页 <title> 后缀、OG site_name 与 JSON-LD
	SiteName string
	// SiteURL 站点规范地址（如 https://specs.x-rv.com），用于分享页 canonical / OG。
	// 留空则按请求的 X-Forwarded-Proto / Host 推导；生产建议显式配置，避免
	// canonical 依赖可被伪造的请求头。
	SiteURL string
	// WebDistDir 前端构建产物目录。分享页 SSR 会读取其中的 index.html
	// 注入 meta 与正文；容器内即 nginx 的站点根目录。
	WebDistDir  string
	DatabaseURL string

	// AutoCreateDB 运行迁移前自动创建 DATABASE_URL 指向的库。
	// 仅用于首次部署（实例存在但库还没建）；默认关闭，避免库名写错时
	// 静默创建一个空库、让应用带着空白数据跑起来。
	AutoCreateDB bool

	// DAuth d-auth 账户中心接入配置。BaseURL 留空 = 不启用（开发环境降级启动，
	// 生产环境直接拒绝启动）。
	DAuth DAuthConfig

	// AllowedOrigins 是 CORS 放行来源白名单（逗号分隔的 origin）。
	// 生产同源部署可留空；留空时仅开发环境回显请求 Origin。
	AllowedOrigins []string

	AI AIConfig
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("HTTP_PORT", "8401")
	viper.SetDefault("PUBLIC_URL_BASE", "")
	viper.SetDefault("SITE_NAME", "参数配置表")
	// 站点规范地址。留空则按请求头推导；生产建议设置，让 canonical/OG 稳定。
	viper.SetDefault("SITE_URL", "")
	// 容器内前端产物挂在 nginx 站点根目录；本地跑后端时可用 WEB_DIST_DIR 指向 web/dist
	viper.SetDefault("WEB_DIST_DIR", "/usr/share/nginx/html")
	viper.SetDefault("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/dspecs?sslmode=disable")
	// 首次部署时设为 true 可自动建库，之后建议改回 false
	viper.SetDefault("AUTO_CREATE_DB", false)

	// d-auth：留空则开发环境降级启动（登录/资料接口报「未启用」），生产拒绝启动。
	// DAUTH_BASE_URL 不要带 /v1（SDK 会自己拼）。API Key 只存后端，绝不下发前端。
	viper.SetDefault("DAUTH_BASE_URL", "")
	viper.SetDefault("DAUTH_SSO_BASE_URL", "")
	viper.SetDefault("DAUTH_CLIENT_KEY", "")
	viper.SetDefault("DAUTH_COOKIE_NAME", "dauth_access")
	viper.SetDefault("DAUTH_API_KEY", "")
	viper.SetDefault("DAUTH_JWT_ISSUER", "")

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
		AppEnv:         strings.TrimSpace(viper.GetString("APP_ENV")),
		HTTPPort:       strings.TrimSpace(viper.GetString("HTTP_PORT")),
		PublicURLBase:  strings.TrimRight(strings.TrimSpace(viper.GetString("PUBLIC_URL_BASE")), "/"),
		SiteName:       strings.TrimSpace(viper.GetString("SITE_NAME")),
		SiteURL:        strings.TrimRight(strings.TrimSpace(viper.GetString("SITE_URL")), "/"),
		WebDistDir:     strings.TrimSpace(viper.GetString("WEB_DIST_DIR")),
		DatabaseURL:    strings.TrimSpace(viper.GetString("DATABASE_URL")),
		AutoCreateDB:   viper.GetBool("AUTO_CREATE_DB"),
		DAuth:          loadDAuthConfig(),
		AllowedOrigins: splitCSV(viper.GetString("ALLOWED_ORIGINS")),
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

// loadDAuthConfig 装配 d-auth 配置并补齐联动默认值：
// SSOBaseURL 留空回落 BaseURL（生产两者同域）；JWTIssuer 留空同样回落 BaseURL，
// 因为 d-auth 的 iss 就是它的对外地址（DAUTH_JWT_ISSUER 缺省取 DAUTH_PUBLIC_URL）。
func loadDAuthConfig() DAuthConfig {
	baseURL := strings.TrimRight(strings.TrimSpace(viper.GetString("DAUTH_BASE_URL")), "/")

	ssoBaseURL := strings.TrimRight(strings.TrimSpace(viper.GetString("DAUTH_SSO_BASE_URL")), "/")
	if ssoBaseURL == "" {
		ssoBaseURL = baseURL
	}

	issuer := strings.TrimSpace(viper.GetString("DAUTH_JWT_ISSUER"))
	if issuer == "" {
		issuer = baseURL
	}

	cookieName := strings.TrimSpace(viper.GetString("DAUTH_COOKIE_NAME"))
	if cookieName == "" {
		cookieName = "dauth_access"
	}

	return DAuthConfig{
		BaseURL:    baseURL,
		SSOBaseURL: ssoBaseURL,
		ClientKey:  strings.TrimSpace(viper.GetString("DAUTH_CLIENT_KEY")),
		CookieName: cookieName,
		APIKey:     strings.TrimSpace(viper.GetString("DAUTH_API_KEY")),
		JWTIssuer:  issuer,
	}
}

// splitCSV 按逗号切分并去掉空项与首尾空白。
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func duration(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
