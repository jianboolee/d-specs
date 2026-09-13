<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'

import { ApiError } from '@/api/http'
import * as api from '@/api/tables'
import type { TableSummary } from '@/types/api'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

/**
 * 参数表搜索面板。
 *
 * 刻意不含弹窗外壳：侧栏用 TableSearchDialog 居中弹出，将来做独立搜索页时
 * 直接内联渲染本组件即可，查询与键盘交互只有这一份实现。
 *
 * 搜索走后端 listTables（按 user_id 过滤），因此天然只搜到当前用户自己的表。
 */
const props = withDefaults(
  defineProps<{
    placeholder?: string
    autofocus?: boolean
  }>(),
  {
    placeholder: '搜索参数表',
    autofocus: true,
  },
)

const emit = defineEmits<{
  select: [table: TableSummary]
  close: []
}>()

/** 输入防抖时长：太短会每敲一个字打一次请求，太长又会显得迟钝 */
const DEBOUNCE_MS = 250

const keyword = ref('')
const results = ref<TableSummary[]>([])
const loading = ref(false)
const activeIndex = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)
const listRef = ref<HTMLElement | null>(null)

const isEmpty = computed(() => !loading.value && results.value.length === 0)
const emptyText = computed(() =>
  keyword.value.trim() ? '没有匹配的参数表' : '还没有参数表',
)

let timer: number | undefined
let seq = 0

async function runSearch() {
  const current = ++seq
  loading.value = true
  try {
    // 空关键词时后端按 updated_at 返回最近更新的表，正好作为默认列表
    const res = await api.listTables({ keyword: keyword.value.trim(), size: 50 })
    // 丢弃过期响应：防抖 + 快速输入时可能先后返回，只认最后一次
    if (current !== seq) return
    results.value = res.items
    activeIndex.value = 0
  } catch (err) {
    if (current !== seq) return
    results.value = []
    // 搜索失败不弹全局 toast 打扰用户，表现为空结果即可
    console.warn(err instanceof ApiError ? err.message : '搜索失败')
  } finally {
    if (current === seq) loading.value = false
  }
}

watch(keyword, () => {
  if (timer) window.clearTimeout(timer)
  timer = window.setTimeout(runSearch, DEBOUNCE_MS)
})

function onKeydown(event: KeyboardEvent) {
  // 中文输入法组词期间的回车不当作选中
  if (event.isComposing) return

  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      move(1)
      break
    case 'ArrowUp':
      event.preventDefault()
      move(-1)
      break
    case 'Enter':
      event.preventDefault()
      choose(activeIndex.value)
      break
    case 'Escape':
      event.preventDefault()
      emit('close')
      break
  }
}

function move(delta: number) {
  if (results.value.length === 0) return
  const next = activeIndex.value + delta
  // 上下循环，避免在两端停住
  activeIndex.value = (next + results.value.length) % results.value.length
  scrollActiveIntoView()
}

async function scrollActiveIntoView() {
  await nextTick()
  const el = listRef.value?.querySelector<HTMLElement>('.is-active')
  el?.scrollIntoView({ block: 'nearest' })
}

function choose(index: number) {
  const table = results.value[index]
  if (table) emit('select', table)
}

/** 新标签页打开公开发现页：本面板搜的是自己的表，发现页搜的是全站已发布的表 */
function openDiscover() {
  window.open('/discover', '_blank', 'noopener')
}

onMounted(() => {
  void runSearch()
  if (props.autofocus) {
    // 让弹窗动画结束后再聚焦，避免焦点被 Dialog 抢回
    void nextTick(() => inputRef.value?.focus())
  }
})

defineExpose({ focus: () => inputRef.value?.focus() })
</script>

<template>
  <div class="search-panel">
    <div class="search-field">
      <i class="ri-search-line" />
      <input
        ref="inputRef"
        v-model="keyword"
        class="search-input"
        type="text"
        :placeholder="placeholder"
        autocomplete="off"
        @keydown="onKeydown"
      />
      <i v-if="loading" class="ri-loader-4-line spin" />

      <!-- 快捷入口：在新标签页浏览全站已发布的参数表 -->
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            class="discover-btn"
            aria-label="在新标签页浏览公开参数表"
            @click="openDiscover"
          >
            <i class="ri-external-link-line" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="bottom">在新标签页浏览公开参数表</TooltipContent>
      </Tooltip>
    </div>

    <div ref="listRef" class="search-results ds-scroll-y">
      <button
        v-for="(table, i) in results"
        :key="table.id"
        type="button"
        class="result-item"
        :class="{ 'is-active': i === activeIndex }"
        @mousemove="activeIndex = i"
        @click="choose(i)"
      >
        <i class="ri-file-list-line" />
        <span class="result-main">
          <span class="ds-ellipsis result-title">{{ table.title }}</span>
          <span v-if="table.description" class="ds-ellipsis result-desc">
            {{ table.description }}
          </span>
        </span>
        <span v-if="table.pinned" class="result-pin" title="已置顶">
          <i class="ri-pushpin-fill" />
        </span>
        <span class="result-count">{{ table.item_count }}</span>
      </button>

      <div v-if="isEmpty" class="search-empty">{{ emptyText }}</div>
    </div>
  </div>
</template>

<style scoped>
.search-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.search-field {
  position: relative;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid var(--ds-border);
}

.search-field > i:first-child {
  position: absolute;
  left: 18px;
  color: var(--ds-text-tertiary);
  font-size: 18px;
  pointer-events: none;
}

.search-field > i.spin {
  margin-left: 10px;
  color: var(--ds-text-tertiary);
  font-size: 16px;
}

.search-input {
  flex: 1;
  min-width: 0;
  height: 52px;
  padding-left: 30px;
  border: none;
  outline: none;
  background: transparent;
  color: var(--ds-text);
  font-family: inherit;
  font-size: var(--ds-font-md);
}

.search-input::placeholder {
  color: var(--ds-text-tertiary);
}

/* 右侧快捷入口：与侧栏 head-btn / 画布 ghost-btn 同尺寸同手感 */
.discover-btn {
  flex: none;
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-tertiary);
  font-size: 17px;
  cursor: pointer;
}

.discover-btn:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.search-results {
  flex: 1;
  min-height: 0;
  max-height: 340px;
  padding: 8px;
}

.result-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text);
  text-align: left;
  cursor: pointer;
}

.result-item.is-active {
  background: var(--ds-surface-muted);
}

.result-item > i {
  flex: none;
  font-size: 17px;
  color: var(--ds-text-tertiary);
}

.result-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.result-title {
  font-size: var(--ds-font-sm);
  color: var(--ds-text);
}

.result-desc {
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

.result-pin {
  flex: none;
  font-size: 12px;
  color: var(--ds-text-tertiary);
}

.result-count {
  flex: none;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
  font-variant-numeric: tabular-nums;
}

.search-empty {
  padding: 28px 12px;
  text-align: center;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

.spin {
  animation: spin 1.1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
