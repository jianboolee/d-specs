# 接口测试（HTTP + CLI）

基于 [httpYac](https://httpyac.github.io/)，支持 IDE 点击发送与 CLI 批量执行。

## 日常使用流程

```bash
# 终端 1：先启动 API
make dev

# 终端 2：创建测试账号并把 token 写入 http-client.private.env.json
make http-sync

# CLI 跑接口
make http-test              # 全部
make http-test-auth         # 仅认证模块
```

**IDE 测单个请求**：安装 httpYac 插件 → 选环境 `dev` → 打开 `test/http/10-auth.http` → Send。

**首次使用**还需执行一次：`make http-install`

> `40-chat.http` 会真实调用 DeepSeek，每轮约 3~8 秒；只想跑快的用例时用
> `make http-test-auth` / `make http-test-tables` 之类的模块参数跳过它。

---

## 目录说明

```
test/http/                     按模块拆分的 .http 用例
test/scripts/                  sync-env.sh、run.sh
http-client.env.json           公共环境（可提交）
http-client.private.env.json   token 等（已 gitignore）
```

| 文件 | 模块 |
|------|------|
| `00-health.http` | 健康检查与依赖就绪（PostgreSQL / Redis / LLM） |
| `10-auth.http` | 注册 / 登录 / 当前用户，含越权与校验失败的反例 |
| `20-tables.http` | 配置表 CRUD、变更、乐观锁、版本恢复、复制 |
| `30-share.http` | 分享开启/关闭、公开读取、token 复用与失效 |
| `40-chat.http` | AI 对话（SSE），含改动落库与会话多轮续接 |

## 环境变量

| 变量 | 说明 |
|------|------|
| `baseUrl` | 默认 `http://localhost:8300` |
| `email` | 测试账号邮箱，默认 `demo@dspecs.dev` |
| `token` | `make http-sync` 自动填充 |
| `userId` | `make http-sync` 自动填充 |

`sync-env.sh` 支持用环境变量覆盖：

```bash
DS_BASE_URL=http://localhost:8300 DS_TEST_EMAIL=me@example.com make http-sync
```

## 新增接口

1. 在对应 `test/http/XX-模块.http` 增加 `###` 请求
2. 需登录：加 `Authorization: Bearer {{token}}`
3. 断言用 httpYac 语法（见下方注意事项）
4. `make http-test` 验证

> 后端约定：`code === 0` 为成功，非 0 时 `message` 是可直接展示的中文提示。
> 断言 `message` 的内容时注意保持与后端文案同步。

## 断言语法注意事项

httpYac 的断言有几个容易踩的坑，**写错了测试会静默通过**：

| 坑 | 说明 | 正确写法 |
|----|------|----------|
| 值加引号 | 引号会被当成值的一部分 | `?? body message == 请先登录` |
| `===` 被截断 | 解析器只认 `==`，`===` 会报 `== =` 错误 | `?? body code == 0` |
| **裸真值断言恒真** | `?? js response.parsedBody.data.token` 在字段**不存在**时也通过 | `?? js response.parsedBody.data.token != undefined` |
| `response.body` 是原始字符串 | 不是解析后的对象，取属性会得到 `undefined` | 用 `response.parsedBody` |

最后一条最危险：它会让测试看起来是绿的，实际什么都没验证。
本项目里所有 `js` 存在性断言一律写成 `!= undefined`。

数值比较可直接用 `?? body code == 401`；嵌套路径同样支持
（`?? body data.status == ok`）。
