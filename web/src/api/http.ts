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

/** 下载结果：文件名（可能来自响应头）与内容 */
export interface DownloadResult {
  blob: Blob
  filename: string
}

/**
 * 下载文件（如导出的 xlsx）。
 *
 * 不能复用 request()：它恒按 JSON 信封解析，遇到二进制会抛错。
 * 也不能用 <a href> 直接指到接口：鉴权走 Authorization 头，
 * 而浏览器发起链接跳转时带不上这个头，所以必须 fetch + Blob。
 */
export async function download(path: string): Promise<DownloadResult> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, { headers })
  } catch {
    throw new ApiError('网络异常，请检查后端服务是否已启动', -1, 0)
  }

  if (response.status === 401) {
    clearToken()
    onUnauthorized?.()
    throw new ApiError('登录已过期，请重新登录', 401, 401)
  }

  // 失败时后端仍返回 JSON 信封，不能把它当成文件存下来
  const contentType = response.headers.get('Content-Type') ?? ''
  if (contentType.includes('application/json')) {
    try {
      const payload = (await response.json()) as ApiResponse<unknown>
      throw new ApiError(payload.message || '导出失败', payload.code, response.status)
    } catch (err) {
      if (err instanceof ApiError) throw err
      throw new ApiError(`导出失败（HTTP ${response.status}）`, -1, response.status)
    }
  }

  if (!response.ok) {
    throw new ApiError(`导出失败（HTTP ${response.status}）`, -1, response.status)
  }

  return {
    blob: await response.blob(),
    filename: filenameFromHeader(response.headers.get('Content-Disposition')),
  }
}

/**
 * 从 Content-Disposition 解析文件名。
 *
 * 优先取 RFC 5987 的 filename*（中文表名走这里，是 UTF-8 百分号编码），
 * 回退到 ASCII 的 filename=；都没有则返回空串，由调用方兜底命名。
 */
function filenameFromHeader(header: string | null): string {
  if (!header) return ''

  const extended = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (extended?.[1]) {
    try {
      return decodeURIComponent(extended[1])
    } catch {
      // 编码异常时退回下面的 ASCII 分支
    }
  }

  const plain = /filename="?([^";]+)"?/i.exec(header)
  return plain?.[1]?.trim() ?? ''
}
