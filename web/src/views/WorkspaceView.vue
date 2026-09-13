<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import AppSidebar from '@/components/AppSidebar.vue'
import ChatPanel from '@/components/chat/ChatPanel.vue'
import ShareDialog from '@/components/table/ShareDialog.vue'
import SpecCanvas from '@/components/table/SpecCanvas.vue'
import { ApiError } from '@/api/http'
import * as shareApi from '@/api/share'
import { useChatStore } from '@/stores/chat'
import { useSpecTableStore } from '@/stores/specTable'
import { formatRelativeTime } from '@/utils/time'
import type { ShareStatus } from '@/types/api'
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

// ---------- 分享 ----------

const shareStatus = ref<ShareStatus | null>(null)
const shareBusy = ref(false)

const shareEnabled = computed(() => shareStatus.value?.enabled === true)
const shareUrl = computed(() => shareStatus.value?.url ?? '')

/** 拉一次分享状态：标题旁的公开标识、菜单里的开启/关闭与预览都依赖它 */
async function loadShareStatus() {
  if (!tableId.value) return
  try {
    shareStatus.value = await shareApi.getShareStatus(tableId.value)
  } catch {
    // 读不到就当作未分享，不影响主流程
    shareStatus.value = null
  }
}

/** 菜单里的一键开启/关闭，省去打开分享弹窗 */
async function toggleShare() {
  if (!spec.table || shareBusy.value) return
  shareBusy.value = true
  try {
    shareStatus.value = shareEnabled.value
      ? await shareApi.disableShare(spec.table.id)
      : await shareApi.enableShare(spec.table.id)
    toast.success(shareEnabled.value ? '已开启分享' : '已关闭分享，原链接立即失效')
    // 侧栏的公开标识读列表里的 visibility，刷新一次让它同步
    void spec.loadTables()
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    shareBusy.value = false
  }
}

/** 预览公开页；未开启分享时不可点，因此 url 必然存在 */
function previewShare() {
  if (!shareUrl.value) return
  window.open(shareUrl.value, '_blank', 'noopener')
}

/** 复制公开链接；未开启分享时不可点，因此 url 必然存在 */
async function copyShareLink() {
  if (!shareUrl.value) return
  try {
    // 后端返回相对路径，补成完整地址再复制
    await navigator.clipboard.writeText(`${window.location.origin}${shareUrl.value}`)
    toast.success('链接已复制')
  } catch {
    toast.warning('复制失败，请手动复制')
  }
}

/** 分享弹窗里改了状态：重新拉取并同步侧栏 */
async function onShareChanged() {
  await loadShareStatus()
  void spec.loadTables()
}

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
  // 分享状态独立于表内容，单独拉一次（失败不挡路）
  void loadShareStatus()
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

/**
 * 待删除的参数 + 确认框可见性分开存。
 *
 * 不能只用 removingItem 兼任「弹窗是否打开」：AlertDialogAction 自身就是
 * DialogClose，它的关闭回调（把状态置空）会先于这里的 @click 执行，导致
 * confirmRemoveItem 读到的目标已被清空。拆开后关闭只收起弹窗，不丢目标。
 */
const removingItem = ref<{ id: string; name: string } | null>(null)
const showRemoveItem = ref(false)

function handleRemoveItem(itemId: string, name: string) {
  removingItem.value = { id: itemId, name }
  showRemoveItem.value = true
}

async function confirmRemoveItem() {
  const target = removingItem.value
  if (!target) return
  try {
    await spec.removeItem(target.id, target.name)
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '删除失败')
  } finally {
    showRemoveItem.value = false
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

/** 让 AI 通读当前表，指出产品类型与还缺哪些关键参数（只给建议，不改动表格） */
async function handleAnalyzeTurn() {
  await chat.analyze()
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
              <!-- 已开启分享：圆点放在标题左侧，悬停说明含义 -->
              <Tooltip v-if="shareEnabled">
                <TooltipTrigger as-child>
                  <span class="head-share-dot" />
                </TooltipTrigger>
                <TooltipContent side="bottom">已开启分享</TooltipContent>
              </Tooltip>
              <span class="head-title ds-ellipsis" :title="title">{{ title }}</span>

              <!-- 表级操作紧挨标题：它改的是这张表，和右侧的会话操作分开放 -->
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <button type="button" class="head-btn" aria-label="参数表操作" title="参数表操作">
                    <i class="ri-more-line" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" class="min-w-[150px]">
                  <DropdownMenuItem @select="openDuplicate">
                    <i class="ri-file-copy-line" />
                    创建副本
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="openRename">
                    <i class="ri-edit-line" />
                    重命名
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="showShare = true">
                    <i class="ri-share-forward-line" />
                    分享
                  </DropdownMenuItem>
                  <!-- 快速开关，不必进分享弹窗 -->
                  <DropdownMenuItem :disabled="shareBusy" @select="toggleShare">
                    <i :class="shareEnabled ? 'ri-link-unlink-m' : 'ri-link'" />
                    {{ shareEnabled ? '关闭分享' : '开启分享' }}
                  </DropdownMenuItem>
                  <!-- 未开启分享时没有公开地址，预览与复制链接都置灰 -->
                  <DropdownMenuItem :disabled="!shareEnabled" @select="previewShare">
                    <i class="ri-external-link-line" />
                    预览
                  </DropdownMenuItem>
                  <DropdownMenuItem :disabled="!shareEnabled" @select="copyShareLink">
                    <i class="ri-link-m" />
                    复制链接
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem variant="destructive" @select="removingTable = true">
                    <i class="ri-delete-bin-line" />
                    删除
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>

            <div class="head-actions">
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
            :analyzing="chat.analyzing"
            :loading="chat.loadingHistory"
            @send="handleSend"
            @analyze="handleAnalyzeTurn"
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
          :description="description"
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
      @changed="onShareChanged"
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
    <AlertDialog :open="showRemoveItem" @update:open="(v) => (showRemoveItem = v)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除参数</AlertDialogTitle>
          <AlertDialogDescription>
            确定删除「{{ removingItem?.name }}」？删除会记录为一个新版本，之后仍可撤销。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel @click="showRemoveItem = false">取消</AlertDialogCancel>
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
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  line-height: 1.3;
}

/* 已开启分享：与侧栏同款浅蓝圆点，放在标题左侧 */
.head-share-dot {
  flex: none;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ds-share-dot);
}

.head-title {
  min-width: 0;
  font-size: var(--ds-font-md);
  font-weight: 600;
  color: var(--ds-text);
  line-height: 1.3;
}

.head-actions {
  /* 会话操作固定在最右，表级操作紧挨标题 */
  margin-left: auto;
  flex: none;
  display: flex;
  align-items: center;
  gap: 4px;
}

.head-btn {
  position: relative;
  flex: none;
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
