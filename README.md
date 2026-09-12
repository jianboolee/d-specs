# d-specs 参数配置表

用对话的方式，为企业创建和维护产品的参数配置表，并分享出去。

一张参数表 = 一个产品的参数；表内每行 = 一条参数项（参数名 / 参数值 / 分组）。
参数值自带单位，例如 `5000mAh`、`8小时`、`132kW`。
用户可以创建任意多张表，也可以直接让 AI 通过对话建表、改值、调分组。

## 技术栈

| 层 | 选型 |
|----|------|
| 前端 | Vue 3 + Vite + TypeScript + naive-ui + SCSS + Pinia |
| 后端 | Go + Gin + GORM + PostgreSQL |
| AI | DeepSeek（首版只接 Flash 档位） |

## 目录

```
api/          Go 后端服务（详见 api/README.md）
web/          Vue 前端
docs/PLAN.md  技术规划（模型设计、AI 协议、路线图与决策记录）
```

## 快速开始

前置：本地已运行 PostgreSQL。本项目复用 `~/Sites/docker/postgres18`
共享实例，需要先建库：

```bash
docker exec -e PGPASSWORD=postgres postgres18 \
  psql -h 127.0.0.1 -U postgres -d postgres -c "CREATE DATABASE dspecs"
```

然后：

```bash
# 后端
cd api
cp .env.example .env        # 填入 DEEPSEEK_API_KEY
make migrate                # 建表
make dev                    # :8300

# 前端（另开终端）
cd web
npm install --cache ./.npm-cache
npm run dev                 # :5173，已配置 /api 代理到 8300
```

访问 http://localhost:5173 ，用演示账号 `demo@dspecs.dev` / `demo12345` 登录。

登录后落在欢迎页，点「我的参数表」进入列表，再点任意一张表进入工作区。

工作区里右侧画布可以直接改参数名和参数值；左侧对话框里也可以直接说
「加一个参数：电池容量 5000mAh，分组续航」，AI 会自动应用改动并高亮
变化的位置，随时可以撤销。画布右上角的「分享」可以生成只读链接。

## 验证 AI 连通性

这是 Phase 0 的关键项：确定只接 Flash 档位、首版又做到 Phase 2，那么
function calling 是否稳定就是整个 Phase 2 的前提假设。

```bash
cd api && make ai-check
```

它会依次验证：模型列表、基础对话、tool calling 的参数正确性、
以及「建表 / 改值 / 批量条件改动 / 只读提问」四个 Agent 全链路场景。

## 构建与发布镜像

发布镜像 `jianboo/d-specs` 是**全栈单镜像**：nginx 托管前端产物，并把
`/api` 与 `/health` 反代到同容器内的 Go 服务，监听 80 端口。

```bash
make docker-build     # 只在本机构建，不推送
make docker-release   # 构建并推送到 Docker Hub
```

`make docker-release` 默认发布 **latest**。需要同时保留一个版本号时显式指定：

```bash
make docker-release DOCKER_TAG=v1.0.0
```

推送前先登录（凭据不要写进仓库）：

```bash
docker login
```

默认按 `linux/amd64` 构建（常见云服务器架构）。本机是 Apple Silicon 想本地试跑，
用 `DOCKER_PLATFORMS=linux/arm64` 覆盖；需要同时发布两种架构时传
`DOCKER_PLATFORMS=linux/amd64,linux/arm64`（多平台需要 buildx）。

### 运行

PostgreSQL 是**外部依赖**，通过环境变量注入（变量清单见 `api/.env.example`）。
容器启动时默认自动执行数据库迁移，可用 `RUN_MIGRATIONS=false` 关闭：

```bash
docker run -d -p 8080:80 --name dspecs \
  -e DATABASE_URL="postgresql://user:pass@host:5432/dspecs?sslmode=disable" \
  -e JWT_SECRET="<openssl rand -base64 48 | tr -d '/+='>" \
  -e DEEPSEEK_API_KEY="<key>" \
  -e PUBLIC_URL_BASE="" \
  jianboo/d-specs:v1.0.0
```

也可以把迁移当一次性任务单独跑（`migrate` 支持 `up` / `down N` / `version`）：

```bash
docker run --rm -e DATABASE_URL="..." jianboo/d-specs:v1.0.0 migrate up
```

本地试跑可直接 `make docker-run`（需要 `DATABASE_URL` / `JWT_SECRET`
/ `DEEPSEEK_API_KEY` 已在当前 shell 环境里；从宿主机访问数据库时把 host 写成
`host.docker.internal`）。

镜像内置健康检查，直接打后端 `/health`（而非 nginx），避免把「nginx 活着、
后端已挂」误判为健康。容器内任一时间只有一个 API 与一个 nginx 进程，任一
意外退出都会让容器整体退出，交由编排系统重启。

## 用 docker compose 部署（自建服务器）

`deploy/` 下提供三套编排方案，按「数据库由谁托管」选择：

| 文件 | 数据库 | 适用场景 |
|------|--------|----------|
| `deploy/docker-compose.yml` | 本栈托管 PostgreSQL 18 | 全新服务器，从零起 | 
| `deploy/portainer-stack.yml` | 本栈托管 PostgreSQL 18 | 同上，走 Portainer |
| `deploy/portainer-stack-external-db.yml` | **使用现有数据库** | 服务器已有 PG，只想跑应用 |

### 方案一：连同数据库一起起（自建服务器）

```bash
cd deploy
cp .env.example .env     # 至少填 POSTGRES_PASSWORD 与 JWT_SECRET
docker compose up -d
```

默认访问 `http://<服务器>:8080`。`deploy/.env` 已在 `.gitignore` 中，不会被提交。

### 方案二：只用应用 + 现有数据库（Portainer 推荐）

适合团队已有共享 PostgreSQL 的情况，只部署一个应用容器：

1. Portainer → Stacks → Add stack，粘贴 `deploy/portainer-stack-external-db.yml`
2. 点 **Load variables from .env file**，导入 `deploy/production.env`
3. 确认 `DATABASE_URL` 指向正确的库（库不存在时见下方说明）
4. Deploy

`deploy/production.env` 含真实密钥，同样已在 `.gitignore` 中。它只负责把一个
应用容器接上现有库，**不做数据库备份/持久化**——那是数据库实例自己的事。

### 首次部署：库还不存在怎么办

连上一个「实例存在、但库还没建」的 PostgreSQL 时，会报：

```
FATAL: database "dspecs" does not exist (SQLSTATE 3D000)
```

这是必然的：`CREATE DATABASE` 无法在目标库内执行（连都连不上），所以应用
没法「先建库再连库」。两种解决方式：

**方式 A：交给应用自动建库（推荐）**

在 `deploy/production.env` 里保持：

```sh
AUTO_CREATE_DB=true
```

容器启动执行迁移前会先连维护库（`postgres`，失败则退回 `template1`）执行
`CREATE DATABASE`，然后正常迁移。**库建好后建议改回 `AUTO_CREATE_DB=false`**：
默认关闭，是为了避免 `DATABASE_URL` 里库名写错时静默建出一个空库、让应用带着
空白数据跑起来。

需要该账号有建库权限。多副本同时启动也安全——建库冲突与迁移竞争都已处理
（迁移用会话级 advisory lock 串行化）。

**方式 B：手动建库**

```sh
docker exec -e PGPASSWORD=<密码> postgres18 \
  psql -h 127.0.0.1 -U postgres -d postgres -c "CREATE DATABASE dspecs"
```

### 关键设计

- **不镜像数据库端口**：自托管方案里 Postgres 只在 compose 网络内可达，不暴露到宿主机；
- **`depends_on: service_healthy`**：生产模式下数据库连不上应用会直接退出，
  所以自托管方案必须等依赖健康后再启动，否则容器会反复重启；
- **Postgres 18 数据卷挂 `/var/lib/postgresql`**：18 起数据目录改为
  `/var/lib/postgresql/18/docker`，挂成旧路径 `.../data` 会让数据落在容器层、
  重启即丢；
- **必填变量缺省即报错**：未设置数据库密码 / `JWT_SECRET` 时 compose 直接失败，
  避免用弱默认密钥把服务跑起来；
- **未配 `DEEPSEEK_API_KEY` 仍可启动**：应用正常服务，仅 AI 对话不可用
  （`/health` 会返回 `llm: false`）；
- **外部库方案配了 `extra_hosts`**：让 `host.docker.internal` 在 Linux 上也能解析，
  便于连接同宿主机的数据库容器；
- **`AUTO_CREATE_DB` 默认 false**：只在首次部署时显式开启，避免库名写错时静默建空库；
- **迁移加 advisory lock**：并发/滚动部署时把迁移串行化，避免
  `CREATE TABLE IF NOT EXISTS` 的检查与创建不原子导致的唯一冲突重启。

## 当前进度

首版已全部完成：

| 阶段 | 状态 |
|------|------|
| Phase 0 骨架 | ✅ 后端骨架、迁移、认证、AI 层、前端脚手架 |
| Phase 1 参数表 CRUD | ✅ 手工编辑、版本快照、撤销重做、复制表、跨用户隔离 |
| Phase 2 AI 对话 + 分享 | ✅ SSE 对话、改动联动、会话审计、只读分享链接 |

现在可以真实使用：注册登录 → 建表 → 手工编辑或直接让 AI 改 → 随时撤销 → 分享出去。

后续方向见 [docs/PLAN.md](docs/PLAN.md) 第 9 节。
