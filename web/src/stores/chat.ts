import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as chatApi from '@/api/chat'
import { ApiError } from '@/api/http'
import { useSpecTableStore } from './specTable'
import type { ChatChange, ChatEvent, ChatSession } from '@/types/api'

/** 界面上的一条消息 */
export interface UiMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  /** 正在流式接收中 */
  streaming?: boolean
  /** 正在执行的工具，例如「search_items」 */
  toolLabel?: string
  /** 本次落地的变更 */
  changes?: ChatChange[]
  /** 落地后产生的版本号 */
  appliedRevision?: number
  /** 被安全阀拦下、等待确认 */
  pending?: string
  error?: string
  /** 已撤销 */
  undone?: boolean
  /**
   * 从历史记录恢复的消息。
   *
   * 历史里只存了文本，没有当时对应的 revision，所以不提供「撤销这一条」——
   * 要回退请用画布上的撤销（它是按版本走的）。
   */
  restored?: boolean
}

/** 工具名的中文说明，让「正在检索参数」比裸工具名好懂 */
const TOOL_LABELS: Record<string, string> = {
  get_schema: '正在查看参数表结构…',
  search_items: '正在检索参数…',
  get_items: '正在读取参数…',
  count_items: '正在统计参数数量…',
  propose_ops: '正在应用改动…',
}

export const useChatStore = defineStore('chat', () => {
  const messages = ref<UiMessage[]>([])
  const sessions = ref<ChatSession[]>([])
  const sessionId = ref('')
  const sending = ref(false)
  const loadingHistory = ref(false)
  const lastError = ref('')

  let controller: AbortController | null = null
  let seq = 0
  /** 已拉取会话列表的表，避免重复请求 */
  let sessionsTableId = ''

  const canSend = computed(() => !sending.value)
  const isEmpty = computed(() => messages.value.length === 0)
  const hasHistory = computed(() => sessions.value.length > 0)

  const currentSession = computed(() =>
    sessions.value.find((s) => s.id === sessionId.value),
  )

  function nextId(prefix: string) {
    seq += 1
    return `${prefix}-${Date.now()}-${seq}`
  }

  /** 把一段文字追加到消息上；分段之间补一个空行，避免与已有内容粘在一起 */
  function appendText(msg: UiMessage, chunk: string) {
    const text = chunk.trim()
    if (!text) return
    msg.text = msg.text ? `${msg.text}\n\n${text}` : text
  }

  // ---------- 会话历史 ----------

  /**
   * 打开某张表时调用：拉会话列表，并自动恢复最近一次对话。
   *
   * 拉取失败不抛错也不挡路 —— 用户至少还能开始一段新对话。
   */
  async function initForTable(tableId: string) {
    reset()
    sessionsTableId = tableId
    try {
      sessions.value = await chatApi.listSessions(tableId)
      const latest = sessions.value[0]
      if (latest) {
        await openSession(latest.id)
      }
    } catch {
      sessions.value = []
    }
  }

  async function refreshSessions() {
    if (!sessionsTableId) return
    try {
      sessions.value = await chatApi.listSessions(sessionsTableId)
    } catch {
      // 列表刷新失败不影响对话本身
    }
  }

  async function openSession(id: string) {
    if (id === sessionId.value) return

    abort()
    loadingHistory.value = true
    try {
      const rows = await chatApi.listMessages(id)
      messages.value = rows.map((row) => ({
        id: row.id,
        role: row.role,
        text: row.content,
        restored: true,
      }))
      sessionId.value = id
    } catch (err) {
      lastError.value = err instanceof ApiError ? err.message : '历史对话加载失败'
    } finally {
      loadingHistory.value = false
    }
  }

  /** 开始一段新对话 */
  function newSession() {
    abort()
    sessionId.value = ''
    messages.value = []
  }

  // ---------- 发送 ----------

  async function send(text: string) {
    const content = text.trim()
    if (!content || sending.value) return

    const spec = useSpecTableStore()
    if (!spec.table) {
      lastError.value = '请先打开一张参数表'
      return
    }

    const revisionBefore = spec.table.current_revision

    messages.value.push({ id: nextId('u'), role: 'user', text: content })

    const assistant: UiMessage = {
      id: nextId('a'),
      role: 'assistant',
      text: '',
      streaming: true,
    }
    messages.value.push(assistant)

    sending.value = true
    lastError.value = ''
    controller = new AbortController()

    try {
      await chatApi.streamChat(
        spec.table.id,
        { sessionId: sessionId.value, message: content },
        {
          signal: controller.signal,
          onEvent: (event) => handleEvent(event, assistant, revisionBefore),
        },
      )
    } catch (err) {
      // 用户主动停止不算错误
      if (!(err instanceof DOMException && err.name === 'AbortError')) {
        const message = err instanceof ApiError ? err.message : '对话失败，请稍后重试'
        assistant.error = message
        lastError.value = message
      }
    } finally {
      assistant.streaming = false
      assistant.toolLabel = undefined
      sending.value = false
      controller = null
      // 首次对话会新建会话，刷新列表让切换器里的条目与消息数保持准确
      void refreshSessions()
    }
  }

  function handleEvent(event: ChatEvent, assistant: UiMessage, revisionBefore: number) {
    switch (event.type) {
      case 'start':
        if (event.session_id) sessionId.value = event.session_id
        break

      case 'text':
        if (event.text) appendText(assistant, event.text)
        break

      case 'tool_call':
        assistant.toolLabel = TOOL_LABELS[event.tool_name ?? ''] ?? `正在调用 ${event.tool_name}…`
        break

      case 'tool_result':
        assistant.toolLabel = undefined
        break

      case 'applied':
        assistant.toolLabel = undefined
        assistant.changes = event.changes ?? []
        assistant.appliedRevision = event.revision
        void syncCanvas(revisionBefore, event.changes ?? [])
        break

      case 'pending':
        assistant.pending = event.message ?? '本次改动较大，等待确认'
        assistant.changes = event.changes ?? []
        break

      case 'done':
        assistant.toolLabel = undefined
        if (event.reply) appendText(assistant, event.reply)
        if (!assistant.text) assistant.text = '（模型没有返回文字说明）'
        break

      case 'error':
        assistant.error = event.message ?? 'AI 处理失败'
        lastError.value = assistant.error
        break
    }
  }

  /** 对话改了数据之后，把画布同步到最新状态并高亮变化项 */
  async function syncCanvas(revisionBefore: number, changes: ChatChange[]) {
    const spec = useSpecTableStore()
    try {
      await spec.refreshItems()
      spec.noteExternalChange(revisionBefore)
      spec.flash(changes.map((c) => c.item_id))
    } catch {
      // 画布刷新失败不影响对话本身，用户手动刷新即可
    }
  }

  /** 停止当前的流式回复 */
  function abort() {
    controller?.abort()
    controller = null
    sending.value = false
  }

  /**
   * 撤销某条 AI 消息带来的改动。
   *
   * 复用画布的撤销逻辑（restore 到上一个版本），并把消息标记为已撤销，
   * 让对话记录与表格状态保持一致。
   */
  async function undoMessage(messageId: string) {
    const spec = useSpecTableStore()
    const msg = messages.value.find((m) => m.id === messageId)
    if (!msg || msg.restored || !spec.canUndo) return false

    const res = await spec.undo()
    if (res) {
      msg.undone = true
      return true
    }
    return false
  }

  function reset() {
    abort()
    messages.value = []
    sessions.value = []
    sessionId.value = ''
    lastError.value = ''
    sessionsTableId = ''
  }

  return {
    messages,
    sessions,
    sessionId,
    currentSession,
    sending,
    loadingHistory,
    lastError,
    canSend,
    isEmpty,
    hasHistory,
    initForTable,
    refreshSessions,
    openSession,
    newSession,
    send,
    abort,
    undoMessage,
    reset,
  }
})
