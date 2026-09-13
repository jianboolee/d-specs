import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'

import * as chatApi from '@/api/chat'
import { ApiError } from '@/api/http'
import { useSpecTableStore } from './specTable'
import type { ChatAction, ChatChange, ChatEvent, ChatSession } from '@/types/api'

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
  /**
   * 本轮回复的建议动作，点击即发送。
   *
   * 服务端不落库，刷新或切换会话后就没有了 —— 这里不需要特殊处理。
   */
  actions?: ChatAction[]
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
  /** 正在跑「完善参数表」分析（用于入口按钮的 loading 态） */
  const analyzing = ref(false)
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

  /**
   * 接收一段流式正文增量。
   *
   * 服务端按 token 推送 delta，直接拼在末尾即可；不做 trim，
   * 否则会吃掉正文里的空格与换行，破坏 Markdown 排版。
   */
  function appendText(msg: UiMessage, chunk: string) {
    if (!chunk) return
    msg.text += chunk
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

    await streamInto(
      (opts) =>
        chatApi.streamChat(
          spec.table!.id,
          { sessionId: sessionId.value, message: content },
          opts,
        ),
      revisionBefore,
    )
  }

  /**
   * 触发一轮「完善参数表」：让 AI 通读当前表，指出产品类型与还缺哪些关键参数。
   *
   * 不 push 用户消息 —— 这是系统发起的分析，不是用户发言。结果由服务端渲染成
   * Markdown 作为一条 assistant 消息流式返回，标签也在服务端落库。
   */
  async function analyze() {
    if (sending.value) return

    const spec = useSpecTableStore()
    if (!spec.table) {
      lastError.value = '请先打开一张参数表'
      return
    }

    const revisionBefore = spec.table.current_revision
    analyzing.value = true
    try {
      await streamInto(
        (opts) => chatApi.analyzeStream(spec.table!.id, { sessionId: sessionId.value }, opts),
        revisionBefore,
        '分析失败，请稍后重试',
      )
    } finally {
      analyzing.value = false
    }
  }

  /**
   * 跑一轮流式助手回复：建占位消息、接线事件、收尾复位。
   *
   * send 与 analyze 的差别只在「发什么请求、要不要先 push 用户消息」，
   * 其余（占位消息的响应式、错误处理、会话列表刷新）共用这一份实现。
   */
  async function streamInto(
    call: (opts: {
      signal: AbortSignal
      onEvent: (event: ChatEvent) => void
    }) => Promise<void>,
    revisionBefore: number,
    fallbackError = '对话失败，请稍后重试',
  ) {
    /*
     * 必须用 reactive() 包一层再 push。
     *
     * 直接把普通对象 push 进响应式数组时，数组里存的是原始对象；之后对这条
     * 原始引用（assistant.text += ...）的修改绕过了 Proxy，不会触发重新渲染。
     * 流式正文因此会一直积攒到别的响应式变化（如 sending 置回 false）才一次性
     * 显示出来 —— 那样就白流了。
     */
    const assistant = reactive<UiMessage>({
      id: nextId('a'),
      role: 'assistant',
      text: '',
      streaming: true,
    })
    messages.value.push(assistant)

    sending.value = true
    lastError.value = ''
    controller = new AbortController()

    try {
      await call({
        signal: controller.signal,
        onEvent: (event) => handleEvent(event, assistant, revisionBefore),
      })
    } catch (err) {
      // 用户主动停止不算错误
      if (!(err instanceof DOMException && err.name === 'AbortError')) {
        const message = err instanceof ApiError ? err.message : fallbackError
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
        // 正文已经由 text 增量拼好，这里只用 reply 兜底：
        // 例如服务端判定截断后直接给了兜底文案、或正文为空时的占位说明。
        if (!assistant.text.trim()) {
          assistant.text = event.reply?.trim() || '（模型没有返回文字说明）'
        }
        if (event.actions?.length) assistant.actions = event.actions
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
      const hasTableChange = changes.some((c) => c.kind === 'table')
      // 只有参数项改动才会推进版本号。表级改动（标题/描述）不产生版本，
      // 这里压栈会留下一个「撤销到同一版本」的空操作，因此跳过。
      if (changes.some((c) => c.kind !== 'table')) {
        spec.noteExternalChange(revisionBefore)
      }
      // 表级改动改了标题，侧栏列表用的是另一份数据（spec.tables），
      // 不刷新的话侧栏仍显示旧表名。
      if (hasTableChange) void spec.loadTables()
      // 表级变更是元信息，没有可高亮的参数项（item_id 为空）
      const itemIds = changes.map((c) => c.item_id).filter(Boolean)
      if (itemIds.length > 0) spec.flash(itemIds)
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
    analyzing.value = false
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
    analyzing,
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
    analyze,
    abort,
    undoMessage,
    reset,
  }
})
