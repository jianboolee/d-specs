<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { toast } from 'vue-sonner'

import * as shareApi from '@/api/share'
import { ApiError } from '@/api/http'
import type { ShareStatus } from '@/types/api'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const props = defineProps<{
  show: boolean
  tableId: string
  tableTitle: string
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  /** 开启/关闭成功后通知外部刷新（侧栏与标题旁的公开标识依赖它） */
  changed: []
}>()

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
    toast.error(describe(err))
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
    toast.success(enabled.value ? '已开启分享' : '已关闭分享，原链接立即失效')
    // 让外部（侧栏列表、标题旁的公开标识）同步最新状态
    emit('changed')
  } catch (err) {
    toast.error(describe(err))
  } finally {
    busy.value = false
  }
}

async function copyLink() {
  if (!fullUrl.value) return
  try {
    await navigator.clipboard.writeText(fullUrl.value)
    toast.success('链接已复制')
  } catch {
    toast.warning('复制失败，请手动选中链接')
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
  <Dialog :open="show" @update:open="(v: boolean) => emit('update:show', v)">
    <DialogContent class="sm:max-w-[480px]">
      <DialogHeader>
        <DialogTitle>分享参数表</DialogTitle>
        <DialogDescription>「{{ tableTitle }}」</DialogDescription>
      </DialogHeader>

      <div v-if="loading" class="share-loading">加载中…</div>

      <template v-else>
        <div class="share-state">
          <div class="state-text">
            <strong>{{ enabled ? '分享已开启' : '分享未开启' }}</strong>
            <span v-if="enabled">
              公开链接，任何人可查看、也可能被搜索引擎收录 · 已被访问
              {{ status?.view_count ?? 0 }} 次
            </span>
            <span v-else>开启后会生成一个公开的只读链接，可随时关闭</span>
          </div>
          <Button :variant="enabled ? 'outline' : 'default'" :disabled="busy" @click="toggle">
            {{ enabled ? '关闭分享' : '开启分享' }}
          </Button>
        </div>

        <div v-if="enabled && fullUrl" class="share-link">
          <input class="link-input" :value="fullUrl" readonly @focus="($event.target as HTMLInputElement).select()" />
          <Button size="sm" variant="outline" @click="copyLink">复制</Button>
          <Button size="sm" variant="ghost" @click="openLink">预览</Button>
        </div>

        <p class="share-tip">
          页面公开可被搜索引擎收录。关闭分享后链接立即失效并返回 404；再次开启会
          复用同一个链接，已发出去的不受影响。
        </p>
      </template>

      <DialogFooter>
        <Button variant="outline" @click="emit('update:show', false)">完成</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.share-loading {
  padding: 28px 0;
  text-align: center;
  font-size: var(--ds-font-sm);
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
  font-size: var(--ds-font-sm);
  font-weight: 600;
  color: var(--ds-text);
}

.state-text span {
  font-size: var(--ds-font-xs);
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
  font-size: var(--ds-font-xs);
  outline: none;
}

.link-input:focus {
  color: var(--ds-text);
}

.share-tip {
  margin: 14px 0 0;
  font-size: var(--ds-font-xs);
  line-height: 1.72;
  color: var(--ds-text-tertiary);
}
</style>
