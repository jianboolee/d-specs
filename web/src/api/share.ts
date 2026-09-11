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
export function getPublicSpec(token: string) {
  return request<PublicSpec>(`/public/specs/${token}`)
}
