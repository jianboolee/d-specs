import { createApp } from 'vue'

import App from './App.vue'
import router from './router'
import { pinia } from './stores'

import '@/assets/main.css'
import '@/assets/tailwind.css'
// 分享页样式做成全局：服务端注入的静态 HTML 没有 scoped 的 data-v 属性，
// 必须由 <head> 里同步加载的样式表覆盖，首屏才不会无样式闪烁
import '@/assets/share.css'
import 'remixicon/fonts/remixicon.css'

const app = createApp(App)

app.use(pinia)
app.use(router)

app.mount('#app')
