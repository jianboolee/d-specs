<script setup lang="ts">
import { computed } from 'vue'

import type { SpecItem } from '@/types/api'
import { groupItems } from '@/utils/specGroups'

/**
 * 参数表文档视图：把一张表排成适合阅读与打印的清单。
 *
 * 由打印页（PrintView）渲染，应用内不直接使用 —— 打印文档要独占一个页面，
 * 不能挂在工作台外壳里，否则打印时得反过来把外壳藏掉，既脆弱又容易漏。
 *
 * 复用 share.css 里 .share-page 那套文档样式：分享页与本页是同一形态，
 * 没必要再写一份几乎相同的 CSS；本页另加 .doc-compact 收紧留白，纸张上更省行。
 *
 * 刻意不显示「参数个数」「导出日期」这类统计信息：打印件要的是参数本身，
 * 这些数字占行且对读表的人没有意义。
 */
const props = withDefaults(
  defineProps<{
    title?: string
    description?: string
    industry?: string
    items?: SpecItem[]
    /** 分栏数：1 列（默认）或 2 列并排，打印页可切换以省纸 */
    columns?: 1 | 2
  }>(),
  { title: '', description: '', industry: '', items: () => [], columns: 1 },
)

const groups = computed(() => groupItems(props.items))
</script>

<template>
  <div class="share-page doc-compact" :class="{ 'doc-cols-2': columns === 2 }">
    <main class="share-main">
      <div class="spec-head">
        <h1>{{ title || '参数表' }}</h1>
        <p v-if="description" class="spec-desc">{{ description }}</p>
        <span v-if="industry" class="tag">{{ industry }}</span>
      </div>

      <!-- 分组容器：两列时用 grid 并排，分组各自成块，不会被拆开 -->
      <div class="doc-groups">
        <section v-for="(group, gi) in groups" :key="group.name || gi" class="group">
          <h2 v-if="group.name">{{ group.name }}</h2>
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
      </div>

      <div v-if="groups.length === 0" class="spec-empty">这张表还没有参数</div>

      <footer class="share-foot">由小野驴科技提供技术支持</footer>
    </main>
  </div>
</template>

<style scoped>
/* 空表提示。其余样式都在全局 share.css 的 .share-page 命名空间下 */
.spec-empty {
  padding: 32px 0;
  text-align: center;
  font-size: var(--ds-font-sm);
  color: var(--ds-text-tertiary);
}
</style>
