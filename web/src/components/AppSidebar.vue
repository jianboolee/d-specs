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
import { Input } from '@/components/ui/input'
import { useAuthStore } from '@/stores/auth'
import { useSpecTableStore } from '@/stores/specTable'
import { useCreateTable } from '@/composables/useCreateTable'
import { useThemeMode } from '@/composables/useThemeMode'
import { useSidebar } from '@/composables/useSidebar'
import BrandMark from '@/components/BrandMark.vue'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const spec = useSpecTableStore()
const { open: openCreate } = useCreateTable()
const { mode: themeMode, toggle: toggleTheme } = useThemeMode()
const { collapsed, toggle: toggleSidebar } = useSidebar()

const keyword = ref('')

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return spec.tables
  return spec.tables.filter((t) => t.title.toLowerCase().includes(kw))
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
  if (activeId.value === id) return
  void router.push({ name: 'workspace', params: { id } })
}

// 直接唤起全局新建弹窗，停在当前页，不再跳到列表页
function goCreate() {
  openCreate()
}

// 「全部」直接打开参数表列表页
function goAll() {
  void router.push({ name: 'tables' })
}

onMounted(() => {
  void spec.loadTables().catch(() => undefined)
})
</script>

<template>
  <aside class="sidebar" :class="{ collapsed }">
    <div class="sidebar-head">
      <template v-if="!collapsed">
        <button class="brand-btn" type="button" title="返回首页" @click="router.push({ name: 'home' })">
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
        <TooltipContent :side="collapsed ? 'right' : 'bottom'">
          {{ collapsed ? '展开侧栏' : '收起侧栏' }}
        </TooltipContent>
      </Tooltip>
    </div>

    <button class="new-btn" type="button" :title="collapsed ? '新建参数表' : undefined" @click="goCreate">
      <i class="ri-add-line" />
      <span v-if="!collapsed">新建参数表</span>
    </button>

    <div v-if="!collapsed" class="search">
      <div class="search-box">
        <i class="ri-search-line" />
        <Input
          v-model="keyword"
          class="search-input"
          placeholder="搜索参数表"
          type="search"
        />
      </div>
    </div>

    <div class="table-list ds-scroll-y">
      <!-- 「我的参数表」：进入列表页的入口，样式与下面的表条目保持一致。
           它保留图标作为标识（收起时图标也是唯一的可点击标识）。 -->
      <button
        type="button"
        class="table-item list-all"
        :class="{ active: isAllActive }"
        :title="collapsed ? '我的参数表' : undefined"
        @click="goAll"
      >
        <i class="ri-file-list-3-line" />
        <span v-if="!collapsed" class="ds-ellipsis">我的参数表</span>
        <span v-if="!collapsed && spec.tables.length > 0" class="item-count">
          {{ spec.tables.length }}
        </span>
      </button>

      <!-- 收起时只保留上面的「我的参数表」，不铺开具体表 -->
      <template v-if="!collapsed">
        <button
          v-for="table in filtered"
          :key="table.id"
          type="button"
          class="table-item"
          :class="{ active: table.id === activeId }"
          @click="openTable(table.id)"
        >
          <!-- 已公开分享：圆点放在左侧图标列，弱提示且不挤占标题 -->
          <span v-if="table.visibility === 'link'" class="dot-public" title="已公开分享" />
          <span class="ds-ellipsis">{{ table.title }}</span>
          <span class="item-count">{{ table.item_count }}</span>
        </button>

        <div v-if="filtered.length === 0" class="list-empty">
          <template v-if="keyword">没有匹配的参数表</template>
          <template v-else-if="spec.listLoading">加载中…</template>
          <template v-else>还没有参数表<br />点上方按钮创建第一张</template>
        </div>
      </template>
    </div>

    <div class="sidebar-foot">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button class="user-btn" type="button" :title="collapsed ? displayName : undefined">
            <span class="avatar">{{ avatarText }}</span>
            <span v-if="!collapsed" class="user-meta">
              <span class="ds-ellipsis user-name">{{ displayName }}</span>
              <span class="ds-ellipsis user-sub">{{ auth.user?.email }}</span>
            </span>
            <i v-if="!collapsed" class="ri-more-2-fill" />
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

/* 侧边栏的头部/按钮/搜索/底部都要固定，只有列表滚动 */
.sidebar-head,
.new-btn,
.search,
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

.search {
  padding: 2px 12px 6px;
}

.search-box {
  position: relative;
  display: flex;
  align-items: center;
}

.search-box > i {
  position: absolute;
  left: 13px;
  color: var(--ds-text-tertiary);
  font-size: 17px;
  pointer-events: none;
  z-index: 1;
}

.search-input {
  padding-left: 38px;
  height: 38px;
  border-radius: var(--ds-radius);
  /* 侧栏是浅灰底，输入框用白底填充，更有存在感 */
  background: var(--ds-surface);
  font-size: var(--ds-font-sm);
}

.search-input:hover {
  border-color: var(--ds-border-strong);
}

.search-input:focus-visible {
  border-color: var(--ds-text-tertiary);
}

.table-list {
  flex: 1;
  min-height: 0;
  padding: 12px 8px;
}

/* 「我的参数表」是分组入口（带图标），下面的表条目是它的子项。
   子项缩进到与入口文字左对齐，形成层级；不用加粗，选中态也只靠底色区分。 */
.list-all {
  margin-bottom: 6px;
}

/* 左边距 = 入口的 padding-left(10) + 图标(18) + gap(10)，让子项文字与入口文字对齐。
   同时作为圆点的定位容器。 */
.table-item:not(.list-all) {
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

/* 已公开分享：圆点绝对定位在左侧图标列，不占位、不影响标题对齐 */
.dot-public {
  position: absolute;
  left: 16px;
  top: 50%;
  transform: translateY(-50%);
  width: 6px;
  height: 6px;
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
</style>
