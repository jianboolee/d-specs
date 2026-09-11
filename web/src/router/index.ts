import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { pinia } from '@/stores'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/tables' },
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
      // 分享页：独立路由，走独立懒加载 chunk，不加载编辑器与对话面板
      path: '/s/:token',
      name: 'share',
      component: () => import('@/views/ShareView.vue'),
      meta: { public: true, layout: 'standalone' },
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
