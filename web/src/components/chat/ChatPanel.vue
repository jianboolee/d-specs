<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

import type { ChatChange } from '@/types/api'

/** 与 chat store 的 UiMessage 结构一致（结构化匹配，不直接依赖 store） */
export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  streaming?: boolean
  toolLabel?: string
  changes?: ChatChange[]
  appliedRevision?: number
  pending?: string
  error?: string
  undone?: boolean
  /** 从历史记录恢复的消息（没有当时的版本号，因此不提供逐条撤销） */
  restored?: boolean
}

const props = defineProps<{
  messages: ChatMessage[]
  sending?: boolean
  disabled?: boolean
  /** 正在拉取历史对话 */
  loading?: boolean
}>()

const emit = defineEmits<{
  send: [text: string]
  stop: []
  undo: [messageId: string]
}>()

const draft = ref('')
const listRef = ref<HTMLElement | null>(null)
const textareaRef = ref<HTMLTextAreaElement | null>(null)

/** 空态下的快捷建议，点击后填入输入框 */
const suggestions = [
  '帮我建一张房车的参数表',
  '把续航里程改成 800km',
  '给动力分组下的参数都加上单位',
  '把这张表按分组重新整理一遍',
]

/** 输入框随内容自动增高，超过上限后内部滚动 */
const MAX_INPUT_HEIGHT = 148

function autoResize() {
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, MAX_INPUT_HEIGHT)}px`
}

function pickSuggestion(text: string) {
  draft.value = text
  textareaRef.value?.focus()
  autoResize()
}

/** 消息条数或最后一条的文字变化时滚到底部（流式时文字是持续增长的） */
const tailText = computed(() => {
  const list = props.messages
  const last = list[list.length - 1]
  return last ? last.text : ''
})
watch([() => props.messages.length, tailText], scrollToBottom)

async function scrollToBottom() {
  await nextTick()
  const el = listRef.value
  if (el) el.scrollTop = el.scrollHeight
}

function submit() {
  const text = draft.value.trim()
  if (!text || props.sending || props.disabled) return
  emit('send', text)
  draft.value = ''
}

function onKeydown(event: KeyboardEvent) {
  // Enter 发送，Shift + Enter 换行
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    submit()
  }
}

function hasChanges(msg: ChatMessage) {
  return (msg.changes?.length ?? 0) > 0
}

function kindLabel(kind: ChatChange['kind']) {
  switch (kind) {
    case 'add':
      return '新增'
    case 'update':
      return '修改'
    case 'delete':
      return '删除'
    default:
      return '移动'
  }
}
</script>

<template>
  <section class="chat">
    <div ref="listRef" class="chat-list ds-scroll-y">
      <div v-for="msg in messages" :key="msg.id" class="msg" :class="msg.role">
        <div v-if="msg.role === 'assistant'" class="msg-avatar">
          <i class="ri-sparkling-2-fill" />
        </div>

        <div class="msg-body">
          <div v-if="msg.text" class="msg-bubble" :class="{ undone: msg.undone }">{{ msg.text }}</div>

          <div v-if="msg.streaming && !msg.text && !msg.toolLabel" class="msg-bubble typing-bubble">
            <span class="typing"><span /><span /><span /></span>
          </div>

          <div v-if="msg.toolLabel" class="tool-line">
            <i class="ri-loader-4-line spin" />
            <span>{{ msg.toolLabel }}</span>
          </div>

          <div v-if="hasChanges(msg)" class="changes" :class="{ undone: msg.undone }">
            <div class="changes-head">
              <i :class="msg.undone ? 'ri-arrow-go-back-line' : 'ri-checkbox-circle-fill'" />
              <span v-if="msg.undone">已撤销这次改动</span>
              <span v-else>
                已修改 {{ msg.changes!.length }} 处
                <template v-if="msg.appliedRevision">· r{{ msg.appliedRevision }}</template>
              </span>
              <button
                v-if="!msg.undone && !msg.restored"
                type="button"
                class="undo"
                @click="emit('undo', msg.id)"
              >
                <i class="ri-arrow-go-back-line" />
                撤销
              </button>
            </div>

            <ul>
              <li v-for="(c, i) in msg.changes" :key="`${c.item_id}-${i}`">
                <span class="change-kind" :class="c.kind">{{ kindLabel(c.kind) }}</span>
                <span class="change-name">{{ c.name }}</span>
                <template v-if="c.kind === 'update'">
                  <span class="change-from">{{ c.before || '—' }}</span>
                  <i class="ri-arrow-right-line" />
                  <span class="change-to">{{ c.after || '—' }}</span>
                </template>
                <template v-else-if="c.kind === 'add'">
                  <span class="change-to">{{ c.after || '—' }}</span>
                </template>
                <template v-else-if="c.kind === 'delete'">
                  <span class="change-from">{{ c.before || '—' }}</span>
                </template>
              </li>
            </ul>
          </div>

          <div v-if="msg.pending" class="pending">
            <i class="ri-error-warning-line" />
            <span>{{ msg.pending }}</span>
          </div>

          <div v-if="msg.error" class="msg-error">
            <i class="ri-error-warning-line" />
            <span>{{ msg.error }}</span>
          </div>
        </div>
      </div>

      <!-- 拉取历史时不要闪空态引导 -->
      <div v-if="loading" class="chat-loading">
        <i class="ri-loader-4-line spin" />
        <span>正在加载历史对话…</span>
      </div>

      <!-- 空态作为插槽放在滚动区内，避免用绝对定位叠在列表上 -->
      <div v-else-if="messages.length === 0 && !sending" class="chat-empty">
        <slot name="empty" />
        <div class="suggestions">
          <button
            v-for="s in suggestions"
            :key="s"
            type="button"
            class="suggestion"
            @click="pickSuggestion(s)"
          >
            {{ s }}
          </button>
        </div>
      </div>
    </div>

    <div class="composer">
      <div class="composer-box">
        <textarea
          ref="textareaRef"
          v-model="draft"
          class="composer-input"
          rows="1"
          placeholder="告诉 AI 你要怎么调整这张参数表…"
          :disabled="disabled"
          @keydown="onKeydown"
          @input="autoResize"
        />
        <button v-if="sending" type="button" class="stop-btn" title="停止" @click="emit('stop')">
          <i class="ri-stop-fill" />
        </button>
        <button
          v-else
          type="button"
          class="send-btn"
          :disabled="!draft.trim() || disabled"
          @click="submit"
        >
          <i class="ri-arrow-up-line" />
        </button>
      </div>
      <p class="composer-hint">
        Enter 发送 · Shift + Enter 换行 · AI 改动会自动应用，可随时撤销
      </p>
    </div>
  </section>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--ds-bg);
}

/*
 * 正文列限宽：面板比 --ds-chat-max 宽时，左右留白把内容居中，
 * 滚动条仍贴在面板右缘。窄面板下退回固定的 24px 内边距。
 */
.chat-list {
  flex: 1;
  min-height: 0;
  padding-block: 24px 8px;
  padding-inline: max(24px, calc((100% - var(--ds-chat-max)) / 2));
}

/* 历史加载态：占满滚动区居中 */
.chat-loading {
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

/* 空态：在滚动区里垂直居中，内容超高时随列表一起滚动 */
.chat-empty {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 18px;
  padding: 24px 8px 40px;
}

.chat-empty :deep(p) {
  margin: 0;
  max-width: 360px;
  font-size: var(--ds-font-base);
  line-height: 1.9;
  color: var(--ds-text-tertiary);
  white-space: pre-wrap;
  text-align: center;
}

.suggestions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  max-width: 340px;
}

.suggestion {
  padding: 6px 12px;
  border-radius: 16px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.suggestion:hover {
  background: var(--ds-border);
  color: var(--ds-text);
}

.msg {
  display: flex;
  gap: 12px;
  margin-bottom: 24px;
  align-items: flex-start;
}

.msg.user {
  justify-content: flex-end;
}

.msg-body {
  min-width: 0;
  max-width: 82%;
}

.msg.user .msg-body {
  display: flex;
  justify-content: flex-end;
}

.msg-avatar {
  width: 30px;
  height: 30px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--ds-primary);
  color: var(--ds-primary-foreground);
  font-size: var(--ds-font-base);
}

.msg-bubble {
  display: inline-block;
  padding: 10px 14px;
  border-radius: 16px;
  font-size: var(--ds-font-base);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

/* assistant：无气泡，直接流式文字（ChatGPT 风格） */
.msg.assistant .msg-bubble {
  padding: 2px 0;
  background: transparent;
  border: none;
  color: var(--ds-text);
}

/* user：浅灰圆角气泡，右下角收小 */
.msg.user .msg-bubble {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
  border-bottom-right-radius: 6px;
}

.msg-bubble.undone {
  opacity: 0.55;
}

.typing-bubble {
  padding: 10px 14px;
  background: var(--ds-surface-muted);
  border-radius: 16px;
  border-top-left-radius: 6px;
}

.tool-line {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}

.spin {
  animation: spin 1.1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.changes {
  margin-top: 9px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--ds-surface-muted);
}

.changes.undone {
  opacity: 0.6;
}

.changes-head {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-secondary);
}

.changes-head > i {
  color: #18a058;
  font-size: var(--ds-font-md);
}

.changes.undone .changes-head > i {
  color: var(--ds-text-tertiary);
}

.undo {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border-radius: 6px;
  background: var(--ds-surface-muted);
  color: var(--ds-text-secondary);
  font-size: var(--ds-font-xs);
  cursor: pointer;
}

.undo:hover {
  background: var(--ds-border);
  color: var(--ds-text);
}

.changes ul {
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}

.changes li {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 0;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-secondary);
  flex-wrap: wrap;
}

.changes li > i {
  color: var(--ds-text-tertiary);
  font-size: var(--ds-font-xs);
}

.change-kind {
  flex: none;
  padding: 0 5px;
  border-radius: 4px;
  font-size: var(--ds-font-xs);
  background: var(--ds-surface-muted);
  color: var(--ds-text-tertiary);
}

.change-kind.add {
  background: rgba(24, 160, 88, 0.12);
  color: #18a058;
}

.change-kind.delete {
  background: rgba(214, 69, 69, 0.12);
  color: #d64545;
}

.change-kind.update {
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
}

.change-name {
  color: var(--ds-text);
}

.change-from {
  color: var(--ds-text-tertiary);
  text-decoration: line-through;
}

.change-to {
  color: var(--ds-primary);
  font-weight: 500;
}

.pending,
.msg-error {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 9px;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: var(--ds-font-xs);
}

.pending {
  border: 1px solid rgba(245, 166, 35, 0.4);
  background: rgba(245, 166, 35, 0.1);
  color: #b8830a;
}

.msg-error {
  border: 1px solid rgba(214, 69, 69, 0.35);
  background: rgba(214, 69, 69, 0.08);
  color: #c23b3b;
}

.typing {
  display: inline-flex;
  gap: 4px;
}

.typing span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ds-text-tertiary);
  animation: blink 1.2s infinite ease-in-out;
}

.typing span:nth-child(2) {
  animation-delay: 0.18s;
}

.typing span:nth-child(3) {
  animation-delay: 0.36s;
}

@keyframes blink {
  0%,
  60%,
  100% {
    opacity: 0.28;
  }
  30% {
    opacity: 1;
  }
}

.composer {
  flex: none;
  padding-block: 10px 16px;
  /* 输入区与消息列同宽对齐 */
  padding-inline: max(22px, calc((100% - var(--ds-chat-max)) / 2));
}

.composer-box {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 10px 10px 10px 16px;
  border: 1px solid var(--ds-border);
  border-radius: 16px;
  background: var(--ds-surface);
  box-shadow: var(--ds-shadow-sm);
  transition:
    border-color 0.16s ease,
    box-shadow 0.16s ease;
}

.composer-box:focus-within {
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.06);
}

.composer-input {
  flex: 1;
  min-width: 0;
  max-height: 148px;
  border: none;
  outline: none;
  resize: none;
  overflow-y: auto;
  background: transparent;
  color: var(--ds-text);
  font-family: inherit;
  font-size: var(--ds-font-base);
  line-height: 1.7;
  padding: 4px 0;
}

.composer-input::placeholder {
  color: var(--ds-text-tertiary);
}

.send-btn,
.stop-btn {
  width: 32px;
  height: 32px;
  flex: none;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 10px;
  color: var(--ds-primary-foreground);
  font-size: 16px;
  cursor: pointer;
  transition:
    transform 0.12s ease,
    background-color 0.16s ease,
    opacity 0.16s ease;
}

.send-btn {
  background: var(--ds-primary);
}

.send-btn:hover:not(:disabled) {
  background: var(--ds-primary-hover);
  transform: translateY(-1px) scale(1.04);
}

.stop-btn {
  background: var(--ds-text-secondary);
  color: #fff;
}

.send-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.composer-hint {
  margin: 8px 2px 0;
  font-size: var(--ds-font-xs);
  color: var(--ds-text-tertiary);
}
</style>
