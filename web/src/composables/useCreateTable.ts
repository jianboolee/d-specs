import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { ApiError } from '@/api/http'
import { useSpecTableStore } from '@/stores/specTable'

// 新建是「一键完成」的动作：不再弹窗要求先填标题，直接建一张默认名的表
// 并进入工作区，用户在那里可以随时重命名。
// 共享 creating 是为了所有入口（侧栏 / 工作台 / 列表页）在同一时刻只有一次请求，
// 连点也不会建出多张表。
const creating = ref(false)

export function useCreateTable() {
  const router = useRouter()
  const spec = useSpecTableStore()

  async function create() {
    if (creating.value) return
    creating.value = true
    try {
      const res = await spec.createTable({})
      await router.push({ name: 'workspace', params: { id: res.table.id } })
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : '创建失败，请稍后重试')
    } finally {
      creating.value = false
    }
  }

  return { creating, create }
}
