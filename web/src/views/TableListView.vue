<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import AppSidebar from '@/components/AppSidebar.vue'
import { ApiError } from '@/api/http'
import { useSpecTableStore } from '@/stores/specTable'
import { useCreateTable } from '@/composables/useCreateTable'
import type { TableSummary } from '@/types/api'
import { formatRelativeTime } from '@/utils/time'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const router = useRouter()
const spec = useSpecTableStore()

onMounted(() => {
  void spec.loadTables().catch((err) => toast.error(describe(err)))
})

// ---------- 新建 ----------

// 弹窗由 App 全局挂载一份，这里只负责唤起，避免各自维护
const { open: openCreate } = useCreateTable()

// ---------- 行操作 ----------

function open(table: TableSummary) {
  void router.push({ name: 'workspace', params: { id: table.id } })
}

async function duplicate(table: TableSummary) {
  try {
    const res = await spec.duplicateTable(table.id, `${table.title} 副本`)
    toast.success(`已复制为「${res.table.title}」`)
  } catch (err) {
    toast.error(describe(err))
  }
}

async function togglePin(table: TableSummary) {
  const next = !table.pinned
  try {
    await spec.setPinned(table.id, next)
    toast.success(next ? '已置顶' : '已取消置顶')
  } catch (err) {
    toast.error(describe(err))
  }
}

// ---------- 重命名（Dialog + Input） ----------

const renaming = ref<TableSummary | null>(null)
const renameValue = ref('')

function openRename(table: TableSummary) {
  renaming.value = table
  renameValue.value = table.title
}

async function submitRename() {
  const table = renaming.value
  if (!table) return
  const title = renameValue.value.trim()
  if (!title || title === table.title) {
    renaming.value = null
    return
  }
  try {
    await spec.renameTable(table.id, title)
    toast.success('已重命名')
  } catch (err) {
    toast.error(describe(err))
  } finally {
    renaming.value = null
  }
}

// ---------- 删除（AlertDialog） ----------

const removing = ref<TableSummary | null>(null)

async function confirmRemove() {
  const table = removing.value
  if (!table) return
  try {
    await spec.removeTable(table.id)
    toast.success(`已删除「${table.title}」`)
  } catch (err) {
    toast.error(describe(err))
  } finally {
    removing.value = null
  }
}

// ---------- 展示 ----------

function describe(err: unknown) {
  return err instanceof ApiError ? err.message : '操作失败，请稍后重试'
}
</script>

<template>
  <div class="shell">
    <AppSidebar />

    <main class="main">
      <header class="topbar">
        <div class="topbar-title">
          <button type="button" class="back-btn" title="返回首页" @click="router.push({ name: 'home' })">
            <i class="ri-arrow-left-line" />
          </button>
          <h2>我的参数表</h2>
          <span v-if="spec.tables.length > 0" class="title-count">{{ spec.tables.length }}</span>
        </div>
        <Button size="sm" @click="openCreate">
          <i class="ri-add-line" />
          新建参数表
        </Button>
      </header>

      <div class="content ds-scroll-y">
        <!-- 空态 -->
        <div v-if="spec.tables.length === 0 && !spec.listLoading" class="empty">
          <div class="empty-mark">
            <i class="ri-file-list-line" />
          </div>
          <h3>还没有参数表</h3>
          <p>每张参数表对应一个产品。<br />建好之后可以直接让 AI 帮你填充参数。</p>
          <Button @click="openCreate">
            <i class="ri-add-line" />
            新建第一张参数表
          </Button>
        </div>

        <!-- 列表 -->
        <div v-else class="grid">
          <article
            v-for="table in spec.tables"
            :key="table.id"
            class="card"
            @click="open(table)"
          >
            <div class="card-head">
              <h3 class="ds-ellipsis">{{ table.title }}</h3>
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <button class="more-btn" type="button" title="更多操作" @click.stop>
                    <i class="ri-more-2-fill" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem @select="togglePin(table)">
                    <i :class="table.pinned ? 'ri-unpin-line' : 'ri-pushpin-line'" />
                    {{ table.pinned ? '取消置顶' : '置顶' }}
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="open(table)">
                    <i class="ri-arrow-right-up-line" />
                    打开
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="duplicate(table)">
                    <i class="ri-file-copy-line" />
                    复制为新表
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="openRename(table)">
                    <i class="ri-edit-line" />
                    重命名
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem class="text-destructive" @select="removing = table">
                    <i class="ri-delete-bin-line" />
                    删除
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>

            <p v-if="table.description" class="card-desc ds-ellipsis">{{ table.description }}</p>
            <p v-else class="card-desc placeholder">暂无描述</p>

            <div class="card-foot">
              <span v-if="table.industry" class="tag">{{ table.industry }}</span>
              <span class="stat">{{ table.item_count }} 个参数</span>
              <span class="dot">·</span>
              <span class="stat">r{{ table.current_revision }}</span>
              <span class="time">{{ formatRelativeTime(table.updated_at) }}</span>
            </div>
          </article>
        </div>
      </div>
    </main>

    <!-- 重命名 -->
    <Dialog :open="!!renaming" @update:open="(v) => !v && (renaming = null)">
      <DialogContent class="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>重命名参数表</DialogTitle>
        </DialogHeader>
        <Input v-model="renameValue" maxlength="120" placeholder="产品名称" autofocus @keydown.enter.prevent="submitRename" />
        <DialogFooter>
          <Button variant="outline" @click="renaming = null">取消</Button>
          <Button @click="submitRename">保存</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 删除确认 -->
    <AlertDialog :open="!!removing" @update:open="(v) => !v && (removing = null)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除参数表</AlertDialogTitle>
          <AlertDialogDescription>
            确定删除「{{ removing?.title }}」？删除后该表不再出现在列表里。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel @click="removing = null">取消</AlertDialogCancel>
          <AlertDialogAction variant="destructive" @click="confirmRemove">删除</AlertDialogAction>
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

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 20px;
  height: 56px;
  flex: none;
  border-bottom: 1px solid var(--ds-border);
  background: var(--ds-surface);
}

.topbar-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.back-btn {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 16px;
  cursor: pointer;
}

.back-btn:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.topbar h2 {
  margin: 0;
  font-size: var(--ds-font-base);
  font-weight: 600;
}

.title-count {
  padding: 1px 8px;
  border-radius: 20px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-xs);
}

.content {
  flex: 1;
  min-height: 0;
  padding: 20px;
}

/* 空态 */

.empty {
  max-width: 440px;
  margin: 8vh auto 0;
  text-align: center;
}

.empty-mark {
  width: 60px;
  height: 60px;
  margin: 0 auto 18px;
  display: grid;
  place-items: center;
  border-radius: 18px;
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
  font-size: 26px;
}

.empty h3 {
  margin: 0 0 8px;
  font-size: 17px;
  font-weight: 600;
  color: var(--ds-text);
}

.empty p {
  margin: 0 0 22px;
  font-size: var(--ds-font-sm);
  line-height: 1.85;
  color: var(--ds-text-secondary);
}

/* 卡片网格 */

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(258px, 1fr));
  gap: 14px;
}

.card {
  display: flex;
  flex-direction: column;
  padding: 15px 16px;
  border: 1px solid var(--ds-border);
  border-radius: var(--ds-radius-lg);
  background: var(--ds-surface);
  cursor: pointer;
  transition: all 0.16s ease;
}

.card:hover {
  border-color: var(--ds-primary);
  box-shadow: 0 6px 20px rgba(18, 22, 34, 0.07);
}

.card-head {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.card-head h3 {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: var(--ds-font-md);
  font-weight: 600;
  color: var(--ds-text);
}

.more-btn {
  width: 24px;
  height: 24px;
  flex: none;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--ds-text-tertiary);
  cursor: pointer;
  opacity: 0;
}

.card:hover .more-btn {
  opacity: 1;
}

.more-btn:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.card-desc {
  margin: 6px 0 14px;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-secondary);
  min-height: 19px;
}

.card-desc.placeholder {
  color: var(--ds-text-tertiary);
}

.card-foot {
  margin-top: auto;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
  font-variant-numeric: tabular-nums;
}

.tag {
  padding: 1px 7px;
  border-radius: 20px;
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
  font-size: var(--ds-font-xs);
}

.stat {
  font-variant-numeric: tabular-nums;
}

.dot {
  color: var(--ds-border-strong);
}

.time {
  margin-left: auto;
}
</style>
