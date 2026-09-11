/**
 * 相对时间：刚刚 / N 分钟前 / N 小时前 / N 天前，超过 30 天显示日期。
 *
 * 参数表与会话列表都用它，避免两处各写一份格式不一致。
 */
export function formatRelativeTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''

  const diff = Date.now() - date.getTime()
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour

  if (diff < minute) return '刚刚'
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`
  if (diff < 30 * day) return `${Math.floor(diff / day)} 天前`

  return date.toLocaleDateString('zh-CN')
}
