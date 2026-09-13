import { ref } from 'vue'

// 模块级单例：新建参数表的弹窗由 App 全局挂载一份，任意页面都能唤起。
// 用共享状态而不是路由 query，是为了点「新建参数表」时留在当前页、只弹窗，
// 不先跳到列表页再打开。
const visible = ref(false)

export function useCreateTable() {
  function open() {
    visible.value = true
  }

  function close() {
    visible.value = false
  }

  return { visible, open, close }
}
