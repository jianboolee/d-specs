import { ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'

/**
 * 手机端外壳：侧栏与画布在窄屏下都变成按需弹出的覆盖层。
 *
 * 刻意与 useSidebar / useCanvas 分开，也不落 localStorage：
 * 那两个是桌面端的布局偏好，是要长期记住的；手机端覆盖层的开合是随手的、
 * 临时的，默认都应关闭。混在一起会互相污染（手机上开一次抽屉，
 * 把用户的桌面布局也改掉了）。
 */
const sidebarOpen = ref(false)
const canvasOpen = ref(false)

export function useMobileShell() {
  // 侧栏与画布同为覆盖层，同时打开会叠在一起，所以互斥
  function openSidebar() {
    canvasOpen.value = false
    sidebarOpen.value = true
  }

  function closeSidebar() {
    sidebarOpen.value = false
  }

  function openCanvas() {
    sidebarOpen.value = false
    canvasOpen.value = true
  }

  function closeCanvas() {
    canvasOpen.value = false
  }

  function closeAll() {
    sidebarOpen.value = false
    canvasOpen.value = false
  }

  return { sidebarOpen, canvasOpen, openSidebar, closeSidebar, openCanvas, closeCanvas, closeAll }
}

/**
 * 是否处于手机布局。断点与应用外壳的媒体查询保持一致（`max-width: 768px`），
 * 两边不同步会出现「JS 认为是手机、CSS 认为是桌面」的错位。
 */
export function useIsMobile() {
  return useMediaQuery('(max-width: 768px)')
}
