import { ref, watch } from 'vue'

const STORAGE_KEY = 'dspecs.theme'

type ThemeMode = 'light' | 'dark'

function initialMode(): ThemeMode {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'light' || saved === 'dark') return saved
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

// 模块级单例：主题是全局状态，没必要为此引入一个 store
const mode = ref<ThemeMode>(initialMode())

watch(
  mode,
  (value) => {
    localStorage.setItem(STORAGE_KEY, value)
    document.documentElement.dataset.theme = value
    // 同时维护 .dark 类，供 shadcn-vue / Tailwind 的 @custom-variant dark 使用
    document.documentElement.classList.toggle('dark', value === 'dark')
  },
  { immediate: true },
)

export function useThemeMode() {
  function toggle() {
    mode.value = mode.value === 'dark' ? 'light' : 'dark'
  }

  return { mode, toggle }
}
