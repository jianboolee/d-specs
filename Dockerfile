# syntax=docker/dockerfile:1

# ============================================================
# d-specs 发布镜像（全栈单镜像）
#
#   前端：Vite 构建 web/dist，运行时由 nginx 托管
#   后端：Go 编译 server / migrate 两个静态二进制
#   运行时：nginx 监听 80，/api 与 /health 反代到 127.0.0.1:8300
#
# PostgreSQL / Redis 是外部依赖，通过环境变量注入（见 api/.env.example）
# ============================================================

# ---------- 1. 构建前端 ----------
FROM node:22-alpine AS web

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

COPY web/ ./
RUN npm run build

# ---------- 2. 构建后端 ----------
FROM golang:1.25-alpine AS api

WORKDIR /src
COPY api/go.mod api/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY api/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ---------- 3. 运行时 ----------
FROM alpine:3.21

# ca-certificates：访问 DeepSeek HTTPS 接口；tzdata：日志与时间格式化
RUN apk add --no-cache nginx ca-certificates tzdata

WORKDIR /app

COPY --from=api /out/server /app/server
COPY --from=api /out/migrate /app/migrate
# migrate 以工作目录为基准查找 migrations/*.up.sql
COPY api/migrations /app/migrations

COPY --from=web /web/dist /usr/share/nginx/html
COPY docker/nginx.conf /etc/nginx/http.d/default.conf
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# 配置一律走环境变量（config 使用 viper.AutomaticEnv，不依赖 .env 文件）
ENV APP_ENV=production \
    HTTP_PORT=8300 \
    RUN_MIGRATIONS=true

EXPOSE 80

# 探针直接打后端 /health，避免把「nginx 活着但后端已挂」当成健康
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8300/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["serve"]
