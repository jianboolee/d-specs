<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import AppSidebar from '@/components/AppSidebar.vue'
import BrandMark from '@/components/BrandMark.vue'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth'
import { useSpecTableStore } from '@/stores/specTable'
import { useCreateTable } from '@/composables/useCreateTable'

const router = useRouter()
const auth = useAuthStore()
const spec = useSpecTableStore()
const { open: openCreate } = useCreateTable()

const displayName = computed(() => auth.user?.name || auth.user?.email || '')

const greeting = computed(() =>
  displayName.value ? `${displayName.value}，欢迎回来` : '欢迎回来',
)

const tableCount = computed(() => spec.tables.length)

function goTables() {
  void router.push({ name: 'tables' })
}

</script>

<template>
  <div class="shell">
    <AppSidebar />

    <main class="home ds-scroll-y">
      <div class="hero">
        <div class="mark-wrap">
          <BrandMark :size="60" :radius-ratio="0.3" />
        </div>

        <h1>{{ greeting }}</h1>
        <p class="tagline">用对话的方式，为每个产品维护一份参数表</p>

        <div class="actions">
          <Button size="lg" @click="goTables">
            <i class="ri-file-list-line" />
            我的参数表
            <span v-if="tableCount > 0" class="count">{{ tableCount }}</span>
          </Button>

          <Button size="lg" variant="outline" @click="openCreate">
            <i class="ri-add-line" />
            新建参数表
          </Button>
        </div>

        <ul class="points">
          <li v-for="point in [
            { icon: 'ri-sparkling-2-line', title: '对话建表', desc: '说清楚产品是什么，AI 直接生成参数项' },
            { icon: 'ri-table-line', title: '可视化编辑', desc: '参数名与参数值直接改，改动可随时撤销' },
            { icon: 'ri-share-line', title: '一键分享', desc: '生成只读链接，客户打开就能看' },
          ]" :key="point.title" class="point">
            <i :class="point.icon" />
            <div>
              <strong>{{ point.title }}</strong>
              <span>{{ point.desc }}</span>
            </div>
          </li>
        </ul>
      </div>
    </main>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100%;
  overflow: hidden;
  background: var(--ds-bg);
}

.home {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  background:
    radial-gradient(circle at 22% 12%, var(--ds-primary-soft), transparent 46%),
    var(--ds-bg);
}

.hero {
  margin: auto;
  max-width: 768px;
  padding: 48px 28px 64px;
  text-align: center;
}

/* BrandMark 自身不负责水平居中，这里包一层 */
.mark-wrap {
  display: flex;
  justify-content: center;
  margin-bottom: 22px;
}

.hero h1 {
  margin: 0 0 10px;
  font-size: 24px;
  font-weight: 600;
  letter-spacing: -0.2px;
  color: var(--ds-text);
}

.tagline {
  margin: 0 0 30px;
  font-size: var(--ds-font-md);
  line-height: 1.8;
  color: var(--ds-text-secondary);
}

.actions {
  display: flex;
  gap: 10px;
  justify-content: center;
  flex-wrap: wrap;
}

/* 「我的参数表」按钮上的数量角标 */
.count {
  margin-left: 7px;
  padding: 0 7px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.22);
  font-size: var(--ds-font-xs);
  line-height: 1.7;
}

.points {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin: 46px 0 0;
  padding: 0;
  list-style: none;
  text-align: left;
}

.point {
  display: flex;
  gap: 9px;
  padding: 14px;
  border: 1px solid var(--ds-border);
  border-radius: var(--ds-radius-lg);
  background: var(--ds-surface);
}

.point > i {
  flex: none;
  margin-top: 1px;
  color: var(--ds-primary);
  font-size: 17px;
}

.point strong {
  display: block;
  margin-bottom: 3px;
  font-size: var(--ds-font-sm);
  font-weight: 600;
  color: var(--ds-text);
}

.point span {
  font-size: var(--ds-font-xs);
  line-height: 1.65;
  color: var(--ds-text-tertiary);
}

@media (max-width: 720px) {
  .points {
    grid-template-columns: 1fr;
    margin-top: 32px;
  }
}
</style>
