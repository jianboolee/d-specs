package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jianboolee/d-specs/internal/ai/provider"
	"github.com/jianboolee/d-specs/internal/config"
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/infrastructure/database"
	"github.com/jianboolee/d-specs/internal/infrastructure/dauthclient"
	"github.com/jianboolee/d-specs/internal/infrastructure/logger"
	"github.com/jianboolee/d-specs/internal/middleware"
	"github.com/jianboolee/d-specs/internal/router"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type App struct {
	cfg    *config.Config
	log    *slog.Logger
	server *http.Server
	db     *gorm.DB
}

func New(cfg *config.Config) (*App, error) {
	log := logger.New(cfg.AppEnv)

	if !cfg.IsDevelopment() {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 开发模式下允许依赖缺失降级启动，方便先跑通前端与 AI 验证
	db, dbErr := connectPostgres(ctx, log, cfg)
	if dbErr != nil && !cfg.IsDevelopment() {
		return nil, dbErr
	}

	llm, llmErr := connectDeepSeek(log, cfg)
	if llmErr != nil && !cfg.IsDevelopment() {
		closeDB(db)
		return nil, llmErr
	}

	// d-auth 是唯一的登录通道：生产缺配置 fail-fast，开发缺配置降级启动
	dauth, dauthErr := connectDAuth(log, cfg, cfg.IsDevelopment())
	if dauthErr != nil {
		closeDB(db)
		return nil, dauthErr
	}

	logDependencyStatus(log, db != nil, llm != nil, cfg.IsDevelopment())

	engine := gin.New()
	engine.Use(gin.Logger())
	engine.Use(middleware.RequestID())
	engine.Use(middleware.CORS(cfg))
	engine.Use(middleware.Recover(log))

	d := &deps.Deps{
		Config:   cfg,
		DB:       db,
		LLM:      llm,
		Logger:   log,
		Verifier: dauth.Verifier,
		Profiles: dauth.Profiles,
	}
	router.Setup(engine, d)

	return &App{
		cfg: cfg,
		log: log,
		server: &http.Server{
			Addr:    ":" + cfg.HTTPPort,
			Handler: engine,
		},
		db: db,
	}, nil
}

func (a *App) Run() error {
	errCh := make(chan error, 1)

	go func() {
		a.log.Info("server starting", "addr", a.server.Addr, "env", a.cfg.AppEnv)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		a.log.Info("server shutting down", "signal", sig.String())
	}

	shutdownTimeout := 10 * time.Second
	if a.cfg.IsDevelopment() {
		shutdownTimeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	closeDB(a.db)

	return nil
}

func connectPostgres(ctx context.Context, log *slog.Logger, cfg *config.Config) (*gorm.DB, error) {
	db, err := database.NewGormDB(ctx, cfg.DatabaseURL, cfg.IsDevelopment())
	if err != nil {
		log.Warn("PostgreSQL 连接失败",
			"error", err,
			"env", cfg.AppEnv,
			"hint", "请检查 DATABASE_URL 是否正确、数据库是否可达、dspecs 库是否已创建",
		)
		if cfg.IsDevelopment() {
			log.Warn("开发模式：将继续启动，但注册/登录等依赖数据库的功能不可用")
			return nil, err
		}
		return nil, fmt.Errorf("postgres: %w", err)
	}

	log.Info("PostgreSQL 已连接")
	return db, nil
}

func connectDeepSeek(log *slog.Logger, cfg *config.Config) (provider.Provider, error) {
	if cfg.AI.DeepSeek.APIKey == "" {
		log.Info("DeepSeek 未配置（DEEPSEEK_API_KEY 为空），AI 对话功能不可用")
		return nil, nil
	}

	p, err := provider.NewDeepSeek(provider.DeepSeekOptions{
		APIKey:  cfg.AI.DeepSeek.APIKey,
		BaseURL: cfg.AI.DeepSeek.BaseURL,
		Model:   cfg.AI.DeepSeek.Model,
		Timeout: cfg.AI.DeepSeek.Timeout,
	})
	if err != nil {
		log.Warn("DeepSeek 客户端初始化失败",
			"error", err,
			"hint", "请检查 DEEPSEEK_* 配置",
		)
		if cfg.IsDevelopment() {
			return nil, err
		}
		return nil, fmt.Errorf("deepseek: %w", err)
	}

	log.Info("DeepSeek 已就绪", "model", p.Model(), "base_url", cfg.AI.DeepSeek.BaseURL)
	return p, nil
}

// dauthBundle 是 d-auth 接入所需的两件套：
//   - Verifier 本地验签 access token（登录态判定，必须有）；
//   - Profiles 按用户 ID 取脱敏展示资料（缺失时展示降级为空）。
type dauthBundle struct {
	Verifier dauthclient.TokenVerifier
	Profiles dauthclient.ProfileFetcher
}

// connectDAuth 构造 d-auth 接入组件。
//
// 登录态由 d-auth 下发的共享 cookie 承载，本后端本地验签即认人，因此
// 必须的配置是 DAUTH_BASE_URL（拉 JWKS 公钥的 origin）与 DAUTH_CLIENT_KEY
// （前端拼登录跳转用）：
//   - 非生产环境缺配置：返回一个占位 bundle，服务照常启动，
//     登录接口不可用（本地验签会失败），方便离线开发；
//   - 生产环境缺配置直接报错拒绝启动（fail-fast）——带着空配置上线
//     等于全站无法登录，静默半坏比启动失败危险得多。
//
// API Key 只注入后端，绝不下发前端；缺失时展示降级（昵称/头像为空），
// 不影响登录判定与授权。
func connectDAuth(log *slog.Logger, cfg *config.Config, isDev bool) (dauthBundle, error) {
	if cfg.DAuth.BaseURL == "" || cfg.DAuth.ClientKey == "" {
		if isDev {
			log.Warn("d-auth 未配置（DAUTH_BASE_URL / DAUTH_CLIENT_KEY 缺失）：登录不可用",
				"hint", "本地开发请配置账户中心，或仅使用不需要登录的公开页",
			)
			return dauthBundle{Verifier: dauthclient.NewTokenVerifier("", "")}, nil
		}
		return dauthBundle{}, errors.New(
			"d-auth 未配置：DAUTH_BASE_URL 与 DAUTH_CLIENT_KEY 均为必填（用户身份唯一来源，缺失则无法登录）。" +
				"向 d-auth 管理员索取配置后重试",
		)
	}

	verifier := dauthclient.NewTokenVerifier(cfg.DAuth.BaseURL, cfg.DAuth.JWTIssuer)
	profiles := dauthclient.NewProfileClient(cfg.DAuth.BaseURL, cfg.DAuth.APIKey)
	if cfg.DAuth.APIKey == "" {
		log.Warn("DAUTH_API_KEY 未配置：昵称/头像将无法展示（登录不受影响）")
	}

	log.Info("d-auth 已启用",
		"base_url", cfg.DAuth.BaseURL,
		"sso_base_url", cfg.DAuth.SSOBaseURL,
		"client_key", cfg.DAuth.ClientKey,
		"cookie_name", cfg.DAuth.CookieName,
	)
	return dauthBundle{Verifier: verifier, Profiles: profiles}, nil
}

func logDependencyStatus(log *slog.Logger, dbOK, llmOK, isDev bool) {
	if dbOK && llmOK {
		return
	}
	if !dbOK {
		log.Warn("当前运行状态：PostgreSQL 不可用")
	}
	if !llmOK {
		log.Warn("当前运行状态：DeepSeek 不可用（AI 对话功能将不可用）")
	}
	if isDev {
		log.Warn("开发模式已降级启动，请修复上述依赖后重启服务")
	}
}

func closeDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	_ = sqlDB.Close()
}
