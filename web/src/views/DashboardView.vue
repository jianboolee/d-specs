<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'

import AppSidebar from '@/components/AppSidebar.vue'
import BrandMark from '@/components/BrandMark.vue'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth'
import { useSpecTableStore } from '@/stores/specTable'
import { useCreateTable } from '@/composables/useCreateTable'

/**
 * 工作台：登录后的入口页。
 *
 * 只放「回到我的参数表」与「新建」两个动作，产品介绍已挪到根路由的
 * LandingView，不在登录后的页面里重复。
 */
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

onMounted(() => {
  document.title = '工作台 - 参数配置表'
})
</script>

<template>
  <div class="shell">
    <AppSidebar />

    <main class="dashboard ds-scroll-y">
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

.dashboard {
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

/* ==================== 手机端 ==================== */

@media (max-width: 768px) {
  /* 收窄内边距，也让居中内容离左上角汉堡按钮更远 */
  .hero {
    padding: 32px 20px 48px;
  }

  .hero h1 {
    font-size: 21px;
  }
}
</style>
