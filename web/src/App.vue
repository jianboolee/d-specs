<script setup lang="ts">
import CreateTableDialog from '@/components/CreateTableDialog.vue'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useThemeMode } from '@/composables/useThemeMode'

// 确保主题在应用启动时立即生效：写入 data-theme 与 .dark 类，供 shadcn/sonner 深色模式使用
useThemeMode()
</script>

<template>
  <TooltipProvider>
    <router-view />
    <!-- 全局唯一的新建弹窗：侧栏/首页/列表页都唤起它，不再靠路由跳转 -->
    <CreateTableDialog />
    <Toaster />
  </TooltipProvider>
</template>
