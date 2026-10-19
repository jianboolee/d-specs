import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { authApi, type SSOConfig } from '@/api/auth'
import type { UserProfile } from '@/types/api'

// 登录态的唯一来源是 d-auth 下发的 `.x-rv.com` 共享会话 cookie（前端读不到内容），
// 因此这里只缓存「会话探针返回的资料」，不持有任何 token。
export const useAuthStore = defineStore('auth', () => {
  const user = ref<UserProfile | null>(null)
  /** 首次探测完成前为 true，避免路由守卫在刷新时误判为未登录 */
  const loading = ref(true)

  const isLoggedIn = computed(() => user.value !== null)

  /**
   * 会话探针：拉一次后端 /auth/session 判断是否已登录。
   * 未登录返回 { user: null }，不抛错（适合启动时调用）。
   */
  async function probe() {
    loading.value = true
    try {
      const res = await authApi.session()
      user.value = res.user ?? null
    } catch {
      user.value = null
    } finally {
      loading.value = false
    }
  }

  /**
   * 整页跳转到 d-auth 中央登录页（SSO），登录成功后由 d-auth 回跳本地址。
   * target 是登录后要回到的本站路径（默认当前地址）；会转成绝对 URL 交给 return_to。
   * 只在用户显式点击「登录」时调用——不要放进路由守卫自动触发。
   */
  async function loginWithSSO(target?: string) {
    const back = new URL(target || window.location.href, window.location.origin).href
    let cfg: SSOConfig | null = null
    try {
      cfg = await authApi.config()
    } catch {
      /* 配置拉取失败：回退到默认 d-auth 地址 */
    }
    const base = (cfg?.sso_base_url || 'https://auth.x-rv.com').replace(/\/$/, '')
    const client = cfg?.client_key || 'web_specs'
    const url = `${base}/login?client=${encodeURIComponent(client)}&return_to=${encodeURIComponent(back)}`
    window.location.href = url
  }

  /**
   * 登出：直接调 d-auth 的 `POST /v1/auth/logout`（同站 XHR，浏览器自动带上
   * `dauth_refresh` cookie——后端因 Path 隔离收不到它，无法代做）。
   * d-auth 会删掉续期白名单项并清两枚 `.x-rv.com` cookie。失败也清本地状态。
   *
   * 用完后必须整页跳转到 `/login`：`dauth_access` 是 HttpOnly 的，前端清不掉，
   * 只能靠 d-auth 的 Set-Cookie 让浏览器丢弃；整页刷新确保内存里的登录态归零。
   */
  async function logout() {
    let base = 'https://auth.x-rv.com'
    let client = 'web_specs'
    try {
      const cfg = await authApi.config()
      base = (cfg?.sso_base_url || base).replace(/\/$/, '')
      client = cfg?.client_key || client
    } catch {
      /* 配置拉取失败：回退默认 d-auth 地址与 client_key */
    }
    try {
      await fetch(`${base}/v1/auth/logout`, {
        method: 'POST',
        credentials: 'include',
        // d-auth 对 cookie 凭据下的非 GET 请求要求 X-DAuth-Client（CSRF 门）。
        headers: { 'Content-Type': 'application/json', 'X-DAuth-Client': client },
        body: '{}',
      })
    } catch {
      /* 网络失败：仍清本地状态，避免用户卡在「看似已登录」 */
    } finally {
      user.value = null
      window.location.href = '/login'
    }
  }

  return { user, loading, isLoggedIn, probe, loginWithSSO, logout }
})