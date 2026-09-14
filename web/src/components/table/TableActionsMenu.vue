<script setup lang="ts">
import { computed, ref } from 'vue'
import { toast } from 'vue-sonner'

import * as shareApi from '@/api/share'
import { ApiError } from '@/api/http'
import { useSpecTableStore } from '@/stores/specTable'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import DeleteTableDialog from './DeleteTableDialog.vue'
import EditDescriptionDialog from './EditDescriptionDialog.vue'
import ShareDialog from './ShareDialog.vue'
import TableNameDialog from './TableNameDialog.vue'

/**
 * 参数表操作菜单：列表页与详情页共用。
 *
 * 只做「这张表」的操作，不负责导航 —— 复制后跳转、删除后回列表都由调用方决定，
 * 因为两个页面的去向不同（列表页跳去副本，详情页删除后要离开当前页）。
 */
const props = withDefaults(
  defineProps<{
    /** `TableSummary` 与 `TableDetail` 的公共字段，两者都满足 */
    table: {
      id: string
      title: string
      description: string
      pinned: boolean
      visibility: 'private' | 'link'
    }
    align?: 'start' | 'end'
    /** 列表页需要「打开」；详情页本身已打开，不显示 */
    showOpen?: boolean
    /**
     * 是否已开启分享。详情页传权威状态（标题旁的圆点也需要它）；
     * 列表页不传，回退用 visibility —— 后端保证两者一致。
     */
    shareEnabled?: boolean
  }>(),
  { align: 'start', showOpen: false, shareEnabled: undefined },
)

const emit = defineEmits<{
  open: []
  duplicated: [id: string]
  deleted: []
  /** 分享状态变化，调用方据此刷新圆点/列表 */
  'share-changed': []
}>()

const spec = useSpecTableStore()

const enabled = computed(() =>
  props.shareEnabled === undefined ? props.table.visibility === 'link' : props.shareEnabled,
)

/** 公开链接是确定性的 /s/<表id>，直接拼即可，不必为每张卡请求一次分享状态 */
const shareUrl = computed(() => `/s/${props.table.id}`)

// ---------- 置顶 / 分享开关 ----------

const pinBusy = ref(false)
const shareBusy = ref(false)

async function togglePin() {
  if (pinBusy.value) return
  pinBusy.value = true
  try {
    await spec.setPinned(props.table.id, !props.table.pinned)
    toast.success(props.table.pinned ? '已取消置顶' : '已置顶')
  } catch (err) {
    toast.error(describe(err, '操作失败'))
  } finally {
    pinBusy.value = false
  }
}

async function toggleShare() {
  if (shareBusy.value) return
  shareBusy.value = true
  try {
    if (enabled.value) {
      await shareApi.disableShare(props.table.id)
      toast.success('已关闭分享，原链接立即失效')
    } else {
      await shareApi.enableShare(props.table.id)
      toast.success('已开启分享')
    }
    emit('share-changed')
  } catch (err) {
    toast.error(describe(err, '操作失败'))
  } finally {
    shareBusy.value = false
  }
}

/** 预览公开页；未开启分享时菜单项置灰，因此链接必然有效 */
function previewShare() {
  if (!enabled.value) return
  window.open(shareUrl.value, '_blank', 'noopener')
}

async function copyShareLink() {
  if (!enabled.value) return
  try {
    // 后端返回的是相对路径，补成完整地址再复制
    await navigator.clipboard.writeText(`${window.location.origin}${shareUrl.value}`)
    toast.success('链接已复制')
  } catch {
    toast.warning('复制失败，请手动复制')
  }
}

// ---------- 弹窗编排 ----------

const showShare = ref(false)

const nameMode = ref<'rename' | 'duplicate' | null>(null)
const nameInitial = ref('')
const nameSubmitting = ref(false)

/** 弹窗开合与 nameMode 是同一份状态，写回时关闭即清空目标 */
const nameOpen = computed({
  get: () => nameMode.value !== null,
  set: (value) => {
    if (!value) nameMode.value = null
  },
})

function openRename() {
  nameInitial.value = props.table.title
  nameMode.value = 'rename'
}

function openDuplicate() {
  // 默认带「副本」，用户多半会改成目标产品的名字
  nameInitial.value = `${props.table.title} 副本`
  nameMode.value = 'duplicate'
}

async function submitName(next: string) {
  const mode = nameMode.value
  if (!mode || nameSubmitting.value) return

  nameSubmitting.value = true
  try {
    if (mode === 'duplicate') {
      const res = await spec.duplicateTable(props.table.id, next)
      toast.success(`已复制为「${res.table.title}」`)
      nameMode.value = null
      emit('duplicated', res.table.id)
      return
    }

    // 名字没变就不用发请求
    if (next !== props.table.title) {
      await spec.renameTable(props.table.id, next)
      toast.success('已重命名')
    }
    nameMode.value = null
  } catch (err) {
    toast.error(describe(err, mode === 'duplicate' ? '复制失败' : '重命名失败'))
  } finally {
    nameSubmitting.value = false
  }
}

const showDelete = ref(false)
const deleteSubmitting = ref(false)

async function confirmDelete() {
  if (deleteSubmitting.value) return
  deleteSubmitting.value = true
  try {
    await spec.removeTable(props.table.id)
    toast.success(`已删除「${props.table.title}」`)
    showDelete.value = false
    emit('deleted')
  } catch (err) {
    toast.error(describe(err, '删除失败'))
  } finally {
    deleteSubmitting.value = false
  }
}

// ---------- 编辑描述 ----------

const showDescription = ref(false)
const descriptionSubmitting = ref(false)

async function submitDescription(next: string) {
  if (descriptionSubmitting.value) return
  // 描述可以留空（清空），没变化就不用发请求
  if (next === props.table.description) {
    showDescription.value = false
    return
  }

  descriptionSubmitting.value = true
  try {
    await spec.updateMeta(props.table.id, { description: next })
    toast.success('已更新描述')
    showDescription.value = false
  } catch (err) {
    toast.error(describe(err, '更新失败'))
  } finally {
    descriptionSubmitting.value = false
  }
}

function onShareChanged() {
  emit('share-changed')
}

function describe(err: unknown, fallback: string) {
  return err instanceof ApiError ? err.message : fallback
}
</script>

<template>
  <div class="table-actions">
    <DropdownMenu>
      <DropdownMenuTrigger as-child>
        <!-- 触发器交给调用方：它来决定用哪个按钮、要不要 @click.stop -->
        <slot name="trigger">
          <button type="button" class="default-trigger" aria-label="参数表操作" title="参数表操作">
            <i class="ri-more-line" />
          </button>
        </slot>
      </DropdownMenuTrigger>

      <DropdownMenuContent :align="align" class="min-w-[150px]">
        <DropdownMenuItem @select="togglePin">
          <i :class="table.pinned ? 'ri-unpin-line' : 'ri-pushpin-line'" />
          {{ table.pinned ? '取消置顶' : '置顶' }}
        </DropdownMenuItem>

        <DropdownMenuItem v-if="showOpen" @select="emit('open')">
          <i class="ri-arrow-right-up-line" />
          打开
        </DropdownMenuItem>

        <DropdownMenuItem @select="openDuplicate">
          <i class="ri-file-copy-line" />
          创建副本
        </DropdownMenuItem>

        <DropdownMenuItem @select="openRename">
          <i class="ri-edit-line" />
          重命名
        </DropdownMenuItem>

        <DropdownMenuItem @select="showDescription = true">
          <i class="ri-text-block" />
          {{ table.description ? '编辑描述' : '添加描述' }}
        </DropdownMenuItem>

        <DropdownMenuItem @select="showShare = true">
          <i class="ri-share-forward-line" />
          分享
        </DropdownMenuItem>

        <!-- 快速开关，不必进分享弹窗 -->
        <DropdownMenuItem :disabled="shareBusy" @select="toggleShare">
          <i :class="enabled ? 'ri-link-unlink-m' : 'ri-link'" />
          {{ enabled ? '关闭分享' : '开启分享' }}
        </DropdownMenuItem>

        <!-- 未开启分享时没有公开地址，预览与复制链接都置灰 -->
        <DropdownMenuItem :disabled="!enabled" @select="previewShare">
          <i class="ri-external-link-line" />
          预览
        </DropdownMenuItem>

        <DropdownMenuItem :disabled="!enabled" @select="copyShareLink">
          <i class="ri-link-m" />
          复制链接
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem variant="destructive" @select="showDelete = true">
          <i class="ri-delete-bin-line" />
          删除
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>

    <TableNameDialog
      v-model:open="nameOpen"
      :mode="nameMode ?? 'rename'"
      :initial-value="nameInitial"
      :submitting="nameSubmitting"
      @submit="submitName"
    />

    <DeleteTableDialog
      v-model:open="showDelete"
      :title="table.title"
      :submitting="deleteSubmitting"
      @confirm="confirmDelete"
    />

    <EditDescriptionDialog
      v-model:open="showDescription"
      :initial-value="table.description"
      :submitting="descriptionSubmitting"
      @submit="submitDescription"
    />

    <ShareDialog
      v-model:show="showShare"
      :table-id="table.id"
      :table-title="table.title"
      @changed="onShareChanged"
    />
  </div>
</template>

<style scoped>
/* 默认触发器：详情页直接用，尺寸与 chat-head 的 head-btn 一致 */
.default-trigger {
  flex: none;
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 18px;
  cursor: pointer;
}

.default-trigger:hover {
  background: var(--ds-surface-muted);
  color: var(--ds-text);
}
</style>
