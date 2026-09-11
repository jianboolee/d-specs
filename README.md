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

前置：本地已运行 PostgreSQL 与 Redis。本项目复用 `~/Sites/docker/postgres18`
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

## 当前进度

首版已全部完成：

| 阶段 | 状态 |
|------|------|
| Phase 0 骨架 | ✅ 后端骨架、迁移、认证、AI 层、前端脚手架 |
| Phase 1 参数表 CRUD | ✅ 手工编辑、版本快照、撤销重做、复制表、跨用户隔离 |
| Phase 2 AI 对话 + 分享 | ✅ SSE 对话、改动联动、会话审计、只读分享链接 |

现在可以真实使用：注册登录 → 建表 → 手工编辑或直接让 AI 改 → 随时撤销 → 分享出去。

后续方向见 [docs/PLAN.md](docs/PLAN.md) 第 9 节。
