import { createApp } from 'vue'

import App from './App.vue'
import router from './router'
import { pinia } from './stores'
import { setUnauthorizedHandler } from './api/http'
import { useAuthStore } from './stores/auth'

import '@/assets/main.css'
import '@/assets/tailwind.css'
// 分享页样式做成全局：服务端注入的静态 HTML 没有 scoped 的 data-v 属性，
// 必须由 <head> 里同步加载的样式表覆盖，首屏才不会无样式闪烁
import '@/assets/share.css'
import 'remixicon/fonts/remixicon.css'

const app = createApp(App)

app.use(pinia)
app.use(router)

// 续期失败后的 401 = 会话失效（refresh 过期/登出/吊销）：清空登录态并回登录页。
// 路由守卫只在导航时兜底；这里处理「已停在页面上、后台请求突然 401」的情况。
setUnauthorizedHandler(() => {
  const auth = useAuthStore(pinia)
  auth.user = null
  const current = router.currentRoute.value
  if (current.name !== 'login' && !current.meta.public) {
    void router.replace({ name: 'login' })
  }
})

app.mount('#app')
