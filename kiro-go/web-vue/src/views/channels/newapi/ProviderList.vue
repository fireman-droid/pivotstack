<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NDataTable, NInput, NSelect, NSpace, NPopconfirm, useMessage, type DataTableColumns, type DataTableRowKey } from 'naive-ui'
import { Plus, RefreshCw, Edit, Trash2, Search } from 'lucide-vue-next'
import PageContainer from '../../../components/common/PageContainer.vue'
import PageHeader from '../../../components/common/PageHeader.vue'
import Toolbar from '../../../components/common/Toolbar.vue'
import StatusBadge from '../../../components/common/StatusBadge.vue'
import MonoValue from '../../../components/common/MonoValue.vue'
import EmptyState from '../../../components/common/EmptyState.vue'
import BatchActionBar from '../../../components/common/BatchActionBar.vue'
import ProviderDrawer from '../../../components/admin/newapi/ProviderDrawer.vue'
import { listProviders, syncProvider, deleteProvider, type NewAPIProvider } from '../../../api/admin/providers'
import { useTablePagination } from '../../../composables/useTablePagination'
import { useRowClickToggle } from '../../../composables/useRowClickToggle'

const message = useMessage()
const router = useRouter()
const pagination = useTablePagination(20)
const loading = ref(false)
const rows = ref<NewAPIProvider[]>([])
const search = ref('')
const status = ref('all')
const drawerShow = ref(false)
const drawerRow = ref<NewAPIProvider | null>(null)
const checkedRowKeys = ref<DataTableRowKey[]>([])
const batchRunning = ref(false)
const selectedRows = computed(() => rows.value.filter(r => checkedRowKeys.value.includes(r.id)))
const rowProps = useRowClickToggle<NewAPIProvider>(checkedRowKeys, r => r.id)

async function runBatch<T>(items: T[], op: (r: T) => Promise<unknown>, label: string) {
  if (!items.length) { message.info(`${label}：无需处理的目标`); return }
  batchRunning.value = true
  const r = await Promise.allSettled(items.map(op))
  const ok = r.filter(x => x.status === 'fulfilled').length
  const fail = r.length - ok
  batchRunning.value = false
  if (fail === 0) message.success(`${label}：${ok} 条成功`)
  else message.warning(`${label}：${ok} 成功 / ${fail} 失败`)
  await reload()
  checkedRowKeys.value = []
}
const bulkSync = () => runBatch(selectedRows.value, r => syncProvider(r.id), '批量同步')
const bulkDelete = () => runBatch(selectedRows.value, r => deleteProvider(r.id), '批量删除')

const statusOptions = [
  { label: '全部状态', value: 'all' },
  { label: '启用', value: 'enabled' },
  { label: '禁用', value: 'disabled' },
  { label: '同步异常', value: 'error' },
]

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(row => {
    if (status.value === 'enabled' && !row.enabled) return false
    if (status.value === 'disabled' && row.enabled) return false
    if (status.value === 'error' && !row.lastSyncError) return false
    if (!q) return true
    return [row.name, row.id, row.baseUrl].some(v => String(v || '').toLowerCase().includes(q))
  })
})

const columns: DataTableColumns<NewAPIProvider> = [
  { type: 'selection' },
  {
    title: '名称',
    key: 'name',
    width: 220,
    render: row => h('div', { class: 'name-cell' }, [
      h('span', { class: 'name-main' }, row.name || row.id),
      h('span', { class: 'name-sub' }, row.id),
    ]),
  },
  { title: 'baseURL', key: 'baseUrl', width: 320, ellipsis: { tooltip: true }, render: row => h(MonoValue, { value: row.baseUrl || '-' }) },
  { title: '物化 token 数', key: 'tokenCount', width: 130, align: 'center', render: row => h('span', { class: 'mono' }, String(row.channelCount ?? row.tokenCount ?? 0)) },
  { title: '用户 ID', key: 'userId', width: 110, align: 'center', render: row => h(MonoValue, { value: String(row.userId ?? '-') }) },
  {
    title: '同步状态',
    key: 'status',
    width: 110,
    align: 'center',
    render: row => h(StatusBadge, {
      status: row.lastSyncError ? 'error' : row.enabled ? 'enabled' : 'disabled',
      label: row.lastSyncError ? '异常' : row.enabled ? '正常' : '禁用',
    }),
  },
  { title: '最近同步时间', key: 'lastSyncAt', width: 170, align: 'center', render: row => h('span', { class: 'mono' }, formatTime(row.lastSyncAt)) },
  {
    title: '操作',
    key: 'actions',
    width: 240,
    align: 'center',
    render: row => h(NSpace, { size: 4, justify: 'center' }, () => [
      h(NButton, { size: 'tiny', quaternary: true, onClick: (e: MouseEvent) => stop(e, () => router.push({ name: 'ChannelsNewAPIDetail', params: { id: row.id } })) },
        { default: () => '详情' }),
      h(NButton, { size: 'tiny', quaternary: true, onClick: (e: MouseEvent) => runSync(e, row) }, { default: () => '同步' }),
      h(NButton, { size: 'tiny', quaternary: true, onClick: (e: MouseEvent) => stop(e, () => openEdit(row)) },
        { default: () => '编辑', icon: () => h(Edit, { size: 13 }) }),
      h(NPopconfirm, {
        onPositiveClick: () => doDelete(row),
        positiveText: '删除',
        negativeText: '取消',
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { icon: () => h(Trash2, { size: 13 }) }),
        default: () => `删除上游「${row.name || row.id}」？所有已物化渠道会被一并清理（软删）。`,
      }),
    ]),
  },
]

function openCreate() {
  drawerRow.value = null
  drawerShow.value = true
}
function openEdit(row: NewAPIProvider) {
  drawerRow.value = row
  drawerShow.value = true
}
async function doDelete(row: NewAPIProvider) {
  try {
    await deleteProvider(row.id)
    message.success('已删除')
    reload()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

function formatTime(ts?: number) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false })
}
function stop(e: MouseEvent, fn: () => void) {
  e.stopPropagation()
  fn()
}
async function reload() {
  loading.value = true
  try {
    rows.value = await listProviders()
  } catch (e: any) {
    message.error(e?.message || '加载上游失败')
  } finally {
    loading.value = false
  }
}
async function runSync(e: MouseEvent, row: NewAPIProvider) {
  e.stopPropagation()
  await syncProvider(row.id)
  message.success('已触发同步')
  reload()
}
onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="渠道 · NewAPI"
      :kicker-dot="'#707070'"
      title="NewAPI 上游"
      desc="管理 new-api 兼容的上游网关 + 物化渠道"
    >
      <template #actions>
        <n-button type="primary" size="small" @click="openCreate">
          <template #icon><Plus :size="14" /></template>
          接入新上游
        </n-button>
      </template>
    </PageHeader>

    <ProviderDrawer v-model:show="drawerShow" :row="drawerRow" @saved="reload" />

    <Toolbar>
      <template #left>
        <n-input v-model:value="search" clearable size="small" placeholder="搜索名称 / ID / baseURL" style="width: 280px">
          <template #prefix><Search :size="14" /></template>
        </n-input>
        <n-select v-model:value="status" :options="statusOptions" size="small" style="width: 140px" />
      </template>
      <template #right>
        <n-button size="small" :loading="loading" @click="reload">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </n-button>
      </template>
    </Toolbar>

    <BatchActionBar :count="checkedRowKeys.length" @clear="checkedRowKeys = []">
      <n-button size="small" :loading="batchRunning" @click="bulkSync">
        <template #icon><RefreshCw :size="13" /></template>批量同步
      </n-button>
      <n-popconfirm @positive-click="bulkDelete" positive-text="删除" negative-text="取消">
        <template #trigger>
          <n-button size="small" type="error" :loading="batchRunning">
            <template #icon><Trash2 :size="13" /></template>批量删除
          </n-button>
        </template>
        删除选中的 {{ checkedRowKeys.length }} 个上游？所有物化渠道会被一并软删。
      </n-popconfirm>
    </BatchActionBar>

    <n-data-table
      v-if="filtered.length || loading"
      v-model:checked-row-keys="checkedRowKeys"
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="row => row.id"
      :row-props="rowProps"
      :pagination="pagination"
      :scroll-x="1350"
      size="small"
      striped
    />
    <EmptyState v-else icon="○" title="还没有上游" desc="接入一个 new-api 兼容网关后会显示在这里" />
  </PageContainer>
</template>

<style scoped>
.name-cell { display: flex; flex-direction: column; gap: 3px; }
.name-main { color: #ededed; font-weight: 500; }
.name-sub { color: #707070; font-family: "Geist Mono", ui-monospace, monospace; font-size: 12px; }
:deep(.n-data-table-tr) { cursor: pointer; }
</style>
