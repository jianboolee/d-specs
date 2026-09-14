<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'

import type { SpecItem } from '@/types/api'
import { useCanvas } from '@/composables/useCanvas'
import { useIsMobile, useMobileShell } from '@/composables/useMobileShell'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const props = withDefaults(
  defineProps<{
    items: SpecItem[]
    /** 表名。手机上画布是全屏覆盖层，会盖住对话区的标题，所以这里也要显示一份 */
    title?: string
    /** 表简介，有内容时展示在参数列表上方 */
    description?: string
    highlightIds?: string[]
    revision?: number
    saving?: boolean
    canUndo?: boolean
    canRedo?: boolean
    readonly?: boolean
  }>(),
  {
    title: '',
    description: '',
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
  /** 表名/描述的元信息修改（就地编辑标题与描述） */
  'update-meta': [patch: { title?: string; description?: string }]
  share: []
  undo: []
  redo: []
}>()

const { collapsed, toggle: toggleCanvas } = useCanvas()

// 手机上画布是全屏覆盖层，用临时的 canvasOpen 控制，不碰桌面那套持久化收起状态
const isMobile = useIsMobile()
const { canvasOpen, closeCanvas } = useMobileShell()

// 对外「看不见」的统一判断：手机看覆盖层，桌面看收起态
const hidden = computed(() => (isMobile.value ? !canvasOpen.value : collapsed.value))

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

// ---------- 表名 / 描述的就地编辑 ----------

const descRef = ref<HTMLTextAreaElement | null>(null)

/**
 * 描述框随内容自适应高度。
 *
 * 先置 auto 再读 scrollHeight：不这样，收缩（删行）时高度只会停在最大值。
 * 两次写在同一帧内，不会看到中间态。
 */
function resizeDesc() {
  const el = descRef.value
  if (!el) return
  // 画布收起时宽度为 0（桌面收起态），此时按 0 宽度量出来的高度会失真，跳过
  if (el.offsetWidth === 0) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

/**
 * 收起/展开是宽度动画，动画途中量高度同样会失真。
 * 等宽度过渡真正结束、宽度落到最终值后再量一次。
 */
function onCanvasTransitionEnd(event: TransitionEvent) {
  if (event.propertyName === 'width') resizeDesc()
}

// 描述由外部变化（打开表、AI 改过）或画布可见性变化时，同步高度
watch([() => props.description, hidden], async () => {
  await nextTick()
  resizeDesc()
})

onMounted(() => resizeDesc())

/** 标题必填：清空视为放弃本次编辑，回退成原值 */
function commitTitle(event: Event) {
  const el = event.target as HTMLInputElement
  const next = el.value.trim()
  if (next === props.title) {
    el.value = props.title
    return
  }
  if (!next) {
    el.value = props.title
    return
  }
  emit('update-meta', { title: next })
}

/** 描述可以留空。立刻把输入归一化，免得页面里留下多余空白 */
function commitDescription(event: Event) {
  const el = event.target as HTMLTextAreaElement
  const next = el.value.trim()
  el.value = next
  resizeDesc()
  if (next === props.description) return
  emit('update-meta', { description: next })
}
</script>

<template>
  <!-- 手机端画布是右侧滑入的抽屉：左侧留出的空隙盖一层遮罩，点击即关闭。
       遮罩始终留在 DOM 里、用 opacity/visibility 切换，不用 v-if 移除 ——
       iOS Safari 上移除 position:fixed 元素偶尔会残留合成层（见样式注释）。 -->
  <div class="canvas-backdrop" :class="{ 'is-visible': canvasOpen }" @click="closeCanvas" />

  <!-- 收起后宽度为 0，inert 让内部按钮一并退出键盘焦点顺序 -->
  <section
    class="canvas"
    :class="{ collapsed: collapsed && !isMobile, 'mobile-open': canvasOpen }"
    :inert="hidden"
    @transitionend="onCanvasTransitionEnd"
  >
    <header class="canvas-head">
      <div class="canvas-head-left">
        <!-- 手机上画布是从右侧滑入的抽屉，关闭放最左：它是「退出这一层」，
             与右侧的撤销/重做/分享（都是对内容做动作）分开放，不易误点。
             刻意用 ✕ 而不是左箭头，避免被理解成浏览器后退。 -->
        <button
          v-if="isMobile"
          type="button"
          class="ghost-btn icon-only close-btn"
          aria-label="关闭参数表"
          @click="closeCanvas"
        >
          <i class="ri-close-line" />
        </button>

        <span v-if="saving" class="saving">
          <i class="ri-loader-4-line spin" />
          保存中
        </span>
        <span v-else class="revision">版本: r{{ revision }}</span>
      </div>

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

        <!-- 分享是核心能力：常驻工具栏，与撤销/重做同款图标按钮 -->
        <Tooltip v-if="!readonly">
          <TooltipTrigger as-child>
            <button
              type="button"
              class="ghost-btn icon-only"
              aria-label="分享"
              @click="emit('share')"
            >
              <i class="ri-share-forward-line" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">生成只读链接分享这张表</TooltipContent>
        </Tooltip>

        <!-- 桌面：收起画布成 0 宽，从对话头部按钮还原（见 WorkspaceView）。
             手机上画布是覆盖层，用不着收起态，关闭按钮已在头部最左 -->
        <Tooltip v-if="!isMobile">
          <TooltipTrigger as-child>
            <button
              type="button"
              class="ghost-btn icon-only"
              aria-label="收起画布"
              @click="toggleCanvas"
            >
              <i class="ri-layout-right-line" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">收起画布</TooltipContent>
        </Tooltip>
      </div>
    </header>

    <div class="canvas-body ds-scroll-y">
      <!-- 表名与描述都可以就地修改，和参数单元格一样直接改、失焦提交。
           两者都常驻 1px 透明边框，focus 只换颜色，盒子尺寸不变，聚焦时不跳动 -->
      <input
        class="canvas-title"
        :value="title"
        :readonly="readonly"
        placeholder="未命名参数表"
        aria-label="参数表名称"
        @change="commitTitle"
        @keydown.enter="($event.target as HTMLInputElement).blur()"
      />

      <textarea
        ref="descRef"
        class="canvas-desc"
        :value="description"
        :readonly="readonly"
        rows="1"
        placeholder="添加描述，说明这张表是做什么的"
        aria-label="参数表描述"
        @input="resizeDesc"
        @change="commitDescription"
      />

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
  /* 宽度自适应，不写死像素：按工作区宽度取一份比例，再夹在可读区间内。
     可伸可缩——窗口变宽时跟着变宽，变窄时收缩到还能放下表格的尺寸；
     对话区的 min-width 在这之前兜住下限（见 WorkspaceView 的 .chat-col）。 */
  flex: 0 1 auto;
  width: clamp(200px, 38%, 620px);
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--ds-surface);
  border-left: 1px solid var(--ds-border);
  transition: width 0.2s ease;
}

/* 画布被压窄时版本号先让位。操作按钮是 flex:none，必须保证撤销/重做/分享/收起
   始终可见可点——否则收起按钮被挤出可视区，用户就没法把画布收起来了。 */
.revision,
.saving {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

/* 窄画布时收紧头部间距，把空间让给按钮 */
@media (min-width: 769px) and (max-width: 1100px) {
  .canvas-head {
    padding: 0 10px;
    gap: 8px;
  }
}

/* 收起：宽度归零让位给对话区，边框一并去掉避免残留 1px 竖线 */
.canvas.collapsed {
  width: 0;
  border-left-width: 0;
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

/* 左侧一组：手机上是「关闭 + 版本号」，桌面只有版本号 */
.canvas-head-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
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
  color: var(--ds-icon);
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

/* 去掉文案后的图标按钮：与 chat-head 的 head-btn 保持同一尺寸 */
.ghost-btn.icon-only {
  width: 34px;
  height: 34px;
  padding: 0;
  justify-content: center;
  font-size: 18px;
}

/* 关闭按钮：它是「退出这一层」，不铺底色，避免看起来像撤销/分享那样的内容操作。
   连 hover 也不铺底色，只让图标颜色变深作为反馈。 */
.ghost-btn.close-btn,
.ghost-btn.close-btn:hover {
  background: transparent;
  color: var(--ds-text-secondary);
}

/* 承接被禁用按钮的指针事件（见模板注释） */
.tip-wrap {
  display: inline-flex;
  cursor: pointer;
}

.tip-wrap:has(> .ghost-btn:disabled) {
  cursor: not-allowed;
}

.canvas-body {
  flex: 1;
  min-height: 0;
  padding: 16px 18px 28px;
}

/* 表名：正文里的标题，可直接编辑。比描述略大、比对话区标题克制 */
.canvas-title {
  display: block;
  width: 100%;
  margin: 0 0 10px;
  padding: 3px 7px;
  /* 边框常驻（透明），focus 只换颜色：不改变盒模型，聚焦时不闪动 */
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--ds-text);
  font-family: inherit;
  font-size: var(--ds-font-base);
  font-weight: 600;
  line-height: 1.4;
  outline: none;
}

.canvas-title:hover:not([readonly]) {
  border-color: var(--ds-border-strong);
}

.canvas-title:focus {
  border-color: var(--ds-primary);
  background: var(--ds-surface);
}

.canvas-title::placeholder {
  color: var(--ds-text-tertiary);
  font-weight: 400;
}

/* 表简介：浅底块，与参数表内容区分开。同样就地可改。
   由 JS 按内容撑高，所以固定 resize:none、不设 height */
.canvas-desc {
  display: block;
  width: 100%;
  margin: 0 0 16px;
  padding: 9px 11px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-family: inherit;
  font-size: var(--ds-font-xs);
  line-height: 1.7;
  resize: none;
  /* 保留换行；长串（如没有空格的型号）也要在框内折行，不横向溢出 */
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  overflow-y: auto;
  /* 描述再长也不该把参数表顶到屏幕外，超出后内部滚动 */
  max-height: 220px;
  outline: none;
}

.canvas-desc:hover:not([readonly]) {
  border-color: var(--ds-border-strong);
}

.canvas-desc:focus {
  border-color: var(--ds-primary);
  background: var(--ds-surface);
  color: var(--ds-text);
}

.canvas-desc::placeholder {
  color: var(--ds-text-tertiary);
}

.canvas-empty {
  padding: 56px 0;
  text-align: center;
  color: var(--ds-text-tertiary);
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

/* 触屏没有 hover：行删除按钮改为常驻，否则它在手机上等于不存在 */
@media (hover: none) {
  .row-btn {
    opacity: 1;
  }
}

/* ==================== 手机端：画布变右侧抽屉 ==================== */

/* 左侧空隙的遮罩：点击关闭。用媒体查询控制显隐而不是模板里的 isMobile，
   避免 JS 判断与 CSS 断点出现时序差异时遮罩缺失。 */
.canvas-backdrop {
  display: none;
}

@media (max-width: 768px) {
  .canvas-backdrop {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 41;
    background: rgba(0, 0, 0, 0.4);
    opacity: 0;
    visibility: hidden;
    transition:
      opacity 0.22s ease,
      visibility 0.22s ease;
  }

  .canvas-backdrop.is-visible {
    opacity: 1;
    visibility: visible;
  }

  .canvas {
    /* 左侧留出的一截空隙，露出后面的对话：一眼看出这是「盖上来的一层」而不是
       导航到了新页面，用户就不会下意识去点浏览器后退。改这个值即可调整空隙宽度。 */
    --canvas-peek: 40px;

    position: fixed;
    top: 0;
    bottom: 0;
    right: 0;
    left: var(--canvas-peek);
    /* 关键：必须重置基类的宽度。基类是 clamp(200px, 38%, 620px)，而手机上 38%
       只有一百多像素，会被夹到 200px 下限 —— 画布于是又窄又不贴右缘，整层错位。
       置为 auto 后宽度由 left/right 自动推导 = 视口宽 − 空隙宽，正好贴满右侧。 */
    width: auto;
    /* 必须低于弹窗层（ui/dialog 等一律 z-50）：画布里的分享、删除确认都要弹窗，
       盖到它们上面就会出现「点了没反应」。层级契约见 AppSidebar 同段注释。 */
    z-index: 42;
    border-left: none;
    /* left 有偏移，且宽度 = 视口 − 偏移，所以 translateX(100%) 恰好把它推到屏幕外 */
    transform: translateX(100%);
    /* 关闭态一并置 visibility:hidden：只靠 translate 移出屏幕，iOS Safari 有时
       仍保留这层不重绘，关掉后右侧还留着一条半透明底板 */
    visibility: hidden;
    transition:
      transform 0.22s ease,
      visibility 0.22s ease;
  }

  .canvas.mobile-open {
    transform: translateX(0);
    visibility: visible;
  }

  /* 手机上画布铺满右侧，内边距也收一点 */
  .canvas-head {
    padding: 0 12px 0 16px;
  }

  .canvas-body {
    padding: 14px 14px 24px;
  }
}
</style>
