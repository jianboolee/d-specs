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
    // 表格套一层滚动容器：宽表在窄消息列里横向滚动，而不是把气泡撑破
    table(token) {
      let head = ''
      for (const cell of token.header) head += this.tablecell(cell)
      let body = ''
      for (const row of token.rows) {
        let cells = ''
        for (const cell of row) cells += this.tablecell(cell)
        body += this.tablerow({ text: cells })
      }
      if (body) body = `<tbody>${body}</tbody>`
      return `<div class="md-table-wrap"><table><thead>${this.tablerow({ text: head })}</thead>${body}</table></div>`
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

/*
 * 表格：外层容器负责圆角与横向滚动，内部表格铺满可用宽度。
 * overflow 交给容器，单元格只画内框，避免表格自己的边框与容器边框重影。
 */
.md :deep(.md-table-wrap) {
  margin: 0 0 14px;
  border: 1px solid var(--ds-border);
  border-radius: 10px;
  overflow-x: auto;
  overscroll-behavior-x: contain;
}

.md :deep(.md-table-wrap)::-webkit-scrollbar {
  height: 6px;
}

.md :deep(table) {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.94em;
  line-height: 1.6;
}

.md :deep(th),
.md :deep(td) {
  padding: 8px 12px;
  border-bottom: 1px solid var(--ds-border);
  border-right: 1px solid var(--ds-border);
  vertical-align: top;
  /* markdown 的 :---: 对齐会落成 align 属性，这里的 left 会被下面的属性选择器覆盖回去 */
  text-align: left;
}

/* 最后一列去掉右边框、最后一行去掉下边框，交给容器兜底，避免与容器边框重影 */
.md :deep(th:last-child),
.md :deep(td:last-child) {
  border-right: none;
}

.md :deep(tr:last-child > th),
.md :deep(tr:last-child > td) {
  border-bottom: none;
}

.md :deep(th) {
  background: var(--ds-surface-muted);
  font-weight: 600;
  white-space: nowrap;
}

/* 隔行浅色，长表格更好数行；用 text 令牌混色，明暗主题都成立 */
.md :deep(tbody tr:nth-child(even)) {
  background: color-mix(in srgb, var(--ds-text) 3%, transparent);
}

/* markdown 的 :---: 对齐落成 align 属性，属性选择器优先级更高，能盖回上面的 left */
.md :deep([align='center']) {
  text-align: center;
}

.md :deep([align='right']) {
  text-align: right;
}

.md :deep(input[type='checkbox']) {
  margin-right: 6px;
  vertical-align: middle;
}
</style>
