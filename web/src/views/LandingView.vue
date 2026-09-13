<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

import BrandMark from '@/components/BrandMark.vue'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth'

/**
 * 项目介绍页（根路由 `/`）。
 *
 * 公开可访问，不加载侧栏与编辑器：面向搜索引擎与首次到访的人，
 * 说明这是什么、能做什么，再把人引到登录或工作台。
 *
 * 已登录用户照常看到本页，只是行动按钮换成「进入工作台」——
 * 这样根地址对所有人一致，也便于被收录。
 */
const router = useRouter()
const auth = useAuthStore()

const points = [
  {
    icon: 'ri-sparkling-2-line',
    title: '对话建表',
    desc: '说清楚产品是什么，AI 直接生成参数项，不用一行行手敲',
  },
  {
    icon: 'ri-table-line',
    title: '可视化编辑',
    desc: '参数名与参数值直接改，每次改动都有版本，随时可撤销',
  },
  {
    icon: 'ri-share-line',
    title: '一键分享',
    desc: '生成只读链接，客户打开就能看，不必登录、不必导文件',
  },
]

function goLogin() {
  void router.push({ name: 'login' })
}

function goDashboard() {
  void router.push({ name: 'dashboard' })
}

function goDiscover() {
  void router.push({ name: 'discover' })
}

onMounted(() => {
  document.title = '参数配置表 - 用对话的方式维护产品参数'
})
</script>

<template>
  <div class="landing ds-scroll-y">
    <header class="landing-head">
      <div class="brand">
        <BrandMark :size="30" />
        <span>参数配置表</span>
      </div>
      <div class="head-actions">
        <button type="button" class="head-link" @click="goDiscover">发现</button>
        <Button v-if="auth.isLoggedIn" size="sm" @click="goDashboard">进入工作台</Button>
        <Button v-else size="sm" @click="goLogin">登录</Button>
      </div>
    </header>

    <main class="landing-main">
      <section class="hero">
        <h1>用对话的方式<br />为每个产品维护一份参数表</h1>
        <p class="lead">
          告诉 AI 这是什么产品，它会帮你把电池容量、充电功率这些参数整理成一张表。
          表格常驻可编辑，改错了随时撤销，做完了生成链接就能发给客户。
        </p>

        <div class="cta">
          <Button v-if="auth.isLoggedIn" size="lg" @click="goDashboard">
            <i class="ri-dashboard-line" />
            进入工作台
          </Button>
          <Button v-else size="lg" @click="goLogin">
            <i class="ri-login-box-line" />
            登录 / 注册
          </Button>
          <Button size="lg" variant="outline" @click="goDiscover">
            <i class="ri-compass-3-line" />
            浏览公开参数表
          </Button>
        </div>
      </section>

      <ul class="points">
        <li v-for="point in points" :key="point.title" class="point">
          <i :class="point.icon" />
          <strong>{{ point.title }}</strong>
          <span>{{ point.desc }}</span>
        </li>
      </ul>
    </main>

    <footer class="landing-foot">由小野驴科技提供技术支持</footer>
  </div>
</template>

<style scoped>
.landing {
  height: 100%;
  display: flex;
  flex-direction: column;
  background:
    radial-gradient(circle at 78% 6%, var(--ds-primary-soft), transparent 42%),
    var(--ds-bg);
}

.landing-head {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: 60px;
  padding: 0 24px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: var(--ds-font-md);
  font-weight: 600;
  color: var(--ds-text);
}

.head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.head-link {
  padding: 6px 12px;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-sm);
  cursor: pointer;
}

.head-link:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.landing-main {
  flex: 1;
  min-height: 0;
  width: 100%;
  max-width: 860px;
  margin: 0 auto;
  padding: 40px 24px 56px;
}

.hero {
  text-align: center;
  padding: 32px 0 8px;
}

.hero h1 {
  margin: 0 0 18px;
  font-size: 34px;
  line-height: 1.35;
  font-weight: 700;
  letter-spacing: -0.4px;
  color: var(--ds-text);
}

.lead {
  max-width: 560px;
  margin: 0 auto 30px;
  font-size: var(--ds-font-md);
  line-height: 1.9;
  color: var(--ds-text-secondary);
}

.cta {
  display: flex;
  gap: 10px;
  justify-content: center;
  flex-wrap: wrap;
}

.points {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
  margin: 56px 0 0;
  padding: 0;
  list-style: none;
}

.point {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 18px;
  border: 1px solid var(--ds-border);
  border-radius: var(--ds-radius-lg);
  background: var(--ds-surface);
}

.point > i {
  color: var(--ds-primary);
  font-size: 22px;
}

.point strong {
  font-size: var(--ds-font-base);
  font-weight: 600;
  color: var(--ds-text);
}

.point span {
  font-size: var(--ds-font-sm);
  line-height: 1.75;
  color: var(--ds-text-tertiary);
}

.landing-foot {
  flex: none;
  padding: 20px 24px 26px;
  text-align: center;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

@media (max-width: 720px) {
  .hero h1 {
    font-size: 26px;
  }

  .points {
    grid-template-columns: 1fr;
    margin-top: 36px;
  }

  .landing-main {
    padding: 24px 16px 40px;
  }
}
</style>
