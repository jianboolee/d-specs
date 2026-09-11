# d-specs API

产品产品配置表的后端服务。Go + Gin + PostgreSQL + DeepSeek。

## 快速开始

```bash
cp .env.example .env      # 填入 DEEPSEEK_API_KEY
make migrate              # 建表
make dev                  # 启动（:8300，热重载）
```

首次需要确保 `dspecs` 库存在（共享的 postgres18 实例）：

```bash
docker exec -e PGPASSWORD=postgres postgres18 \
  psql -h 127.0.0.1 -U postgres -d postgres -c "CREATE DATABASE dspecs"
```

## 常用命令

| 命令 | 说明 |
|------|------|
| `make dev` | 开发模式启动（Air 热重载，默认 :8300） |
| `make migrate` | 执行 SQL migration（默认 up） |
| `make migrate-down` | 回退最近一次迁移 |
| `make ai-check` | 验证 DeepSeek 连通性与 function calling |
| `make test` / `make vet` / `make fmt` | 测试 / 静态检查 / 格式化 |

## 目录结构

```
cmd/
  server/     HTTP 服务入口
  migrate/    SQL migration 执行器
  aicheck/    DeepSeek 连通性与 function calling 验证
internal/
  app/        依赖装配与生命周期
  config/     viper 配置加载
  deps/       共享依赖容器
  contextx/   context 透传（当前登录用户等）
  infrastructure/
    auth/     JWT 签发与校验
    cache/    Redis
    database/ GORM + PostgreSQL
    idgen/    实体 ID（UUID v7）与分享 token 生成
    logger/   slog
  middleware/ auth / cors / recover / request_id
  response/   统一响应体与中文校验错误
  router/     路由总装
  modules/
    auth/     注册 / 登录 / JWT
    spec/     配置表 CRUD、变更执行器、版本与撤销
    chat/     AI 对话编排、SSE、会话与审计落库
    share/    只读分享链接
  ai/
    provider/ LLM 供应商抽象 + DeepSeek 实现
    ops/      配置表变更协议：校验与应用（纯逻辑，重点单测）
    tools/    暴露给模型的工具定义与只读工具实现
    agent/    agent loop + system prompt + ApplyHook
pkg/
  quantity/   数值校验：判断「5000mAh」是不是「数字 + 可选单位」
migrations/   SQL 迁移（按时间戳命名，成对 up/down）
```

## 架构要点

### AI 只能输出结构化操作，绝不整表覆盖

模型改变数据的唯一通道是 `propose_ops` 工具，产出的是一组结构化 `Op`
（`add_item` / `update_item` / `delete_item` / …），由 `internal/ai/ops`
校验通过后才落地。这样变更天然是增量的、可审计的、可撤销的、可预览的。

校验不通过时，错误会作为 tool result 回灌给模型让它自我修正 —— 这是只接
Flash 档位时可靠性的主要来源，详见 `internal/ai/agent/agent.go`。

### 手工编辑与 AI 编辑共用一套执行器

`ops.Validate` / `ops.Apply` 是纯函数，不依赖数据库。手工编辑、AI 变更、
模板套用全部走同一套校验与版本逻辑，不会出现「AI 改的能撤销、手改的不能」
这类行为割裂。

### 服务端兜底条件式语义

模型不必逐条枚举「要改哪几个参数」，可以表达条件式目标
（`target: {group: "续航"}`），由 `ops.expandTarget` 展开。
这既省 token，也消除了弱模型逐条枚举时的漏项问题。

### 上下文只带 schema，按需取数

system prompt 只包含参数名与分组，不含参数值；需要值时模型用
`search_items` / `get_items` 主动拉取。配置表可能有几百条，全量塞进上下文
必然撑爆窗口（见 `internal/ai/agent/prompt.go`）。

## 配置

见 `.env.example`。几个关键项：

| 变量 | 说明 |
|------|------|
| `DEEPSEEK_MODEL` | 模型 id，以 `GET /models` 返回为准，用 `make ai-check` 验证 |
| `AI_MAX_TOOL_ROUNDS` | agent loop 轮数上限，同时是校验失败重试次数上限 |
| `AI_AUTO_APPLY_MAX_OPS` | 安全阀：单次 ops 超过该条数转为待用户确认；设 0 关闭 |
| `AI_MAX_TOKENS` | 思考与正文共享的预算，见下方实测结论 |

## `deepseek-flash` 实测结论

`make ai-check` 在真实账号上跑出的结果，几条会影响实现的事实：

**它是推理模型。** 响应里带 `reasoning_content` 字段，且**思考与正文共享
`max_tokens` 预算** —— 预算给小了会出现「思考把 token 用完、正文为空」。
provider 已单独保留该字段（不展示、不回传），agent 在正文为空且
`finish_reason=length` 时会给出明确提示而不是抛一条空白回复。

**模型 id 不是 `deepseek-chat`。** 该账号可用的是 `deepseek-flash` 与
`deepseek-v4-pro`。写死猜测的 id 会静默走到别的档位，所以 `ai-check`
第二步专门做列表核对。

**思考 token 占比很高。** 建表场景 completion=1349，其中思考 1068（约 79%）。
成本估算不能只看正文长度。

**前缀缓存命中率约 90%。** agent loop 每轮重发增长中的前缀，实测命中
4608~7680 token。这正是 context 策略（稳定 system prompt + 只读工具按需取数）
想要的结果：多轮对话的边际 prompt 成本被显著摊薄。

**function calling 稳定。** 4 个 Agent 场景（空表建表、按名称改值、批量条件
改动、只读提问）全部通过，参数值含单位原样写入（`value=5000mAh`），
只读提问也确实没有产生变更。Phase 2 无需切换到 JSON 模式退路。

单次交互约 2~7 秒、2~3 轮工具调用。

## 接口

```
GET  /health                            健康检查
GET  /health/ready                      依赖就绪检查（含 LLM）

POST /api/v1/auth/register              注册
POST /api/v1/auth/login                 登录
GET  /api/v1/me                         当前用户

# 配置表（均需 Bearer Token）
GET    /api/v1/tables                   列表（keyword / page / size）
POST   /api/v1/tables                   建表
GET    /api/v1/tables/:id               详情（表 + 全部参数项 + 分组）
PATCH  /api/v1/tables/:id               改标题 / 描述 / 行业
DELETE /api/v1/tables/:id               软删除
POST   /api/v1/tables/:id/duplicate     复制表（换产品改几个值）

# 变更与版本
POST   /api/v1/tables/:id/apply         统一变更入口
GET    /api/v1/tables/:id/revisions     版本列表
POST   /api/v1/tables/:id/restore       恢复到指定版本

# AI 对话
POST   /api/v1/tables/:id/chat          对话（SSE 流式）
GET    /api/v1/tables/:id/sessions      会话列表
GET    /api/v1/sessions/:sid/messages   会话消息

# 分享
GET    /api/v1/tables/:id/share         分享状态
POST   /api/v1/tables/:id/share         开启分享
DELETE /api/v1/tables/:id/share         关闭分享
GET    /api/v1/public/specs/:token      公开只读内容（无需鉴权）
```

统一响应体：

```json
{ "code": 0, "message": "ok", "data": { } }
```

`code` 为 0 表示成功，非 0 时等于 HTTP 状态码，`message` 为可直接展示的中文提示。

### 变更协议

`apply` 是**唯一**的写入通道，手工编辑与 AI 变更共用：

```json
{
  "base_revision": 3,
  "source": "user",
  "summary": "把电池容量改成 5000mAh",
  "ops": [
    { "op": "update_item", "name": "电池容量", "patch": { "value": "5000mAh" } }
  ]
}
```

- `base_revision` 是乐观锁依据，过期返回 **409**
- `ops` 校验不通过返回 **422**，`message` 里是可直接展示的中文原因
- 支持的 op：`add_item` / `add_items` / `update_item` / `delete_item` / `reorder_items` / `set_group`
- 定位优先级：`item_id` > `target` 条件 > `name`（重名会报歧义并提示改用 `item_id`）
- `patch` 的语义：**省略字段 = 不改，传空字符串 = 改成空**

### 撤销 / 重做的语义

`restore` 是把目标版本的快照写成**新版本**，而不是回退版本号。
历史因此保持线性且不可变，撤销本身也是一次可追溯、可再撤销的变更。
撤销/重做的栈由前端维护 —— 只有它知道用户实际走过哪些版本。

### 租户隔离

`Repository` 的每个方法第一个业务参数都是 `userID`，且必须出现在查询条件里。
这是有意为之：隔离不能依赖 handler 或 service 自觉，签名定死之后，
漏加过滤条件会直接编译不过，而不是变成一次越权查询。

越权访问一律返回 **404「配置表不存在」**，而不是 403 ——
否则等于告诉攻击者这个 id 是真实存在的。

**唯一的例外**是 `share.Repository.resolveByToken`：公开分享页天然没有登录用户，
安全边界完全由 token 承担（32 位随机串 + 必须 `enabled=true`）。
它在代码里被显式标注，且只允许通过它读取分享内容。

### AI 对话（SSE）

```
POST /api/v1/tables/:id/chat
{ "session_id": "可选，为空则新建会话", "message": "把电池容量改成 5000mAh" }
```

响应是 `text/event-stream`，事件类型：

| 事件 | 含义 |
|------|------|
| `start` | 会话已建立，带 `session_id` |
| `text` | 模型在调用工具前说的话 |
| `tool_call` | 正在调用某工具（`tool_name` / `tool_args`） |
| `tool_result` | 工具返回 |
| `applied` | 变更已落库，带 `revision` 与 `changes[]` |
| `pending` | 规模过大，转为待确认（`AI_AUTO_APPLY_MAX_OPS`） |
| `done` | 整轮结束，带 `reply` 与 token 用量 |
| `error` | 出错 |

**HTTP 状态码只在建流之前有意义**：一旦开始流式输出就固定是 200。
所以「表不存在」「消息为空」「AI 未配置」都在写 SSE 头之前校验完，
以正常的 JSON 错误返回（404 / 400 / 503），前端可以靠状态码区分。

### AI 变更如何落库

agent 不直接写库，而是通过 `ApplyHook` 回调交给 `spec.Service.Apply`：

```
模型产出 ops → agent 校验 → ApplyHook → spec.Service.Apply（同一套执行器）
                                      → 乐观锁 + revision 快照
                                      → ops 明细写入 ai_operations 审计
```

这样 AI 变更与手工变更共享同一份校验、版本与撤销语义。
hook 的返回值里带上最新快照，模型后续的读操作看到的都是真实数据；
409 之类的并发冲突会作为工具错误回灌，模型可以自己重试。

会话历史回放时**只带 user / assistant 的文本，不回放 tool 回合**：
工具返回的是「当时那张表」，塞回上下文既占 token 又可能与当前状态冲突。


