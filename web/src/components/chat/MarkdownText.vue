<script setup lang="ts">
import { computed } from 'vue'
import DOMPurify from 'dompurify'
import { marked } from 'marked'

/**
 * AI 回复用的 Markdown 渲染。
 *
 * 模型返回的是 Markdown 源码（**加粗**、列表、代码块等），直接插值会把记号
 * 原样显示出来。这里用 marked 解析成 HTML，再交给 DOMPurify 过滤后再 v-html，
 * 避免模型输出里的脚本 / 事件属性被当成可执行内容。
 *
 * 只用于 assistant 消息：用户自己输入的内容按纯文本展示，保证换行与原文一致。
 */
const props = defineProps<{ text: string }>()

marked.use({
  gfm: true,
  breaks: true,
  renderer: {
    // 链接统一在新标签打开，并补上 noopener，防止新页面通过 window.opener 反向操作本页
    link({ href, title, tokens }) {
      const text = this.parser.parseInline(tokens)
      const titleAttr = title ? ` title="${title}"` : ''
      return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer">${text}</a>`
    },
  },
})

const html = computed(() => {
  if (!props.text) return ''
  const raw = marked.parse(props.text, { async: false })
  // img 在聊天里几乎用不到，也避免模型输出外域图片造成请求外泄；
  // target 不在 DOMPurify 默认白名单里，需要显式放行，否则链接会在当前页跳走
  return DOMPurify.sanitize(raw, { FORBID_TAGS: ['img'], ADD_ATTR: ['target'] })
})
</script>

<template>
  <div class="md" v-html="html" />
</template>

<style scoped>
/*
 * v-html 注入的节点没有 scoped 的 data-v 属性，所以下面统一用 :deep()。
 */
.md {
  font-size: inherit;
  line-height: 1.75;
  color: inherit;
  word-break: break-word;
}

.md :deep(> :first-child) {
  margin-top: 0;
}

.md :deep(> :last-child) {
  margin-bottom: 0;
}

.md :deep(p) {
  margin: 0 0 12px;
}

.md :deep(h1),
.md :deep(h2),
.md :deep(h3),
.md :deep(h4),
.md :deep(h5),
.md :deep(h6) {
  margin: 20px 0 10px;
  font-weight: 600;
  line-height: 1.4;
}

.md :deep(h1) {
  font-size: 1.35em;
}

.md :deep(h2) {
  font-size: 1.2em;
}

.md :deep(h3) {
  font-size: 1.08em;
}

.md :deep(h4),
.md :deep(h5),
.md :deep(h6) {
  font-size: 1em;
}

.md :deep(ul),
.md :deep(ol) {
  margin: 0 0 12px;
  padding-left: 24px;
}

.md :deep(li) {
  margin: 4px 0;
}

.md :deep(li > p) {
  margin: 0;
}

.md :deep(li::marker) {
  color: var(--ds-text-tertiary);
}

.md :deep(code) {
  padding: 2px 5px;
  border-radius: 4px;
  background: var(--ds-surface-muted);
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.88em;
}

.md :deep(pre) {
  margin: 0 0 12px;
  padding: 12px 14px;
  border-radius: 10px;
  background: var(--ds-surface-muted);
  overflow-x: auto;
}

.md :deep(pre code) {
  padding: 0;
  background: none;
  font-size: 13px;
  line-height: 1.6;
}

.md :deep(blockquote) {
  margin: 0 0 12px;
  padding: 2px 0 2px 12px;
  border-left: 3px solid var(--ds-border-strong);
  color: var(--ds-text-secondary);
}

.md :deep(a) {
  color: var(--ds-text);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.md :deep(a:hover) {
  color: var(--ds-text-secondary);
}

.md :deep(hr) {
  margin: 18px 0;
  border: none;
  border-top: 1px solid var(--ds-border);
}

/* 宽表格自己横向滚动，不撑破消息列 */
.md :deep(table) {
  display: block;
  width: max-content;
  max-width: 100%;
  margin: 0 0 12px;
  border-collapse: collapse;
  overflow-x: auto;
  font-size: 0.94em;
}

.md :deep(th),
.md :deep(td) {
  padding: 6px 10px;
  border: 1px solid var(--ds-border);
  text-align: left;
}

.md :deep(th) {
  background: var(--ds-surface-muted);
  font-weight: 600;
}

.md :deep(input[type='checkbox']) {
  margin-right: 6px;
  vertical-align: middle;
}
</style>
