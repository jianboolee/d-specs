<script setup lang="ts">
import { computed } from 'vue'
import {
  NConfigProvider,
  NDialogProvider,
  NLoadingBarProvider,
  NMessageProvider,
  darkTheme,
  dateZhCN,
  zhCN,
  type GlobalThemeOverrides,
} from 'naive-ui'

import { useThemeMode } from '@/composables/useThemeMode'

const { mode } = useThemeMode()

const theme = computed(() => (mode.value === 'dark' ? darkTheme : null))

const themeOverrides = computed<GlobalThemeOverrides>(() => ({
  common: {
    primaryColor: mode.value === 'dark' ? '#7B7FF5' : '#5B5FEF',
    primaryColorHover: '#7B7FF5',
    primaryColorPressed: '#4A4EDB',
    primaryColorSuppl: '#7B7FF5',
    borderRadius: '10px',
  },
}))
</script>

<template>
  <n-config-provider
    :theme="theme"
    :theme-overrides="themeOverrides"
    :locale="zhCN"
    :date-locale="dateZhCN"
  >
    <n-loading-bar-provider>
      <n-message-provider :max="3">
        <n-dialog-provider>
          <router-view />
        </n-dialog-provider>
      </n-message-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>
