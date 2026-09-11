<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NDropdown, NInput, useMessage } from 'naive-ui'
import type { DropdownOption } from 'naive-ui'

import { useAuthStore } from '@/stores/auth'
import { useSpecTableStore } from '@/stores/specTable'
import { useThemeMode } from '@/composables/useThemeMode'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()
const spec = useSpecTableStore()
const { mode: themeMode, toggle: toggleTheme } = useThemeMode()

const keyword = ref('')

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return spec.tables
  return spec.tables.filter((t) => t.title.toLowerCase().includes(kw))
})

const activeId = computed(() => (route.name === 'workspace' ? String(route.params.id ?? '') : ''))

const displayName = computed(() => auth.user?.name || auth.user?.email || '未登录')
const avatarText = computed(() => displayName.value.slice(0, 1).toUpperCase())

const userOptions = computed<DropdownOption[]>(() => [
  { key: 'theme', label: themeMode.value === 'dark' ? '切换到浅色' : '切换到深色' },
  { type: 'divider', key: 'd1' },
  { key: 'logout', label: '退出登录' },
])

function onUserSelect(key: string) {
  if (key === 'theme') {
    toggleTheme()
    return
  }
  if (key === 'logout') {
    auth.logout()
    spec.reset()
    message.success('已退出登录')
    void router.replace({ name: 'login' })
  }
}

function openTable(id: string) {
  if (activeId.value === id) return
  void router.push({ name: 'workspace', params: { id } })
}

// 新建入口落在列表页：它负责弹窗与后续跳转，侧边栏只做导航
function goCreate() {
  void router.push({ name: 'tables', query: { new: '1' } })
}

onMounted(() => {
  void spec.loadTables().catch(() => undefined)
})
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-head">
      <div class="brand-mark">配</div>
      <button class="brand-name" type="button" title="返回首页" @click="router.push({ name: 'home' })">
        产品配置表
      </button>
    </div>

    <button class="new-btn" type="button" @click="goCreate">
      <i class="ri-add-line" />
      <span>新建配置表</span>
    </button>

    <div class="search">
      <n-input v-model:value="keyword" size="small" placeholder="搜索配置表" clearable>
        <template #prefix>
          <i class="ri-search-line" />
        </template>
      </n-input>
    </div>

    <div class="table-list ds-scroll-y">
      <template v-if="filtered.length > 0">
        <button
          v-for="table in filtered"
          :key="table.id"
          type="button"
          class="table-item"
          :class="{ active: table.id === activeId }"
          :title="table.title"
          @click="openTable(table.id)"
        >
          <i class="ri-file-list-3-line" />
          <span class="ds-ellipsis">{{ table.title }}</span>
          <span class="item-count">{{ table.item_count }}</span>
        </button>
      </template>

      <div v-else class="list-empty">
        <template v-if="keyword">没有匹配的配置表</template>
        <template v-else-if="spec.listLoading">加载中…</template>
        <template v-else>还没有配置表<br />点上方按钮创建第一张</template>
      </div>
    </div>

    <div class="sidebar-foot">
      <n-dropdown :options="userOptions" trigger="click" placement="top-start" @select="onUserSelect">
        <button class="user-btn" type="button">
          <span class="avatar">{{ avatarText }}</span>
          <span class="user-meta">
            <span class="ds-ellipsis user-name">{{ displayName }}</span>
            <span class="ds-ellipsis user-sub">{{ auth.user?.email }}</span>
          </span>
          <i class="ri-more-2-fill" />
        </button>
      </n-dropdown>
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
  background: var(--ds-surface);
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
  padding: 14px 14px 10px;
}

.brand-mark {
  width: 26px;
  height: 26px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--ds-primary);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
}

.brand-name {
  flex: 1;
  border: none;
  background: transparent;
  padding: 0;
  text-align: left;
  font-size: 14px;
  font-weight: 600;
  color: var(--ds-text);
  cursor: pointer;
}

.new-btn {
  display: flex;
  align-items: center;
  gap: 7px;
  margin: 4px 12px 10px;
  padding: 8px 12px;
  border: 1px solid var(--ds-border);
  border-radius: 9px;
  background: var(--ds-surface);
  color: var(--ds-text);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.16s ease;
}

.new-btn:hover {
  border-color: var(--ds-primary);
  color: var(--ds-primary);
  background: var(--ds-primary-soft);
}

.search {
  padding: 0 12px 8px;
}

.table-list {
  flex: 1;
  min-height: 0;
  padding: 2px 8px 12px;
}

.table-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  margin-bottom: 1px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.table-item:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.table-item.active {
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
  font-weight: 500;
}

.table-item i {
  flex: none;
  font-size: 14px;
}

.table-item .ds-ellipsis {
  flex: 1;
  min-width: 0;
}

.item-count {
  flex: none;
  font-size: 11px;
  color: var(--ds-text-tertiary);
  font-variant-numeric: tabular-nums;
}

.list-empty {
  padding: 28px 12px;
  text-align: center;
  font-size: 12.5px;
  line-height: 1.8;
  color: var(--ds-text-tertiary);
}

.sidebar-foot {
  padding: 10px;
  border-top: 1px solid var(--ds-border);
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

.avatar {
  width: 28px;
  height: 28px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--ds-primary);
  color: #fff;
  font-size: 12.5px;
  font-weight: 600;
}

.user-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.user-name {
  font-size: 13px;
  color: var(--ds-text);
}

.user-sub {
  font-size: 11.5px;
  color: var(--ds-text-tertiary);
}

.user-btn > i {
  flex: none;
  color: var(--ds-text-tertiary);
  font-size: 15px;
}
</style>
