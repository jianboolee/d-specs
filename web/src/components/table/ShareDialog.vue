<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NModal, useMessage } from 'naive-ui'

import * as shareApi from '@/api/share'
import { ApiError } from '@/api/http'
import type { ShareStatus } from '@/types/api'

const props = defineProps<{
  show: boolean
  tableId: string
  tableTitle: string
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const message = useMessage()

const loading = ref(false)
const busy = ref(false)
const status = ref<ShareStatus | null>(null)

const enabled = computed(() => status.value?.enabled === true)

/** 后端返回相对路径，这里补成完整地址方便复制 */
const fullUrl = computed(() => {
  if (!status.value?.url) return ''
  return `${window.location.origin}${status.value.url}`
})

watch(
  () => props.show,
  (visible) => {
    if (visible) void load()
  },
)

async function load() {
  loading.value = true
  try {
    status.value = await shareApi.getShareStatus(props.tableId)
  } catch (err) {
    message.error(describe(err))
  } finally {
    loading.value = false
  }
}

async function toggle() {
  busy.value = true
  try {
    status.value = enabled.value
      ? await shareApi.disableShare(props.tableId)
      : await shareApi.enableShare(props.tableId)
    message.success(enabled.value ? '已开启分享' : '已关闭分享，原链接立即失效')
  } catch (err) {
    message.error(describe(err))
  } finally {
    busy.value = false
  }
}

async function copyLink() {
  if (!fullUrl.value) return
  try {
    await navigator.clipboard.writeText(fullUrl.value)
    message.success('链接已复制')
  } catch {
    message.warning('复制失败，请手动选中链接')
  }
}

function openLink() {
  if (!status.value?.url) return
  window.open(status.value.url, '_blank', 'noopener')
}

function describe(err: unknown) {
  return err instanceof ApiError ? err.message : '操作失败，请稍后重试'
}
</script>

<template>
  <n-modal
    :show="show"
    :mask-closable="!busy"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <div class="share-card">
      <h3>分享参数表</h3>
      <p class="share-sub">「{{ tableTitle }}」</p>

      <div v-if="loading" class="share-loading">加载中…</div>

      <template v-else>
        <div class="share-state">
          <div class="state-text">
            <strong>{{ enabled ? '分享已开启' : '分享未开启' }}</strong>
            <span v-if="enabled">
              任何人拿到链接都可以查看，无需登录 · 已被访问 {{ status?.view_count ?? 0 }} 次
            </span>
            <span v-else>开启后会生成一个只读链接，可随时关闭</span>
          </div>
          <n-button :type="enabled ? 'default' : 'primary'" :loading="busy" @click="toggle">
            {{ enabled ? '关闭分享' : '开启分享' }}
          </n-button>
        </div>

        <div v-if="enabled && fullUrl" class="share-link">
          <input class="link-input" :value="fullUrl" readonly @focus="($event.target as HTMLInputElement).select()" />
          <n-button size="small" @click="copyLink">复制</n-button>
          <n-button size="small" quaternary @click="openLink">预览</n-button>
        </div>

        <p class="share-tip">
          关闭分享后原链接立即失效；再次开启会复用同一个链接，已发出去的不受影响。
        </p>
      </template>

      <div class="share-actions">
        <n-button quaternary @click="emit('update:show', false)">完成</n-button>
      </div>
    </div>
  </n-modal>
</template>

<style scoped>
.share-card {
  width: min(480px, calc(100vw - 32px));
  padding: 22px 24px;
  border-radius: 16px;
  background: var(--ds-surface);
  border: 1px solid var(--ds-border);
}

.share-card h3 {
  margin: 0 0 4px;
  font-size: 16px;
  font-weight: 600;
  color: var(--ds-text);
}

.share-sub {
  margin: 0 0 18px;
  font-size: 12.5px;
  color: var(--ds-text-tertiary);
}

.share-loading {
  padding: 28px 0;
  text-align: center;
  font-size: 13px;
  color: var(--ds-text-tertiary);
}

.share-state {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--ds-border);
  border-radius: 12px;
  background: var(--ds-surface-muted);
}

.state-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.state-text strong {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--ds-text);
}

.state-text span {
  font-size: 12px;
  line-height: 1.6;
  color: var(--ds-text-secondary);
}

.share-link {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

.link-input {
  flex: 1;
  min-width: 0;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--ds-border);
  border-radius: 8px;
  background: var(--ds-surface);
  color: var(--ds-text-secondary);
  font-family: inherit;
  font-size: 12.5px;
  outline: none;
}

.link-input:focus {
  border-color: var(--ds-primary);
  color: var(--ds-text);
}

.share-tip {
  margin: 14px 0 0;
  font-size: 11.5px;
  line-height: 1.72;
  color: var(--ds-text-tertiary);
}

.share-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}
</style>
