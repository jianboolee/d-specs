<script setup lang="ts">
import TableSearchPanel from './TableSearchPanel.vue'
import type { TableSummary } from '@/types/api'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'

/**
 * 居中弹出的搜索面板外壳。
 *
 * 只是把可复用的 TableSearchPanel 包进 Dialog；将来做独立搜索页时直接用 Panel，
 * 不需要这个组件。
 */
const open = defineModel<boolean>('open', { default: false })

defineEmits<{
  select: [table: TableSummary]
}>()

function close() {
  open.value = false
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="p-0 gap-0 overflow-hidden sm:max-w-[560px]"
      :show-close-button="false"
      @open-auto-focus.prevent
    >
      <!-- Dialog 要求有标题，这里视觉上隐藏，交给输入框的 placeholder 引导 -->
      <DialogTitle class="sr-only">搜索参数表</DialogTitle>
      <TableSearchPanel
        @select="$emit('select', $event)"
        @close="close"
      />
    </DialogContent>
  </Dialog>
</template>
