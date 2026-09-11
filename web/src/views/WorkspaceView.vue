<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, useDialog, useMessage } from 'naive-ui'

import AppSidebar from '@/components/AppSidebar.vue'
import ChatPanel from '@/components/chat/ChatPanel.vue'
import ShareDialog from '@/components/table/ShareDialog.vue'
import SpecCanvas from '@/components/table/SpecCanvas.vue'
import { ApiError } from '@/api/http'
import { useChatStore } from '@/stores/chat'
import { useSpecTableStore } from '@/stores/specTable'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const spec = useSpecTableStore()
const chat = useChatStore()

const tableId = computed(() => String(route.params.id ?? ''))

const title = computed(() => spec.table?.title ?? '')
const description = computed(() => spec.table?.description ?? '')
const revision = computed(() => spec.table?.current_revision ?? 0)

const showShare = ref(false)

const chatPlaceholder = computed(() => {
  if (chat.messages.length > 0) return ''
  return (
    '直接告诉我要怎么调整这张表，例如：\n\n' +
    '· 帮我建一张蓝牙耳机的参数表\n' +
    '· 把电池容量改成 5000mAh\n' +
    '· 续航分组下的参数都加上单位\n\n' +
    '改动会自动应用，随时可以撤销。'
  )
})

async function load() {
  if (!tableId.value) return
  chat.reset()
  try {
    await spec.openTable(tableId.value)
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      message.error('参数表不存在或已删除')
    } else {
      message.error(err instanceof ApiError ? err.message : '加载失败')
    }
    void router.replace({ name: 'tables' })
  }
}

onMounted(load)
watch(tableId, load)

// ---------- 画布 ----------

async function handleUpdateItem(itemId: string, patch: { name?: string; value?: string; unit?: string }) {
  try {
    await spec.updateItem(itemId, patch, '手工修改参数')
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '保存失败')
  }
}

async function handleAddItem(draft: {
  name: string
  value: string
  unit: string
  group_name: string
  value_type: 'text' | 'number'
}) {
  try {
    await spec.addItem({
      name: draft.name,
      value: draft.value,
      unit: draft.unit,
      value_type: draft.value_type,
      group_name: draft.group_name,
    })
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '新增失败')
  }
}

function handleRemoveItem(itemId: string, name: string) {
  dialog.warning({
    title: '删除参数',
    content: `确定删除「${name}」？删除会记录为一个新版本，之后仍可撤销。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await spec.removeItem(itemId, name)
      } catch (err) {
        message.error(err instanceof ApiError ? err.message : '删除失败')
      }
    },
  })
}

async function handleRename(next: string) {
  if (!spec.table) return
  try {
    await spec.renameTable(spec.table.id, next)
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '重命名失败')
  }
}

async function handleUndo() {
  try {
    const res = await spec.undo()
    if (res) message.success(res.summary || '已撤销')
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '撤销失败')
  }
}

async function handleRedo() {
  try {
    const res = await spec.redo()
    if (res) message.success(res.summary || '已重做')
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '重做失败')
  }
}

// ---------- 对话 ----------

async function handleSend(text: string) {
  await chat.send(text)
  if (chat.lastError) message.error(chat.lastError)
}

async function handleUndoMessage(messageId: string) {
  try {
    const ok = await chat.undoMessage(messageId)
    if (ok) message.success('已撤销这次 AI 改动')
    else message.warning('没有可撤销的版本')
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '撤销失败')
  }
}
</script>

<template>
  <div class="shell">
    <AppSidebar />

    <main class="main">
      <div v-if="spec.loading && !spec.table" class="loading">加载中…</div>

      <div v-else-if="spec.table" class="workspace">
        <div class="chat-col">
          <header class="chat-head">
            <span class="dot" :class="{ busy: chat.sending }" />
            <span class="head-title">AI 助手</span>
            <span class="head-sub">直接说你要怎么改</span>
          </header>

          <ChatPanel
            :messages="chat.messages"
            :sending="chat.sending"
            @send="handleSend"
            @stop="chat.abort"
            @undo="handleUndoMessage"
          >
            <template #empty>
              <p v-if="chatPlaceholder">{{ chatPlaceholder }}</p>
            </template>
          </ChatPanel>
        </div>

        <SpecCanvas
          :title="title"
          :description="description"
          :items="spec.items"
          :highlight-ids="spec.highlightIds"
          :revision="revision"
          :saving="spec.saving"
          :can-undo="spec.canUndo"
          :can-redo="spec.canRedo"
          @update-item="handleUpdateItem"
          @add-item="handleAddItem"
          @remove-item="handleRemoveItem"
          @rename="handleRename"
          @share="showShare = true"
          @undo="handleUndo"
          @redo="handleRedo"
        />
      </div>

      <div v-else class="loading">
        <p>参数表不存在</p>
        <n-button size="small" @click="router.replace({ name: 'tables' })">返回列表</n-button>
      </div>
    </main>

    <ShareDialog
      v-if="spec.table"
      v-model:show="showShare"
      :table-id="spec.table.id"
      :table-title="spec.table.title"
    />
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100%;
  overflow: hidden;
  background: var(--ds-bg);
}

.main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.loading {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--ds-text-tertiary);
  font-size: 13px;
}

.workspace {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.chat-col {
  width: 42%;
  min-width: 340px;
  max-width: 620px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.chat-head {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 56px;
  padding: 0 22px;
  border-bottom: 1px solid var(--ds-border);
  border-right: 1px solid var(--ds-border);
  background: var(--ds-surface);
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #18a058;
}

.dot.busy {
  background: var(--ds-primary);
  animation: pulse 1.1s ease-in-out infinite;
}

@keyframes pulse {
  50% {
    opacity: 0.35;
  }
}

.head-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ds-text);
}

.head-sub {
  font-size: 12px;
  color: var(--ds-text-tertiary);
}

/* 空态引导由 ChatPanel 的 empty 插槽渲染在滚动区内 */
.chat-col > :deep(.chat) {
  flex: 1;
  min-height: 0;
}
</style>
