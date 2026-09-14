<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { ApiError, download } from '@/api/http'
import { downloadBlob } from '@/utils/download'

/**
 * 导出页：独立页面触发 xlsx 下载。
 *
 * 为什么要有这一页（而不是直接在应用里 fetch 下载）：
 * 导出接口需要 Authorization 头，而浏览器点击链接时带不上这个头，
 * 直接链到 /api/... 会 401。做成一个正常的前端路由后，页面在浏览器里
 * 带着 localStorage 里的 token 去请求，链接就能被点击、也能被 AI 直接给出来
 * （AI 回复里的链接点击即打开本页）。
 *
 * 打开方式：应用内菜单与 AI 回复的链接都用新标签页打开，所以下载完成后
 * 可以尝试收起本页（脚本打开的窗口才允许 close）。
 */
const route = useRoute()

const tableId = String(route.params.id ?? '')

type Status = 'loading' | 'done' | 'error'

const status = ref<Status>('loading')
const errorText = ref('')

/** 成功后的自动关闭定时器，卸载时要清掉 */
let closeTimer: number | undefined

async function run() {
  status.value = 'loading'
  errorText.value = ''
  try {
    const res = await download(`/tables/${tableId}/export`)
    downloadBlob(res.blob, res.filename || '参数表.xlsx')
    status.value = 'done'
    // 下载已触发，停一下让用户看到反馈再尝试收起本页
    closeTimer = window.setTimeout(() => window.close(), 1200)
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : '导出失败'
    status.value = 'error'
  }
}

/** 手动关闭。用户直接访问本页时 close 会被浏览器忽略，不影响使用 */
function close() {
  window.close()
}

function retry() {
  if (closeTimer) window.clearTimeout(closeTimer)
  void run()
}

onMounted(run)

onUnmounted(() => {
  if (closeTimer) window.clearTimeout(closeTimer)
})
</script>

<template>
  <div class="export-page">
    <div class="export-card">
      <template v-if="status === 'loading'">
        <i class="ri-loader-4-line spin" />
        <h1>正在导出…</h1>
        <p>正在生成 Excel 文件，请稍候。</p>
      </template>

      <template v-else-if="status === 'done'">
        <i class="ri-checkbox-circle-fill ok" />
        <h1>已开始下载</h1>
        <p>文件已保存到浏览器下载目录，本页可以关闭。</p>
        <div class="actions">
          <button type="button" class="btn" @click="retry">重新下载</button>
          <button type="button" class="btn ghost" @click="close">关闭</button>
        </div>
      </template>

      <template v-else>
        <i class="ri-error-warning-line err" />
        <h1>导出失败</h1>
        <p>{{ errorText }}</p>
        <div class="actions">
          <button type="button" class="btn" @click="retry">重试</button>
          <button type="button" class="btn ghost" @click="close">关闭</button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.export-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: var(--ds-bg);
}

.export-card {
  max-width: 360px;
  text-align: center;
}

.export-card > i {
  font-size: 38px;
  color: var(--ds-text-tertiary);
}

.export-card > i.ok {
  color: var(--ds-share-dot);
}

.export-card > i.err {
  color: #d64545;
}

.export-card h1 {
  margin: 14px 0 8px;
  font-size: 17px;
  font-weight: 600;
  color: var(--ds-text);
}

.export-card p {
  margin: 0;
  font-size: var(--ds-font-sm);
  line-height: 1.7;
  color: var(--ds-text-secondary);
}

.actions {
  display: flex;
  gap: 8px;
  justify-content: center;
  margin-top: 20px;
}

.btn {
  padding: 7px 16px;
  border: 1px solid var(--ds-border);
  border-radius: 8px;
  background: var(--ds-surface-muted);
  color: var(--ds-text);
  font-size: var(--ds-font-xs);
  cursor: pointer;
}

.btn:hover {
  border-color: var(--ds-primary);
}

.btn.ghost {
  background: transparent;
  color: var(--ds-text-secondary);
}

.spin {
  animation: spin 1.1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
