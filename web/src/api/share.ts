import { request } from './http'
import type { PublicSpec, ShareStatus } from '@/types/api'

export function getShareStatus(tableId: string) {
  return request<ShareStatus>(`/tables/${tableId}/share`)
}

/** 开启分享。重复调用会复用同一个 token，已发出的链接不会失效 */
export function enableShare(tableId: string) {
  return request<ShareStatus>(`/tables/${tableId}/share`, { method: 'POST' })
}

export function disableShare(tableId: string) {
  return request<ShareStatus>(`/tables/${tableId}/share`, { method: 'DELETE' })
}

/** 公开只读内容，无需鉴权 */
/** 公开只读内容，无需鉴权。key 为表 id（历史 token 亦可） */
export function getPublicSpec(id: string) {
  return request<PublicSpec>(`/public/specs/${id}`)
}
