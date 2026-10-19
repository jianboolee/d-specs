# d-specs 对接 d-auth 账户中心（跨子域 SSO）

> 参考实现：d-note（`docs/dauth-integration.md`）。上游文档：d-auth 仓库
> [`docs/cookie.md`](https://github.com/jianboolee/d-auth/blob/main/docs/cookie.md)、
> [`docs/integration-go.md`](https://github.com/jianboolee/d-auth/blob/main/docs/integration-go.md)、
> [`docs/sso-return-to.md`](https://github.com/jianboolee/d-auth/blob/main/docs/sso-return-to.md)。

## 1. 拓扑与边界

```
浏览器（d-specs 前端）
   │  未登录：整页跳转 auth.x-rv.com 登录（带回跳）
   │  已登录：自动携带 `.x-rv.com` 共享 cookie（dauth_access = access JWT）
   │  登出：直接调 auth.x-rv.com（同站 XHR，带 dauth_refresh cookie）
   ▼
d-specs Go 后端 ──┬─► 本地验签 dauth_access（RS256 + JWKS，不查库、不请求 d-auth）
                  ├─► d-auth GET /v1/users/:id（API Key，取昵称/头像展示资料）
                  └─► d-auth GET /v1/.well-known/jwks.json（本地验签公钥）
```

- **d-specs 不维护会话表、不签发自己的 JWT/cookie、不代理登录**。登录态由
  d-auth 下发的 `dauth_access` cookie（值是可本地验签的 access JWT）承载，
  后端直接验签。
- **本站保留一个极简 `/login` 页**：只是入口占位 + 「登录」按钮（点击才跳 d-auth
  SSO），不做账号密码表单；登出后也停在这里。
- **⚠️ 部署域名必须落在 `x-rv.com` 下（`specs.x-rv.com`）**，与 `auth.x-rv.com`
  同属 `x-rv.com`，共享 cookie 才能生效。历史域名 `specs.21rv.com` 与
  `auth.x-rv.com` 不同源域，**物理上收不到该 cookie**，SSO 无法工作。
- 浏览器全程只跟 `specs.x-rv.com` 通信，前端**不直连 d-auth 的业务 API**；仅两处例外：
  登录**页面**跳转（地址由 `/api/v1/auth/config` 下发）、登出**接口**
  `POST {d-auth}/v1/auth/logout`。

### 为什么是本地验签 + API Key 取资料

d-auth 的 access token 是自证的 RS256 JWT，其读路径不查库，因此 d-specs 直接
验签即可，业务请求零网络依赖。但 JWT 里**只有 `sub` / `roles`**，没有昵称/头像——
展示资料改用一枚 `users:read` 的 API Key 按 `sub` 向 d-auth 的脱敏接口
`/v1/users/:id` 另取。两件事要分开看：

| 用途 | 来源 | 依赖 d-auth 在线？ |
| --- | --- | --- |
| 登录判定 + 授权 | 本地验签 JWT 的 `sub` | 否（仅拉 JWKS 时短暂依赖） |
| 展示（昵称/头像/掩码手机号） | API Key 调 `/v1/users/:id` | 是（失败降级为空，不阻断登录） |

## 2. d-specs 后端接口（前端可见的全部认证面）

| 方法 | 路径 | 鉴权 | 行为 |
| --- | --- | --- | --- |
| GET | `/api/v1/auth/config` | 公开 | 下发 `sso_base_url` 与 `client_key`，供前端拼登录跳转 |
| GET | `/api/v1/auth/session` | 公开 | 本地验签 cookie/Bearer；未登录 `{user:null}`，登录返回资料（API Key 取） |
| GET | `/api/v1/me` | 登录 | 登录主体资料（本地验签 + API Key 取展示字段） |

**登录不经过 d-specs 后端**：未登录访问需登录的页面时，前端停在本站 `/login` 页；
用户点「登录」才整页跳转 `{DAUTH_SSO_BASE_URL}/login?client={client_key}&return_to={目标地址}`。
登录成功后 d-auth 回跳 `return_to`。

**登出也不经过 d-specs 后端**：refresh token 在 `dauth_refresh` cookie 里，其 `Path` 被
d-auth 限死为 `/v1/auth`，后端收不到、也无法吊销。因此登出由**前端直接**调
`POST {DAUTH_SSO_BASE_URL}/v1/auth/logout`（同站 XHR，浏览器自动带 refresh cookie；带
`X-DAuth-Client` 头过 CSRF 门），随后整页跳回 `/login`。

业务路由（tables / share / chat）统一经 `middleware.AuthRequired`，凭证分流：

| 凭证 | 路径 | 场景 |
| --- | --- | --- |
| `dauth_access` cookie（`.x-rv.com` 共享，值为 access JWT） | 本地 RS256 + JWKS 验签 | 浏览器（主链路） |
| `Authorization: Bearer <d-auth access token>` | 本地 RS256 + JWKS 验签 | 小程序 / 其它客户端（token 模式） |

两条路径**共用同一套本地验签**。公钥取不到（JWKS 拉不动）统一返回 **503**，
避免把用户误判成未登录踢下线；token 无效 / 已过期 → **401**。

> access token 默认 1 小时过期。过期后前端会收到 401 → 回 `/login`；此时 d-auth 的
> 中心会话若仍有效，点一次「登录」即免登回跳（见 d-auth 的 `return-target` 免登场景）。
> **静默续期（`POST {d-auth}/v1/auth/refresh`）本期未做**，见 §8。

## 3. 代码落点

| 文件 | 职责 |
| --- | --- |
| `api/internal/app/app.go` | 组装验签器、资料客户端、CORS（`connectDAuth`） |
| `api/internal/middleware/auth.go` | 本地验签（cookie / Bearer）+ `VerifyLocal` 探针 |
| `api/internal/infrastructure/dauthclient/verifier.go` | access token 验签器（SDK `Verifier`，RS256 + JWKS） |
| `api/internal/infrastructure/dauthclient/profile.go` | 用 API Key 调 `/v1/users/:id` 取展示资料 |
| `api/internal/modules/auth/handler.go` | config / session / me 处理器 |
| `web/src/api/auth.ts` | 前端认证客户端（只打 `/api/v1/auth/*`；登出直连 d-auth） |
| `web/src/stores/auth.ts` | 登录态 store（探针 / SSO 跳转 / 直接调 d-auth 登出） |
| `web/src/views/LoginView.vue` | 本站极简登录页（点「登录」才跳 d-auth SSO） |
| `web/src/router/index.ts` | 路由守卫：未登录 → 本站 `/login`（不自动跳 SSO） |

> 展示资料走官方 gosdk 的 `Client.UserProfile`（需 `users:read` 密钥）；本地验签用
> `Verifier.VerifyToken`——浏览器 cookie 里的值就是 access JWT，可直接喂给它。

## 4. 环境变量（d-specs 侧）

```bash
DAUTH_BASE_URL=https://auth.x-rv.com        # 拉 JWKS 与取资料的 origin，不带 /v1
DAUTH_SSO_BASE_URL=https://auth.x-rv.com    # 浏览器登录跳转/登出用的 origin；留空回落 BASE_URL
DAUTH_CLIENT_KEY=web_specs                  # 子系统标识：登录 client= 参数
DAUTH_COOKIE_NAME=__Secure-dauth_access     # d-auth 承载 access JWT 的 cookie 名（须与 d-auth 一致）
DAUTH_API_KEY=sk-...                         # API Key（users:read），按 ID 取展示资料
DAUTH_JWT_ISSUER=https://auth.x-rv.com      # iss 校验；留空回落 BASE_URL（非密钥）
ALLOWED_ORIGINS=                            # 生产同源部署可留空
```

> `__Secure-` 前缀允许设 `Domain`（`__Host-` 不允许）；`DAUTH_COOKIE_NAME` 生产必须用
> `__Secure-` 前缀，否则 `DAUTH_COOKIE_DOMAIN=.x-rv.com` 无法生效。
> 生产缺 `DAUTH_BASE_URL` / `DAUTH_CLIENT_KEY` 会拒绝启动（fail-fast）；`DAUTH_API_KEY`
> 缺失只降级展示（昵称/头像为空），不影响登录。

## 5. 需要 d-auth 管理员 / 运维确认的事

生产（`auth.x-rv.com`）实测状态：

| 项 | 状态 |
| --- | --- |
| `DAUTH_COOKIE_DOMAIN=.x-rv.com`、`DAUTH_COOKIE_SECURE=true`、`DAUTH_COOKIE_NAME=__Secure-dauth_access` | ✅ 已配 |
| `DAUTH_JWT_ALG=RS256`（JWKS 有 RS256 公钥） | ✅ 已配 |
| `DAUTH_ALLOWED_ORIGINS` 含 `https://specs.x-rv.com` | ✅ 已配 |
| 子系统 client `web_specs` | ✅ 已登记 |
| API Key（`users:read`）供展示资料用 | ✅ 已建（托管在 `DAUTH_API_KEY`，不写入仓库） |
| **`DAUTH_ALLOWED_RETURN_ORIGINS` 含 `https://specs.x-rv.com`** | ⚠️ **待补**（当前只有 `https://note.x-rv.com`） |

**唯一待办**：把 `https://specs.x-rv.com` 加进 d-auth 生产的 `DAUTH_ALLOWED_RETURN_ORIGINS`
（多个 origin 用逗号分隔），然后重启 d-auth 容器。**不加的后果**：登录成功后 d-auth 不会
自动跳回本站，用户需手动点回；其余功能（本地验签、业务请求）不受影响。

```bash
# 业务机 47.95.176.70：
#   编辑 /opt/d-auth/.env.prod，把这行改为：
#     DAUTH_ALLOWED_RETURN_ORIGINS=https://note.x-rv.com,https://specs.x-rv.com
#   然后重启（注意：会短暂影响 note/d-camp 等所有子系统的登录）
cd /opt/d-auth && docker compose up -d
```

登出无需回跳：d-auth 按 `CookieDomain` 清两枚 cookie，d-specs 自然随之失效。

## 6. 生产部署（specs.x-rv.com）

与 d-note 同形态：入口机外层 nginx 终止 `*.x-rv.com` 通配证书并反代到业务机的
d-specs 容器；容器内 nginx 托管 SPA 并把 `/api/*` 剥前缀转给 Go（与本地 Vite 的
`/api` 代理同义）。

| 角色 | 地址 | 内容 |
| --- | --- | --- |
| 入口机 | 39.107.77.69（172.17.255.63） | 外层 nginx（`specs.x-rv.com.conf`）+ postgres18 + dauth/d-camp/d-note |
| 业务机 | 47.95.176.70（172.17.255.64） | d-specs 容器 `d-specs`，`0.0.0.0:8400` → 容器 80 |

请求链：`入口机 nginx → 172.17.255.64:8400 → docker → 容器 nginx:80 → 127.0.0.1:8401(Go)`。

一键发布（仓库根目录，需已 `docker login` ACR）：

```bash
make image        # 交叉构建 linux/amd64 并推 ACR registry.cn-beijing.aliyuncs.com/xrv/d-specs
make env-prod     # 首次：把 env.prod.example 铺到业务机 /opt/d-specs/.env.prod（不覆盖已填的）
#  → 编辑 /opt/d-specs/.env.prod（DATABASE_URL / DAUTH_* / DEEPSEEK_API_KEY / SITE_URL）
make deploy       # = image + deploy-only
make deploy-only  # 只拉镜像重启（TAG=<日期tag> 可回滚）
make nginx        # 下发入口机 vhost 并 reload
```

数据库：复用入口机 `postgres18:5432` 上的 `dspecs` 库（业务机经内网可达）。
容器启动时自动跑迁移（`RUN_MIGRATIONS=true`）。

生产 env 要点（完整模板见 `deploy/env.prod.example`）：`DAUTH_CLIENT_KEY=web_specs`、
`DAUTH_COOKIE_NAME=__Secure-dauth_access`、`DAUTH_API_KEY=sk-...`、`APP_ENV=production`、
`HTTP_PORT=8401`、`WEB_DIST_DIR=/usr/share/nginx/html`、`SITE_URL=https://specs.x-rv.com`、
`ALLOWED_ORIGINS` 留空（同源）。

## 7. 本地开发（注意 SSO 无法直接复现）

本地 d-auth 的 `DAUTH_COOKIE_DOMAIN` 为空（host-only），`localhost` 之间无法共享 cookie，
所以 SSO 主链路在纯 `localhost` 下跑不通。可选方案：

- **推荐**：用本地子域 + hosts，例如 `specs.local.test` / `auth.local.test` 都指向
  127.0.0.1，本地 d-auth 配 `DAUTH_COOKIE_DOMAIN=.local.test`，即可复现跨子域 cookie。
- 或临时保留一条 dev 专用登录路径（当前未实现，需要时再加）。
- **接口测试**（`api/test/`）不受影响：`sync-env.sh` 直连 d-auth 密码登录取 access
  token，用 `Authorization: Bearer` 本地验签，绕开 cookie。

本地起 d-specs 后端（8401）与前端（8402）；d-auth 见其 `docs/cookie.md` 本地说明。

## 8. 已知边界 / 后续

- **登录判定零网络**：业务请求只做本地验签（拉一次 JWKS 后按 kid 缓存），d-auth 抖动
  不影响参数表；只有展示资料（昵称/头像）需要 API Key 调 d-auth，失败时降级为空展示。
- **吊销窗口**：access JWT 在有效期内（默认 1h）始终验签通过，登出/改密/停用不会让它
  立刻失效——这是 d-auth 读路径纯验签的既定取舍。要更紧就调小 d-auth 的
  `DAUTH_JWT_ACCESS_TTL`。
- **改资料（昵称/头像）在 d-auth 账户中心完成**，本站无对应接口（原 `PATCH /me` 已移除）。
- **静默续期未做**：token 过期即 401 → 回 `/login`。要无缝续期需在前端拦截 401 后调
  `POST {d-auth}/v1/auth/refresh`（`credentials: include`）再重试；依赖 d-auth 的 refresh
  白名单，可在后续按需补。（d-note 当前也未做。）
- token 客户端（小程序）走 `Bearer` 本地验签，与浏览器同一条本地验签路径。
- 跨子域 cookie 依赖双方同属 `x-rv.com`；若部署到不同站点需改 `SameSite=None + Secure`
  并收紧 CORS（见 `ALLOWED_ORIGINS`）。