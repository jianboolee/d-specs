<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ApiError } from '@/api/http'
import { useAuthStore } from '@/stores/auth'
import { useThemeMode } from '@/composables/useThemeMode'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()
const { mode: themeMode, toggle: toggleTheme } = useThemeMode()

const mode = ref<'login' | 'register'>('login')
const loading = ref(false)
const formRef = ref<FormInst | null>(null)

const form = reactive({ email: '', password: '', name: '' })

const isRegister = computed(() => mode.value === 'register')

const rules = computed<FormRules>(() => ({
  email: [
    { required: true, message: '请输入邮箱', trigger: ['input', 'blur'] },
    { type: 'email', message: '邮箱格式不正确', trigger: ['input', 'blur'] },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: ['input', 'blur'] },
    { min: 8, message: '密码至少 8 位', trigger: ['input', 'blur'] },
  ],
}))

function switchMode(next: 'login' | 'register') {
  mode.value = next
  formRef.value?.restoreValidation()
}

async function submit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  try {
    if (isRegister.value) {
      await auth.register(form.email, form.password, form.name || undefined)
      message.success('注册成功，已自动登录')
    } else {
      await auth.login(form.email, form.password)
      message.success('登录成功')
    }

    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/tables'
    await router.replace(redirect)
  } catch (err) {
    message.error(err instanceof ApiError ? err.message : '操作失败，请稍后重试')
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
        <div class="brand-mark">配</div>
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

      <n-form
        ref="formRef"
        :model="form"
        :rules="rules"
        :show-label="false"
        size="large"
        @keydown.enter.prevent="submit"
      >
        <n-form-item v-if="isRegister" path="name">
          <n-input v-model:value="form.name" placeholder="称呼（可留空）" maxlength="80" />
        </n-form-item>

        <n-form-item path="email">
          <n-input v-model:value="form.email" placeholder="邮箱" autocomplete="username" />
        </n-form-item>

        <n-form-item path="password">
          <n-input
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            placeholder="密码"
            :autocomplete="isRegister ? 'new-password' : 'current-password'"
          />
        </n-form-item>
      </n-form>

      <n-button type="primary" size="large" block :loading="loading" @click="submit">
        {{ isRegister ? '创建账号' : '登录' }}
      </n-button>

      <div class="hint">
        <template v-if="isRegister">
          注册后即可创建你自己的参数表，每张表对应一个产品。
        </template>
        <template v-else>
          演示账号：<code>demo@dspecs.dev</code> / <code>demo12345</code>
        </template>
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

.brand-mark {
  width: 42px;
  height: 42px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: var(--ds-primary);
  color: #fff;
  font-size: 18px;
  font-weight: 600;
}

.brand-text h1 {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--ds-text);
}

.brand-text p {
  margin: 2px 0 0;
  font-size: 12.5px;
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

.tab {
  flex: 1;
  padding: 7px 0;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 13.5px;
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
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--ds-text-tertiary);
}

.hint code {
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: 12px;
}
</style>
