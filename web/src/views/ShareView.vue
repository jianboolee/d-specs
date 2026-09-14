<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import * as shareApi from '@/api/share'
import { ApiError } from '@/api/http'
import type { PublicSpec } from '@/types/api'

/**
 * 分享只读页。
 *
 * 刻意不引入 AppSidebar / ChatPanel / 编辑器组件：这个页面通过独立路由懒加载，
 * 外部访客不该为编辑器与对话面板付出 bundle 代价。
 *
 * 公开地址用表 id（/s/:id）。页面本身是公开可收录的，SEO 元信息与正文由后端
 * 在 SSR 时就已经写进入口 HTML；这里只负责 Vue 接管后的交互渲染。
 */
const route = useRoute()

const tableId = computed(() => String(route.params.id ?? ''))

/**
 * 服务端 SSR 时已注入的数据（见 api/internal/modules/share/page.go）。
 * 用它作为初始状态：Vue 接管后无需再请求一次接口，也不会出现
 * 「完整内容 → 加载中 → 完整内容」的闪烁。
 */
const seed = readSeed()

const loading = ref(!seed)
const spec = ref<PublicSpec | null>(seed)
const errorText = ref('')

const totalItems = computed(() =>
  spec.value ? spec.value.groups.reduce((sum, g) => sum + g.items.length, 0) : 0,
)

const updatedAt = computed(() => {
  if (!spec.value?.updated_at) return ''
  const d = new Date(spec.value.updated_at)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
})

/**
 * 导出 xlsx 的公开地址。
 *
 * 公开接口不需要 Authorization 头，所以直接用 <a download> 即可，
 * 不必像应用内那样先 fetch 成 Blob。
 */
const exportUrl = computed(() => `/api/v1/public/specs/${tableId.value}/export`)

function printPage() {
  window.print()
}

onMounted(async () => {
  // 已有 SSR 数据就直接展示，不必再打一次接口；
  // 直接访问（无 SSR，例如前端 dev server）时才回退到接口拉取。
  if (spec.value) {
    document.title = spec.value.title || '参数配置表'
    return
  }

  try {
    spec.value = await shareApi.getPublicSpec(tableId.value)
    document.title = spec.value.title || '参数配置表'
  } catch (err) {
    errorText.value =
      err instanceof ApiError ? err.message : '加载失败，请确认链接是否正确'
  } finally {
    loading.value = false
  }
})

/** 读取服务端注入的初始数据；不存在或格式异常时返回 null */
function readSeed(): PublicSpec | null {
  const el = document.getElementById('__SHARE_SPEC__')
  if (!el?.textContent) return null
  try {
    return JSON.parse(el.textContent) as PublicSpec
  } catch {
    return null
  }
}
</script>

<template>
  <div class="share-page">
    <main class="share-main">
      <div v-if="loading" class="state">加载中…</div>

      <div v-else-if="errorText" class="state error">
        <i class="ri-link-unlink-muted" />
        <h1>无法打开这份参数表</h1>
        <p>{{ errorText }}</p>
        <p class="state-hint">可能是创建者已关闭分享，或者链接不完整。</p>
      </div>

      <template v-else-if="spec">
        <div class="spec-head">
          <h1>{{ spec.title }}</h1>
          <p v-if="spec.description" class="spec-desc">{{ spec.description }}</p>
          <div class="spec-meta">
            <span v-if="spec.industry" class="tag">{{ spec.industry }}</span>
            <span>{{ totalItems }} 个参数</span>
            <template v-if="updatedAt">
              <span class="dot">·</span>
              <span>更新于 {{ updatedAt }}</span>
            </template>
          </div>

          <!-- 导出工具栏：公开接口与分享页同一访问边界，无需鉴权，直接下载 -->
          <div class="share-toolbar">
            <a class="toolbar-btn" :href="exportUrl" download>
              <i class="ri-file-excel-2-line" />
              Excel
            </a>
            <button type="button" class="toolbar-btn" @click="printPage">
              <i class="ri-file-pdf-2-line" />
              PDF
            </button>
          </div>
        </div>

        <section v-for="(group, gi) in spec.groups" :key="group.name || gi" class="group">
          <h2>{{ group.name || '参数' }}</h2>
          <dl class="spec-list">
            <div v-for="(item, ii) in group.items" :key="`${item.name}-${ii}`" class="spec-row">
              <dt>{{ item.name }}</dt>
              <dd>
                <span class="spec-value" :class="{ empty: !item.value }">
                  {{ item.value || '待补充' }}
                </span>
              </dd>
            </div>
          </dl>
        </section>
      </template>
    </main>

    <footer class="share-foot">由小野驴科技提供技术支持</footer>
  </div>
</template>
