import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { pinia } from '@/stores'

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

export default router
