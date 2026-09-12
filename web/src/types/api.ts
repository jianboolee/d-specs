/** 后端统一响应体 */
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data?: T
}

/** 用户资料 */
export interface UserProfile {
  id: string
  email: string
  name: string
}

/** 登录 / 注册成功后的返回 */
export interface AuthPayload {
  token: string
  expires_at: string
  user: UserProfile
}

// ==================== 参数表 ====================

/** 参数值类型 */
export type SpecValueType = 'text' | 'number' | 'bool' | 'enum'

/**
 * 参数项。
 *
 * 一张参数表 = 一个产品的参数配置表，表内每行是一条参数项。
 * 参数表只有两列：参数名、参数值 —— 单位直接写在值里（5000mAh、8小时）。
 */
export interface SpecItem {
  id: string
  name: string
  value: string
  value_type: SpecValueType
  enum_options?: string[]
  group_name: string
  remark: string
  sort_order: number
}

/** 表元信息（不含参数项） */
export interface TableDetail {
  id: string
  title: string
  description: string
  industry: string
  visibility: 'private' | 'link'
  current_revision: number
  created_at: string
  updated_at: string
}

/** 列表项，带参数项数量 */
export interface TableSummary {
  id: string
  title: string
  description: string
  industry: string
  visibility: 'private' | 'link'
  current_revision: number
  item_count: number
  created_at: string
  updated_at: string
}

export interface TableListResult {
  items: TableSummary[]
  total: number
  page: number
  size: number
}

/** 表详情：元信息 + 全部参数项 + 分组清单 */
export interface TableDetailResult {
  table: TableDetail
  items: SpecItem[]
  groups: string[]
}

/** 版本列表项 */
export interface RevisionMeta {
  revision: number
  source: 'user' | 'ai' | 'template' | 'import' | 'restore'
  summary: string
  item_count: number
  created_at: string
}

// ==================== 变更协议 ====================

export type OpType =
  | 'add_item'
  | 'add_items'
  | 'update_item'
  | 'delete_item'
  | 'reorder_items'
  | 'set_group'

/** 新建参数项 */
export interface ItemDraft {
  name: string
  /** 参数值，单位直接写在里面（5000mAh、8小时） */
  value?: string
  value_type?: SpecValueType
  enum_options?: string[]
  group_name?: string
  remark?: string
}

/**
 * 修改参数项；只填需要改的字段。
 *
 * 注意语义：字段省略 = 不改；传空字符串 = 改成空。
 */
export interface ItemPatch {
  name?: string
  value?: string
  value_type?: SpecValueType
  enum_options?: string[]
  group_name?: string
  remark?: string
}

/**
 * 条件式目标，由服务端展开。
 *
 * 做批量修改时用它而不是逐条枚举，例如「动力分组下全部参数」。
 */
export interface OpTarget {
  group?: string
  name_in?: string[]
  all?: boolean
}

/** 一条操作 */
export interface Op {
  op: OpType
  item_id?: string
  name?: string
  target?: OpTarget
  item?: ItemDraft
  items?: ItemDraft[]
  patch?: ItemPatch
  order?: string[]
  item_ids?: string[]
  group_name?: string
}

export type ChangeKind = 'add' | 'update' | 'delete' | 'move'

/** 单条变更，用于表格高亮 */
export interface Change {
  kind: ChangeKind
  item_id: string
  name: string
}

export interface ApplyResult {
  table: TableDetail
  revision: number
  changes: Change[]
  summary: string
}

export type ChangeSource = 'user' | 'ai' | 'template' | 'import'

// ==================== 对话 ====================

export type ChatEventType =
  | 'start'
  | 'text'
  | 'tool_call'
  | 'tool_result'
  | 'applied'
  | 'pending'
  | 'done'
  | 'error'

/** 一次变更的摘要，用于对话里的改动卡片与画布高亮 */
export interface ChatChange {
  kind: ChangeKind
  item_id: string
  name: string
  before?: string
  after?: string
}

export interface ChatUsage {
  prompt_tokens: number
  completion_tokens: number
  reasoning_tokens: number
  cached_tokens: number
}

/** SSE 推来的一个事件 */
export interface ChatEvent {
  type: ChatEventType
  text?: string
  tool_name?: string
  tool_args?: string
  summary?: string
  changes?: ChatChange[]
  revision?: number
  message?: string
  session_id?: string
  message_id?: string
  reply?: string
  usage?: ChatUsage
}

export interface ChatSession {
  id: string
  table_id: string
  title: string
  created_at: string
  updated_at: string
  /** 会话内的消息条数 */
  message_count: number
}

export interface ChatMessageRecord {
  id: string
  role: 'user' | 'assistant'
  content: string
  created_at: string
}

// ==================== 分享 ====================

export interface ShareStatus {
  enabled: boolean
  token?: string
  /** 相对路径，例如 /s/xxxx；前端用 location.origin 拼成完整地址 */
  url?: string
  view_count: number
}

export interface PublicSpec {
  title: string
  description: string
  industry: string
  updated_at: string
  groups: PublicGroup[]
}

export interface PublicGroup {
  name: string
  items: PublicItem[]
}

export interface PublicItem {
  name: string
  value: string
  value_type: SpecValueType
  remark?: string
}
