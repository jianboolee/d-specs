<script setup lang="ts">
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

/**
 * 导出参数表。
 *
 * 两个格式都走独立页面（新标签页打开），而不是在当前页生成：
 * - xlsx：导出接口需要 Authorization 头，单独成页后链接才能被点击，
 *   也才能被 AI 在回复里直接给出。
 * - PDF：打印页独立渲染整张表，不必反过来把应用外壳藏掉。
 */
const props = defineProps<{
  tableId: string
  tableTitle: string
}>()

const open = defineModel<boolean>('open', { default: false })

function openPage(path: string) {
  window.open(`/tables/${props.tableId}${path}`, '_blank')
  open.value = false
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-[420px]">
      <DialogHeader>
        <DialogTitle>导出参数表</DialogTitle>
        <DialogDescription>导出「{{ tableTitle }}」的全部参数，按分组排列。</DialogDescription>
      </DialogHeader>

      <div class="export-options">
        <button type="button" class="export-option" @click="openPage('/export')">
          <i class="ri-file-excel-2-line" />
          <span class="option-main">
            <strong>Excel（.xlsx）</strong>
            <span>在新标签页打开并下载，可在 Excel / WPS 中继续编辑</span>
          </span>
        </button>

        <button type="button" class="export-option" @click="openPage('/print')">
          <i class="ri-file-pdf-2-line" />
          <span class="option-main">
            <strong>PDF</strong>
            <span>在新标签页打开打印预览，选择“另存为 PDF”即可保存</span>
          </span>
        </button>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">取消</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.export-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.export-option {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--ds-border);
  border-radius: 10px;
  background: var(--ds-surface);
  color: var(--ds-text);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease;
}

.export-option:hover {
  border-color: var(--ds-primary);
}

.export-option > i {
  flex: none;
  color: var(--ds-primary);
  font-size: 20px;
}

.option-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.option-main strong {
  font-size: var(--ds-font-sm);
  font-weight: 600;
}

.option-main span {
  font-size: var(--ds-font-xs);
  line-height: 1.6;
  color: var(--ds-text-tertiary);
}
</style>
