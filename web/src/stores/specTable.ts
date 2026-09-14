import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as api from '@/api/tables'
import { ApiError } from '@/api/http'
import type { Op, SpecItem, TableDetail, TableSummary } from '@/types/api'

/** 高亮持续时间，与 main.css 里的 ds-flash 动画时长保持一致 */
const FLASH_MS = 2000

export interface ItemGroup {
  name: string
  items: SpecItem[]
}

export const useSpecTableStore = defineStore('specTable', () => {
  // ---------- 列表 ----------
  const tables = ref<TableSummary[]>([])
  const listLoading = ref(false)

  // ---------- 当前表 ----------
  const table = ref<TableDetail | null>(null)
  const items = ref<SpecItem[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const highlightIds = ref<string[]>([])

  /**
   * 撤销 / 重做栈，存的是「要恢复到的版本号」。
   *
   * 服务端的 restore 是「把目标版本写成新版本」，所以这里只需要记住
   * 用户走过哪些版本；每张表的历史是线性且不可变的。
   */
  const undoStack = ref<number[]>([])
  const redoStack = ref<number[]>([])

  const canUndo = computed(() => undoStack.value.length > 0)
  const canRedo = computed(() => redoStack.value.length > 0)
  const isEmpty = computed(() => items.value.length === 0)

  /** 按 group_name 分组，保持参数项原有顺序 */
  const groups = computed<ItemGroup[]>(() => {
    const map = new Map<string, SpecItem[]>()
    for (const item of items.value) {
      const key = item.group_name || ''
      const list = map.get(key)
      if (list) list.push(item)
      else map.set(key, [item])
    }
    return [...map.entries()].map(([name, list]) => ({ name, items: list }))
  })

  // ---------- 列表操作 ----------

  async function loadTables() {
    listLoading.value = true
    try {
      const res = await api.listTables({ size: 100 })
      tables.value = res.items
    } finally {
      listLoading.value = false
    }
  }

  async function createTable(input: { title: string; description?: string; industry?: string }) {
    const res = await api.createTable(input)
    await loadTables()
    return res
  }

  async function removeTable(id: string) {
    await api.deleteTable(id)
    if (table.value?.id === id) {
      table.value = null
      items.value = []
    }
    await loadTables()
  }

  async function duplicateTable(id: string, title: string) {
    const res = await api.duplicateTable(id, title)
    await loadTables()
    return res
  }

  async function renameTable(id: string, title: string) {
    const updated = await api.updateTable(id, { title })
    if (table.value?.id === id) {
      table.value = { ...table.value, ...updated }
    }
    await loadTables()
    return updated
  }

  /** 置顶 / 取消置顶。走元信息更新，不产生 revision。 */
  async function setPinned(id: string, pinned: boolean) {
    const updated = await api.updateTable(id, { pinned })
    if (table.value?.id === id) {
      table.value = { ...table.value, ...updated }
    }
    // 排序由服务端按 pinned 优先，重新拉一次列表让顺序生效
    await loadTables()
    return updated
  }

  /**
   * 通用元信息更新（标题/描述/行业）。
   *
   * 与 renameTable 一样：返回的详情合并进当前表，再刷新列表让卡片同步。
   * 元信息更新不产生 revision，因此不进撤销栈。
   */
  async function updateMeta(id: string, patch: api.UpdateTableInput) {
    const updated = await api.updateTable(id, patch)
    if (table.value?.id === id) {
      table.value = { ...table.value, ...updated }
    }
    await loadTables()
    return updated
  }

  // ---------- 当前表 ----------

  async function openTable(id: string) {
    loading.value = true
    try {
      const res = await api.getTable(id)
      table.value = res.table
      items.value = res.items

      // 刚打开时没有本地历史，但可以让「撤销」立刻可用：
      // 当前是 rN，就默认允许退回到 rN-1
      undoStack.value = res.table.current_revision > 0 ? [res.table.current_revision - 1] : []
      redoStack.value = []
      highlightIds.value = []
    } finally {
      loading.value = false
    }
  }

  async function refreshItems() {
    if (!table.value) return
    const res = await api.getTable(table.value.id)
    table.value = res.table
    items.value = res.items
  }

  // ---------- 变更 ----------

  /**
   * 应用一组操作。
   *
   * 变更后重新拉取参数项而不是本地打补丁：值的归一化（如 5000mAh 拆成
   * 5000 + mAh）只有服务端说了算，本地推算容易与真实状态不一致。
   */
  async function applyOps(ops: Op[], summary = '') {
    if (!table.value) throw new ApiError('没有打开的参数表', -1, 0)

    const base = table.value.current_revision
    saving.value = true
    try {
      const res = await api.applyOps(table.value.id, {
        baseRevision: base,
        ops,
        summary,
      })

      undoStack.value.push(base)
      redoStack.value = []

      table.value = res.table
      await refreshItems()
      flash(res.changes.map((c) => c.item_id))
      return res
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        // 版本冲突：把最新状态拉回来，让用户看到真实数据再重试
        await refreshItems().catch(() => undefined)
      }
      throw err
    } finally {
      saving.value = false
    }
  }

  async function undo() {
    if (!table.value || undoStack.value.length === 0) return null

    const target = undoStack.value.pop() as number
    const leaving = table.value.current_revision

    saving.value = true
    try {
      const res = await api.restoreRevision(table.value.id, target)
      redoStack.value.push(leaving)
      table.value = res.table
      await refreshItems()
      return res
    } catch (err) {
      undoStack.value.push(target)
      throw err
    } finally {
      saving.value = false
    }
  }

  async function redo() {
    if (!table.value || redoStack.value.length === 0) return null

    const target = redoStack.value.pop() as number
    const leaving = table.value.current_revision

    saving.value = true
    try {
      const res = await api.restoreRevision(table.value.id, target)
      undoStack.value.push(leaving)
      table.value = res.table
      await refreshItems()
      return res
    } catch (err) {
      redoStack.value.push(target)
      throw err
    } finally {
      saving.value = false
    }
  }

  // ---------- 高亮 ----------

  let flashTimer: number | undefined

  function flash(ids: string[]) {
    if (ids.length === 0) return
    highlightIds.value = ids
    if (flashTimer) window.clearTimeout(flashTimer)
    flashTimer = window.setTimeout(() => {
      highlightIds.value = []
    }, FLASH_MS)
  }

  /**
   * 记录一次「不由本 store 发起」的变更（目前只有 AI 对话）。
   *
   * 变更本身已经落库了，这里只补上撤销栈 —— 否则用户在 AI 改完之后
   * 会发现画布上的「撤销」按钮对这次改动无效。
   */
  function noteExternalChange(previousRevision: number) {
    undoStack.value.push(previousRevision)
    redoStack.value = []
  }

  // ---------- 便捷操作 ----------

  /** 改单个参数项的字段 */
  function updateItem(itemId: string, patch: Op['patch'], summary = '修改参数') {
    return applyOps([{ op: 'update_item', item_id: itemId, patch }], summary)
  }

  /** 新增参数项 */
  function addItem(draft: NonNullable<Op['item']>) {
    return applyOps([{ op: 'add_item', item: draft }], `新增参数「${draft.name}」`)
  }

  /** 删除参数项 */
  function removeItem(itemId: string, name: string) {
    return applyOps([{ op: 'delete_item', item_id: itemId }], `删除参数「${name}」`)
  }

  function reset() {
    table.value = null
    items.value = []
    undoStack.value = []
    redoStack.value = []
    highlightIds.value = []
  }

  return {
    // 列表
    tables,
    listLoading,
    loadTables,
    createTable,
    removeTable,
    duplicateTable,
    renameTable,
    setPinned,
    updateMeta,

    // 当前表
    table,
    items,
    groups,
    loading,
    saving,
    isEmpty,
    openTable,
    refreshItems,
    reset,

    // 变更
    applyOps,
    updateItem,
    addItem,
    removeItem,
    undo,
    redo,
    canUndo,
    canRedo,

    // 高亮
    highlightIds,
    flash,
    noteExternalChange,
  }
})
