<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'

import type { SpecItem } from '@/types/api'

const props = withDefaults(
  defineProps<{
    title: string
    description?: string
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
  'update-item': [itemId: string, patch: { name?: string; value?: string; unit?: string }]
  'add-item': [
    draft: { name: string; value: string; unit: string; group_name: string; value_type: 'text' | 'number' },
  ]
  'remove-item': [itemId: string, name: string]
  rename: [title: string]
  share: []
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
function commitField(item: SpecItem, field: 'name' | 'value' | 'unit', raw: string) {
  const next = raw.trim()
  if (next === item[field]) return
  // 名称不允许被清空，其余字段允许（表示置空）
  if (field === 'name' && next === '') return
  emit('update-item', item.id, { [field]: next })
}

// ---------- 新增参数 ----------

const addingGroup = ref<string | null>(null)
const draft = ref({ name: '', value: '', unit: '' })
const nameInput = ref<HTMLInputElement | null>(null)

async function startAdd(groupName: string) {
  addingGroup.value = groupName
  draft.value = { name: '', value: '', unit: '' }
  await nextTick()
  nameInput.value?.focus()
}

function cancelAdd() {
  addingGroup.value = null
}

function confirmAdd() {
  const name = draft.value.name.trim()
  if (!name) return
  const unit = draft.value.unit.trim()
  emit('add-item', {
    name,
    value: draft.value.value.trim(),
    unit,
    group_name: addingGroup.value ?? '',
    // 填了单位说明这是个带单位的量（198 g、5000 mAh），用 number 让服务端
    // 把数值与单位拆开、并校验其可解析性；没填单位就按 text 存，最宽松
    value_type: unit ? 'number' : 'text',
  })
  addingGroup.value = null
}

// ---------- 标题编辑 ----------

const titleDraft = ref('')
const editingTitle = ref(false)

function startRename() {
  if (props.readonly) return
  titleDraft.value = props.title
  editingTitle.value = true
}

function commitRename() {
  editingTitle.value = false
  const next = titleDraft.value.trim()
  if (!next || next === props.title) return
  emit('rename', next)
}

function isHighlighted(id: string) {
  return props.highlightIds.includes(id)
}
</script>

<template>
  <section class="canvas">
    <header class="canvas-head">
      <div class="canvas-title">
        <input
          v-if="editingTitle"
          v-model="titleDraft"
          class="title-input"
          @blur="commitRename"
          @keydown.enter.prevent="commitRename"
          @keydown.esc="editingTitle = false"
        />
        <h3 v-else class="ds-ellipsis" :class="{ clickable: !readonly }" @click="startRename">
          {{ title }}
        </h3>
        <span v-if="description" class="ds-ellipsis canvas-desc">{{ description }}</span>
      </div>

      <div class="canvas-actions">
        <span v-if="saving" class="saving">
          <i class="ri-loader-4-line spin" />
          保存中
        </span>
        <span v-else class="revision" :title="`当前版本 r${revision}`">r{{ revision }}</span>

        <button type="button" class="ghost-btn" :disabled="readonly || !canUndo" @click="emit('undo')">
          <i class="ri-arrow-go-back-line" /> 撤销
        </button>
        <button type="button" class="ghost-btn" :disabled="readonly || !canRedo" @click="emit('redo')">
          <i class="ri-arrow-go-forward-line" /> 重做
        </button>
        <button v-if="!readonly" type="button" class="ghost-btn" @click="emit('share')">
          <i class="ri-share-line" /> 分享
        </button>
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
                <th class="col-value">值</th>
                <th class="col-unit">单位</th>
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
                <td class="col-unit">
                  <input
                    class="cell-input muted"
                    :value="item.unit"
                    :readonly="readonly"
                    placeholder="—"
                    @change="commitField(item, 'unit', ($event.target as HTMLInputElement).value)"
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
            <input v-model="draft.value" class="cell-input" placeholder="值" @keydown.enter.prevent="confirmAdd" />
            <input v-model="draft.unit" class="cell-input" placeholder="单位" @keydown.enter.prevent="confirmAdd" />
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
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--ds-surface);
}

.canvas-head {
  flex: none;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 0 18px;
  height: 56px;
  border-bottom: 1px solid var(--ds-border);
}

.canvas-title {
  flex: 1;
  min-width: 0;
}

.canvas-title h3 {
  margin: 0;
  font-size: 14.5px;
  font-weight: 600;
  color: var(--ds-text);
}

.canvas-title h3.clickable {
  cursor: text;
  border-radius: 5px;
  padding: 1px 4px;
  margin-left: -4px;
}

.canvas-title h3.clickable:hover {
  background: var(--ds-surface-muted);
}

.title-input {
  width: 100%;
  font-size: 14.5px;
  font-weight: 600;
  color: var(--ds-text);
  border: 1px solid var(--ds-primary);
  border-radius: 6px;
  padding: 2px 6px;
  outline: none;
  background: var(--ds-surface);
  font-family: inherit;
}

.canvas-desc {
  display: block;
  font-size: 12px;
  color: var(--ds-text-tertiary);
}

.canvas-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
}

.revision,
.saving {
  padding: 2px 7px;
  border-radius: 6px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-tertiary);
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
}

.saving {
  display: inline-flex;
  align-items: center;
  gap: 4px;
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
  padding: 5px 9px;
  border: 1px solid var(--ds-border);
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 12.5px;
  cursor: pointer;
}

.ghost-btn:hover:not(:disabled) {
  border-color: var(--ds-primary);
  color: var(--ds-primary);
}

.ghost-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
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
  font-size: 13px;
  line-height: 1.8;
}

.empty-add {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 14px;
  border: 1px solid var(--ds-border);
  border-radius: 9px;
  background: var(--ds-surface);
  color: var(--ds-text-secondary);
  font-size: 13px;
  cursor: pointer;
}

.empty-add:hover {
  border-color: var(--ds-primary);
  color: var(--ds-primary);
}

.group {
  margin-bottom: 20px;
}

.group-head {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--ds-text-secondary);
}

.group-head i {
  color: var(--ds-text-tertiary);
  font-size: 13px;
}

.group-count {
  padding: 0 6px;
  border-radius: 20px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-tertiary);
  font-size: 11px;
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
  font-size: 11.5px;
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
  width: 36%;
}

.col-value {
  width: 32%;
}

.col-unit {
  width: 20%;
  color: var(--ds-text-secondary);
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
  font-size: 13px;
  outline: none;
}

.cell-input.muted {
  color: var(--ds-text-secondary);
}

.cell-input:hover:not([readonly]) {
  border-color: var(--ds-border-strong);
}

.cell-input:focus {
  border-color: var(--ds-primary);
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
  font-size: 13px;
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
  padding: 5px 10px;
  border: 1px dashed var(--ds-border-strong);
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-tertiary);
  font-size: 12.5px;
  cursor: pointer;
}

.add-row:hover {
  border-color: var(--ds-primary);
  color: var(--ds-primary);
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

.new-row .cell-input:first-child {
  flex: 1.2;
}

.new-row .cell-input:nth-child(2) {
  flex: 1;
}

.new-row .cell-input:nth-child(3) {
  flex: 0.7;
}

.confirm-btn,
.cancel-btn {
  flex: none;
  padding: 5px 11px;
  border-radius: 7px;
  font-size: 12.5px;
  cursor: pointer;
  border: 1px solid transparent;
}

.confirm-btn {
  background: var(--ds-primary);
  color: #fff;
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
