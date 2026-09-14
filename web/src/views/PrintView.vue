<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import * as tables from '@/api/tables'
import { ApiError } from '@/api/http'
import type { SpecItem, TableDetail } from '@/types/api'
import SpecDocument from '@/components/table/SpecDocument.vue'

/**
 * 打印页：独立页面渲染整张表，进页面就唤起打印，供「导出 PDF」使用。
 *
 * 为什么单独一个页面而不是在原页面打印：
 * 原页面是 height:100% + overflow:hidden 的三栏应用外壳，要在打印时把它藏起来，
 * 得靠 body * { visibility: hidden } 之类的将就写法，既容易被新样式破坏，
 * 也容易漏掉某个fixed 元素。独占一页则完全不需要这些。
 *
 * 打开方式：ExportDialog 用 window.open 新标签页打开。因此打印结束后可以
 * 主动关掉这个标签页（脚本打开的窗口才允许 close）。
 */
const route = useRoute()

const tableId = String(route.params.id ?? '')

const table = ref<TableDetail | null>(null)
const items = ref<SpecItem[]>([])
const errorText = ref('')
const loading = ref(true)

/** 分栏数：2 列并排更省纸，只在屏幕上切换（工具栏本身不打印） */
const columns = ref<1 | 2>(1)

async function load() {
  try {
    const res = await tables.getTable(tableId)
    table.value = res.table
    items.value = res.items
    // 打印对话框默认用 document.title 作为「另存为 PDF」的默认文件名
    document.title = res.table.title || '参数表'
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

/** 手动触发打印：自动唤起被拦截时，用户还能从这个按钮再来一次 */
function print() {
  window.print()
}

function close() {
  window.close()
}

// 打印后不自动关闭本页：留给用户确认排版、必要时重打。
// 想恢复「打印完自动关页」时，在这里加 window.addEventListener('afterprint', () => window.close()) 即可
// （afterprint 在取消打印时也会触发，届时取消也会关页）。

onMounted(async () => {
  await load()
  if (errorText.value) return
  // 等一帧，确保文档渲染与样式就绪再唤打印，否则可能打出空白页
  await nextTick()
  requestAnimationFrame(() => window.print())
})
</script>

<template>
  <div class="print-page">
    <!-- 工具栏不打印：导出/返回的操作入口，纸上不需要 -->
    <div class="print-bar no-print">
      <span class="print-title">{{ table?.title || '参数表' }}</span>
      <div class="print-actions">
        <!-- 分栏切换：长表两列并排可省一半纸 -->
        <div class="col-toggle">
          <button
            type="button"
            class="col-btn"
            :class="{ active: columns === 1 }"
            title="单列"
            @click="columns = 1"
          >
            <i class="ri-layout-row-line" />
            单列
          </button>
          <button
            type="button"
            class="col-btn"
            :class="{ active: columns === 2 }"
            title="双列"
            @click="columns = 2"
          >
            <i class="ri-layout-column-line" />
            双列
          </button>
        </div>
        <button type="button" class="print-btn" @click="print">
          <i class="ri-printer-line" />
          打印 / 另存为 PDF
        </button>
        <button type="button" class="print-btn ghost" @click="close">关闭</button>
      </div>
    </div>

    <div v-if="loading" class="print-state">加载中…</div>
    <div v-else-if="errorText" class="print-state error">{{ errorText }}</div>
    <SpecDocument
      v-else-if="table"
      :title="table.title"
      :description="table.description"
      :industry="table.industry"
      :items="items"
      :columns="columns"
    />
  </div>
</template>

<style scoped>
.print-page {
  min-height: 100%;
  background: var(--ds-bg);
}

.print-bar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--ds-border);
  background: var(--ds-surface);
}

.print-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--ds-font-sm);
  font-weight: 600;
  color: var(--ds-text);
}

.print-actions {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 分栏切换：两段式按钮，当前档位高亮 */
.col-toggle {
  display: inline-flex;
  border: 1px solid var(--ds-border);
  border-radius: 8px;
  overflow: hidden;
}

.col-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  border: none;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  cursor: pointer;
}

.col-btn + .col-btn {
  border-left: 1px solid var(--ds-border);
}

.col-btn.active {
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
}

.print-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 12px;
  border: 1px solid var(--ds-border);
  border-radius: 8px;
  background: var(--ds-surface-muted);
  color: var(--ds-text);
  font-size: var(--ds-font-xs);
  cursor: pointer;
}

.print-btn:hover {
  border-color: var(--ds-primary);
}

.print-btn.ghost {
  background: transparent;
  color: var(--ds-text-secondary);
}

.print-state {
  padding: 64px 0;
  text-align: center;
  font-size: var(--ds-font-sm);
  color: var(--ds-text-tertiary);
}

.print-state.error {
  color: var(--ds-text-secondary);
}

@media print {
  /* 工具栏不打印 */
  .no-print {
    display: none;
  }

  .print-page {
    background: #fff;
  }
}
</style>
