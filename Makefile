.PHONY: help api-dev api-run api-build migrate migrate-down ai-check api-test web-dev web-build web-install install

## 帮助

help:
	@echo "d-specs 常用命令"
	@echo ""
	@echo "  make install      安装前后端依赖"
	@echo "  make migrate      执行数据库迁移"
	@echo "  make api-dev      启动后端 :8300（热重载）"
	@echo "  make web-dev      启动前端 :5173"
	@echo "  make ai-check     验证 DeepSeek 连通性与 function calling"
	@echo "  make build        构建前后端产物"
	@echo "  make api-test     运行后端测试"
	@echo ""

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
