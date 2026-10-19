<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { useThemeMode } from '@/composables/useThemeMode'
import BrandMark from '@/components/BrandMark.vue'
import { Button } from '@/components/ui/button'

const auth = useAuthStore()
const route = useRoute()
const { mode: themeMode, toggle: toggleTheme } = useThemeMode()
const redirecting = ref(false)

// 点「登录」跳 d-auth 的中央登录页（SSO）；带上是登录后要回到的本站路径：
// 优先用守卫记下的 redirect（用户本来想去的页面），否则回工作台。
function login() {
  if (redirecting.value) return
  redirecting.value = true
  const target = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
  void auth.loginWithSSO(target)
}
</script>

<template>
  <main class="login-page">
    <button class="theme-toggle" type="button" :title="themeMode === 'dark' ? '切换到浅色' : '切换到深色'" @click="toggleTheme">
      <i :class="themeMode === 'dark' ? 'ri-sun-line' : 'ri-moon-line'" />
    </button>

    <div class="login-content">
      <div class="brand">
        <BrandMark :size="56" />
        <span class="brand-name">参数配置表</span>
      </div>

      <Button
        class="rounded-full px-6"
        size="lg"
        :disabled="redirecting"
        @click="login"
      >
        <i class="ri-login-circle-line" />
        {{ redirecting ? '正在跳转…' : '登录' }}
      </Button>
    </div>
  </main>
</template>

<style scoped>
/*
 * 极简登录页（对齐 d-note 的风格）：整页平铺底色，居中一列「品牌标 + 登录按钮」，
 * 不套卡片边框/阴影，也不铺渐变背景 —— 这一页只是 SSO 的入口，不承载表单。
 */
.login-page {
  position: relative;
  height: 100%;
  display: flex;
  padding: 24px;
  background: var(--ds-sidebar-bg);
}

/* 单列居中：用 margin:auto 而不是父级 align-items:center，
   内容高于视口时才不会被裁掉顶部 */
.login-content {
  margin: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
}

.brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.brand-name {
  font-size: var(--ds-font-sm);
  font-weight: 500;
  letter-spacing: 0.02em;
  color: var(--ds-text-secondary);
}

.theme-toggle {
  position: absolute;
  top: 20px;
  right: 20px;
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--ds-text-tertiary);
  font-size: 16px;
  cursor: pointer;
  transition: color 0.16s ease, background 0.16s ease;
}

.theme-toggle:hover {
  color: var(--ds-text);
  background: var(--ds-primary-soft);
}
</style>