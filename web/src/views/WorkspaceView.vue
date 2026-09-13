<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import AppSidebar from '@/components/AppSidebar.vue'
import ChatPanel from '@/components/chat/ChatPanel.vue'
import ShareDialog from '@/components/table/ShareDialog.vue'
import SpecCanvas from '@/components/table/SpecCanvas.vue'
import { ApiError } from '@/api/http'
import { useChatStore } from '@/stores/chat'
import { useSpecTableStore } from '@/stores/specTable'
import { formatRelativeTime } from '@/utils/time'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

const route = useRoute()
const router = useRouter()
const spec = useSpecTableStore()
const chat = useChatStore()

const tableId = computed(() => String(route.params.id ?? ''))

const title = computed(() => spec.table?.title ?? '')
const description = computed(() => spec.table?.description ?? '')
const revision = computed(() => spec.table?.current_revision ?? 0)

const showShare = ref(false)
const showSessions = ref(false)

// ---------- 表级操作（标题右侧的「操作」菜单） ----------

/**
 * 重命名与复制共用一个「输入新名字」的弹窗：两者都只是换一个标题，
 * 区别只在复制会以新名字另建一份副本。
 */
type NameAction = 'rename' | 'duplicate'

const nameAction = ref<NameAction | null>(null)
const nameValue = ref('')
const nameSubmitting = ref(false)

const nameDialogTitle = computed(() =>
  nameAction.value === 'duplicate' ? '复制参数表' : '重命名参数表',
)

function openRename() {
  nameAction.value = 'rename'
  nameValue.value = title.value
}

function openDuplicate() {
  nameAction.value = 'duplicate'
  // 默认带上「副本」，用户多半会把它改成目标产品的名字
  nameValue.value = `${title.value} 副本`
}

function closeNameDialog() {
  if (nameSubmitting.value) return
  nameAction.value = null
}

async function submitName() {
  const table = spec.table
  const action = nameAction.value
  const next = nameValue.value.trim()
  if (!table || !action || !next || nameSubmitting.value) return

  nameSubmitting.value = true
  try {
    if (action === 'duplicate') {
      const res = await spec.duplicateTable(table.id, next)
      toast.success(`已复制为「${res.table.title}」`)
      nameAction.value = null
      // 复制通常是为了改几个值做个变体，直接切到副本更顺手
      void router.push({ name: 'workspace', params: { id: res.table.id } })
      return
    }

    // 名字没变就不用发请求
    if (next !== table.title) {
      await spec.renameTable(table.id, next)
      toast.success('已重命名')
    }
    nameAction.value = null
  } catch (err) {
    const fallback = action === 'duplicate' ? '复制失败' : '重命名失败'
    toast.error(err instanceof ApiError ? err.message : fallback)
  } finally {
    nameSubmitting.value = false
  }
}

const chatPlaceholder = computed(
  () =>
    '直接告诉我要怎么调整这张表，例如：\n\n' +
    '· 帮我建一张房车的参数表\n' +
    '· 把续航里程改成 800km\n' +
    '· 动力分组下的参数都加上单位\n\n' +
    '改动会自动应用，随时可以撤销。',
)

async function load() {
  if (!tableId.value) return
  try {
    await spec.openTable(tableId.value)
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      toast.error('参数表不存在或已删除')
    } else {
      toast.error(err instanceof ApiError ? err.message : '加载失败')
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
    toast.error(err instanceof ApiError ? err.message : '保存失败')
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
    toast.error(err instanceof ApiError ? err.message : '新增失败')
  }
}

const removingItem = ref<{ id: string; name: string } | null>(null)

function handleRemoveItem(itemId: string, name: string) {
  removingItem.value = { id: itemId, name }
}

async function confirmRemoveItem() {
  const target = removingItem.value
  if (!target) return
  try {
    await spec.removeItem(target.id, target.name)
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '删除失败')
  } finally {
    removingItem.value = null
  }
}

const removingTable = ref(false)

async function confirmRemoveTable() {
  const table = spec.table
  if (!table) return
  try {
    await spec.removeTable(table.id)
    toast.success(`已删除「${table.title}」`)
    void router.replace({ name: 'tables' })
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '删除失败')
  } finally {
    removingTable.value = false
  }
}

async function handleUndo() {
  try {
    const res = await spec.undo()
    if (res) toast.success(res.summary || '已撤销')
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '撤销失败')
  }
}

async function handleRedo() {
  try {
    const res = await spec.redo()
    if (res) toast.success(res.summary || '已重做')
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '重做失败')
  }
}

// ---------- 对话 ----------

async function handleSend(text: string) {
  await chat.send(text)
  if (chat.lastError) toast.error(chat.lastError)
}

async function handleUndoMessage(messageId: string) {
  try {
    const ok = await chat.undoMessage(messageId)
    if (ok) toast.success('已撤销这次 AI 改动')
    else toast.warning('没有可撤销的版本')
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '撤销失败')
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
            <div class="head-text">
              <span class="head-title ds-ellipsis" :title="title">{{ title }}</span>
              <span v-if="description" class="head-sub ds-ellipsis">{{ description }}</span>
            </div>

            <div class="head-actions">
              <!-- 表级操作统一收在这里，后续新增的表操作也放这个菜单 -->
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <button type="button" class="head-btn" aria-label="操作" title="操作">
                    <i class="ri-more-line" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" class="min-w-[150px]">
                  <DropdownMenuItem @select="openDuplicate">
                    <i class="ri-file-copy-line" />
                    复制
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="openRename">
                    <i class="ri-edit-line" />
                    重命名
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem variant="destructive" @select="removingTable = true">
                    <i class="ri-delete-bin-line" />
                    删除
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>

              <Tooltip>
                <TooltipTrigger as-child>
                  <button type="button" class="head-btn" @click="showSessions = true">
                    <i class="ri-history-line" />
                    <span v-if="chat.hasHistory" class="badge">{{ chat.sessions.length }}</span>
                  </button>
                </TooltipTrigger>
                <TooltipContent side="bottom">历史对话</TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger as-child>
                  <button type="button" class="head-btn" @click="startNewSession">
                    <i class="ri-add-line" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="bottom">新建对话</TooltipContent>
              </Tooltip>
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
          :items="spec.items"
          :highlight-ids="spec.highlightIds"
          :revision="revision"
          :saving="spec.saving"
          :can-undo="spec.canUndo"
          :can-redo="spec.canRedo"
          @update-item="handleUpdateItem"
          @add-item="handleAddItem"
          @remove-item="handleRemoveItem"
          @share="showShare = true"
          @undo="handleUndo"
          @redo="handleRedo"
        />
      </div>

      <div v-else class="loading">
        <p>参数表不存在</p>
        <Button size="sm" variant="outline" @click="router.replace({ name: 'tables' })">返回列表</Button>
      </div>
    </main>

    <ShareDialog
      v-if="spec.table"
      v-model:show="showShare"
      :table-id="spec.table.id"
      :table-title="spec.table.title"
    />

    <!-- 历史对话 -->
    <Dialog v-model:open="showSessions">
      <DialogContent class="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>历史对话</DialogTitle>
          <DialogDescription>切换到之前的对话，或新建一段对话</DialogDescription>
        </DialogHeader>

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

        <div class="panel-actions">
          <Button variant="outline" @click="startNewSession">
            <i class="ri-add-line" />
            新建对话
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- 删除参数 -->
    <AlertDialog :open="!!removingItem" @update:open="(v) => !v && (removingItem = null)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除参数</AlertDialogTitle>
          <AlertDialogDescription>
            确定删除「{{ removingItem?.name }}」？删除会记录为一个新版本，之后仍可撤销。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel @click="removingItem = null">取消</AlertDialogCancel>
          <AlertDialogAction variant="destructive" @click="confirmRemoveItem">删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- 重命名 / 复制（共用一个输入新名字的弹窗） -->
    <Dialog :open="!!nameAction" @update:open="(v) => !v && closeNameDialog()">
      <DialogContent class="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{{ nameDialogTitle }}</DialogTitle>
          <DialogDescription v-if="nameAction === 'duplicate'">
            将以新名字创建一份副本，包含当前全部参数。
          </DialogDescription>
        </DialogHeader>
        <Input
          v-model="nameValue"
          maxlength="120"
          placeholder="参数表名称"
          autofocus
          @keydown.enter.prevent="submitName"
        />
        <DialogFooter>
          <Button variant="outline" :disabled="nameSubmitting" @click="closeNameDialog">取消</Button>
          <Button :disabled="!nameValue.trim() || nameSubmitting" @click="submitName">
            <i v-if="nameSubmitting" class="ri-loader-4-line animate-spin" />
            {{ nameAction === 'duplicate' ? '创建副本' : '保存' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 删除参数表 -->
    <AlertDialog :open="removingTable" @update:open="(v) => !v && (removingTable = false)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除参数表</AlertDialogTitle>
          <AlertDialogDescription>
            确定删除「{{ spec.table?.title }}」？删除后该表不再出现在列表里。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel @click="removingTable = false">取消</AlertDialogCancel>
          <AlertDialogAction variant="destructive" @click="confirmRemoveTable">删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
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
  font-size: var(--ds-font-sm);
}

.workspace {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.chat-col {
  flex: 1;
  min-width: 0;
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
  background: var(--ds-surface);
}

.head-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}

.head-title {
  font-size: var(--ds-font-md);
  font-weight: 600;
  color: var(--ds-text);
  line-height: 1.3;
}

.head-sub {
  font-size: var(--ds-font-xs);
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
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-icon);
  font-size: 18px;
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
  color: var(--ds-primary-foreground);
  font-size: var(--ds-font-xs);
  line-height: 1;
}

/* ---------- 历史对话弹窗 ---------- */

.panel-empty {
  padding: 24px 8px;
  text-align: center;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

.panel-list {
  max-height: 320px;
}

.session-item {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
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
  font-size: var(--ds-font-sm);
  color: var(--ds-text);
}

.session-item.active .session-title {
  color: var(--ds-primary);
  font-weight: 500;
}

.session-meta {
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

.panel-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

/* 空态引导由 ChatPanel 的 empty 插槽渲染在滚动区内 */
.chat-col > :deep(.chat) {
  flex: 1;
  min-height: 0;
}
</style>
