import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { pinia } from '@/stores'
import { useMobileShell } from '@/composables/useMobileShell'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      // 根地址是项目介绍页：公开、可被收录，不依赖登录态
      path: '/',
      name: 'landing',
      component: () => import('@/views/LandingView.vue'),
      meta: { public: true },
    },
    {
      // 工作台＝登录后的入口页（原首页内容搬到这里）
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/tables',
      name: 'tables',
      component: () => import('@/views/TableListView.vue'),
    },
    {
      path: '/tables/:id',
      name: 'workspace',
      component: () => import('@/views/WorkspaceView.vue'),
    },
    {
      // 打印页：独立页面渲染整张表并自动唤起打印，供「导出 PDF」用。
      // 需要登录（未登录会跳登录），打开方式是新标签页。
      path: '/tables/:id/print',
      name: 'print',
      component: () => import('@/views/PrintView.vue'),
    },
    {
      // 导出页：独立页面触发 xlsx 下载，供「导出 Excel」与 AI 回复里的链接用。
      // 单独成页才能被链接点击（下载接口要 Authorization 头，直接链接口会 401）。
      path: '/tables/:id/export',
      name: 'export',
      component: () => import('@/views/ExportView.vue'),
    },
    {
      // 分享页：独立路由，走独立懒加载 chunk，不加载编辑器与对话面板。
      // 公开地址用表 id，便于被搜索引擎收录与人工识别。
      path: '/s/:id',
      name: 'share',
      component: () => import('@/views/ShareView.vue'),
      meta: { public: true, layout: 'standalone' },
    },
    {
      // 发现页：罗列所有已发布（分享开启中）的参数表，无需登录
      path: '/discover',
      name: 'discover',
      component: () => import('@/views/DiscoverView.vue'),
      meta: { public: true },
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore(pinia)

  if (!auth.ready) {
    await auth.restore()
  }

  if (to.meta.public) {
    return true
  }

  if (!auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  return true
})

// 抽屉/画布覆盖层是临时状态，哪怕导航完成也要关掉：用返回键或链接离开时，
// 状态若留着，遮罩会在新页面上重新渲染成一层甩不掉的蒙层。
router.afterEach(() => {
  useMobileShell().closeAll()
})

export default router
