import { ref, watch } from 'vue'

const STORAGE_KEY = 'dspecs.canvas.collapsed'

function initialCollapsed(): boolean {
  return localStorage.getItem(STORAGE_KEY) === '1'
}

// 模块级单例：画布收起状态跨路由切换时保持
const collapsed = ref(initialCollapsed())

watch(collapsed, (value) => {
  localStorage.setItem(STORAGE_KEY, value ? '1' : '0')
})

export function useCanvas() {
  function toggle() {
    collapsed.value = !collapsed.value
  }

  return { collapsed, toggle }
}
