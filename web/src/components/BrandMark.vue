<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    /** 边长（px） */
    size?: number
    /** 圆角占边长的比例，0–0.5 */
    radiusRatio?: number
  }>(),
  { size: 28, radiusRatio: 0.28 },
)

/**
 * 换成自家 logo：把图片放到 web/public/logo.png 即可，不需要改代码。
 * 详见 web/public/README.md。
 *
 * 文件不存在时会自动退回「配」字方块（不会显示裂图），
 * 所以放上 logo 之前，控制台会有一条 /logo.png 的 404 —— 那是预期的。
 */
const LOGO_SRC = '/logo.png'

const logoMissing = ref(false)

const boxStyle = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  borderRadius: `${Math.round(props.size * props.radiusRatio)}px`,
}))

const textStyle = computed(() => ({
  fontSize: `${Math.max(11, Math.round(props.size * 0.42))}px`,
}))
</script>

<template>
  <div class="brand-mark" :class="{ 'with-logo': !logoMissing }" :style="boxStyle">
    <img v-if="!logoMissing" :src="LOGO_SRC" alt="" @error="logoMissing = true" />
    <span v-else :style="textStyle">配</span>
  </div>
</template>

<style scoped>
.brand-mark {
  flex: none;
  display: grid;
  place-items: center;
  overflow: hidden;
  background: var(--ds-primary);
  color: #fff;
  font-weight: 600;
  line-height: 1;
  user-select: none;
}

/* 用图片时容器不再铺底色，让 logo 自己说话（自带底色或透明底都行） */
.brand-mark.with-logo {
  background: transparent;
}

.brand-mark img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
</style>
