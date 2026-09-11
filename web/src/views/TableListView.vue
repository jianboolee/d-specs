<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DropdownOption, FormInst, FormRules } from 'naive-ui'

import AppSidebar from '@/components/AppSidebar.vue'
import { ApiError } from '@/api/http'
import { useAuthStore } from '@/stores/auth'
import { useSpecTableStore } from '@/stores/specTable'
import type { TableSummary } from '@/types/api'
import { formatRelativeTime } from '@/utils/time'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const dialog = useDialog()
const auth = useAuthStore()
const spec = useSpecTableStore()

const greeting = computed(() => {
  const name = auth.user?.name || auth.user?.email || ''
  return name ? `${name}，欢迎回来` : '欢迎回来'
})

onMounted(() => {
  void spec.loadTables().catch((err) => message.error(describe(err)))
})

// ---------- 新建 ----------

const showCreate = ref(false)
const creating = ref(false)
const formRef = ref<FormInst | null>(null)
const form = reactive({ title: '', description: '', industry: '' })

const rules: FormRules = {
  title: [
    { required: true, message: '请输入产品名称', trigger: ['input', 'blur'] },
    { max: 120, message: '产品名称不能超过 120 字', trigger: ['input', 'blur'] },
  ],
}

function openCreate() {
  form.title = ''
  form.description = ''
  form.industry = ''
  showCreate.value = true
}

async function submitCreate() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  creating.value = true
  try {
    const res = await spec.createTable({
      title: form.title.trim(),
      description: form.description.trim(),
      industry: form.industry.trim(),
    })
    showCreate.value = false
    message.success('参数表已创建')
    await router.push({ name: 'workspace', params: { id: res.table.id } })
  } catch (err) {
    message.error(describe(err))
  } finally {
    creating.value = false
  }
}

// 侧边栏的「新建参数表」通过 query 把用户送到这里
watch(
  () => route.query.new,
  (flag) => {
    if (flag === '1') {
      openCreate()
      void router.replace({ name: 'tables' })
    }
  },
  { immediate: true },
)

// ---------- 行操作 ----------

function open(table: TableSummary) {
  void router.push({ name: 'workspace', params: { id: table.id } })
}

function actionsFor(table: TableSummary): DropdownOption[] {
  return [
    { key: 'open', label: '打开' },
    { key: 'duplicate', label: '复制为新表' },
    { key: 'rename', label: '重命名' },
    { key: 'delete', label: '删除' },
  ]
}

function onAction(key: string, table: TableSummary) {
  switch (key) {
    case 'open':
      open(table)
      break
    case 'duplicate':
      duplicate(table)
      break
    case 'rename':
      rename(table)
      break
    case 'delete':
      remove(table)
      break
  }
}

async function duplicate(table: TableSummary) {
  try {
    const res = await spec.duplicateTable(table.id, `${table.title} 副本`)
    message.success(`已复制为「${res.table.title}」`)
  } catch (err) {
    message.error(describe(err))
  }
}

async function rename(table: TableSummary) {
  let next = table.title
  dialog.create({
    title: '重命名参数表',
    content: () =>
      h(NInput, {
        defaultValue: table.title,
        maxlength: 120,
        autofocus: true,
        placeholder: '产品名称',
        'onUpdate:value': (value: string) => {
          next = value
        },
        onKeydown: (e: KeyboardEvent) => {
          if (e.key === 'Enter') e.preventDefault()
        },
      }),
    positiveText: '保存',
    negativeText: '取消',
    onPositiveClick: async () => {
      const title = next.trim()
      if (!title || title === table.title) return
      try {
        await spec.renameTable(table.id, title)
        message.success('已重命名')
      } catch (err) {
        message.error(describe(err))
      }
    },
  })
}

async function remove(table: TableSummary) {
  try {
    await spec.removeTable(table.id)
    message.success('已删除')
  } catch (err) {
    message.error(describe(err))
  }
}

// ---------- 展示 ----------

function describe(err: unknown) {
  return err instanceof ApiError ? err.message : '操作失败，请稍后重试'
}
</script>

<template>
  <div class="shell">
    <AppSidebar />

    <main class="main">
      <header class="topbar">
        <h2>我的参数表</h2>
        <n-button size="small" type="primary" @click="openCreate">
          <template #icon><i class="ri-add-line" /></template>
          新建参数表
        </n-button>
      </header>

      <div class="content ds-scroll-y">
        <!-- 空态 -->
        <div v-if="spec.tables.length === 0 && !spec.listLoading" class="empty">
          <div class="empty-mark">
            <i class="ri-file-list-3-line" />
          </div>
          <h3>{{ greeting }}</h3>
          <p>
            每张参数表对应一个产品。<br />
            你可以手工维护参数，也可以直接让 AI 帮你建表、改值和调整分组。
          </p>
          <n-button type="primary" @click="openCreate">
            <template #icon><i class="ri-add-line" /></template>
            新建第一张参数表
          </n-button>
        </div>

        <!-- 列表 -->
        <div v-else class="grid">
          <article
            v-for="table in spec.tables"
            :key="table.id"
            class="card"
            @click="open(table)"
          >
            <div class="card-head">
              <h3 class="ds-ellipsis">{{ table.title }}</h3>
              <n-popconfirm @positive-click="remove(table)">
                <template #trigger>
                  <n-dropdown
                    :options="actionsFor(table)"
                    trigger="click"
                    placement="bottom-end"
                    @select="(key: string) => onAction(key, table)"
                  >
                    <button class="more-btn" type="button" @click.stop>
                      <i class="ri-more-2-fill" />
                    </button>
                  </n-dropdown>
                </template>
                确定删除「{{ table.title }}」？
              </n-popconfirm>
            </div>

            <p v-if="table.description" class="card-desc ds-ellipsis">{{ table.description }}</p>
            <p v-else class="card-desc placeholder">暂无描述</p>

            <div class="card-foot">
              <span v-if="table.industry" class="tag">{{ table.industry }}</span>
              <span class="stat">{{ table.item_count }} 个参数</span>
              <span class="dot">·</span>
              <span class="stat">r{{ table.current_revision }}</span>
              <span class="time">{{ formatRelativeTime(table.updated_at) }}</span>
            </div>
          </article>
        </div>
      </div>
    </main>

    <!-- 新建参数表 -->
    <n-modal v-model:show="showCreate" :mask-closable="!creating">
      <div class="modal-card">
        <h3>新建参数表</h3>
        <p class="modal-hint">一张表对应一个产品，创建后可以在左侧对话里让 AI 帮你填充参数。</p>

        <n-form ref="formRef" :model="form" :rules="rules" label-placement="top" @keydown.enter.prevent>
          <n-form-item label="产品名称" path="title">
            <n-input v-model:value="form.title" placeholder="例如：某型号蓝牙耳机" maxlength="120" autofocus />
          </n-form-item>
          <n-form-item label="简介">
            <n-input
              v-model:value="form.description"
              type="textarea"
              placeholder="选填"
              :rows="2"
              maxlength="2000"
            />
          </n-form-item>
          <n-form-item label="行业">
            <n-input v-model:value="form.industry" placeholder="选填，例如：消费电子" maxlength="64" />
          </n-form-item>
        </n-form>

        <div class="modal-actions">
          <n-button quaternary :disabled="creating" @click="showCreate = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="submitCreate">创建</n-button>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100%;
  overflow: hidden;
  background: var(--ds-bg);
}

.main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 20px;
  height: 56px;
  flex: none;
  border-bottom: 1px solid var(--ds-border);
  background: var(--ds-surface);
}

.topbar h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.content {
  flex: 1;
  min-height: 0;
  padding: 20px;
}

/* 空态 */

.empty {
  max-width: 440px;
  margin: 8vh auto 0;
  text-align: center;
}

.empty-mark {
  width: 60px;
  height: 60px;
  margin: 0 auto 18px;
  display: grid;
  place-items: center;
  border-radius: 18px;
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
  font-size: 26px;
}

.empty h3 {
  margin: 0 0 8px;
  font-size: 17px;
  font-weight: 600;
  color: var(--ds-text);
}

.empty p {
  margin: 0 0 22px;
  font-size: 13.5px;
  line-height: 1.85;
  color: var(--ds-text-secondary);
}

/* 卡片网格 */

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(258px, 1fr));
  gap: 14px;
}

.card {
  display: flex;
  flex-direction: column;
  padding: 15px 16px;
  border: 1px solid var(--ds-border);
  border-radius: var(--ds-radius-lg);
  background: var(--ds-surface);
  cursor: pointer;
  transition: all 0.16s ease;
}

.card:hover {
  border-color: var(--ds-primary);
  box-shadow: 0 6px 20px rgba(18, 22, 34, 0.07);
}

.card-head {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.card-head h3 {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--ds-text);
}

.more-btn {
  width: 24px;
  height: 24px;
  flex: none;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--ds-text-tertiary);
  cursor: pointer;
  opacity: 0;
}

.card:hover .more-btn {
  opacity: 1;
}

.more-btn:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}

.card-desc {
  margin: 6px 0 14px;
  font-size: 12.5px;
  color: var(--ds-text-secondary);
  min-height: 19px;
}

.card-desc.placeholder {
  color: var(--ds-text-tertiary);
}

.card-foot {
  margin-top: auto;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  color: var(--ds-text-tertiary);
  font-variant-numeric: tabular-nums;
}

.tag {
  padding: 1px 7px;
  border-radius: 20px;
  background: var(--ds-primary-soft);
  color: var(--ds-primary);
  font-size: 11px;
}

.stat {
  font-variant-numeric: tabular-nums;
}

.dot {
  color: var(--ds-border-strong);
}

.time {
  margin-left: auto;
}

/* 弹窗 */

.modal-card {
  width: min(440px, calc(100vw - 32px));
  padding: 22px 24px;
  border-radius: 16px;
  background: var(--ds-surface);
  border: 1px solid var(--ds-border);
}

.modal-card h3 {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
  color: var(--ds-text);
}

.modal-hint {
  margin: 0 0 18px;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--ds-text-tertiary);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
