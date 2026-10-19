// d-specs 认证 API：只跟自己的后端通信，前端不直连 d-auth（登出除外）。
// 登录走 d-auth 的中央登录页（SSO）整页跳转；登录态由 d-auth 下发的
// `dauth_access` cookie 承载（值是可本地验签的 JWT，但 JS 读不到），
// 因此这里不出现任何 token。
import { request } from './http'
import type { UserProfile } from '@/types/api'

// SSOConfig 是登录跳转所需的 d-auth 地址与应用标识（由后端 /auth/config 下发，
// 避免前端硬编码 d-auth 域名）。
export interface SSOConfig {
  sso_base_url: string
  client_key: string
}

export const authApi = {
  /** SSO 配置：d-auth origin 与 client_key。 */
  config: () => request<SSOConfig>('/auth/config'),

  /** 会话探针：未登录返回 { user: null }，不抛错（适合启动时判登录态）。 */
  session: () => request<{ user: UserProfile | null }>('/auth/session'),

  /** 当前用户资料（以 d-auth 为源，脱敏）。 */
  me: () => request<UserProfile>('/me'),
}