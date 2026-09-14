/**
 * 触发一次浏览器下载。
 *
 * 用临时 <a download> 而不是 location.assign：后者在部分浏览器会直接
 * 打开文件而不是保存。URL 用完即释放，避免内存泄漏。
 */
export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
