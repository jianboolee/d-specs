import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as authApi from '@/api/auth'
import { clearToken, getToken, setToken, setUnauthorizedHandler } from '@/api/http'
import type { UserProfile } from '@/types/api'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<UserProfile | null>(null)
  /** 是否已完成首次登录态恢复，避免刷新时守卫误判为未登录 */
  const ready = ref(false)

  const isLoggedIn = computed(() => user.value !== null)

  function applySession(token: string, profile: UserProfile) {
    setToken(token)
    user.value = profile
  }

  async function login(email: string, password: string) {
    const res = await authApi.login({ email, password })
    applySession(res.token, res.user)
    return res.user
  }

  async function register(email: string, password: string, name?: string) {
    const res = await authApi.register({ email, password, name })
    applySession(res.token, res.user)
    return res.user
  }

  function logout() {
    clearToken()
    user.value = null
  }

  /** 应用启动时用本地 token 换取用户资料 */
  async function restore() {
    if (!getToken()) {
      ready.value = true
      return
    }
    try {
      user.value = await authApi.fetchMe()
    } catch {
      clearToken()
      user.value = null
    } finally {
      ready.value = true
    }
  }

  // token 失效时（任意接口返回 401）同步清空本地登录态
  setUnauthorizedHandler(() => {
    user.value = null
  })

  return { user, ready, isLoggedIn, login, register, logout, restore }
})
