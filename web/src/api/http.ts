import type { ApiResponse } from '@/types/api'

const TOKEN_KEY = 'dspecs.token'

let onUnauthorized: (() => void) | null = null

/** 注册 401 回调，由 auth store 在初始化时注入，避免 http 层依赖路由 */
export function setUnauthorizedHandler(handler: () => void) {
  onUnauthorized = handler
}

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

/** 业务错误：携带后端返回的 code，message 可直接展示给用户 */
export class ApiError extends Error {
  readonly code: number
  readonly status: number

  constructor(message: string, code: number, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  query?: Record<string, string | number | undefined>
}

/**
 * 统一请求封装。
 *
 * 后端约定：HTTP 状态码表达传输层结果，body.code 表达业务结果。
 * 这里把非 0 的 code 统一转成 ApiError，让调用方只处理异常分支。
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, query } = options

  let url = `/api/v1${path}`
  if (query) {
    const search = new URLSearchParams()
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined && value !== '') search.set(key, String(value))
    }
    const qs = search.toString()
    if (qs) url += `?${qs}`
  }

  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (body !== undefined) headers.set('Content-Type', 'application/json')

  let response: Response
  try {
    response = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError('网络异常，请检查后端服务是否已启动', -1, 0)
  }

  if (response.status === 401) {
    clearToken()
    onUnauthorized?.()
    throw new ApiError('登录已过期，请重新登录', 401, 401)
  }

  let payload: ApiResponse<T>
  try {
    payload = (await response.json()) as ApiResponse<T>
  } catch {
    throw new ApiError(`服务响应异常（HTTP ${response.status}）`, -1, response.status)
  }

  if (payload.code !== 0) {
    throw new ApiError(payload.message || '请求失败', payload.code, response.status)
  }

  return payload.data as T
}
