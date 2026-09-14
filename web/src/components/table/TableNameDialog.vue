<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

/**
 * 输入新名字的弹窗：重命名与创建副本共用。
 *
 * 两者都只是「换一个标题」，区别只在副本会以新名字另建一份。
 * 纯展示组件，不发请求：确认后把名字抛给调用方，由它决定调哪个接口。
 */
const props = withDefaults(
  defineProps<{
    open: boolean
    mode: 'rename' | 'duplicate'
    /** 打开时的初始值：重命名为原名，副本为「原名 副本」 */
    initialValue?: string
    submitting?: boolean
  }>(),
  { initialValue: '', submitting: false },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  submit: [value: string]
}>()

const isDuplicate = computed(() => props.mode === 'duplicate')
const title = computed(() => (isDuplicate.value ? '复制参数表' : '重命名参数表'))

const value = ref('')

/**
 * 只在弹窗打开时重置输入值。
 *
 * 不能 watch(initialValue)：弹窗关闭后调用方往往会把目标清空，初始值随之变化，
 * 那样会把用户正在输入的内容冲掉。
 */
watch(
  () => props.open,
  (visible) => {
    if (visible) value.value = props.initialValue
  },
)

const canSubmit = computed(() => value.value.trim() !== '' && !props.submitting)

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
    <DialogContent class="sm:max-w-[400px]">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription v-if="isDuplicate">
          将以新名字创建一份副本，包含当前全部参数。
        </DialogDescription>
      </DialogHeader>

      <Input
        v-model="value"
        maxlength="120"
        placeholder="参数表名称"
        autofocus
        @keydown.enter.prevent="submit"
      />

      <DialogFooter>
        <Button variant="outline" :disabled="submitting" @click="close">取消</Button>
        <Button :disabled="!canSubmit" @click="submit">
          <i v-if="submitting" class="ri-loader-4-line animate-spin" />
          {{ isDuplicate ? '创建副本' : '保存' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
