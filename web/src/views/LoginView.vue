<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { ApiError } from '@/api/http'
import { useAuthStore } from '@/stores/auth'
import { useThemeMode } from '@/composables/useThemeMode'
import BrandMark from '@/components/BrandMark.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const { mode: themeMode, toggle: toggleTheme } = useThemeMode()

const mode = ref<'login' | 'register'>('login')
const loading = ref(false)
const showPassword = ref(false)

const form = reactive({ email: '', password: '', name: '' })
const errors = reactive<{ email?: string; password?: string }>({})

const isRegister = computed(() => mode.value === 'register')

function switchMode(next: 'login' | 'register') {
  mode.value = next
  errors.email = undefined
  errors.password = undefined
}

function validate(): boolean {
  errors.email = undefined
  errors.password = undefined

  const email = form.email.trim()
  if (!email) {
    errors.email = '请输入邮箱'
    return false
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    errors.email = '邮箱格式不正确'
    return false
  }
  if (!form.password) {
    errors.password = '请输入密码'
    return false
  }
  if (isRegister.value && form.password.length < 8) {
    errors.password = '密码至少 8 位'
    return false
  }
  return true
}

async function submit() {
  if (!validate()) return

  loading.value = true
  try {
    if (isRegister.value) {
      await auth.register(form.email, form.password, form.name || undefined)
      toast.success('注册成功，已自动登录')
    } else {
      await auth.login(form.email, form.password)
      toast.success('登录成功')
    }

    // 没有指定去向时回首页（欢迎页），而不是直接摊开参数表列表
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '操作失败，请稍后重试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <button class="theme-toggle" type="button" @click="toggleTheme">
      <i :class="themeMode === 'dark' ? 'ri-sun-line' : 'ri-moon-line'" />
    </button>

    <div class="login-card">
      <div class="brand">
        <BrandMark :size="42" />
        <div class="brand-text">
          <h1>参数配置表</h1>
          <p>用对话的方式，为每个产品维护一份参数表</p>
        </div>
      </div>

      <div class="tabs">
        <button
          type="button"
          class="tab"
          :class="{ active: !isRegister }"
          @click="switchMode('login')"
        >
          登录
        </button>
        <button
          type="button"
          class="tab"
          :class="{ active: isRegister }"
          @click="switchMode('register')"
        >
          注册
        </button>
      </div>

      <form class="login-form" @submit.prevent="submit">
        <div v-if="isRegister" class="field">
          <i class="field-icon ri-user-line" aria-hidden="true" />
          <Input
            v-model="form.name"
            placeholder="称呼（可留空）"
            maxlength="80"
            autocomplete="nickname"
          />
        </div>

        <div class="field">
          <Label class="sr-only" for="login-email">邮箱</Label>
          <i class="field-icon ri-mail-line" aria-hidden="true" />
          <Input
            id="login-email"
            v-model="form.email"
            placeholder="邮箱"
            autocomplete="username"
            :aria-invalid="!!errors.email"
            @input="errors.email = undefined"
          />
          <p v-if="errors.email" class="field-error">{{ errors.email }}</p>
        </div>

        <div class="field">
          <Label class="sr-only" for="login-password">密码</Label>
          <i class="field-icon ri-lock-2-line" aria-hidden="true" />
          <div class="password-wrap">
            <Input
              id="login-password"
              v-model="form.password"
              :type="showPassword ? 'text' : 'password'"
              placeholder="密码"
              :autocomplete="isRegister ? 'new-password' : 'current-password'"
              :aria-invalid="!!errors.password"
              @input="errors.password = undefined"
            />
            <button
              type="button"
              class="password-toggle"
              :title="showPassword ? '隐藏密码' : '显示密码'"
              @click="showPassword = !showPassword"
            >
              <i :class="showPassword ? 'ri-eye-off-line' : 'ri-eye-line'" />
            </button>
          </div>
          <p v-if="errors.password" class="field-error">{{ errors.password }}</p>
        </div>

        <Button type="submit" size="lg" class="w-full login-submit" :disabled="loading">
          {{ isRegister ? '创建账号' : '登录' }}
        </Button>
      </form>

      <div v-if="isRegister" class="hint">
        注册后即可创建你自己的参数表，每张表对应一个产品。
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  position: relative;
  height: 100%;
  overflow-y: auto;
  display: flex;
  padding: 24px;
  background:
    radial-gradient(circle at 12% 18%, var(--ds-primary-soft), transparent 42%),
    radial-gradient(circle at 88% 82%, var(--ds-primary-soft), transparent 46%),
    var(--ds-bg);
}

.theme-toggle {
  position: absolute;
  top: 20px;
  right: 20px;
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: 1px solid var(--ds-border);
  border-radius: 10px;
  background: var(--ds-surface);
  color: var(--ds-text-secondary);
  font-size: 16px;
  cursor: pointer;
}

.theme-toggle:hover {
  color: var(--ds-text);
}

/*
 * 用 margin:auto 而不是父级 align-items:center 来做居中：
 * 卡片比视口高时，align-items:center 会把顶部裁掉且滚不上去。
 */
.login-card {
  margin: auto;
  width: 100%;
  max-width: 392px;
  padding: 32px;
  border: 1px solid var(--ds-border);
  border-radius: 18px;
  background: var(--ds-surface);
  box-shadow: 0 18px 48px rgba(18, 22, 34, 0.08);
}

.brand {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 24px;
}


.brand-text h1 {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--ds-text);
}

.brand-text p {
  margin: 2px 0 0;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

.tabs {
  display: flex;
  gap: 4px;
  padding: 3px;
  margin-bottom: 20px;
  border-radius: 10px;
  background: var(--ds-surface-muted);
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.field {
  position: relative;
}

/*
 * 登录表单的输入框比组件默认（h-8）更高更宽，并给左侧图标留出内边距。
 * 用 :deep 提高优先级覆盖 Input 内置的 Tailwind 尺寸类。
 */
.login-form :deep(input) {
  height: 46px;
  padding-left: 42px;
  padding-right: 14px;
  border-radius: 12px;
  font-size: var(--ds-font-base);
}

/* 左侧图标：按输入框高度（46px）垂直居中，出现错误文字时也不会跟着偏移 */
.field-icon {
  position: absolute;
  left: 14px;
  top: 0;
  height: 46px;
  display: flex;
  align-items: center;
  pointer-events: none;
  font-size: 18px;
  color: var(--ds-text-tertiary);
  transition: color 0.16s ease;
}

.field:focus-within .field-icon {
  color: var(--ds-text);
}

/* 提交按钮与输入框等高，视觉上成一组 */
.login-submit {
  height: 46px;
  font-size: var(--ds-font-base);
}

.password-wrap {
  position: relative;
}

.password-wrap :deep(input) {
  padding-right: 40px;
}

.password-toggle {
  position: absolute;
  top: 50%;
  right: 8px;
  transform: translateY(-50%);
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--ds-text-tertiary);
  cursor: pointer;
}

.password-toggle:hover {
  color: var(--ds-text-secondary);
}

.field-error {
  margin: 5px 2px 0;
  font-size: var(--ds-font-xs);
  color: var(--ds-destructive, #d64545);
}

.tab {
  flex: 1;
  padding: 7px 0;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-sm);
  cursor: pointer;
  transition: all 0.16s ease;
}

.tab.active {
  background: var(--ds-surface);
  color: var(--ds-text);
  font-weight: 500;
  box-shadow: 0 1px 3px rgba(18, 22, 34, 0.08);
}

.hint {
  margin-top: 18px;
  text-align: center;
  font-size: var(--ds-font-xs);
  line-height: 1.7;
  color: var(--ds-text-tertiary);
}
</style>
