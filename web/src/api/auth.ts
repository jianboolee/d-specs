// d-specs 认证 API：只跟自己的后端通信；续期/登出除外（refresh cookie
// Path 限死 /v1/auth，只有浏览器直连 d-auth 才能带上）。
// 登录走 d-auth 的中央登录页（SSO）整页跳转；登录态由 d-auth 下发的
// `dauth_access` cookie 承载（值是可本地验签的 JWT，但 JS 读不到），
// 因此这里不出现任何 token。
import { request } from './http'
import { cacheSSOConfig } from './dauth'
import type { UserProfile } from '@/types/api'

// SSOConfig 是登录跳转所需的 d-auth 地址与应用标识（由后端 /auth/config 下发，
// 避免前端硬编码 d-auth 域名）。
export interface SSOConfig {
  sso_base_url: string
  client_key: string
}

export const authApi = {
  /** SSO 配置：d-auth origin 与 client_key。 */
  config: async () => {
    const cfg = await request<SSOConfig>('/auth/config')
    cacheSSOConfig(cfg)
    return cfg
  },

  /** 会话探针：未登录返回 { user: null }，不抛错（适合启动时判登录态）。 */
  session: () => request<{ user: UserProfile | null }>('/auth/session'),

  /** 当前用户资料（以 d-auth 为源，脱敏）。 */
  me: () => request<UserProfile>('/me'),
}
