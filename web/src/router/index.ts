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
      // 本地登录页：不自动跳 SSO，用户点「登录」才整页跳转到 d-auth。
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { public: true, guestOnly: true },
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

  // 分享页（standalone）是给外部访客与爬虫看的，不做登录探测，避免拖慢首屏
  const skipProbe = to.meta.layout === 'standalone'

  if (!skipProbe && auth.loading) {
    await auth.probe()
  }

  // 已登录访问登录页 → 回工作台
  if (to.meta.guestOnly && auth.isLoggedIn) {
    return { name: 'dashboard' }
  }

  // 公开页放行（介绍页/发现页会据此显示「进入工作台」，故仍需上面的探测）
  if (to.meta.public) {
    return true
  }

  // 未登录访问需登录的页面 → 停在本地登录页（不自动跳 SSO，由用户点「登录」触发）。
  // 带上 redirect 记住原本想去的路径，登录回跳后直达。
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
