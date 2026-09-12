<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'

import type { SpecItem } from '@/types/api'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const props = withDefaults(
  defineProps<{
    items: SpecItem[]
    highlightIds?: string[]
    revision?: number
    saving?: boolean
    canUndo?: boolean
    canRedo?: boolean
    readonly?: boolean
  }>(),
  {
    highlightIds: () => [],
    revision: 0,
    saving: false,
    canUndo: false,
    canRedo: false,
    readonly: false,
  },
)

const emit = defineEmits<{
  'update-item': [itemId: string, patch: { name?: string; value?: string }]
  'add-item': [
    draft: { name: string; value: string; group_name: string; value_type: 'text' | 'number' },
  ]
  'remove-item': [itemId: string, name: string]
  share: []
  remove: []
  undo: []
  redo: []
}>()

interface ItemGroup {
  name: string
  items: SpecItem[]
}

const groups = computed<ItemGroup[]>(() => {
  const map = new Map<string, SpecItem[]>()
  for (const item of props.items) {
    const key = item.group_name || ''
    const list = map.get(key)
    if (list) list.push(item)
    else map.set(key, [item])
  }
  return [...map.entries()].map(([name, list]) => ({ name, items: list }))
})

/**
 * 空表时补一个「未分组」占位分组。
 *
 * 这样「添加参数」按钮与新增行表单只有一条渲染路径，
 * 不必为「表里一个分组都还没有」再写一套重复的模板。
 */
const displayGroups = computed<ItemGroup[]>(() =>
  groups.value.length > 0 ? groups.value : [{ name: '', items: [] }],
)

// ---------- 行内编辑 ----------

/**
 * 提交单元格修改。
 *
 * 只在值真正变了时才发请求：`@change` 会在失焦时触发，
 * 用户点进点出不该产生一次变更和一个新版本。
 */
function commitField(item: SpecItem, field: 'name' | 'value', raw: string) {
  const next = raw.trim()
  if (next === item[field]) return
  // 名称不允许被清空，其余字段允许（表示置空）
  if (field === 'name' && next === '') return
  emit('update-item', item.id, { [field]: next })
}

// ---------- 新增参数 ----------

const addingGroup = ref<string | null>(null)
const draft = ref({ name: '', value: '' })
const nameInput = ref<HTMLInputElement | null>(null)

async function startAdd(groupName: string) {
  addingGroup.value = groupName
  draft.value = { name: '', value: '' }
  await nextTick()
  nameInput.value?.focus()
}

function cancelAdd() {
  addingGroup.value = null
}

function confirmAdd() {
  const name = draft.value.name.trim()
  if (!name) return
  const value = draft.value.value.trim()
  emit('add-item', {
    name,
    value,
    group_name: addingGroup.value ?? '',
    // 值以数字开头就按 number 存（服务端会校验它是不是「数字 + 可选单位」），
    // 否则按 text。用 text 时最宽松，任何写法都不会被拒。
    value_type: /^[-+]?\d/.test(value) ? 'number' : 'text',
  })
  addingGroup.value = null
}

function isHighlighted(id: string) {
  return props.highlightIds.includes(id)
}
</script>

<template>
  <section class="canvas">
    <header class="canvas-head">
      <span v-if="saving" class="saving">
        <i class="ri-loader-4-line spin" />
        保存中
      </span>
      <span v-else class="revision">版本: r{{ revision }}</span>

      <div class="canvas-actions">
        <Tooltip>
          <TooltipTrigger as-child>
            <!-- disabled 按钮不派发 pointer 事件，用 span 承接指针命中 -->
            <span class="tip-wrap">
              <button
                type="button"
                class="ghost-btn icon-only"
                :disabled="readonly || !canUndo"
                aria-label="撤销"
                @click="emit('undo')"
              >
                <i class="ri-arrow-go-back-line" />
              </button>
            </span>
          </TooltipTrigger>
          <TooltipContent side="bottom">撤销</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span class="tip-wrap">
              <button
                type="button"
                class="ghost-btn icon-only"
                :disabled="readonly || !canRedo"
                aria-label="重做"
                @click="emit('redo')"
              >
                <i class="ri-arrow-go-forward-line" />
              </button>
            </span>
          </TooltipTrigger>
          <TooltipContent side="bottom">重做</TooltipContent>
        </Tooltip>

        <Tooltip v-if="!readonly">
          <TooltipTrigger as-child>
            <button
              type="button"
              class="ghost-btn icon-only"
              aria-label="分享"
              @click="emit('share')"
            >
              <i class="ri-share-line" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">分享</TooltipContent>
        </Tooltip>

        <Tooltip v-if="!readonly">
          <TooltipTrigger as-child>
            <button
              type="button"
              class="ghost-btn danger icon-only"
              aria-label="删除这张参数表"
              @click="emit('remove')"
            >
              <i class="ri-delete-bin-line" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">删除这张参数表</TooltipContent>
        </Tooltip>
      </div>
    </header>

    <div class="canvas-body ds-scroll-y">
      <div v-if="items.length === 0 && addingGroup === null" class="canvas-empty">
        <i class="ri-table-line" />
        <p>这张表还没有参数<br />点下面的按钮手动添加，或在左侧对话里让 AI 帮你建</p>
        <button v-if="!readonly" type="button" class="empty-add" @click="startAdd('')">
          <i class="ri-add-line" /> 添加第一个参数
        </button>
      </div>

      <template v-else>
        <div v-for="group in displayGroups" :key="group.name || '__ungrouped__'" class="group">
          <div class="group-head">
            <i class="ri-folder-3-line" />
            <span>{{ group.name || '未分组' }}</span>
            <span class="group-count">{{ group.items.length }}</span>
          </div>

          <table v-if="group.items.length > 0" class="spec-table">
            <thead>
              <tr>
                <th class="col-name">参数名</th>
                <th class="col-value">参数值</th>
                <th v-if="!readonly" class="col-actions" />
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in group.items" :key="item.id" :class="{ 'ds-flash': isHighlighted(item.id) }">
                <td class="col-name">
                  <input
                    class="cell-input"
                    :value="item.name"
                    :readonly="readonly"
                    @change="commitField(item, 'name', ($event.target as HTMLInputElement).value)"
                  />
                </td>
                <td class="col-value">
                  <input
                    class="cell-input"
                    :value="item.value"
                    :readonly="readonly"
                    :placeholder="item.value_type === 'number' ? '数值' : '—'"
                    :inputmode="item.value_type === 'number' ? 'decimal' : 'text'"
                    @change="commitField(item, 'value', ($event.target as HTMLInputElement).value)"
                  />
                </td>
                <td v-if="!readonly" class="col-actions">
                  <button
                    type="button"
                    class="row-btn"
                    title="删除该参数"
                    @click="emit('remove-item', item.id, item.name)"
                  >
                    <i class="ri-delete-bin-line" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>

          <!-- 新增参数的内联表单：参数名必填，值与单位可以先留空 -->
          <div v-if="addingGroup === group.name" class="new-row">
            <input
              ref="nameInput"
              v-model="draft.name"
              class="cell-input"
              placeholder="参数名（必填）"
              @keydown.enter.prevent="confirmAdd"
              @keydown.esc="cancelAdd"
            />
            <input
              v-model="draft.value"
              class="cell-input"
              placeholder="参数值，如 5000mAh"
              @keydown.enter.prevent="confirmAdd"
            />
            <button type="button" class="confirm-btn" :disabled="!draft.name.trim()" @click="confirmAdd">确定</button>
            <button type="button" class="cancel-btn" @click="cancelAdd">取消</button>
          </div>

          <button v-else-if="!readonly" type="button" class="add-row" @click="startAdd(group.name)">
            <i class="ri-add-line" /> 添加参数
          </button>
        </div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.canvas {
  /* 画布固定宽度：多余空间全部留给左侧对话栏，浏览器宽度变化时只影响聊天区 */
  flex: none;
  width: 460px;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--ds-surface);
  border-left: 1px solid var(--ds-border);
}

.canvas-head {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 18px;
  height: 56px;
}

.canvas-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
}

.revision,
.saving {
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-xs);
  font-variant-numeric: tabular-nums;
}

/* 版本号只作说明文字，无背景无内边距 */
.revision {
  user-select: none;
}

.saving {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 7px;
  border-radius: 6px;
  background: var(--ds-surface-muted);
  color: var(--ds-primary);
}

.spin {
  animation: spin 1.1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.ghost-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  border-radius: 8px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  cursor: pointer;
}

.ghost-btn:hover:not(:disabled) {
  background: var(--ds-border);
  color: var(--ds-text);
}

.ghost-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  /* 指针事件交给外层 .tip-wrap，禁用时 tooltip 仍可弹出 */
  pointer-events: none;
}

/* 去掉文案后的图标按钮：收成与 chat-head 一致的方形热区 */
.ghost-btn.icon-only {
  width: 30px;
  height: 30px;
  padding: 0;
  justify-content: center;
  font-size: var(--ds-font-base);
}

/* 承接被禁用按钮的指针事件（见模板注释） */
.tip-wrap {
  display: inline-flex;
  cursor: pointer;
}

.tip-wrap:has(> .ghost-btn:disabled) {
  cursor: not-allowed;
}

/* 危险操作：平时中性，悬停才显红，避免误点 */
.ghost-btn.danger:hover:not(:disabled) {
  background: rgba(214, 69, 69, 0.1);
  color: #d64545;
}

.canvas-body {
  flex: 1;
  min-height: 0;
  padding: 16px 18px 28px;
}

.canvas-empty {
  padding: 56px 0;
  text-align: center;
  color: var(--ds-text-tertiary);
}

.canvas-empty i {
  font-size: 34px;
}

.canvas-empty p {
  margin: 12px 0 18px;
  font-size: var(--ds-font-sm);
  line-height: 1.8;
}

.empty-add {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 14px;
  border-radius: 9px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-sm);
  cursor: pointer;
}

.empty-add:hover {
  background: var(--ds-border);
  color: var(--ds-text);
}

.group {
  margin-bottom: 20px;
}

.group-head {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: var(--ds-font-xs);
  font-weight: 500;
  color: var(--ds-text-secondary);
}

.group-head i {
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-sm);
}

.group-count {
  padding: 0 6px;
  border-radius: 20px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-xs);
  font-weight: 400;
}

.spec-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  border: 1px solid var(--ds-border);
  border-radius: var(--ds-radius);
  overflow: hidden;
  table-layout: fixed;
}

.spec-table th {
  padding: 7px 12px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-xs);
  font-weight: 500;
  text-align: left;
}

.spec-table td {
  padding: 0 6px 0 0;
  height: 40px;
  border-top: 1px solid var(--ds-border);
  vertical-align: middle;
}

.spec-table td:first-child {
  padding-left: 6px;
}

.spec-table tbody tr:hover td {
  background: var(--ds-surface-muted);
}

.col-name {
  width: 38%;
}

.col-value {
  width: 62%;
}

.col-actions {
  width: 42px;
}

.cell-input {
  width: 100%;
  height: 30px;
  padding: 0 7px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--ds-text);
  font-family: inherit;
  font-size: var(--ds-font-sm);
  outline: none;
}

.cell-input.muted {
  color: var(--ds-text-secondary);
}

.cell-input:hover:not([readonly]) {
  border-color: var(--ds-border-strong);
}

.cell-input:focus {
  background: var(--ds-surface);
}

.cell-input::placeholder {
  color: var(--ds-text-tertiary);
}

.row-btn {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-sm);
  cursor: pointer;
  opacity: 0;
}

.spec-table tbody tr:hover .row-btn {
  opacity: 1;
}

.row-btn:hover {
  background: rgba(220, 70, 70, 0.12);
  color: #d64545;
}

.add-row {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: 7px;
  padding: 6px 12px;
  border-radius: 8px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  cursor: pointer;
}

.add-row:hover {
  background: var(--ds-border);
  color: var(--ds-text);
}

.new-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 7px;
  padding: 6px;
  border: 1px solid var(--ds-primary);
  border-radius: 9px;
  background: var(--ds-primary-soft);
}

.new-row .cell-input {
  background: var(--ds-surface);
  border-color: var(--ds-border);
}

/* 现在是两个输入框：参数名稍窄，值占大头 */
.new-row .cell-input:first-child {
  flex: 0 0 34%;
}

.new-row .cell-input:nth-child(2) {
  flex: 1;
}

.confirm-btn,
.cancel-btn {
  flex: none;
  padding: 6px 14px;
  border-radius: 7px;
  font-size: var(--ds-font-xs);
  cursor: pointer;
  border: 1px solid transparent;
}

.confirm-btn {
  background: var(--ds-primary);
  color: var(--ds-primary-foreground);
}

.confirm-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.cancel-btn {
  background: transparent;
  border-color: var(--ds-border);
  color: var(--ds-text-secondary);
}
</style>
