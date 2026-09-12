import { ref, watch } from 'vue'

const STORAGE_KEY = 'dspecs.sidebar.collapsed'

function initialCollapsed(): boolean {
  return localStorage.getItem(STORAGE_KEY) === '1'
}

// 模块级单例：侧栏收起状态是全局的，跨路由切换时保持
const collapsed = ref(initialCollapsed())

watch(collapsed, (value) => {
  localStorage.setItem(STORAGE_KEY, value ? '1' : '0')
})

export function useSidebar() {
  function toggle() {
    collapsed.value = !collapsed.value
  }

  return { collapsed, toggle }
}
