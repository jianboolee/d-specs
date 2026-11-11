<script setup lang="ts">
import { computed, ref } from 'vue'
import QrcodeVue from 'qrcode.vue'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

/**
 * 分享页的「扫码查看」弹窗。
 *
 * 二维码在浏览器本地生成（qrcode.vue 自带的编码器），不会把公开链接发给任何
 * 第三方服务。画布按 480px 渲染、CSS 缩到 232px：屏幕上清晰，下载下来的 PNG
 * 也足够大，转发到群里不会被压花。
 */
const props = defineProps<{
  open: boolean
  /** 要编码的地址：调用方在打开时取 window.location.href */
  url: string
  /** 仅用于下载文件名 */
  tableTitle?: string
}>()

const emit = defineEmits<{ 'update:open': [boolean] }>()

const open = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
})

const qrBox = ref<HTMLElement | null>(null)

/** 文件名里不能出现路径分隔符等字符，否则部分系统会直接下载失败 */
const fileName = computed(() => {
  const base = (props.tableTitle || '').replace(/[\\/:*?"<>|]/g, ' ').trim()
  return `${base || '参数表'}-二维码.png`
})

function download() {
  const canvas = qrBox.value?.querySelector('canvas')
  if (!canvas) return
  const link = document.createElement('a')
  link.href = canvas.toDataURL('image/png')
  link.download = fileName.value
  link.click()
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="qr-dialog sm:max-w-[340px]">
      <DialogHeader>
        <DialogTitle>扫码查看</DialogTitle>
        <DialogDescription>用手机相机或微信扫一扫，在手机上打开这份参数表。</DialogDescription>
      </DialogHeader>

      <div ref="qrBox" class="qr-box">
        <QrcodeVue
          v-if="url"
          :value="url"
          :size="480"
          :margin="2"
          level="M"
          render-as="canvas"
        />
      </div>

      <p class="qr-url">{{ url }}</p>

      <Button class="qr-download" size="lg" @click="download">
        <i class="ri-download-2-line" />
        点击下载
      </Button>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
/* 二维码需要固定的白底+深色码点，不跟随深色主题，否则部分扫码器识别率会下降 */
.qr-box {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px;
  border-radius: 14px;
  background: #fff;
}

.qr-box :deep(canvas) {
  display: block;
  /* qrcode.vue 会把 draw 尺寸写成内联 style，这里要 !important 才能压住：
     位图仍按 480px（再乘 devicePixelRatio）生成，下载清晰，屏幕上只是缩放显示 */
  width: 232px !important;
  height: 232px !important;
}

.qr-url {
  margin: 0;
  font-size: 11px;
  line-height: 1.6;
  color: var(--ds-text-tertiary);
  text-align: center;
  word-break: break-all;
}

.qr-download {
  width: 100%;
  border-radius: 9999px;
}
</style>
