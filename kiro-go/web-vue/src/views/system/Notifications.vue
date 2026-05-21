<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  NButton, NDataTable, NSelect, NInput, NTag, NPopconfirm, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { Plus, Search, RefreshCw, Pencil, Trash2 } from 'lucide-vue-next'
import {
  adminListNotifications, adminDeleteNotification,
  type AdminItem, type AdminNotification,
} from '../../api/notifications'
import NotificationFormDrawer from '../../components/admin/notif/NotificationFormDrawer.vue'

const message = useMessage()

const rows = ref<AdminItem[]>([])
const loading = ref(false)
const total = ref(0)
const search = ref('')
const status = ref('all')

const drawerShow = ref(false)
const editing = ref<AdminNotification | null>(null)

const statusOptions = [
  { label: '全部', value: 'all' },
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
  { label: '已过期', value: 'expired' },
]

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(it =>
    it.notification.title.toLowerCase().includes(q) ||
    it.notification.body.toLowerCase().includes(q),
  )
})

async function reload() {
  loading.value = true
  try {
    const res = await adminListNotifications(status.value, 200, 0)
    rows.value = res.items
    total.value = res.total
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function onCreate() {
  editing.value = null
  drawerShow.value = true
}
function onEdit(item: AdminItem) {
  editing.value = item.notification
  drawerShow.value = true
}
async function onDelete(item: AdminItem) {
  try {
    await adminDeleteNotification(item.notification.id)
    message.success('已删除')
    reload()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

function onSaved() {
  reload()
}

function statusChipColor(s: string, expired: boolean): { text: string; cls: string } {
  if (expired) return { text: 'EXPIRED', cls: 'chip--expired' }
  if (s === 'draft') return { text: 'DRAFT', cls: 'chip--draft' }
  return { text: 'PUBLISHED', cls: 'chip--published' }
}

function fmtTime(ts?: number) {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false })
}

function targetLabel(n: AdminNotification): string {
  switch (n.targetType) {
    case 'all': return 'all'
    case 'plan': return 'plan: ' + (n.targetValue || []).join(', ')
    case 'group': return 'group: ' + (n.targetValue || []).join(', ')
    case 'userIds': return `user × ${(n.targetValue || []).length}`
  }
  return '—'
}

function readPercent(it: AdminItem) {
  if (!it.stats.targetCount) return '—'
  return `${it.stats.readCount}/${it.stats.targetCount}`
}

const columns: DataTableColumns<AdminItem> = [
  {
    title: '标题',
    key: 'title',
    width: 380,
    render: it => h('div', { class: 'cell-title' }, [
      h('div', { class: 'cell-title__main' }, it.notification.title),
      h('div', { class: 'cell-title__sub' }, it.notification.body.replace(/[*_`#>]/g, '').slice(0, 60)),
    ]),
  },
  {
    title: '目标',
    key: 'target',
    width: 200,
    align: 'center',
    ellipsis: { tooltip: true },
    render: it => h('span', { class: 'cell-mono' }, targetLabel(it.notification)),
  },
  {
    title: '级别',
    key: 'level',
    width: 100,
    align: 'center',
    render: it => h(NTag, {
      size: 'small',
      type: it.notification.level === 'critical' ? 'error' :
            it.notification.level === 'warn' ? 'warning' : 'info',
      bordered: false,
    }, () => it.notification.level.toUpperCase()),
  },
  {
    title: '状态',
    key: 'status',
    width: 110,
    align: 'center',
    render: it => {
      const now = Math.floor(Date.now() / 1000)
      const expired = !!it.notification.expireAt && now >= it.notification.expireAt
      const s = statusChipColor(it.notification.status, expired)
      return h('span', { class: 'sbadge ' + s.cls }, s.text)
    },
  },
  {
    title: '发布时间',
    key: 'publishAt',
    width: 170,
    align: 'center',
    render: it => h('span', { class: 'cell-mono cell-mono--dim' }, fmtTime(it.notification.publishAt)),
  },
  {
    title: '已读率',
    key: 'readRate',
    width: 110,
    align: 'center',
    render: it => h('span', { class: 'cell-mono' }, readPercent(it)),
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    align: 'center',
    render: it => h('div', { class: 'cell-actions' }, [
      h(NButton, { quaternary: true, size: 'tiny', onClick: () => onEdit(it) },
        { default: () => h(Pencil, { size: 14 }) }),
      h(NPopconfirm, {
        onPositiveClick: () => onDelete(it),
      }, {
        default: () => '确认删除？',
        trigger: () => h(NButton, { quaternary: true, size: 'tiny' },
          { default: () => h(Trash2, { size: 14 }) }),
      }),
    ]),
  },
]

const summary = computed(() => {
  const all = rows.value
  const published = all.filter(it => it.notification.status === 'published' && !it.notification.expireAt).length
  const drafts = all.filter(it => it.notification.status === 'draft').length
  const now = Math.floor(Date.now() / 1000)
  const expired = all.filter(it => !!it.notification.expireAt && now >= it.notification.expireAt).length
  return { total: all.length, published, drafts, expired }
})

onMounted(reload)
</script>

<template>
  <div class="page">
    <header class="page__head">
      <div>
        <div class="page__crumb"><b>SYSTEM</b> / 通知管理</div>
        <h1 class="page__title">用户通知</h1>
        <div class="page__sub">
          共 {{ summary.total }} 条 · {{ summary.published }} 已发布 · {{ summary.drafts }} 草稿 · {{ summary.expired }} 已过期
        </div>
      </div>
      <div class="page__actions">
        <NButton size="small" @click="reload">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </NButton>
        <NButton type="primary" size="small" @click="onCreate">
          <template #icon><Plus :size="14" /></template>
          新建通知
        </NButton>
      </div>
    </header>

    <div class="filter-bar">
      <div class="filter-bar__left">
        <NInput
          v-model:value="search"
          placeholder="搜索标题或正文"
          size="small"
          style="max-width:280px"
          clearable
        >
          <template #prefix><Search :size="14" /></template>
        </NInput>
        <NSelect
          v-model:value="status"
          :options="statusOptions"
          size="small"
          style="width:140px"
          @update:value="reload"
        />
      </div>
    </div>

    <NDataTable
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :scroll-x="1190"
      :bordered="false"
      :single-line="false"
      size="small"
      :row-key="(row: AdminItem) => row.notification.id"
      class="ntable"
    />

    <NotificationFormDrawer
      v-model:show="drawerShow"
      :editing="editing"
      @saved="onSaved"
    />
  </div>
</template>

<style scoped>
.page { padding: 24px 32px; background: #000; min-height: 100vh; color: #ededed; }
.page__head {
  display: flex; justify-content: space-between; align-items: flex-end;
  margin-bottom: 24px; padding-bottom: 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.page__crumb { font-size: 11px; letter-spacing: 0.06em; color: #707070; text-transform: uppercase; }
.page__crumb b { color: #ededed; font-weight: 600; }
.page__title { font-size: 18px; font-weight: 600; letter-spacing: -0.01em; margin: 4px 0; color: #ededed; }
.page__sub { font-size: 12px; color: #707070; }
.page__actions { display: flex; gap: 8px; }

.filter-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.filter-bar__left { display: flex; gap: 8px; align-items: center; }

:deep(.cell-title) { min-width: 0; max-width: 100%; overflow: hidden; }
:deep(.cell-title__main),
:deep(.cell-title__sub) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.cell-title__main) { font-size: 13px; color: #ededed; font-weight: 500; }
:deep(.cell-title__sub) { font-size: 11px; color: #707070; margin-top: 2px; }
:deep(.cell-mono) {
  font-family: "Geist Mono", ui-monospace, monospace;
  font-size: 12px; font-variant-numeric: tabular-nums;
}
:deep(.cell-mono--dim) { color: #707070; }
:deep(.cell-actions) { display: inline-flex; gap: 4px; }

:deep(.sbadge) {
  display: inline-block; font-size: 10px; font-weight: 700;
  letter-spacing: 0.08em; padding: 2px 6px; border-radius: 3px;
}
:deep(.chip--draft) { background: rgba(255, 255, 255, 0.06); color: #a1a1a1; }
:deep(.chip--published) { background: rgba(11, 212, 112, 0.12); color: #0bd470; }
:deep(.chip--expired) { background: rgba(255, 255, 255, 0.04); color: #4d4d4d; }
</style>
