<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { ApiError } from '@/api/http'
import { useSpecTableStore } from '@/stores/specTable'
import { useCreateTable } from '@/composables/useCreateTable'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

/**
 * 全局的「新建参数表」弹窗。
 *
 * 挂在 App 上只此一份：侧栏、首页、列表页的入口都唤起它，避免各自维护一份
 * 弹窗、也避免靠路由参数把用户先送到列表页。
 */
const router = useRouter()
const spec = useSpecTableStore()
const { visible, close } = useCreateTable()

const creating = ref(false)
const form = reactive({ title: '' })
const createError = ref('')

// 每次打开都重置表单，避免上次的输入或报错残留
watch(visible, (open) => {
  if (open) {
    form.title = ''
    createError.value = ''
  }
})

async function submit() {
  const title = form.title.trim()
  if (!title) {
    createError.value = '请输入产品名称'
    return
  }
  if (title.length > 120) {
    createError.value = '产品名称不能超过 120 字'
    return
  }

  creating.value = true
  try {
    const res = await spec.createTable({ title })
    close()
    toast.success('参数表已创建')
    await router.push({ name: 'workspace', params: { id: res.table.id } })
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '创建失败')
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="visible">
    <DialogContent class="sm:max-w-[440px]">
      <DialogHeader>
        <DialogTitle>新建参数表</DialogTitle>
      </DialogHeader>

      <div class="modal-hint">一张表对应一个产品，创建后可以在左侧对话里让 AI 帮你填充参数。</div>

      <form class="modal-form" @submit.prevent="submit">
        <div class="field">
          <Label for="create-title">产品名称</Label>
          <Input
            id="create-title"
            v-model="form.title"
            placeholder="例如：某型号越野房车"
            maxlength="120"
            autofocus
            :aria-invalid="!!createError"
            @input="createError = ''"
          />
          <p v-if="createError" class="field-error">{{ createError }}</p>
        </div>
      </form>

      <DialogFooter>
        <Button variant="outline" :disabled="creating" @click="close">取消</Button>
        <Button :disabled="creating" @click="submit">
          <i v-if="creating" class="ri-loader-4-line animate-spin" />
          创建
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.modal-hint {
  margin: 0 0 16px;
  font-size: var(--ds-font-xs);
  line-height: 1.7;
  color: var(--ds-text-tertiary);
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.modal-form .field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.modal-form .field label {
  font-size: var(--ds-font-sm);
  color: var(--ds-text-secondary);
}

/* 弹窗内输入框比组件默认（h-8）更高 */
.modal-form :deep(input) {
  height: 46px;
  padding-inline: 14px;
  font-size: var(--ds-font-base);
}

.field-error {
  margin: 0;
  font-size: var(--ds-font-xs);
  color: #d64545;
}
</style>
