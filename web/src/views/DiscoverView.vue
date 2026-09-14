<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import * as shareApi from '@/api/share'
import { ApiError } from '@/api/http'
import { useAuthStore } from '@/stores/auth'
import type { PublicTableSummary } from '@/types/api'
import { formatRelativeTime } from '@/utils/time'

/**
 * 公开发现页。
 *
 * 罗列所有已发布（分享开启中）的参数表，无需登录。沿用分享页的外壳样式
 * （.share-page / .share-main / .share-foot，见 assets/share.css），
 * 页面专属样式收在下面 scoped 的 .discover-* 里。
 *
 * 纯前端 SPA：列表由接口渲染，不做 SSR —— 这里面向的是「浏览与发现」，
 * 收录已由每张表的 /s/:id 分享页承担。
 */
const router = useRouter()
// 工作台需要登录（路由守卫会把未登录访客弹到 /login），所以公开页上的入口
// 按登录状态区分：登录了进工作台，没登录给登录入口，别把人骗到登录页。
const auth = useAuthStore()

/** 与 TableSearchPanel 一致：输入防抖，避免每敲一个字打一次请求 */
const DEBOUNCE_MS = 250

const keyword = ref('')
const tables = ref<PublicTableSummary[]>([])
const loading = ref(true)
const errorText = ref('')

const isEmpty = computed(() => !loading.value && !errorText.value && tables.value.length === 0)
const emptyText = computed(() =>
  keyword.value.trim() ? '没有匹配的参数表' : '还没有公开的参数表',
)

// 只认最后一次请求：防抖 + 快速输入时可能先后返回
let seq = 0
let timer: number | undefined

async function load() {
  const current = ++seq
  loading.value = true
  errorText.value = ''
  try {
    const res = await shareApi.listPublicTables(keyword.value)
    if (current !== seq) return
    tables.value = res.items
  } catch (err) {
    if (current !== seq) return
    tables.value = []
    errorText.value = err instanceof ApiError ? err.message : '加载失败，请稍后重试'
  } finally {
    if (current === seq) loading.value = false
  }
}

watch(keyword, () => {
  if (timer) window.clearTimeout(timer)
  timer = window.setTimeout(load, DEBOUNCE_MS)
})

onMounted(() => {
  document.title = '发现 - 参数配置表'
  void load()
})

onUnmounted(() => {
  if (timer) window.clearTimeout(timer)
})
</script>

<template>
  <div class="share-page">
    <main class="share-main">
      <div class="discover-head">
        <div class="discover-title">
          <h1>发现</h1>
          <p>浏览所有人已发布的参数表</p>
        </div>
        <button
          v-if="auth.isLoggedIn"
          type="button"
          class="home-link"
          @click="router.push({ name: 'dashboard' })"
        >
          <i class="ri-dashboard-line" />
          进入工作台
        </button>
        <RouterLink v-else class="home-link" :to="{ name: 'login' }">
          <i class="ri-login-box-line" />
          登录
        </RouterLink>
      </div>

      <div class="discover-search">
        <i class="ri-search-line" />
        <input
          v-model="keyword"
          class="discover-input"
          type="text"
          placeholder="搜索已发布的参数表"
          autocomplete="off"
        />
        <i v-if="loading" class="ri-loader-4-line spin" />
      </div>

      <div v-if="errorText" class="discover-state error">
        <i class="ri-error-warning-line" />
        <p>{{ errorText }}</p>
        <button type="button" class="retry-btn" @click="load">重试</button>
      </div>

      <div v-else-if="isEmpty" class="discover-state">
        <i class="ri-inbox-line" />
        <p>{{ emptyText }}</p>
      </div>

      <div v-else class="discover-grid">
        <a
          v-for="table in tables"
          :key="table.id"
          class="discover-card"
          :href="`/s/${table.id}`"
          target="_blank"
          rel="noopener"
        >
          <h3 class="ds-ellipsis">{{ table.title }}</h3>
          <!-- 描述是非必要字段：没有就不占位、不显示「暂无描述」 -->
          <p v-if="table.description" class="card-desc">{{ table.description }}</p>

          <div v-if="table.industry" class="card-tags">
            <span class="tag">{{ table.industry }}</span>
          </div>

          <div class="card-foot">
            <span>{{ table.item_count }} 个参数</span>
            <span class="time">{{ formatRelativeTime(table.updated_at) }}</span>
          </div>
        </a>
      </div>
    </main>

    <footer class="share-foot">由小野驴科技提供技术支持</footer>
  </div>
</template>

<style scoped>
.discover-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.discover-title h1 {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 600;
  color: var(--ds-text);
}

.discover-title p {
  margin: 0;
  font-size: var(--ds-font-sm);
  color: var(--ds-text-secondary);
}

.home-link {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 12px;
  border-radius: 8px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  text-decoration: none;
  cursor: pointer;
}

.home-link:hover {
  background: var(--ds-border);
  color: var(--ds-text);
}

.discover-search {
  position: relative;
  display: flex;
  align-items: center;
  margin-bottom: 20px;
  padding: 0 14px;
  border: 1px solid var(--ds-border);
  border-radius: 10px;
  background: var(--ds-surface);
}

.discover-search > i:first-child {
  position: absolute;
  left: 14px;
  color: var(--ds-text-tertiary);
  font-size: 18px;
  pointer-events: none;
}

.discover-search > i.spin {
  margin-left: 10px;
  color: var(--ds-text-tertiary);
  font-size: 16px;
}

.discover-input {
  flex: 1;
  min-width: 0;
  height: 46px;
  padding-left: 28px;
  border: none;
  outline: none;
  background: transparent;
  color: var(--ds-text);
  font-family: inherit;
  font-size: var(--ds-font-md);
}

.discover-input::placeholder {
  color: var(--ds-text-tertiary);
}

.discover-state {
  padding: 60px 0;
  text-align: center;
  color: var(--ds-text-tertiary);
}

.discover-state i {
  font-size: 32px;
}

.discover-state p {
  margin: 12px 0 0;
  font-size: var(--ds-font-sm);
}

.discover-state.error p {
  margin-bottom: 16px;
}

.retry-btn {
  padding: 7px 16px;
  border-radius: 8px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  cursor: pointer;
}

.retry-btn:hover {
  background: var(--ds-border);
  color: var(--ds-text);
}

.discover-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 14px;
}

.discover-card {
  display: flex;
  flex-direction: column;
  padding: 15px 16px;
  border: 1px solid var(--ds-border);
  border-radius: var(--ds-radius-lg);
  background: var(--ds-surface);
  text-decoration: none;
  cursor: pointer;
  transition: all 0.16s ease;
}

.discover-card:hover {
  border-color: var(--ds-primary);
  box-shadow: 0 6px 20px rgba(18, 22, 34, 0.07);
}

.discover-card h3 {
  margin: 0;
  font-size: var(--ds-font-md);
  font-weight: 600;
  color: var(--ds-text);
}

.card-desc {
  margin: 6px 0 0;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 12px 0 0;
}

.tag {
  padding: 1px 7px;
  border-radius: 20px;
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
  font-size: var(--ds-font-xs);
}

.card-foot {
  /* 没有描述/行业时也要有最小间距，所以间距放在这里而不是上面的 margin 上 */
  margin-top: auto;
  padding-top: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
  font-variant-numeric: tabular-nums;
}

.time {
  margin-left: auto;
}

.spin {
  animation: spin 1.1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
