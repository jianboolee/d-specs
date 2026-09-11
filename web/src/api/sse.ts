import { ApiError, getToken } from './http'

export interface SSEEvent {
  event: string
  data: string
}

export interface SSEOptions {
  onEvent: (event: SSEEvent) => void
  signal?: AbortSignal
}

/**
 * 用 POST + fetch 读 SSE 流。
 *
 * 不用原生 EventSource：它只能发 GET，也没法带 Authorization 头，
 * 而我们的对话接口是 POST + Bearer Token。
 * 这里只实现用得到的部分：按空行切事件块，取 event / data 两类字段。
 */
export async function postSSE(url: string, body: unknown, options: SSEOptions): Promise<void> {
  const headers = new Headers({ 'Content-Type': 'application/json' })
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  let response: Response
  try {
    response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: options.signal,
    })
  } catch (err) {
    if (isAbort(err)) throw err
    throw new ApiError('网络异常，请检查后端服务是否已启动', -1, 0)
  }

  // 后端在建立流之前失败（表不存在、AI 未配置）会返回正常的 JSON 错误
  if (!response.ok) {
    let message = `请求失败（HTTP ${response.status}）`
    try {
      const payload = (await response.json()) as { message?: string }
      if (payload?.message) message = payload.message
    } catch {
      // 响应不是 JSON，用上面的兜底文案
    }
    throw new ApiError(message, response.status, response.status)
  }

  if (!response.body) {
    throw new ApiError('当前环境不支持流式响应', -1, 0)
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })

      let boundary = buffer.indexOf('\n\n')
      while (boundary !== -1) {
        const block = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)

        const parsed = parseBlock(block)
        if (parsed) options.onEvent(parsed)

        boundary = buffer.indexOf('\n\n')
      }
    }
  } finally {
    reader.releaseLock?.()
  }
}

function parseBlock(block: string): SSEEvent | null {
  let event = 'message'
  const dataLines: string[] = []

  for (const line of block.split('\n')) {
    if (line.startsWith(':')) continue // 心跳注释
    if (line.startsWith('event:')) {
      event = line.slice(6).trim()
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).replace(/^ /, ''))
    }
  }

  if (dataLines.length === 0) return null
  return { event, data: dataLines.join('\n') }
}

function isAbort(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}
