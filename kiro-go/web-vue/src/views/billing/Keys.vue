<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton, NDataTable, NInput, NSelect, NSwitch, NPopconfirm, NSpace,
  useMessage, type DataTableColumns, type DataTableRowKey,
} from 'naive-ui'
import { Plus, Search, ChevronRight, Trash2, Pencil, CheckCircle2, XCircle, Download, RefreshCw } from 'lucide-vue-next'
import CopyableText from '../../components/common/CopyableText.vue'
import BatchActionBar from '../../components/common/BatchActionBar.vue'
import KeyFormDrawer from '../../components/admin/keys/KeyFormDrawer.vue'
import KeyCreatedDialog from '../../components/admin/keys/KeyCreatedDialog.vue'
import { listApiKeys, updateApiKey, deleteApiKey, type ApiKeyRow } from '../../api/admin/keys'
import { planLabel } from '../../utils/format.js'
import { useTablePagination } from '../../composables/useTablePagination'
import { useRowClickToggle } from '../../composables/useRowClickToggle'

const message = useMessage()
const router = useRouter()
const pagination = useTablePagination(20)
const loading = ref(false)
const rows = ref<ApiKeyRow[]>([])
const search = ref('')
const plan = ref('all')
const enabled = ref('all')

const checkedRowKeys = ref<DataTableRowKey[]>([])
const batchRunning = ref(false)
const selectedRows = computed(() => rows.value.filter(r => checkedRowKeys.value.includes(r.id)))
const rowProps = useRowClickToggle<ApiKeyRow>(checkedRowKeys, r => r.id)
const togglingId = ref<string>('')

const drawerShow = ref(false)
const drawerRow = ref<ApiKeyRow | null>(null)
const createdShow = ref(false)
const createdRow = ref<ApiKeyRow | null>(null)

// ─── derived ───
const activeCount = computed(() => rows.value.filter(r => r.enabled).length)
const totalBalance = computed(() => rows.value.reduce((s, r) => s + (r.totalBalance ?? (r.balance || 0) + (r.giftBalance || 0)), 0))
const monthAgo = Date.now() / 1000 - 30 * 86400
const newThisMonth = computed(() => rows.value.filter(r => (r.createdAt ?? 0) >= monthAgo).length)

const planOptions = [
  { label: '全部套餐', value: 'all' },
  { label: '余额卡', value: 'credit' },
  { label: '天卡', value: 'timed' },
  { label: '混合', value: 'hybrid' },
]
const statusOptions = [
  { label: '全部状态', value: 'all' },
  { label: '启用', value: 'enabled' },
  { label: '禁用', value: 'disabled' },
]

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(row => {
    if (plan.value !== 'all' && row.plan !== plan.value) return false
    if (enabled.value === 'enabled' && !row.enabled) return false
    if (enabled.value === 'disabled' && row.enabled) return false
    if (!q) return true
    return [row.note, row.key, row.keyMasked, row.id].some(v => String(v || '').toLowerCase().includes(q))
  })
})

// ─── columns ───
function fmtTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}
function fmtMoney(v?: number) { return `¥${Number(v || 0).toFixed(2)}` }
function fmtCount(v?: number) { return Number(v || 0).toLocaleString() }
function relTime(ts?: number) {
  if (!ts) return '从未'
  const diff = Date.now() / 1000 - ts
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  return `${Math.floor(diff / 86400)} 天前`
}

const columns: DataTableColumns<ApiKeyRow> = [
  { type: 'selection' },
  {
    title: '备注',
    key: 'note',
    width: 280,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'k-note' }, row.note || '(无备注)'),
  },
  {
    title: 'Key',
    key: 'key',
    width: 160,
    render: row => h(CopyableText, { text: row.keyMasked || row.key || row.id, mono: true, mask: true }),
  },
  {
    title: '套餐',
    key: 'plan',
    width: 80,
    align: 'center',
    render: row => h('span', { class: 'k-chip' }, planLabel(row.plan) !== '-' ? planLabel(row.plan) : (row.tier || '余额卡')),
  },
  {
    title: '余额',
    key: 'balance',
    width: 110,
    align: 'center',
    render: row => h('span', { class: 'k-mono k-balance' }, fmtMoney(row.totalBalance ?? ((row.balance || 0) + (row.giftBalance || 0)))),
  },
  {
    title: '请求',
    key: 'requests',
    width: 90,
    align: 'center',
    render: row => h('span', { class: 'k-mono k-dim' }, fmtCount(row.requests)),
  },
  {
    title: '上次活跃',
    key: 'lastUsed',
    width: 110,
    align: 'center',
    render: row => h('span', { class: 'k-dim' }, relTime(row.lastUsed)),
  },
  {
    title: '启用',
    key: 'enabled',
    width: 70,
    align: 'center',
    render: row => h(NSwitch, {
      size: 'small',
      value: row.enabled,
      loading: row.id === togglingId.value,
      onUpdateValue: (v: boolean) => toggleEnabled(row, v),
    }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    align: 'center',
    render: row => h(NSpace, { size: 4, justify: 'center' }, () => [
      h(NButton, {
        size: 'tiny', quaternary: true,
        onClick: (e: Event) => { e.stopPropagation(); router.push({ name: 'BillingKeyDetail', params: { id: row.id } }) },
      }, { default: () => '详情', icon: () => h(ChevronRight, { size: 13 }) }),
      h(NButton, {
        size: 'tiny', quaternary: true,
        onClick: (e: Event) => { e.stopPropagation(); openEdit(row) },
      }, { default: () => '编辑', icon: () => h(Pencil, { size: 13 }) }),
      h(NPopconfirm, {
        onPositiveClick: () => doDelete(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error', onClick: (e: Event) => e.stopPropagation() }, { icon: () => h(Trash2, { size: 13 }) }),
        default: () => `永久删除「${row.note || row.id}」？余额将一并清空。`,
      }),
    ]),
  },
]

// ─── batch ops ───
async function runBatch(items: ApiKeyRow[], op: (r: ApiKeyRow) => Promise<unknown>, label: string) {
  if (!items.length) { message.info(`${label}：无需处理的目标`); return }
  batchRunning.value = true
  const results = await Promise.allSettled(items.map(op))
  const ok = results.filter(r => r.status === 'fulfilled').length
  const fail = results.length - ok
  batchRunning.value = false
  if (fail === 0) message.success(`${label}：${ok} 条成功`)
  else if (ok === 0) message.error(`${label}：全部 ${fail} 条失败`)
  else message.warning(`${label}：${ok} 成功 / ${fail} 失败`)
  await reload()
  checkedRowKeys.value = []
}
const bulkEnable = () => runBatch(selectedRows.value.filter(r => !r.enabled), r => updateApiKey(r.id, { enabled: true }), '批量启用')
const bulkDisable = () => runBatch(selectedRows.value.filter(r => r.enabled), r => updateApiKey(r.id, { enabled: false }), '批量禁用')
const bulkDelete = () => runBatch(selectedRows.value, r => deleteApiKey(r.id), '批量删除')

function exportCsv() {
  const list = selectedRows.value.length ? selectedRows.value : rows.value
  const head = ['note', 'keyMasked', 'plan', 'balance', 'giftBalance', 'enabled', 'createdAt']
  const lines = [head.join(',')]
  for (const r of list) {
    lines.push([
      JSON.stringify(r.note || ''), r.keyMasked || r.id, r.plan || r.tier || '',
      r.balance ?? 0, r.giftBalance ?? 0, r.enabled ? 1 : 0,
      r.createdAt ? new Date(r.createdAt * 1000).toISOString() : '',
    ].join(','))
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `api-keys-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
  message.success(`已导出 ${list.length} 条`)
}

async function reload() {
  loading.value = true
  try {
    rows.value = await listApiKeys()
  } catch (e: any) {
    message.error(e?.message || '加载 API Key 失败')
  } finally {
    loading.value = false
  }
}
async function toggleEnabled(row: ApiKeyRow, v: boolean) {
  togglingId.value = row.id
  try {
    await updateApiKey(row.id, { enabled: v })
    row.enabled = v
    message.success(v ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '切换失败')
  } finally {
    togglingId.value = ''
  }
}
async function doDelete(row: ApiKeyRow) {
  try {
    await deleteApiKey(row.id)
    message.success('已删除')
    reload()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}
function openCreate() { drawerRow.value = null; drawerShow.value = true }
function openEdit(row: ApiKeyRow) { drawerRow.value = row; drawerShow.value = true }
function onCreated(row: ApiKeyRow) { createdRow.value = row; createdShow.value = true; reload() }

onMounted(reload)
</script>

<template>
  <div class="admin-page">
    <header class="page-head">
      <div>
        <div class="page-head__crumb"><b>BILLING</b> / API Keys</div>
        <div class="page-head__title">
          <div class="t-display-admin">API Key 管理</div>
          <div class="page-head__sub">对外 sk- 销售 key 管理 · {{ rows.length }} 个 · {{ activeCount }} 活跃</div>
        </div>
      </div>
      <div class="page-head__right">
        <button class="k-btn k-btn--ghost" :disabled="loading" @click="reload">
          <RefreshCw :size="14" :class="{ 'is-spinning': loading }" />
          刷新
        </button>
        <button class="k-btn k-btn--ghost" @click="exportCsv">
          <Download :size="14" />
          导出 CSV
        </button>
        <button class="k-btn k-btn--primary" @click="openCreate">
          <Plus :size="14" />
          创建 Key
        </button>
      </div>
    </header>

    <!-- metric strip -->
    <section class="metric-strip">
      <div class="metric-tile">
        <div class="metric-tile__label">总 Keys</div>
        <div class="metric-tile__num">{{ rows.length }}</div>
        <div class="metric-tile__delta"><span class="t-meta">{{ activeCount }} 活跃 / {{ rows.length - activeCount }} 禁用</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">活跃率</div>
        <div class="metric-tile__num">{{ rows.length ? Math.round(activeCount / rows.length * 100) : 0 }}%</div>
        <div class="metric-tile__delta"><span class="t-meta">enabled / total</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">本月新增</div>
        <div class="metric-tile__num">{{ newThisMonth }}</div>
        <div class="metric-tile__delta"><span class="t-meta">最近 30 天</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">总余额</div>
        <div class="metric-tile__num">¥{{ totalBalance.toFixed(0) }}</div>
        <div class="metric-tile__delta"><span class="t-meta">全部 keys 合计</span></div>
      </div>
    </section>

    <!-- filter bar -->
    <div class="k-filter">
      <NInput v-model:value="search" clearable size="small" placeholder="搜索 备注 / key / id" style="width:280px">
        <template #prefix><Search :size="14" /></template>
      </NInput>
      <NSelect v-model:value="plan" :options="planOptions" size="small" style="width:130px" />
      <NSelect v-model:value="enabled" :options="statusOptions" size="small" style="width:120px" />
    </div>

    <BatchActionBar :count="checkedRowKeys.length" @clear="checkedRowKeys = []">
      <NButton size="small" :loading="batchRunning" @click="bulkEnable">
        <template #icon><CheckCircle2 :size="13" /></template>批量启用
      </NButton>
      <NButton size="small" :loading="batchRunning" @click="bulkDisable">
        <template #icon><XCircle :size="13" /></template>批量禁用
      </NButton>
      <NPopconfirm @positive-click="bulkDelete" positive-text="删除" negative-text="取消">
        <template #trigger>
          <NButton size="small" type="error" :loading="batchRunning">
            <template #icon><Trash2 :size="13" /></template>批量删除
          </NButton>
        </template>
        永久删除选中的 {{ checkedRowKeys.length }} 个 Key？余额将一并清空。
      </NPopconfirm>
    </BatchActionBar>

    <NDataTable
      v-model:checked-row-keys="checkedRowKeys"
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="row => row.id"
      :row-props="rowProps"
      :pagination="pagination"
      :scroll-x="1190"
      :bordered="false"
      :single-line="false"
      size="small"
      class="k-table"
    />

    <KeyFormDrawer
      v-model:show="drawerShow"
      :row="drawerRow"
      @created="onCreated"
      @updated="reload"
    />
    <KeyCreatedDialog v-model:show="createdShow" :row="createdRow" />
  </div>
</template>

<style scoped>
.k-btn {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--st-border);
  border-radius: 4px;
  color: var(--st-text-pri);
  font-size: 12px; font-family: inherit;
  cursor: pointer;
  transition: background 150ms ease, border-color 150ms ease;
}
.k-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); border-color: var(--st-border-strong); }
.k-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.k-btn--ghost { background: transparent; }
.k-btn--primary {
  background: var(--st-primary); color: var(--st-text-inv);
  border-color: transparent;
}
.k-btn--primary:hover:not(:disabled) {
  background: var(--st-primary-hover);
}
.is-spinning { animation: k-spin 0.8s linear infinite; }
@keyframes k-spin { to { transform: rotate(360deg); } }

.k-filter {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 16px;
}

/* table cell styles */
:deep(.k-note) { color: var(--st-text-pri); font-size: 13px; font-weight: 500; }
:deep(.k-mono) { font-family: var(--st-font-mono); font-variant-numeric: tabular-nums; font-size: 12px; }
:deep(.k-balance) { color: var(--st-success); font-weight: 500; }
:deep(.k-dim) { color: var(--st-text-ter); }
:deep(.k-chip) {
  display: inline-flex; align-items: center;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 3px;
  font-size: 11px;
  font-family: var(--st-font-mono);
  color: var(--st-text-pri);
}

/* compact NDataTable to admin v6 atable density */
.k-table :deep(.n-data-table-th) {
  font-size: 11px !important;
  font-weight: 500 !important;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--st-text-ter) !important;
  background: transparent !important;
  height: 32px !important;
  padding: 0 12px !important;
  border-bottom: 1px solid var(--st-border) !important;
}
.k-table :deep(.n-data-table-td) {
  height: 40px !important;
  padding: 0 12px !important;
  font-size: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.k-table :deep(.n-data-table-tr:hover .n-data-table-td) {
  background: rgba(255, 255, 255, 0.04) !important;
}
.k-table :deep(.n-data-table) { background: transparent !important; }
</style>
