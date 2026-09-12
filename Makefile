.PHONY: help api-dev api-run api-build migrate migrate-down ai-check api-test web-dev web-build web-install install \
        docker-build docker-release docker-run nginx-push \
        docker-dev docker-dev-down docker-dev-logs docker-dev-build

## 帮助

help:
	@echo "d-specs 常用命令"
	@echo ""
	@echo "  make install        安装前后端依赖"
	@echo "  make migrate        执行数据库迁移"
	@echo "  make api-dev        启动后端 :$(API_PORT)（热重载）"
	@echo "  make web-dev        启动前端 :$(WEB_PORT)"
	@echo "  make ai-check       验证 DeepSeek 连通性与 function calling"
	@echo "  make build          构建前后端产物"
	@echo "  make api-test       运行后端测试"
	@echo "  make docker-dev     本地开发栈（热重载，不构建发布镜像）:$(WEB_PORT)"
	@echo "  make docker-dev-down 停止本地开发栈"
	@echo "  make docker-build   构建镜像（仅本机，不推送）"
	@echo "  make docker-release 构建并推送 $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@echo "  make docker-run     用最新镜像本地起一个容器 :$(APP_PORT)"
	@echo "  make nginx-push     推送 deploy/nginx/*.conf 到服务器并 reload nginx"
	@echo ""

# ---------- 端口约定 ----------
# 本项目独占 8400-8402，避免与本机其它项目（Vite 默认 5173、常见 8080 等）冲突。
#   8400  生产对外端口（宿主机 -> 容器 80）
#   8401  API 端口（dev 宿主机；生产时仅在容器内，不对外发布）
#   8402  Web dev 端口（vite）
# 需要临时错开时用 make ... API_PORT=... WEB_PORT=... APP_PORT=... 覆盖。
API_PORT ?= 8401
WEB_PORT ?= 8402
# 本地跑发布镜像时映射的宿主机端口，默认与生产对齐
APP_PORT ?= 8400

# ---------- Docker 发布变量 ----------
# 镜像仓库与 tag
DOCKER_IMAGE ?= jianboo/d-specs
# 默认发布 latest；需要留版本号时显式覆盖：make docker-release DOCKER_TAG=v1.0.0
DOCKER_TAG ?= latest
# 目标架构。常见云服务器是 amd64；需要多架构时用逗号分隔，如 linux/amd64,linux/arm64
DOCKER_PLATFORMS ?= linux/amd64
# 本地开发栈的 compose 文件
DOCKER_DEV_FILE ?= docker-compose.dev.yml
# make docker-run 用的容器环境文件。
# 它与 api/.env 是两回事，不要整份复用：api/.env 是「后端直接跑在宿主机」的配置
# （含 APP_ENV=development、WEB_DIST_DIR=../web），这些在容器里会覆盖镜像的正确
# 默认值，导致镜像跑成开发模式、且分享页 SSR 读不到入口 HTML。
DOCKER_ENV_FILE ?= api/.env.docker

## 安装

install: web-install
	@cd api && go mod download

web-install:
	@cd web && npm install --no-audit --no-fund --cache ./.npm-cache

## 后端

api-dev:
	@$(MAKE) -C api dev

api-run:
	@$(MAKE) -C api run

api-build:
	@$(MAKE) -C api build

api-test:
	@$(MAKE) -C api test

migrate:
	@$(MAKE) -C api migrate

migrate-down:
	@$(MAKE) -C api migrate-down

ai-check:
	@$(MAKE) -C api ai-check

## 前端

web-dev:
	@cd web && npm run dev

web-build:
	@cd web && npm run build

web-typecheck:
	@cd web && npm run type-check

## 构建

build: api-build web-build

## Docker
# 本地构建，产物留在本机 docker images 里，不推送
docker-build:
	@echo "构建 $(DOCKER_IMAGE):$(DOCKER_TAG) ($(DOCKER_PLATFORMS))"
	docker build --platform $(DOCKER_PLATFORMS) -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# 发布：构建并推送到远端仓库。Makefile 里凭据走 `docker login`，不要写死在仓库
docker-release:
	@echo "发布 $(DOCKER_IMAGE):$(DOCKER_TAG) ($(DOCKER_PLATFORMS))"
	docker build --platform $(DOCKER_PLATFORMS) -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

# 本地试跑发布镜像（真正验证 release 效果）。
#
# 环境变量有两个来源，按优先级：
#   1) 存在 $(DOCKER_ENV_FILE) 时用它（推荐，见 api/.env.docker.example）
#   2) 否则从当前 shell 透传 DATABASE_URL / JWT_SECRET / DEEPSEEK_API_KEY
#
# 注意 docker run 不会读 api/.env —— 那只有 docker compose 才会读。
# 镜像已内置 APP_ENV=production / HTTP_PORT=8401 / WEB_DIST_DIR=/usr/share/nginx/html /
# RUN_MIGRATIONS=true，无需手工传。
# 端口被占用时覆盖：make docker-run APP_PORT=8490
docker-run:
	@if [ -f "$(DOCKER_ENV_FILE)" ]; then \
		echo "▶ 使用容器环境文件 $(DOCKER_ENV_FILE)"; \
		docker run --rm -p $(APP_PORT):80 \
			--env-file "$(DOCKER_ENV_FILE)" \
			$(DOCKER_IMAGE):$(DOCKER_TAG); \
	else \
		echo "▶ 未找到 $(DOCKER_ENV_FILE)，改用当前 shell 的 DATABASE_URL / JWT_SECRET / DEEPSEEK_API_KEY"; \
		echo "  （可照 api/.env.docker.example 创建该文件，更省事）"; \
		docker run --rm -p $(APP_PORT):80 \
			-e DATABASE_URL \
			-e JWT_SECRET \
			-e DEEPSEEK_API_KEY \
			$(DOCKER_IMAGE):$(DOCKER_TAG); \
	fi

## 本地开发栈（热重载，不构建发布镜像）

# 起 api(air 热重载) + web(vite HMR)。改代码即时生效，无需重新构建镜像。
# 首次会构建 api 的 dev 镜像并 npm ci，之后走缓存。
# 访问 http://localhost:$(WEB_PORT) ；分享页 /s/<id> 的 SEO 注入经 vite 代理到 api，本地即可见。
docker-dev:
	API_PORT=$(API_PORT) WEB_PORT=$(WEB_PORT) \
		docker compose -f $(DOCKER_DEV_FILE) up

docker-dev-down:
	API_PORT=$(API_PORT) WEB_PORT=$(WEB_PORT) \
		docker compose -f $(DOCKER_DEV_FILE) down

docker-dev-logs:
	docker compose -f $(DOCKER_DEV_FILE) logs -f

# dev 镜像只含工具链，不含源码，重建很快；升级 air 或改 Dockerfile.dev 时才需要
docker-dev-build:
	docker compose -f $(DOCKER_DEV_FILE) build --no-cache api

## Nginx

# 推送 deploy/nginx 下的站点配置到服务器，备份 -> 上传 -> nginx -t -> reload，失败自动回滚
# 覆盖目标：make nginx-push NGINX_SERVER=root@1.2.3.4 NGINX_PORT=2222
nginx-push:
	@SERVER=$(NGINX_SERVER) PORT=$(NGINX_PORT) deploy/nginx/upload.sh
