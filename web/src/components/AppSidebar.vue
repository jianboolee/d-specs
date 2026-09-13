<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useAuthStore } from '@/stores/auth'
import { useSpecTableStore } from '@/stores/specTable'
import { useCreateTable } from '@/composables/useCreateTable'
import { useThemeMode } from '@/composables/useThemeMode'
import { useSidebar } from '@/composables/useSidebar'
import { useIsMobile, useMobileShell } from '@/composables/useMobileShell'
import BrandMark from '@/components/BrandMark.vue'
import TableSearchDialog from '@/components/table/TableSearchDialog.vue'
import type { TableSummary } from '@/types/api'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const spec = useSpecTableStore()
const { open: openCreate } = useCreateTable()
const { mode: themeMode, toggle: toggleTheme } = useThemeMode()
const { collapsed: sidebarCollapsed, toggle: toggleSidebar } = useSidebar()

// 手机上侧栏是抽屉，不用桌面那套「收起成图标」的形态
const isMobile = useIsMobile()
const { sidebarOpen, openSidebar, closeSidebar } = useMobileShell()
const isCollapsed = computed(() => !isMobile.value && sidebarCollapsed.value)

const showSearch = ref(false)

/**
 * 按置顶与否拆成虚拟分组。
 *
 * 服务端已按 pinned DESC, updated_at DESC 返回，filter 保序，
 * 所以置顶组与其余组内部都仍是「最近更新」顺序。
 */
const tableGroups = computed(() => {
  const pinned = spec.tables.filter((t) => t.pinned)
  const others = spec.tables.filter((t) => !t.pinned)

  const groups: { label: string; tables: TableSummary[] }[] = []
  if (pinned.length > 0) groups.push({ label: '置顶', tables: pinned })
  // 有置顶时才给其余表加标题：它们的排序就是最近更新；
  // 没有置顶时不加标题，保持原来的平铺外观
  if (others.length > 0) {
    groups.push({ label: pinned.length > 0 ? '最近更新' : '', tables: others })
  }
  return groups
})

const activeId = computed(() => (route.name === 'workspace' ? String(route.params.id ?? '') : ''))

// 停在列表页时，「我的参数表」像被选中的列表条目一样高亮
const isAllActive = computed(() => route.name === 'tables')

const displayName = computed(() => auth.user?.name || auth.user?.email || '未登录')
const avatarText = computed(() => displayName.value.slice(0, 1).toUpperCase())

function onThemeSelect() {
  toggleTheme()
}

function onLogout() {
  auth.logout()
  spec.reset()
  toast.success('已退出登录')
  void router.replace({ name: 'login' })
}

function openTable(id: string) {
  closeSidebar()
  if (activeId.value === id) return
  void router.push({ name: 'workspace', params: { id } })
}

// 直接唤起全局新建弹窗，停在当前页，不再跳到列表页
function goCreate() {
  closeSidebar()
  openCreate()
}

// 「全部」直接打开参数表列表页
function goAll() {
  closeSidebar()
  void router.push({ name: 'tables' })
}

/**
 * 公开发现页：全站已发布的参数表。
 *
 * 用新标签页打开而不是路由跳转 —— 发现页是独立公开页、不带侧栏，
 * 跳过去就回不来，还会丢掉当前正在编辑的表。
 */
function openDiscover() {
  closeSidebar()
  window.open('/discover', '_blank', 'noopener')
}

/** 搜索结果被选中：打开该表并收起搜索面板 */
function openSearchResult(table: TableSummary) {
  showSearch.value = false
  openTable(table.id)
}

onMounted(() => {
  void spec.loadTables().catch(() => undefined)
})
</script>

<template>
  <!-- 手机上抽屉打开时的背景遮罩：点击关闭。加 isMobile 守卫，避免抽屉开着时
       把窗口拉宽到桌面，遮罩残留下来盖住整个桌面布局 -->
  <div v-if="sidebarOpen && isMobile" class="sidebar-backdrop" @click="closeSidebar" />

  <!-- 手机上收纳侧栏后的入口：固定左上角，三个带侧栏的页面共用 -->
  <button v-if="isMobile && !sidebarOpen" class="hamburger" type="button" aria-label="打开侧栏" @click="openSidebar">
    <i class="ri-menu-line" />
  </button>

  <aside class="sidebar" :class="{ collapsed: isCollapsed, 'mobile-open': sidebarOpen }">
    <div class="sidebar-head">
      <template v-if="!isCollapsed">
        <button class="brand-btn" type="button" title="返回工作台" @click="router.push({ name: 'dashboard' })">
          <BrandMark :size="26" />
        </button>
      </template>
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            class="collapse-btn"
            type="button"
            @click="toggleSidebar"
          >
            <i class="ri-layout-left-line" />
          </button>
        </TooltipTrigger>
        <TooltipContent :side="isCollapsed ? 'right' : 'bottom'">
          {{ isCollapsed ? '展开侧栏' : '收起侧栏' }}
        </TooltipContent>
      </Tooltip>
    </div>

    <button class="new-btn" type="button" :title="isCollapsed ? '新建参数表' : undefined" @click="goCreate">
      <i class="ri-add-line" />
      <span v-if="!isCollapsed">新建参数表</span>
    </button>

    <!-- 新建 / 搜索 / 发现 / 我的参数表 归为一组：都是「动作」，与下面的表列表分开 -->
    <div class="sidebar-actions">
      <button
        type="button"
        class="table-item"
        :title="isCollapsed ? '搜索参数表' : undefined"
        @click="showSearch = true"
      >
        <i class="ri-search-line" />
        <span v-if="!isCollapsed" class="ds-ellipsis">搜索</span>
      </button>

      <button
        type="button"
        class="table-item"
        :title="isCollapsed ? '发现' : undefined"
        @click="openDiscover"
      >
        <i class="ri-compass-3-line" />
        <span v-if="!isCollapsed" class="ds-ellipsis">发现</span>
      </button>

      <button
        type="button"
        class="table-item"
        :class="{ active: isAllActive }"
        :title="isCollapsed ? '我的参数表' : undefined"
        @click="goAll"
      >
        <i class="ri-folder-line" />
        <span v-if="!isCollapsed" class="ds-ellipsis">我的参数表</span>
        <span v-if="!isCollapsed && spec.tables.length > 0" class="item-count">
          {{ spec.tables.length }}
        </span>
      </button>
    </div>

    <div class="table-list ds-scroll-y">
      <!-- 收起时只保留上面的入口，不铺开具体表 -->
      <template v-if="!isCollapsed">
        <!-- 置顶表单独成组；没有置顶时不显示任何分组标题，保持原来的平铺 -->
        <template v-for="group in tableGroups" :key="group.label || 'all'">
          <div v-if="group.label" class="list-group-label">{{ group.label }}</div>
          <button
            v-for="table in group.tables"
            :key="table.id"
            type="button"
            class="table-item"
            :class="{ active: table.id === activeId }"
            @click="openTable(table.id)"
          >
            <!-- 已公开分享：圆点放在左侧图标列，不占位、不影响标题对齐 -->
            <span v-if="table.visibility === 'link'" class="dot-public" title="已公开分享" />
            <span class="ds-ellipsis">{{ table.title }}</span>
            <span class="item-count">{{ table.item_count }}</span>
          </button>
        </template>

        <div v-if="spec.tables.length === 0" class="list-empty">
          <template v-if="spec.listLoading">加载中…</template>
          <template v-else>还没有参数表<br />点上方按钮创建第一张</template>
        </div>
      </template>
    </div>

    <div class="sidebar-foot">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button class="user-btn" type="button" :title="isCollapsed ? displayName : undefined">
            <span class="avatar">{{ avatarText }}</span>
            <span v-if="!isCollapsed" class="user-meta">
              <span class="ds-ellipsis user-name">{{ displayName }}</span>
              <span class="ds-ellipsis user-sub">{{ auth.user?.email }}</span>
            </span>
            <i v-if="!isCollapsed" class="ri-more-2-fill" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" class="w-52 ring-0">
          <DropdownMenuItem class="py-2" @select="onThemeSelect">
            <i class="ri-sun-line" v-if="themeMode === 'dark'" />
            <i class="ri-moon-line" v-else />
            {{ themeMode === 'dark' ? '切换到浅色' : '切换到深色' }}
          </DropdownMenuItem>
          <DropdownMenuItem class="py-2 text-destructive" @select="onLogout">
            <i class="ri-logout-box-r-line" />
            退出登录
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </aside>

  <!-- 搜索面板：居中弹出，查询只覆盖当前用户自己的表 -->
  <TableSearchDialog v-model:open="showSearch" @select="openSearchResult" />
</template>

<style scoped>
.sidebar {
  width: var(--ds-sidebar-width);
  flex: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border-right: 1px solid var(--ds-border);
  background: var(--ds-sidebar-bg);
  transition: width 0.2s ease;
}

.sidebar.collapsed {
  width: 64px;
}

/* 侧边栏的头部/新建/动作组/底部都要固定，只有列表滚动 */
.sidebar-head,
.new-btn,
.sidebar-actions,
.sidebar-foot {
  flex: none;
}

.sidebar-head {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 15px 14px;
}

.collapse-btn {
  flex: none;
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-base);
  cursor: pointer;
  transition: background-color 0.12s ease;
}

.collapse-btn:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.sidebar.collapsed .sidebar-head {
  padding: 15px 0;
  justify-content: center;
}

.brand-btn {
  flex: 1;
  display: flex;
  align-items: center;
  border: none;
  background: transparent;
  padding: 0;
  text-align: left;
  cursor: pointer;
}

.new-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin: 12px 12px;
  padding: 9px 14px;
  border: none;
  border-radius: 9999px;
  background: var(--ds-surface);
  color: var(--ds-text);
  font-size: var(--ds-font-md);
  font-weight: 500;
  cursor: pointer;
  box-shadow: var(--ds-shadow-sm);
  transition:
    background-color 0.15s ease,
    box-shadow 0.15s ease;
}

.new-btn:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
  box-shadow: var(--ds-shadow-md);
}

.sidebar.collapsed .new-btn {
  padding: 9px 0;
  justify-content: center;
}

/* 新建 / 搜索 / 我的参数表：三个动作收在一组，与下面的表列表用分隔线分开 */
.sidebar-actions {
  padding: 2px 8px 8px;
  border-bottom: 1px solid var(--ds-border);
}

.table-list {
  flex: 1;
  min-height: 0;
  padding: 12px 8px;
}

/* 列表里的表条目缩进到图标列右侧，给左侧的公开圆点留出位置。
   同时作为圆点的定位容器。 */
.table-list .table-item {
  position: relative;
  padding-left: 38px;
}

.table-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  margin-bottom: 2px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text);
  font-size: var(--ds-font-md);
  text-align: left;
  cursor: pointer;
  transition: background-color 0.12s ease;
}

.table-item:hover {
  background: var(--ds-surface-muted);
}

.table-item.active {
  background: var(--ds-surface-muted);
}

.table-item i {
  flex: none;
  /* 比正文略大，让列表条目更有识别度；颜色与条目文字一致 */
  font-size: 18px;
  color: var(--ds-text);
}

.sidebar.collapsed .table-item {
  padding: 8px 0;
  justify-content: center;
}

.table-item .ds-ellipsis {
  flex: 1;
  min-width: 0;
}

.item-count {
  flex: none;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
  font-variant-numeric: tabular-nums;
}

/* 分组小标题：不可点击，贴左、不与表条目标题对齐 */
.list-group-label {
  padding: 8px 10px;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

/* 已公开分享：圆点绝对定位在左侧图标列，不占位、不影响标题对齐 */
.dot-public {
  position: absolute;
  /* 4px 圆点居中于图标列（图标列 10~28，中心 19） */
  left: 17px;
  top: 50%;
  transform: translateY(-50%);
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--ds-share-dot);
}

.list-empty {
  padding: 28px 12px;
  text-align: center;
  font-size: var(--ds-font-xs);
  line-height: 1.8;
  color: var(--ds-text-tertiary);
}

.sidebar-foot {
  padding: 12px 10px;
}

.sidebar.collapsed .sidebar-foot {
  padding: 12px 8px;
}

.user-btn {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 8px;
  border: none;
  border-radius: 9px;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.user-btn:hover {
  background: var(--ds-surface-muted);
}

.sidebar.collapsed .user-btn {
  padding: 7px 0;
  justify-content: center;
}

.avatar {
  width: 28px;
  height: 28px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  font-weight: 600;
}

.user-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.user-name {
  font-size: var(--ds-font-sm);
  color: var(--ds-text);
}

.user-sub {
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

.user-btn > i {
  flex: none;
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-base);
}

/* ==================== 手机端：侧栏变抽屉 ==================== */

/* 汉堡按钮仅手机出现（模板里已用 isMobile 控制），桌面这些规则全部不生效 */
.hamburger {
  position: fixed;
  top: 10px;
  left: 10px;
  z-index: 40;
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 9px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 20px;
  cursor: pointer;
}

.hamburger:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

/* 遮罩默认不存在，只在手机上打开抽屉时由模板渲染 */
.sidebar-backdrop {
  position: fixed;
  inset: 0;
  z-index: 49;
  background: rgba(0, 0, 0, 0.4);
}

@media (max-width: 768px) {
  .sidebar {
    position: fixed;
    top: 0;
    bottom: 0;
    left: 0;
    z-index: 50;
    width: min(82vw, 300px);
    /* 抽屉收在屏幕外，打开时滑入 */
    transform: translateX(-100%);
    transition: transform 0.22s ease;
  }

  .sidebar.mobile-open {
    transform: translateX(0);
  }

  /* 手机上用抽屉＋遮罩，桌面那套「收起成图标」的按钮不再需要 */
  .collapse-btn {
    display: none;
  }
}
</style>
