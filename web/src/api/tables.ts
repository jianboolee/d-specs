import { request } from './http'
import type {
  ApplyResult,
  ChangeSource,
  Op,
  RevisionMeta,
  TableDetail,
  TableDetailResult,
  TableListResult,
} from '@/types/api'

export interface ListTablesParams {
  keyword?: string
  page?: number
  size?: number
}

export function listTables(params: ListTablesParams = {}) {
  return request<TableListResult>('/tables', {
    query: { keyword: params.keyword, page: params.page, size: params.size },
  })
}

export interface CreateTableInput {
  title: string
  description?: string
  industry?: string
  template_id?: string
}

export function createTable(input: CreateTableInput) {
  return request<TableDetailResult>('/tables', { method: 'POST', body: input })
}

export function getTable(id: string) {
  return request<TableDetailResult>(`/tables/${id}`)
}

export interface UpdateTableInput {
  title?: string
  description?: string
  industry?: string
}

export function updateTable(id: string, input: UpdateTableInput) {
  return request<TableDetail>(`/tables/${id}`, { method: 'PATCH', body: input })
}

export function deleteTable(id: string) {
  return request<{ deleted: boolean }>(`/tables/${id}`, { method: 'DELETE' })
}

/**
 * 复制表：换产品改几个值的高频操作
 */
export function duplicateTable(id: string, title: string) {
  return request<TableDetailResult>(`/tables/${id}/duplicate`, {
    method: 'POST',
    body: { title },
  })
}

export interface ApplyInput {
  /** 乐观锁依据；过期会返回 409 */
  baseRevision: number
  ops: Op[]
  source?: ChangeSource
  summary?: string
}

/**
 * 统一变更入口：手工编辑与 AI 变更走同一个接口。
 *
 * 注意 ops 里的「省略字段」与「传空字符串」语义不同，
 * 构造 patch 时要显式给出想清空的字段。
 */
export function applyOps(id: string, input: ApplyInput) {
  return request<ApplyResult>(`/tables/${id}/apply`, {
    method: 'POST',
    body: {
      base_revision: input.baseRevision,
      ops: input.ops,
      source: input.source ?? 'user',
      summary: input.summary ?? '',
    },
  })
}

export function listRevisions(id: string, limit = 50) {
  return request<RevisionMeta[]>(`/tables/${id}/revisions`, { query: { limit } })
}

/**
 * 恢复到指定版本。
 *
 * 服务端会把目标版本的快照写成**新版本**，而不是回退版本号 ——
 * 历史因此保持线性，撤销本身也是一次可撤销的变更。
 * 撤销/重做的栈由前端维护。
 */
export function restoreRevision(id: string, revision: number) {
  return request<ApplyResult>(`/tables/${id}/restore`, {
    method: 'POST',
    body: { revision },
  })
}
