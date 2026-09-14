<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

/**
 * 编辑表描述。
 *
 * 描述是非必要字段：可以留空，留空即清除。纯展示组件，不发请求。
 */
const props = withDefaults(
  defineProps<{
    open: boolean
    initialValue?: string
    submitting?: boolean
  }>(),
  { initialValue: '', submitting: false },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  submit: [value: string]
}>()

const value = ref('')

/** 与后端 max=2000 对齐，前端先拦住，避免白跑一次请求 */
const MAX_LEN = 2000

// 只在打开时灌入初始值：关闭后调用方常把目标清空，会在输入中途冲掉内容
watch(
  () => props.open,
  (visible) => {
    if (visible) value.value = props.initialValue
  },
)

const canSubmit = computed(() => !props.submitting)

function close() {
  if (props.submitting) return
  emit('update:open', false)
}

function submit() {
  if (!canSubmit.value) return
  emit('submit', value.value.trim())
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => !v && close()">
    <DialogContent class="sm:max-w-[480px]">
      <DialogHeader>
        <DialogTitle>编辑描述</DialogTitle>
        <DialogDescription>一句话说明这张表是做什么的，会显示在卡片和公开分享页上。留空即不显示。</DialogDescription>
      </DialogHeader>

      <Textarea
        v-model="value"
        :maxlength="MAX_LEN"
        placeholder="例如：2026 款房车的配置参数，含电力、水路、底盘三部分"
        class="min-h-[110px]"
        autofocus
      />

      <DialogFooter>
        <Button variant="outline" :disabled="submitting" @click="close">取消</Button>
        <Button :disabled="!canSubmit" @click="submit">
          <i v-if="submitting" class="ri-loader-4-line animate-spin" />
          保存
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
