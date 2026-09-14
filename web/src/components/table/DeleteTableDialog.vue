<script setup lang="ts">
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

/**
 * 删除参数表的确认弹窗。
 *
 * 纯展示组件：确认后抛事件，由调用方决定删完之后去哪。
 */
withDefaults(
  defineProps<{
    open: boolean
    /** 待删除的表名，用于确认文案 */
    title?: string
    submitting?: boolean
  }>(),
  { title: '', submitting: false },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  confirm: []
}>()
</script>

<template>
  <AlertDialog :open="open" @update:open="(v) => emit('update:open', v)">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>删除参数表</AlertDialogTitle>
        <AlertDialogDescription>
          确定删除「{{ title }}」？删除后该表不再出现在列表里。
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="emit('update:open', false)">取消</AlertDialogCancel>
        <AlertDialogAction variant="destructive" :disabled="submitting" @click="emit('confirm')">
          删除
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
