.PHONY: dev run build test fmt vet migrate migrate-up migrate-down ai-check air-install \
        http-install http-sync http-test http-test-health http-test-auth \
        http-test-tables http-test-share http-test-chat

AIR ?= air
MIGRATE_ARGS ?= up

# Go 构建缓存固定放在项目内：默认的 ~/Library/Caches/go-build 在部分受限环境下无写权限
GOCACHE ?= $(CURDIR)/.gocache
export GOCACHE

## 开发

dev: ## 开发模式启动（Air 热重载，默认 :8401）
	@command -v $(AIR) >/dev/null 2>&1 || { \
		echo "air not found. Install with: make air-install"; \
		exit 1; \
	}
	$(AIR)

air-install: ## 安装热重载工具
	go install github.com/air-verse/air@latest

run: ## 普通启动（无热重载）
	go run ./cmd/server

build: ## 编译到 bin/server
	go build -o bin/server ./cmd/server

## 质量

test: ## 运行单元测试
	go test ./...

fmt: ## 格式化
	gofmt -w .

vet: ## 静态检查
	go vet ./...

## 数据库

migrate: ## 执行 SQL migration（默认 up）
	@go run ./cmd/migrate $(MIGRATE_ARGS)

migrate-up: ## 全部向上迁移
	@$(MAKE) migrate MIGRATE_ARGS=up

migrate-down: ## 回退最近一次迁移
	@$(MAKE) migrate MIGRATE_ARGS="down 1"

## AI

ai-check: ## 验证 DeepSeek 连通性与 function calling（Phase 0 关键项）
	@go run ./cmd/aicheck

## 接口测试（httpYac）

http-install: ## 安装 httpYac CLI
	npm install --no-save --no-fund --no-audit --cache ./.npm-cache httpyac

http-sync: ## 创建测试账号并把 token 写入 private env
	@bash test/scripts/sync-env.sh

http-test: ## 跑全部接口用例
	@bash test/scripts/run.sh

http-test-health: ## 仅健康检查
	@bash test/scripts/run.sh health

http-test-auth: ## 仅认证模块
	@bash test/scripts/run.sh auth

http-test-tables: ## 仅参数表模块
	@bash test/scripts/run.sh tables

http-test-share: ## 仅分享模块
	@bash test/scripts/run.sh share

http-test-chat: ## 仅对话模块（会真实调用 AI，较慢）
	@bash test/scripts/run.sh chat
