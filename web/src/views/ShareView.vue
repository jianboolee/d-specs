<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import * as shareApi from '@/api/share'
import { ApiError } from '@/api/http'
import type { PublicSpec } from '@/types/api'

/**
 * 分享只读页。
 *
 * 刻意不引入 AppSidebar / ChatPanel / 编辑器组件：这个页面通过独立路由懒加载，
 * 外部访客不该为编辑器与对话面板付出 bundle 代价。
 */
const route = useRoute()

const loading = ref(true)
const spec = ref<PublicSpec | null>(null)
const errorText = ref('')

const token = computed(() => String(route.params.token ?? ''))

const totalItems = computed(() =>
  spec.value ? spec.value.groups.reduce((sum, g) => sum + g.items.length, 0) : 0,
)

const updatedAt = computed(() => {
  if (!spec.value?.updated_at) return ''
  const d = new Date(spec.value.updated_at)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
})

// 分享页可能被转发，不该进搜索引擎索引
let metaRobots: HTMLMetaElement | null = null

onMounted(async () => {
  metaRobots = document.createElement('meta')
  metaRobots.name = 'robots'
  metaRobots.content = 'noindex, nofollow'
  document.head.appendChild(metaRobots)

  try {
    spec.value = await shareApi.getPublicSpec(token.value)
    document.title = spec.value.title || '参数配置表'
  } catch (err) {
    errorText.value =
      err instanceof ApiError ? err.message : '加载失败，请确认链接是否正确'
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  metaRobots?.remove()
})
</script>

<template>
  <div class="share-page">
    <header class="share-head">
      <div class="brand">
        <div class="brand-mark">配</div>
        <span class="brand-name">参数配置表</span>
      </div>
      <span class="readonly-tag">只读分享</span>
    </header>

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
                <span v-if="item.unit" class="spec-unit">{{ item.unit }}</span>
              </dd>
            </div>
          </dl>
        </section>

        <p class="share-foot">
          本页为只读分享，内容由创建者维护
        </p>
      </template>
    </main>
  </div>
</template>

<style scoped>
.share-page {
  height: 100%;
  overflow-y: auto;
  overscroll-behavior: contain;
  background: var(--ds-bg);
}

/* 头部随页面滚动还是固定？分享页内容可能很长，头部保持固定更好找 */
.share-head {
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--ds-border);
  background: var(--ds-surface);
}

.brand {
  display: flex;
  align-items: center;
  gap: 9px;
}

.brand-mark {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--ds-primary);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
}

.brand-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--ds-text);
}

.readonly-tag {
  margin-left: auto;
  padding: 2px 9px;
  border-radius: 20px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-tertiary);
  font-size: 11.5px;
}

.share-main {
  max-width: 720px;
  margin: 0 auto;
  padding: 30px 20px 64px;
}

.state {
  padding: 72px 0;
  text-align: center;
  color: var(--ds-text-tertiary);
  font-size: 13px;
}

.state.error i {
  font-size: 36px;
}

.state.error h1 {
  margin: 14px 0 8px;
  font-size: 17px;
  font-weight: 600;
  color: var(--ds-text);
}

.state.error p {
  margin: 0;
  font-size: 13px;
}

.state-hint {
  margin-top: 6px !important;
  font-size: 12px !important;
  color: var(--ds-text-tertiary);
}

.spec-head {
  margin-bottom: 30px;
}

.spec-head h1 {
  margin: 0 0 6px;
  font-size: 24px;
  font-weight: 600;
  color: var(--ds-text);
  letter-spacing: -0.2px;
}

.spec-desc {
  margin: 0 0 12px;
  font-size: 13.5px;
  line-height: 1.75;
  color: var(--ds-text-secondary);
}

.spec-meta {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  color: var(--ds-text-tertiary);
}

.tag {
  padding: 2px 8px;
  border-radius: 20px;
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
}

.dot {
  color: var(--ds-border-strong);
}

.group {
  margin-bottom: 26px;
}

.group h2 {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--ds-text-secondary);
}

.spec-list {
  margin: 0;
  border: 1px solid var(--ds-border);
  border-radius: var(--ds-radius-lg);
  background: var(--ds-surface);
  overflow: hidden;
}

.spec-row {
  display: flex;
  align-items: baseline;
  gap: 16px;
  padding: 13px 16px;
}

.spec-row + .spec-row {
  border-top: 1px solid var(--ds-border);
}

.spec-row dt {
  flex: 0 0 42%;
  font-size: 13.5px;
  color: var(--ds-text-secondary);
}

.spec-row dd {
  flex: 1;
  margin: 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--ds-text);
}

.spec-value.empty {
  color: var(--ds-text-tertiary);
  font-weight: 400;
  font-style: italic;
}

.spec-unit {
  margin-left: 5px;
  font-size: 12.5px;
  font-weight: 400;
  color: var(--ds-text-secondary);
}

.share-foot {
  margin: 44px 0 0;
  text-align: center;
  font-size: 12px;
  color: var(--ds-text-tertiary);
}

@media (max-width: 560px) {
  .share-main {
    padding: 22px 16px 48px;
  }

  .spec-head h1 {
    font-size: 20px;
  }

  .spec-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .spec-row dt {
    flex: none;
  }
}
</style>
