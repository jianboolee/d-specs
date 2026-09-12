.PHONY: help api-dev api-run api-build migrate migrate-down ai-check api-test web-dev web-build web-install install \
        docker-build docker-release docker-run

## 帮助

help:
	@echo "d-specs 常用命令"
	@echo ""
	@echo "  make install        安装前后端依赖"
	@echo "  make migrate        执行数据库迁移"
	@echo "  make api-dev        启动后端 :8300（热重载）"
	@echo "  make web-dev        启动前端 :5173"
	@echo "  make ai-check       验证 DeepSeek 连通性与 function calling"
	@echo "  make build          构建前后端产物"
	@echo "  make api-test       运行后端测试"
	@echo "  make docker-build   构建镜像（仅本机，不推送）"
	@echo "  make docker-release 构建并推送 $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@echo "  make docker-run     用最新镜像本地起一个容器 :8080"
	@echo ""

# ---------- Docker 发布变量 ----------
# 镜像仓库与 tag。用法：make docker-release DOCKER_TAG=v1.0.0
DOCKER_IMAGE ?= jianboo/d-specs
# 默认 tag 取最近的 git tag，没有 tag 时退回提交短哈希，例如 v1.2.0 / 46efb9c
DOCKER_TAG ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# 目标架构。常见云服务器是 amd64；需要多架构时用逗号分隔，如 linux/amd64,linux/arm64
DOCKER_PLATFORMS ?= linux/amd64

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

# 本地试跑。数据库需另行可达（默认连宿主机）
docker-run:
	docker run --rm -p 8080:80 \
		-e DATABASE_URL \
		-e JWT_SECRET \
		-e DEEPSEEK_API_KEY \
		$(DOCKER_IMAGE):$(DOCKER_TAG)
