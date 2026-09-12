import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// 后端地址。本机跑 make api-dev 时用默认值；在 docker-dev 里后端是另一个容器，
// compose 会通过 DS_API_TARGET=http://api:8401 覆盖。
const apiTarget = process.env.DS_API_TARGET || 'http://127.0.0.1:8401'

// 有些宿主机（尤其 Docker Desktop 的绑定挂载）收不到文件变更事件，
// 导致 HMR 不触发。设置 DS_VITE_POLL=1 可退回轮询。
const usePolling = process.env.DS_VITE_POLL === '1'

// web dev 端口。默认 8402，刻意避开 Vite 默认的 5173，防止与本机其它项目冲突。
const webPort = Number(process.env.WEB_PORT || 8402)

export default defineConfig({
  base: '/',
  server: {
    host: '0.0.0.0',
    port: webPort,
    // 端口被占用时直接报错，而不是静默换到下一个端口：
    // 后者会让 vite 代理（指向固定的后端）与浏览器访问的地址对不上，难排查。
    strictPort: true,
    open: false,
    watch: { usePolling },
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
      },
      // 分享页与 SEO 入口是后端渲染的（见 api/internal/modules/share/page.go）。
      // 不代理的话 Vite 会把 /s/<id> 当成 SPA 路由，直接返回还带着注入锚点的
      // 原始 index.html，本地就看不到 meta / 正文注入的效果。
      //
      // 刻意不设 changeOrigin：保留 Host=localhost:<WEB_PORT>，页面里的 canonical
      // 就指向本地地址，便于核对；生产走 SITE_URL，不受影响。
      '/s/': { target: apiTarget },
      '/sitemap.xml': { target: apiTarget },
      '/robots.txt': { target: apiTarget },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: true,
    rollupOptions: {
      output: {
        // 分享页保持独立 chunk，避免外部访客加载编辑器与对话面板的代码
        manualChunks: {
          vendor: ['vue', 'vue-router', 'pinia'],
        },
      },
    },
  },
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
