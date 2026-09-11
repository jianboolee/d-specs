import { request } from './http'
import { postSSE } from './sse'
import type { ChatEvent, ChatMessageRecord, ChatSession } from '@/types/api'

export interface StreamChatInput {
  sessionId?: string
  message: string
}

export interface StreamChatOptions {
  onEvent: (event: ChatEvent) => void
  signal?: AbortSignal
}

/**
 * 发起一轮对话，事件通过 onEvent 增量推来。
 *
 * 走 SSE 而不是普通请求：整轮对话含多轮工具调用，通常要几秒；
 * 流式能让「正在检索参数」「已修改 2 处」这类中间状态立刻可见。
 */
export function streamChat(tableId: string, input: StreamChatInput, options: StreamChatOptions) {
  return postSSE(
    `/api/v1/tables/${tableId}/chat`,
    { session_id: input.sessionId ?? '', message: input.message },
    {
      signal: options.signal,
      onEvent: ({ event, data }) => {
        // 事件名与 payload.type 一致；以 payload 为准，避免两端不一致时静默丢事件
        try {
          options.onEvent(JSON.parse(data) as ChatEvent)
        } catch {
          options.onEvent({ type: 'error', message: `无法解析事件 ${event}` })
        }
      },
    },
  )
}

export function listSessions(tableId: string) {
  return request<ChatSession[]>(`/tables/${tableId}/sessions`)
}

export function listMessages(sessionId: string, limit = 100) {
  return request<ChatMessageRecord[]>(`/sessions/${sessionId}/messages`, { query: { limit } })
}
