# d-specs 产品参数配置表 —— 技术规划

> 状态：v0.3（决策已锁定，无待确认项）

---

## 0. 决策基线

| # | 决策 | 影响 |
|---|------|------|
| 1 | **单产品表**：一张表 = 一个产品的参数；用户可创建非常多张表 | 表内每行 = 一条参数项（非型号维度） |
| 2 | **AI 默认自动应用** | 无需逐次确认；靠版本 + 一键撤销兜底 |
| 3 | **先做个人版** | 无 org / RBAC，一律按 `user_id` 隔离 |
| 4 | **只接 DeepSeek Flash** | 单 provider；校验回路与上下文精简成为可靠性生命线 |
| 5 | **首版交付到 Phase 2** | 模板市场、组织能力延后 |
| 6 | **极简分享提前进首版** | 只读公开链接 + 独立只读页，随 P2 一并交付 |

---

## 1. 产品定义

让用户用**对话**的方式创建和维护产品的参数配置表，并分享出去。

### 核心模型

一张表 = 一个产品；表内每行 = 一条参数项。

```
产品：某型号手机
┌──────────┬────────────┬──────────────────┐
│ 分组      │ 参数名      │ 参数值            │
├──────────┼────────────┼──────────────────┤
│ 外观      │ 机身颜色    │ 钛金属            │
│ 外观      │ 重量        │ 198g             │
│ 性能      │ 处理器      │ A18              │
│ 续航      │ 电池容量    │ 5000mAh          │
│ 续航      │ 充电功率    │ 45W              │
└──────────┴────────────┴──────────────────┘
```

**为什么是「行=参数项」而不是「列=参数」：**

- 单产品场景下没有型号维度，强行用列会把每个产品的参数都变成独立的表结构 → 建第二张表就得重新定义字段，而用户会有**非常多**张表
- 行=参数项时，所有表结构完全统一，新建表近乎零成本，**复制表**「换个产品改几个值」直接可用
- **模板成为天然产物**：模板 = 一份参数项清单骨架，套用后生成待填条目
- AI 定位成本极低：一条 item 一个 id，`update_item(name="电池容量")` 语义无歧义

> 未来若要支持多型号对比（如汽车参数表的多车型），扩展方式是给 `spec_items` 加 variant 维度（行=参数项、列=型号）。当前不做，但模型留了路。

### 参数项类型

| 类型 | 用途 | 示例 |
|------|------|------|
| `text` | 自由文本 | 「钛金属」「1.5-2.0kg」 |
| `number` | 单个数值 + 可选单位，服务端校验其可解析 | 「5000mAh」「8小时」 |
| `bool` | 支持 / 不支持 | 「支持」 |
| `enum` | 枚举单选 | 「USB-C / Lightning」 |

> **参数值自带单位，单位不是独立字段。** 这一版把原来的「数值 / 单位」两列合并了：
> 真实规格表就是这么写的（`5000mAh`、`132kW`），拆成两列反而会出现
> 「值有、单位空」这类半截数据，也让 AI 每次都要做一次没必要的拆分。
>
> `number` 类型保留下来只做**校验**（挡掉「约五千」这种写进数值列的写法）
> 并对数值部分建索引列 `value_num`，供后续排序与区间筛选。
> 校验**只判断不改写** —— 用户写的「8小时」原样保留，不会被悄悄改成「8h」。

---

## 2. 数据模型

### spec_tables

| 字段 | 说明 |
|------|------|
| `id` | |
| `user_id` | 归属用户（个人版唯一隔离键） |
| `title` | 产品名，如「XX 手机 A1」 |
| `description` | 简介 |
| `industry` | 行业标签，用于模板推荐 |
| `current_revision` | 乐观锁 + 版本号 |
| `visibility` | `private` / `link` |
| `created_at` / `updated_at` / `deleted_at` | 软删除 |

### spec_items（参数项，表的核心）

| 字段 | 说明 |
|------|------|
| `id` | AI 操作引用它 |
| `table_id` | |
| `name` | 参数名：「电池容量」 |
| `value` | 参数值，TEXT 存储 |
| `value_type` | `text` / `number` / `bool` / `enum` |
| `value_num` | `value_type=number` 时冗余存数值，支撑排序、范围校验、区间筛选 |
| `enum_options` | JSONB，`value_type=enum` 的候选 |
| `group_name` | 分组：「续航」「外观」，可空 |
| `remark` | 备注 |
| `sort_order` | 表内顺序 |

索引：`(table_id, sort_order)`、`(table_id, lower(name))`。

> `name` 不做唯一约束（不同分组下重名是合理的），但创建时若同名给前端提示——AI 定位重名项时必须先问用户。

### 其余表

```sql
users          (id, email UNIQUE, name, password_hash, created_at)

spec_revisions (id, table_id, revision, snapshot JSONB, source, summary, created_at)
               -- snapshot = { items: [...] }，撤销/重做/审计的基础
               -- source = user | ai | template | import
               -- UNIQUE (table_id, revision)

chat_sessions  (id, table_id, user_id, title, created_at)

chat_messages  (id, session_id, role, content, tool_calls JSONB, tool_call_id,
                usage JSONB, created_at)

ai_operations  (id, table_id, message_id, revision, ops JSONB, summary, status,
                error, created_at)
               -- status = applied | rejected | failed | rolled_back

spec_templates (id, name, industry, description, items JSONB, owner_id NULL,
                is_system BOOL, created_at)
               -- items = [{name, value, value_type, group_name, sort_order}]
               -- owner_id IS NULL → 系统内置模板

share_links    (id, table_id, token UNIQUE, enabled BOOL,
                view_count BIGINT, created_at)
               -- token = 短随机串；enabled=false 即关闭分享（不删记录，便于复用）
```

### 不可省略的约束

- 所有查询强制带 `user_id`（在 repository 层做，不靠 handler 自觉）
- 写操作带乐观锁：`UPDATE ... WHERE id=$1 AND current_revision=$2`，冲突返回 409
- 默认过滤 `deleted_at IS NULL`

---

## 3. AI 层设计（核心）

### 3.1 铁律：AI 只能输出结构化操作，绝不能整表覆盖

让 AI 生成整张表再覆盖 → 丢失用户手改数据、幻觉、无法撤销、token 爆炸、无法定位改动。**一行都不能这么写。**

AI 的唯一写通道是一个 tool：`propose_ops`。

### 3.2 Tool 集

**只读工具**（让 AI 按需取数，而非预灌全表）：

| Tool | 作用 |
|------|------|
| `get_schema` | 取参数项名称列表 + 分组 + 各组数量（**不含值**） |
| `search_items` | 按名称关键词/分组检索，返回值与 id |
| `get_items` | 按 id 批量取完整条目 |
| `count_items` | 统计数量，判断影响面 |

**唯一写工具**：

| Tool | 作用 |
|------|------|
| `propose_ops` | 提交 `{summary, ops[]}` |

### 3.3 Operation 集

```
add_item      { name, value, value_type, group_name, remark }
add_items     { items: [...] }              -- 批量，AI 建整张表时用
update_item   { item_id | name, patch: { value, name, group_name, remark } }
delete_item   { item_id | name }
reorder_items { order: [item_id, ...] }
set_group     { item_ids: [...], group_name }
```

### 3.4 Agent Loop

```
用户消息
 → 上下文：system prompt + schema 摘要（参数名列表）+ 近 N 轮对话
 → 调 DeepSeek Flash（带 tools）
 → 若返回 tool_call：
      只读工具 → 执行 → 结果回灌 → 继续循环
      propose_ops → 校验
                    ├─ 失败 → 错误信息回灌 → 重试（≤3 轮）
                    └─ 通过 → 应用 ops → 生成新 revision
                              → SSE 推 applied + diff → 前端高亮
 → 无 tool_call → 文本收尾
 → 记录 usage
```

### 3.5 校验器（可靠性生命线）

只接 Flash 意味着模型更弱，**服务端校验和纠错必须更强**，不能指望模型一次做对。

`ValidateOps(ops, snapshot) → []OpError`，纯函数，重点单测覆盖：

- `item_id` 是否存在（不存在则尝试按 name 兜底解析）
- `name` 是否重名 / 是否为空
- `value_type=number` 时 `value` 能否解析为「数字 + 可选单位」
- `value_type=enum` 时值是否在 `enum_options` 内
- 批量操作的规模上限（防模型失控生成几千条）
- 删除/改名是否产生歧义

任一失败 → 错误作为 tool result 回灌 → 模型自我修正。这是把 Flash 用出可靠性的关键。

### 3.6 服务端兜底：条件式语义

Flash 容易在「所有分组都加一条」这类指令上逐条枚举、漏项或跑偏。对策：**让服务端展开，而不是让模型枚举。**

引导 AI 输出条件式 op，例如：

```json
{ "op": "add_item", "target": { "group": "续航" }, "name": "充电功率", "value": "45W" }
```

由后端展开成对该分组下所有 item 的操作。既省 token 又消除遗漏。

### 3.7 上下文策略

- system prompt 只带**参数名清单 + 分组**（几十个名字，很省），不带值
- 需要值时用 `search_items` / `get_items` 按需拉
- 超 N 轮后对早期对话做摘要压缩
- system prompt + schema 放前缀，复用 DeepSeek context caching 降本
- Flash 上下文更紧，这套策略比用 Pro 时更必需，不是优化项

### 3.8 应用策略（决策 2：默认自动应用）

- 默认 **自动应用** → SSE 推 `applied`，表格局部 patch + 高亮变化项
- AI 消息下方常驻「撤销」按钮（回到上一 revision）
- 保留一个**可配置的安全阀**：满足任一条件时降级为「提案待确认」
  - 单次 ops 数 > 30
  - 涉及删除 item
  - 解析不确定（如数值解析失败后模型仍未澄清）

  该阈值做成配置项，愿意全自动可关掉。

### 3.9 Tool calling vs JSON 模式

优先用 DeepSeek 的 function calling。若实测 Flash 在 tool calling 上稳定性不足（漏参、格式漂移），**退路是切 JSON 输出模式 + 严格 schema 校验**——把 tool 定义降级为 prompt 里的 JSON schema 说明，服务端解析。两者共用同一套 `ValidateOps`，切换成本低。

**✅ 已在 Phase 0 验证，无需切换退路：**

| 结论 | 影响 |
|------|------|
| `deepseek-flash` 是**推理模型**，返回 `reasoning_content` | 思考不展示、不回传；provider 单独保留该字段 |
| 思考与正文**共享 `max_tokens`** | 预算给小了会出现「正文为空」，`AI_MAX_TOKENS` 默认提到 8192 |
| 模型 id **不是 `deepseek-chat`**，实测可用 `deepseek-flash` / `deepseek-v4-pro` | `ai-check` 第二步专门做列表核对，防止静默走错档位 |
| 思考 token 占 completion 的约 79% | 成本估算不能只看正文长度 |
| **前缀缓存命中率约 90%** | 验证了 3.7 的上下文策略，多轮对话边际 prompt 成本被摊薄 |
| 4 个 Agent 场景全部一次通过，参数按 schema 正确拆分 | Phase 2 可按原方案推进 |

> 最后两条尤其重要：缓存命中 90% 说明「稳定 system prompt + 只读工具按需取数」
> 的设计是有效的；而一次通过则说明服务端校验回灌这条回路目前还没被真正压测到
> —— Phase 2 接入真实大表后仍需关注。

---

## 4. API 设计

```
# 认证
POST   /api/v1/auth/register | login | refresh
GET    /api/v1/me

# 参数表
GET    /api/v1/tables                      列表（分页/搜索）
POST   /api/v1/tables                      { title, template_id? }
GET    /api/v1/tables/:id                  详情（table + items + groups）
PATCH  /api/v1/tables/:id
DELETE /api/v1/tables/:id
POST   /api/v1/tables/:id/duplicate        复制表（换产品最高频的操作）

# 变更：手工与 AI 共用同一入口
POST   /api/v1/tables/:id/apply            { base_revision, source, ops[] }
GET    /api/v1/tables/:id/revisions
POST   /api/v1/tables/:id/undo | redo

# 对话（SSE）
POST   /api/v1/tables/:id/chat             返回 text/event-stream
GET    /api/v1/sessions?table_id=
GET    /api/v1/sessions/:sid/messages
POST   /api/v1/ai/operations/:id/apply     提案模式下确认
POST   /api/v1/ai/operations/:id/reject

# 分享（首版）
POST   /api/v1/tables/:id/shares           生成 / 获取只读链接（幂等）
DELETE /api/v1/tables/:id/shares           关闭分享
GET    /api/v1/public/specs/:token         公开只读，无需鉴权

# 模板（结构先留，P3 开放）
GET    /api/v1/templates
POST   /api/v1/tables/:id/apply-template   { template_id, mode: append|replace }
```

> **要点**：手工编辑和 AI 编辑走**同一个 `apply` 执行器**。校验、版本、撤销、审计只有一套，不会出现「AI 改的能撤销、手改的不能」这种割裂。

### SSE 事件协议

```
event: delta        {"text":"好的，我看到当前有 12 个参数…"}
event: tool_call    {"id":"c1","name":"search_items","args":{"keyword":"电池"}}
event: tool_result  {"id":"c1","count":1}
event: proposal     {"proposal_id":"p1","summary":"…","diff":{…},"mode":"auto"}
event: applied      {"revision":13,"proposal_id":"p1","changed_item_ids":[42]}
event: rejected     {"proposal_id":"p2","reason":"待用户确认","diff":{…}}
event: error        {"message":"字段名重复"}
event: done         {"usage":{"prompt":1200,"completion":300}}
```

---

## 5. 前端设计

### 三栏工作区（Canvas / Artifacts 形态）

```
┌──────────┬──────────────────────┬──────────────────────────┐
│ 参数表列表│  对话流               │  参数表画布               │
│          │                      │                          │
│ + 新建    │ ┌────────────────┐   │  分组：续航      [+ 参数]  │
│          │ │ 用户消息        │   │  ┌──────┬─────┬────┐    │
│ ○ 手机A1  │ └────────────────┘   │  │参数名 │参数值    │    │
│ ○ 耳机B2  │ ┌────────────────┐   │  │电池容量│5000│mAh │✦  │
│ ○ 音箱C3  │ │ AI：已更新…     │   │  │充电功率│45  │W   │    │
│          │ │ ▸ 改 1 项  撤销  │   │  └──────┴─────┴────┘    │
│          │ └────────────────┘   │                          │
│          │ ┌────────────────┐   │  [撤销] [重做] [r13]     │
│          │ │ 输入框…    ↑   │   │                          │
│          │ └────────────────┘   │                          │
└──────────┴──────────────────────┴──────────────────────────┘
```

满足「界面跟主流 AI 页面一样」，同时表格常驻可编辑。窄屏时画布收为抽屉。

### 交互细节（决定质感的地方）

1. **流式打字机** —— 首 token 尽快返回
2. **工具调用可视化** —— 显示「正在检索参数…」，不干等
3. **改动高亮** —— `applied` 后对应行暖色闪烁 2 秒淡出（靠 `changed_item_ids` 精准定位）
4. **一键撤销** —— 挂在 AI 消息下方，不必进历史面板
5. **分组折叠** —— 参数多时按 `group_name` 分组折叠
6. **乐观更新** —— 手工编辑立即反映，失败回滚
7. **空态引导** —— 新表给示例 prompt：「帮我建一张手机参数表」「加一个充电功率 45W」

### 技术选型

对齐团队既有栈（`d-ticket/web`），避免维护两套写法：

| 层 | 选择 | 理由 |
|----|------|------|
| 构建 | Vite 7 + Vue 3.5 + TS | 与既有项目一致 |
| UI | naive-ui | 与既有项目一致；聊天区需要的定制部分用局部 SCSS 覆盖 |
| 样式 | SCSS | 不引入 Tailwind，避免与团队既有范式并存 |
| 状态 | Pinia | 对话面板与表格画布之间需共享状态，纯路由不够用 |
| 图标 | remixicon | 与既有项目一致 |
| 表格 | 自建 | 参数表只有参数名 / 参数值两列，用不上 headless 表格库 |
| Markdown | markdown-it + shiki | Phase 2 渲染 AI 回复 |
| SSE | `@microsoft/fetch-event-source` | 原生 EventSource 不支持 POST + header |

后端：Go 1.25 / Gin / GORM + pgx / PostgreSQL 18 / Redis。

> **落地时的调整**：规划早期拟用 Tailwind + TanStack Table，实际改为对齐团队既有栈。
> 理由是 `d-ticket/web` 已经用 naive-ui + SCSS，引入第二套样式范式对长期维护不利；
> 而参数表的列是固定的，也不构成对 headless 表格库的需求。

### 端口与依赖

| 用途 | 地址 |
|------|------|
| 后端 | `:8300` |
| 前端 dev | `:5173`（Vite 代理 `/api` → 8300） |
| PostgreSQL | 复用 `~/Sites/docker/postgres18` 共享实例，库名 `dspecs` |
| Redis | 复用共享实例，`db0` |

### 目录结构

```
cmd/api/main.go
internal/
  config/  domain/  repository/  service/  handler/  middleware/
  ai/
    provider/   DeepSeek 客户端（OpenAI 兼容协议）
    agent/      agent loop：读工具 → 提案 → 校验 → 回灌重试
    tools/      tool schema 与实现
    ops/        operation 解析/校验/应用（纯函数，重点单测）
    context/    上下文构建与压缩
migrations/
pkg/            数值校验、diff、jsonb 辅助
```

```
src/
  views/        LoginView / TableListView / WorkspaceView / ShareView
  components/
    chat/       MessageList / StreamingText / Composer / ToolCallCard / ChangeSummary
    table/      SpecCanvas / ItemTable / GroupSection / ItemEditor / DiffHighlight
  stores/       auth / specTable / chat / ui
  api/          http.ts / sse.ts
  composables/  useSSE / useUndoRedo
```

---

## 6. 路线图（首版已全部完成）

### Phase 0 —— 骨架 ✅ 已完成
- 前后端脚手架（后端派生自 `go-template`，前端对齐 `d-ticket/web` 技术栈）
- 迁移框架 + 首个迁移：`users` / `spec_tables` / `spec_items` / `spec_revisions` / `share_links`
- 注册登录（JWT）、`user_id` 隔离基线、统一的中文错误文案
- 统一响应格式、日志、request id、panic recover
- AI 层：DeepSeek provider + `ops` 校验协议 + tool 定义 + agent loop
- **AI 连通性验证工具** `cmd/aicheck` —— 已实测通过，结论见 3.9
- 单测 94 例（`ops` 校验器与数值解析为重点），接口用例 60 项
- 前端：登录页（已接通后端）、参数表列表页、工作区三栏界面预览、分享只读页

> AI 层与验证工具提前到 P0，是因为决策 4 只接 Flash、决策 5 首版到 P2 ——
> tool calling 的可行性是整个 P2 的前提，不能等到 P2 才发现要换方案。
>
> 同理，`ops` 校验协议与 agent loop 虽属 P2 逻辑，也提前到 P0 落地，
> 目的是让 `make ai-check` 能跑**真实的工具链路**，而不是只验证接口能不能调通。

### Phase 1 —— 参数表 CRUD ✅ 已完成
- `spec_tables` / `spec_items` 完整 CRUD，`Repository` 签名强制 `user_id` 隔离
- 统一 `apply` 执行器接入数据库：乐观锁抢版本号 + revision 快照 + 参数项 upsert
- **恢复版本**作为撤销/重做的底层原语（把目标快照写成新版本，历史保持线性）
- **复制表**（换产品改几个值，最高频操作）
- 前端：列表页接真数据、新建/复制/重命名/删除、画布行内编辑、撤销重做
- 集成测试 11 项覆盖跨用户越权；接口用例 20 项

> Phase 1 刻意不接 AI：先把「没有 AI 也能用」的表格做扎实。AI 只是它的一个输入方式，不是地基。

**几个落地时的判断：**

| 决定 | 理由 |
|------|------|
| 越权一律返回 404 而非 403 | 403 等于告诉攻击者这个 id 真实存在 |
| 参数项同步用「upsert + 删多余」而非全删重插 | 保住已有项的 `created_at`，也让 id 在多次变更间保持稳定 |
| 空表也写 revision 0 | 让「恢复到任意版本」有统一起点，不必特判「那时还没有快照」 |
| 恢复 = 写新版本，不回退版本号 | 历史线性且不可变，撤销本身也可再撤销 |
| 前端变更后重新拉取而非本地打补丁 | 值的归一化（`5000mAh` → `5000` + `mAh`）只有服务端说了算 |
| 单元测试直连真实数据库，连不上则 skip | 越权、乐观锁、jsonb 这些行为只有真库能验，mock 会掩盖问题 |

### Phase 2 —— AI 对话 + 极简分享 ✅ 已完成
- `POST /api/v1/tables/:id/chat`，SSE 流式返回 agent 事件
- `agent.ApplyHook`：AI 产出的 ops 交给 `spec.Service.Apply` 落库，
  与手工变更共用同一套校验、乐观锁、版本快照与撤销
- 会话与消息落库（`chat_sessions` / `chat_messages` / `ai_operations`）
- 前端：自研 SSE 客户端（POST + Bearer）、流式打字、工具调用可视化、
  改动摘要卡片、一键撤销
- 对话 ↔ 画布联动：`applied` 到达后刷新画布并高亮变化项
- **极简分享**：`share_links` + `GET /public/specs/:token` + 独立只读页
- 接口用例 32 项（分享 15 + 对话 12 + 其它）

**几个落地时的判断：**

| 决定 | 理由 |
|------|------|
| HTTP 状态码只在建流之前有意义 | 一旦开始 SSE 输出状态码就固定 200，所以「表不存在 / 消息为空 / AI 未配置」全部在写头之前校验，以正常 JSON 返回 |
| agent 用 ApplyHook 委托落库，而不是自己写库 | agent 保持可单测（`cmd/aicheck` 用内存快照），同时让数据库在对话过程中就是唯一事实源；409 冲突能作为工具错误回灌给模型重试 |
| 历史回放只带 user / assistant 文本，不回放 tool 回合 | 工具返回的是「当时那张表」，塞回上下文既占 token 又可能与当前状态冲突 |
| 自研 SSE 客户端，不引 `@microsoft/fetch-event-source` | 只需要「按空行切块 + 取 event/data」这点功能，几十行换掉一个依赖 |
| 分享 token 关闭后复用而非重新生成 | 否则「关闭再打开」会让已经发出去的链接失效 |
| 公开接口返回独立的精简结构 | 表 id 与参数项 id 对展示毫无用处，不该暴露给拿到链接的外部访客 |

### Phase 3+ —— 延后
模板库与套用 · 导出 xlsx/pdf · 组织与 RBAC · 多人协作 · 分享增强（密码 / 有效期）· 用量看板

---

## 7. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| **Flash tool calling 不稳定** | P2 全盘受阻 | ✅ 已实测通过，无需切换退路（3.9） |
| **Flash 是推理模型，思考挤占正文预算** | 回复为空 | `AI_MAX_TOKENS` 默认 8192；正文为空且 `finish_reason=length` 时显式提示（agent） |
| **Flash 在批量/条件指令上跑偏** | 漏改、错改 | 服务端展开条件式语义（3.6）+ 错误回灌 + 一键撤销 |
| AI 幻觉 item / 值 | 数据被污染 | 严格校验 + 全量审计（`ai_operations`）+ revision 快照 |
| 上下文超限（参数多） | AI 不可用 | schema-only 提示 + 只读工具按需取数（3.7） |
| 并发编辑冲突 | 数据覆盖 | `current_revision` 乐观锁，409 让前端提示刷新 |
| 数值写法不规范（如「约五千」写进数值列） | 参数不准 | `number` 只校验不改写；不合格时报错并提示改用 text |
| 手工 / AI 行为不一致 | 逻辑割裂 | 共用同一个 `apply` 执行器（第 4 节） |
| 自动应用误改用户数据 | 信任受损 | revision 快照 + 一键撤销 + 安全阀 + `ai_operations` 可追溯 |
| 用户数据越权访问 | 严重事故 | repository 层强制 `user_id` 过滤 + 跨用户访问集成测试 |
| 分享链接被爬 / 被索引 | 数据外泄 | token 用足够熵的随机串（不可枚举）；只读页带 `noindex`；关闭分享即时失效 |

---

## 8. 关键认知小结

1. **行=参数项**，不是列=参数 —— 这是「非常多产品 + 模板」两个需求推导出的唯一合理模型
2. **AI 只输出 ops，绝不整表覆盖** —— 一条铁律，破了就是数据事故
3. **校验器比 prompt 重要** —— 只用 Flash 的前提下，可靠性来自服务端而非模型
4. **手工与 AI 共用 apply** —— 一套执行器，避免所有行为割裂
5. **模板是模型的免费副产品** —— 行=参数项的建模让模板只是「一份清单」
6. **只读页必须独立打包** —— 分享页不该加载编辑器与对话面板，这是首版就要守住的边界

---

## 8.1 ID 方案

实体 ID 用 **UUID v7**（RFC 9562），36 字符标准写法（带连字符）。

选 v7 而不是 v4：前 48 位是毫秒时间戳，天然按创建时间有序，作为主键时
B-tree 插入集中在右侧而不是随机打散，页分裂更少；后 74 位随机保证不可枚举。

**分享 token 刻意不用 UUID**：token 是「持有即可访问」的凭证，不是实体标识。
带上时间戳既泄露创建时间，又让前 48 位可预测，对凭证来说是负分 —— 那里要的是
纯随机性，继续用 nanoid。

> 迁移 `202609110004` 把全部 ID 列从 `VARCHAR(32)` 加宽到 36。
> 只加宽不重写：老的 20 位 nanoid 继续有效，两者混存不影响任何逻辑。
> 若要统一格式，需另做一次重写 ID 的迁移，代价是 JWT 的 `sub` 就是 user id ——
> 重写会让所有人重新登录。

---

## 9. 下一步

首版范围（Phase 0 / 1 / 2 + 极简分享）已全部完成并通过验证。
产品现在可以真实使用：注册登录 → 建表 → 手工编辑或对话让 AI 改 → 撤销 → 分享出去。

后续可做的方向，按性价比排序：

1. **模板库**（P3 里最值钱的一项）
   模型是「行=参数项」，模板就是一份参数项清单，套用时直接生成待填条目。
   对「非常多不同产品」这个核心场景，这是最直接的效率提升。
2. **导出 xlsx / pdf**
   企业侧拿参数表去对外报价、投标时几乎必然需要，实现成本不高。
3. **分享增强**：密码保护、有效期、访问统计
   `share_links` 表已经预留了扩展位。
4. **多人协作**：Yjs + WebSocket
   独立课题，建议先用真实用户量验证「同表并发编辑」确实是痛点再投入。

**暂时不建议做的：**

- **给 AI 加更多工具**（如导出、批量导入）。当前 4 个只读工具 + 1 个写工具已经
  覆盖了实测的全部场景，加工具会持续抬高每轮的 prompt 成本（工具定义在每轮
  都要重发），除非有明确的新场景，否则不值得。
- **换更强的模型档位**。`ai-check` 显示 Flash 四个场景一次通过、缓存命中 90%，
  没有证据表明质量是瓶颈。真遇到困难任务，先看是不是 system prompt 或
  校验器的反馈不够清楚。

> 唯一还没被真正压测的是**校验回灌回路**：模型表现太好，至今没有触发过
> 一次失败重试。拿几十项的真实参数表去压一压，是下一阶段最值得做的验证。

---

## 附：本地开发

不另起数据库容器，复用已有的共享实例（`~/Sites/docker/postgres18`），
每个项目一个库，与 `d_post` / `d_saas` 的做法一致：

```bash
docker exec -e PGPASSWORD=postgres postgres18 \
  psql -h 127.0.0.1 -U postgres -d postgres -c "CREATE DATABASE dspecs"
```

```bash
# 后端
cd api && cp .env.example .env && make migrate && make dev     # :8300
# 前端
cd web && npm run dev                                          # :5173
```

根目录 `make help` 列出全部常用命令。

### 两处环境适配

| 情况 | 处理 |
|------|------|
| Go 构建缓存在 `~/Library/Caches` 无写权限 | `GOCACHE` 固定为项目内 `.gocache/`（Makefile 已导出） |
| npm 缓存在 `~/.npm` 无写权限 | `npm install --cache ./.npm-cache`（已加入 .gitignore） |
