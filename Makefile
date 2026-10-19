.PHONY: help api-dev api-run api-build migrate migrate-down ai-check api-test web-dev web-build web-install install \
        docker-build docker-release docker-run \
        docker-dev docker-dev-down docker-dev-logs docker-dev-build \
        image deploy deploy-only env-prod nginx

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
	@echo ""
	@echo "── 生产部署（入口机 nginx → 业务机容器，详见 deploy/）──"
	@echo "  make image        交叉构建 linux/amd64 镜像并推 ACR（tag=$(IMG_TAG)）"
	@echo "  make env-prod     铺 .env.prod 模板到业务机 /opt/d-specs/（不覆盖已填好的）"
	@echo "  make deploy       构建推送 + 业务机拉镜像重启（需 .env.prod 已填）"
	@echo "  make deploy-only  业务机 compose 拉取重启（缺省 latest；TAG=<日期tag> 可回滚）"
	@echo "  make nginx        下发 specs.x-rv.com vhost 到入口机并 reload"
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

# ---------- 生产部署（本机交叉构建 → ACR → 业务机拉镜像） ----------
# 拓扑（与 d-note / d-auth / d-camp 一致）：
#   入口机 39.107.77.69（172.17.255.63，外层 nginx，*.x-rv.com 通配证书）
#   业务机 47.95.176.70（172.17.255.64，d-specs 容器 :8400）
#   域名   https://specs.x-rv.com
# 数据库复用入口机的 postgres18 容器（:5432）上的 dspecs 库。
REGISTRY ?= registry.cn-beijing.aliyuncs.com/xrv/d-specs
IMG_TAG  ?= $(shell date +%Y%m%d-%H%M)
PROD_SSH ?= root@47.95.176.70
EDGE_SSH ?= root@39.107.77.69

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
#   2) 否则从当前 shell 透传 DATABASE_URL / DAUTH_* / DEEPSEEK_API_KEY
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
		echo "▶ 未找到 $(DOCKER_ENV_FILE)，改用当前 shell 的 DATABASE_URL / DAUTH_* / DEEPSEEK_API_KEY"; \
		echo "  （可照 api/.env.docker.example 创建该文件，更省事）"; \
		docker run --rm -p $(APP_PORT):80 \
			-e DATABASE_URL \
			-e DEEPSEEK_API_KEY \
			-e DAUTH_BASE_URL \
			-e DAUTH_CLIENT_KEY \
			-e DAUTH_COOKIE_NAME \
			-e DAUTH_SSO_BASE_URL \
			-e DAUTH_JWT_ISSUER \
			-e DAUTH_API_KEY \
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

## 生产部署（本机交叉构建 → ACR → 业务机直接拉镜像）
# 与 d-note / d-auth / d-camp 同形态：全栈单镜像（nginx 托管 SPA + /api 剥前缀转 Go），
# 业务机只放应用容器，数据库用入口机的 postgres18。

# 交叉构建 linux/amd64 并推到 ACR（同时打日期 tag 和 latest）。
# 注意：本机是 arm64，--platform linux/amd64；Dockerfile 的构建阶段钉在 BUILDPLATFORM，
# 只有运行时阶段的 apk 那步走模拟。
image:
	docker buildx build --platform linux/amd64 \
		-t $(REGISTRY):$(IMG_TAG) -t $(REGISTRY):latest --push .

# 一键发布：构建推送 → 业务机 compose 拉镜像滚动更新（需 /opt/d-specs/.env.prod 已填好）。
# 想只重新部署不重新构建：make deploy-only（缺省用 latest，可 TAG=<日期tag> 指定回滚）
deploy: image deploy-only

deploy-only:
	scp -q deploy/docker-compose.prod.yml $(PROD_SSH):/opt/d-specs/docker-compose.yml
	scp -q deploy/run-prod.sh $(PROD_SSH):/opt/d-specs/run-prod.sh
	ssh $(PROD_SSH) 'TAG=$(if $(TAG),$(TAG),latest) bash /opt/d-specs/run-prod.sh'

# 把 env 模板铺到业务机（不覆盖已有 .env.prod；填好后 make deploy-only）
env-prod:
	ssh $(PROD_SSH) 'mkdir -p /opt/d-specs'
	scp -q deploy/env.prod.example $(PROD_SSH):/opt/d-specs/env.prod.example
	ssh $(PROD_SSH) 'test -f /opt/d-specs/.env.prod || { cp /opt/d-specs/env.prod.example /opt/d-specs/.env.prod && chmod 600 /opt/d-specs/.env.prod && echo "✔ 已生成 /opt/d-specs/.env.prod（待填写 DATABASE_URL / DAUTH_CLIENT_KEY / DAUTH_API_KEY）"; } || echo "· /opt/d-specs/.env.prod 已存在，未覆盖"'

# 下发入口机 vhost（改动前先备份远端旧版；nginx -t 不过就不 reload）
nginx:
	@ssh $(EDGE_SSH) 'test -f /etc/nginx/conf.d/specs.x-rv.com.conf && cp /etc/nginx/conf.d/specs.x-rv.com.conf /etc/nginx/conf.d/backups/specs.x-rv.com_backup_$$(date +%Y%m%d_%H%M%S).conf || true'
	scp -q deploy/nginx/specs.x-rv.com.conf $(EDGE_SSH):/etc/nginx/conf.d/specs.x-rv.com.conf
	ssh $(EDGE_SSH) 'nginx -t && systemctl reload nginx && echo "✔ specs.x-rv.com vhost 已生效"'
