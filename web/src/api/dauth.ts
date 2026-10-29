// d-auth 续期：refresh token 在 `dauth_refresh` cookie（Path=/v1/auth）里，
// d-specs 后端因 Path 隔离收不到，所以只能由浏览器直接向 d-auth 发起。
// 整个过程不把 token 暴露给 JS：cookie 仍是 HttpOnly，前端只负责触发轮换。

import type { SSOConfig } from './auth'

const DEFAULT_BASE_URL = 'https://auth.x-rv.com'
const DEFAULT_CLIENT_KEY = 'web_specs'
const SESSION_HINT_KEY = 'dauth_session_hint'

let cachedConfig: SSOConfig | null = null
let refreshPromise: Promise<boolean> | null = null

/** 认证接口下发 SSO 配置后缓存下来，续期时不必再请求一次 /auth/config。 */
export function cacheSSOConfig(cfg: SSOConfig | null | undefined) {
  if (cfg?.sso_base_url && cfg?.client_key) cachedConfig = cfg
}

export function markDAuthSession() {
  try {
    localStorage.setItem(SESSION_HINT_KEY, '1')
  } catch {
    /* 隐私模式 / 存储被禁用：只是少一次静默续期，不影响登录 */
  }
}

export function hasDAuthSession() {
  try {
    return localStorage.getItem(SESSION_HINT_KEY) === '1'
  } catch {
    return false
  }
}

export function clearDAuthSession() {
  try {
    localStorage.removeItem(SESSION_HINT_KEY)
  } catch {
    /* 同上 */
  }
}

async function ssoConfig(): Promise<SSOConfig> {
  if (cachedConfig) return cachedConfig
  try {
    const res = await fetch('/api/v1/auth/config', { credentials: 'include' })
    if (res.ok) {
      const env = (await res.json()) as { data?: SSOConfig }
      if (env.data?.sso_base_url && env.data?.client_key) {
        cachedConfig = env.data
        return env.data
      }
    }
  } catch {
    /* 配置拉取失败：回退默认 d-auth 地址 */
  }
  return { sso_base_url: DEFAULT_BASE_URL, client_key: DEFAULT_CLIENT_KEY }
}

async function doRefresh(): Promise<boolean> {
  const cfg = await ssoConfig()
  let res: Response
  try {
    res = await fetch(`${cfg.sso_base_url.replace(/\/$/, '')}/v1/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
      // d-auth 对 cookie 凭据下的非 GET 请求要求 X-DAuth-Client（CSRF 门）。
      headers: { 'Content-Type': 'application/json', 'X-DAuth-Client': cfg.client_key },
      body: '{}',
    })
  } catch {
    // 网络失败：保留标记，下次再试，不把用户直接判成掉线。
    return false
  }
  if (res.ok) {
    markDAuthSession()
    return true
  }
  if (res.status === 401) clearDAuthSession()
  return false
}

/**
 * 用 HttpOnly refresh cookie 换一枚新的 access cookie。
 * 并发请求共用一次轮换：d-auth 的 refresh 是单次有效的，并发刷新会互相踩掉。
 */
export function refreshDAuthSession(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = doRefresh().finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}
