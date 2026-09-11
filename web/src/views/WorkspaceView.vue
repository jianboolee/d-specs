<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NPopover, useDialog, useMessage } from 'naive-ui'

import AppSidebar from '@/components/AppSidebar.vue'
import ChatPanel from '@/components/chat/ChatPanel.vue'
import ShareDialog from '@/components/table/ShareDialog.vue'
import SpecCanvas from '@/components/table/SpecCanvas.vue'
import { ApiError } from '@/api/http'
import { useChatStore } from '@/stores/chat'
import { useSpecTableStore } from '@/stores/specTable'
import { formatRelativeTime } from '@/utils/time'

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
const showSessions = ref(false)

/** 当前会话的状态摘要，显示在对话区头部 */
const sessionHint = computed(() => {
  const current = chat.currentSession
  if (!current) return '新对话'
  return `${current.message_count} 条消息 · ${formatRelativeTime(current.updated_at)}`
})

const chatPlaceholder = computed(
  () =>
    '直接告诉我要怎么调整这张表，例如：\n\n' +
    '· 帮我建一张蓝牙耳机的参数表\n' +
    '· 把电池容量改成 5000mAh\n' +
    '· 续航分组下的参数都加上单位\n\n' +
    '改动会自动应用，随时可以撤销。',
)

async function load() {
  if (!tableId.value) return
  try {
    await spec.openTable(tableId.value)
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      message.error('参数表不存在或已删除')
    } else {
      message.error(err instanceof ApiError ? err.message : '加载失败')
    }
    void router.replace({ name: 'tables' })
    return
  }
  // 表打开之后再恢复历史对话，避免两处 loading 互相干扰
  await chat.initForTable(tableId.value)
}

onMounted(load)
watch(tableId, load)

// ---------- 会话切换 ----------

async function pickSession(id: string) {
  showSessions.value = false
  if (id === chat.sessionId) return
  await chat.openSession(id)
}

function startNewSession() {
  showSessions.value = false
  chat.newSession()
}

// ---------- 画布 ----------

async function handleUpdateItem(itemId: string, patch: { name?: string; value?: string }) {
  try {
    await spec.updateItem(itemId, patch, '手工修改参数')
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '保存失败')
  }
}

async function handleAddItem(draft: {
  name: string
  value: string
  group_name: string
  value_type: 'text' | 'number'
}) {
  try {
    await spec.addItem({
      name: draft.name,
      value: draft.value,
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

function handleRemoveTable() {
  const table = spec.table
  if (!table) return
  const title = table.title

  dialog.warning({
    title: '删除参数表',
    content: `确定删除「${title}」？删除后该表不再出现在列表里。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await spec.removeTable(table.id)
        message.success(`已删除「${title}」`)
        void router.replace({ name: 'tables' })
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
            <div class="head-text">
              <span class="head-title">AI 助手</span>
              <span class="head-sub ds-ellipsis">{{ sessionHint }}</span>
            </div>

            <div class="head-actions">
              <n-popover
                v-model:show="showSessions"
                trigger="click"
                placement="bottom-end"
                :show-arrow="false"
                raw
              >
                <template #trigger>
                  <button type="button" class="head-btn" title="历史对话">
                    <i class="ri-history-line" />
                    <span v-if="chat.hasHistory" class="badge">{{ chat.sessions.length }}</span>
                  </button>
                </template>

                <div class="session-panel">
                  <div class="panel-head">历史对话</div>

                  <div v-if="chat.sessions.length === 0" class="panel-empty">
                    这张表还没有对话记录
                  </div>

                  <div v-else class="panel-list ds-scroll-y">
                    <button
                      v-for="s in chat.sessions"
                      :key="s.id"
                      type="button"
                      class="session-item"
                      :class="{ active: s.id === chat.sessionId }"
                      @click="pickSession(s.id)"
                    >
                      <span class="session-title ds-ellipsis">{{ s.title || '未命名对话' }}</span>
                      <span class="session-meta">
                        {{ s.message_count }} 条 · {{ formatRelativeTime(s.updated_at) }}
                      </span>
                    </button>
                  </div>

                  <button type="button" class="panel-new" @click="startNewSession">
                    <i class="ri-add-line" />
                    新建对话
                  </button>
                </div>
              </n-popover>

              <button type="button" class="head-btn" title="新建对话" @click="startNewSession">
                <i class="ri-add-line" />
              </button>
            </div>
          </header>

          <ChatPanel
            :messages="chat.messages"
            :sending="chat.sending"
            :loading="chat.loadingHistory"
            @send="handleSend"
            @stop="chat.abort"
            @undo="handleUndoMessage"
          >
            <template #empty>
              <p>{{ chatPlaceholder }}</p>
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
          @remove="handleRemoveTable"
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
  gap: 10px;
  height: 56px;
  padding: 0 14px 0 22px;
  border-bottom: 1px solid var(--ds-border);
  border-right: 1px solid var(--ds-border);
  background: var(--ds-surface);
}

.dot {
  width: 7px;
  height: 7px;
  flex: none;
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

.head-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}

.head-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ds-text);
}

.head-sub {
  font-size: 11.5px;
  color: var(--ds-text-tertiary);
}

.head-actions {
  flex: none;
  display: flex;
  align-items: center;
  gap: 4px;
}

.head-btn {
  position: relative;
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 15px;
  cursor: pointer;
}

.head-btn:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.badge {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 15px;
  height: 15px;
  padding: 0 4px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--ds-primary);
  color: #fff;
  font-size: 10px;
  line-height: 1;
}

/* ---------- 历史对话面板 ---------- */

.session-panel {
  width: 300px;
  padding: 8px;
  border: 1px solid var(--ds-border);
  border-radius: 12px;
  background: var(--ds-surface);
  box-shadow: 0 12px 32px rgba(18, 22, 34, 0.14);
}

.panel-head {
  padding: 4px 8px 8px;
  font-size: 11.5px;
  color: var(--ds-text-tertiary);
}

.panel-empty {
  padding: 18px 8px;
  text-align: center;
  font-size: 12.5px;
  color: var(--ds-text-tertiary);
}

.panel-list {
  max-height: 288px;
}

.session-item {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 7px 8px;
  margin-bottom: 2px;
  border: none;
  border-radius: 8px;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.session-item:hover {
  background: var(--ds-surface-muted);
}

.session-item.active {
  background: var(--ds-primary-soft);
}

.session-title {
  font-size: 13px;
  color: var(--ds-text);
}

.session-item.active .session-title {
  color: var(--ds-primary);
  font-weight: 500;
}

.session-meta {
  font-size: 11px;
  color: var(--ds-text-tertiary);
}

.panel-new {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  margin-top: 6px;
  padding: 8px;
  border: 1px solid var(--ds-border);
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 12.5px;
  cursor: pointer;
}

.panel-new:hover {
  border-color: var(--ds-primary);
  color: var(--ds-primary);
}

/* 空态引导由 ChatPanel 的 empty 插槽渲染在滚动区内 */
.chat-col > :deep(.chat) {
  flex: 1;
  min-height: 0;
}
</style>
