package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/donktech/d-specs/internal/ai/provider"
	"github.com/donktech/d-specs/internal/config"
	"github.com/donktech/d-specs/internal/deps"
	infraauth "github.com/donktech/d-specs/internal/infrastructure/auth"
	"github.com/donktech/d-specs/internal/infrastructure/cache"
	"github.com/donktech/d-specs/internal/infrastructure/database"
	"github.com/donktech/d-specs/internal/infrastructure/logger"
	"github.com/donktech/d-specs/internal/middleware"
	"github.com/donktech/d-specs/internal/router"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type App struct {
	cfg    *config.Config
	log    *slog.Logger
	server *http.Server
	db     *gorm.DB
	redis  *redis.Client
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

	rdb, redisErr := connectRedis(ctx, log, cfg)
	if redisErr != nil && !cfg.IsDevelopment() {
		closeDB(db)
		return nil, redisErr
	}

	llm, llmErr := connectDeepSeek(log, cfg)
	if llmErr != nil && !cfg.IsDevelopment() {
		closeDB(db)
		closeRedis(rdb)
		return nil, llmErr
	}

	logDependencyStatus(log, db != nil, rdb != nil, llm != nil, cfg.IsDevelopment())

	engine := gin.New()
	engine.Use(gin.Logger())
	engine.Use(middleware.RequestID())
	engine.Use(middleware.CORS())
	engine.Use(middleware.Recover(log))

	d := &deps.Deps{
		Config: cfg,
		DB:     db,
		Redis:  rdb,
		JWT:    infraauth.NewJWTManager(cfg.JWTSecret, cfg.JWTExpire),
		LLM:    llm,
		Logger: log,
	}
	router.Setup(engine, d)

	return &App{
		cfg: cfg,
		log: log,
		server: &http.Server{
			Addr:    ":" + cfg.HTTPPort,
			Handler: engine,
		},
		db:    db,
		redis: rdb,
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
	closeRedis(a.redis)

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

func connectRedis(ctx context.Context, log *slog.Logger, cfg *config.Config) (*redis.Client, error) {
	client, err := cache.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		log.Warn("Redis 连接失败",
			"error", err,
			"env", cfg.AppEnv,
			"hint", "请检查 REDIS_URL 是否正确、Redis 是否可达",
		)
		if cfg.IsDevelopment() {
			log.Warn("开发模式：将继续启动，但依赖缓存的会话能力不可用")
			return nil, err
		}
		return nil, fmt.Errorf("redis: %w", err)
	}

	log.Info("Redis 已连接")
	return client, nil
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

func logDependencyStatus(log *slog.Logger, dbOK, redisOK, llmOK, isDev bool) {
	if dbOK && redisOK && llmOK {
		return
	}
	if !dbOK {
		log.Warn("当前运行状态：PostgreSQL 不可用")
	}
	if !redisOK {
		log.Warn("当前运行状态：Redis 不可用")
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

func closeRedis(rdb *redis.Client) {
	if rdb == nil {
		return
	}
	_ = rdb.Close()
}
