<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { NInput, NSelect, NDataTable, NButton, useMessage, type DataTableColumns } from 'naive-ui'
import { Search, RefreshCw } from 'lucide-vue-next'
import { listLogs, type CallLog } from '../../api/admin/logs'
import { listApiKeys } from '../../api/admin/keys'
import { useTablePagination } from '../../composables/useTablePagination'
import CallLogDrawer from '../../components/admin/ops/CallLogDrawer.vue'
import { fmtCost as fmtCostShared } from '../../utils/format'

const message = useMessage()
const pagination = useTablePagination(50)
const loading = ref(false)
const rows = ref<CallLog[]>([])
const total = ref(0)
const search = ref('')
const statusFilter = ref<'all' | 'success' | 'error'>('all')
const active = ref<CallLog | null>(null)
const keyNoteMap = ref<Record<string, string>>({})

const statusOptions = [
  { label: '全部状态', value: 'all' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'error' },
]

function logStatus(row: CallLog) {
  return row.error || row.status === 'error' ? 'error' : 'success'
}
function fmtTime(row: CallLog) {
  if (row.timestamp) {
    const d = new Date(row.timestamp * 1000)
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
  }
  return row.time?.slice(-8) || '--:--:--'
}
function fmtDuration(ms?: number | null) {
  if (ms == null || ms === 0) return '-'
  const s = ms / 1000
  if (s < 1) return `${ms}ms`
  return `${s.toFixed(1)}s`
}
// 复用全局 fmtCost：自适应精度（≥0.01 用 2 位，<0.01 用 4 位，<0.0001 用 6 位 trim）。
function fmtCost(v?: number) {
  if (v == null) return '-'
  return fmtCostShared(v)
}
function keyDisplay(row: CallLog) {
  if (!row.api_key_id) return '-'
  return keyNoteMap.value[row.api_key_id] || (row as any).api_key_note || row.api_key_id.slice(0, 10)
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(row => {
    if (statusFilter.value !== 'all' && logStatus(row) !== statusFilter.value) return false
    if (!q) return true
    return [row.request_id, keyDisplay(row), row.api_key_id, row.channel_alias, row.channel_id, row.original_model, row.error]
      .some(v => String(v || '').toLowerCase().includes(q))
  })
})

const columns: DataTableColumns<CallLog> = [
  {
    title: '时间',
    key: 'time',
    width: 100,
    align: 'center',
    render: row => h('span', { class: 'cl-mono cl-dim' }, fmtTime(row)),
  },
  {
    title: 'Key',
    key: 'key',
    width: 220,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'cl-key' }, keyDisplay(row)),
  },
  {
    title: '渠道',
    key: 'channel',
    width: 160,
    ellipsis: { tooltip: true },
    render: row => h('span', null, row.channel_alias || row.channel_id || '-'),
  },
  {
    title: '模型',
    key: 'model',
    width: 200,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'cl-chip' }, row.original_model || row.actual_model || '-'),
  },
  {
    title: 'Tokens',
    key: 'tokens',
    width: 180,
    align: 'center',
    render: row => h('span', { class: 'cl-mono cl-tokens' }, [
      h('span', { class: 'cl-mono-strong' }, ((row.input_tokens || 0) + (row.output_tokens || 0)).toLocaleString()),
      h('span', { class: 'cl-dim' }, ` (${row.input_tokens || 0}↑/${row.output_tokens || 0}↓)`),
    ]),
  },
  {
    title: '耗时',
    key: 'duration',
    width: 90,
    align: 'center',
    render: row => h('span', { class: row.duration_ms && row.duration_ms > 3000 ? 'cl-mono cl-warn' : 'cl-mono' }, fmtDuration(row.duration_ms)),
  },
  {
    title: '成本',
    key: 'cost',
    width: 110,
    align: 'center',
    render: row => h('span', { class: 'cl-mono cl-cost' }, fmtCost(row.cost_usd)),
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    align: 'center',
    render: row => h('span', { class: ['cl-atag', logStatus(row) === 'success' ? 'cl-atag--ok' : 'cl-atag--err'] }, [
      h('span', { class: 'cl-atag-dot' }),
      logStatus(row) === 'success' ? '200' : 'ERR',
    ]),
  },
]

async function reload() {
  loading.value = true
  try {
    const [data, keys] = await Promise.all([
      listLogs({ search: search.value, limit: 500 }),
      listApiKeys().then(list => {
        const m: Record<string, string> = {}
        list.forEach(k => { if (k.id) m[k.id] = k.note || '' })
        return m
      }).catch(() => ({} as Record<string, string>)),
    ])
    rows.value = data.logs || []
    total.value = data.total || rows.value.length
    keyNoteMap.value = keys
  } catch (e: any) {
    message.error(e?.message || '加载日志失败')
  } finally {
    loading.value = false
  }
}

onMounted(reload)

const rowProps = (row: CallLog) => ({
  style: 'cursor: pointer',
  onClick: () => { active.value = row },
})
</script>

<template>
  <div class="admin-page">
    <header class="page-head">
      <div>
        <div class="page-head__crumb"><b>OPS</b> / 调用日志</div>
        <div class="page-head__title">
          <div class="t-display-admin">调用日志</div>
          <div class="page-head__sub">共 {{ total }} 条 · 最近 500 条 · 每行可点击看详情</div>
        </div>
      </div>
      <div class="page-head__right">
        <button class="cl-btn cl-btn--ghost" :disabled="loading" @click="reload">
          <RefreshCw :size="14" :class="{ 'is-spinning': loading }" />
          刷新
        </button>
      </div>
    </header>

    <!-- filter bar -->
    <div class="cl-filter">
      <NInput
        v-model:value="search"
        clearable
        size="small"
        placeholder="搜索 request_id / key / channel / model / error"
        style="width:360px"
        @keyup.enter="reload"
      >
        <template #prefix><Search :size="14" /></template>
      </NInput>
      <NSelect
        v-model:value="statusFilter"
        :options="statusOptions"
        size="small"
        style="width:120px"
      />
    </div>

    <!-- atable -->
    <NDataTable
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="row => row.request_id || `${row.timestamp}-${row.original_model}`"
      :row-props="rowProps"
      :pagination="pagination"
      :scroll-x="1150"
      :bordered="false"
      :single-line="false"
      size="small"
      class="cl-table"
    />

    <CallLogDrawer :log="active" @close="active = null" />
  </div>
</template>

<style scoped>
.cl-btn {
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
.cl-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); border-color: var(--st-border-strong); }
.cl-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.cl-btn--ghost { background: transparent; }
.is-spinning { animation: cl-spin 0.8s linear infinite; }
@keyframes cl-spin { to { transform: rotate(360deg); } }

.cl-filter {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 16px;
}

/* table cell styles */
:deep(.cl-mono) {
  font-family: var(--st-font-mono);
  font-variant-numeric: tabular-nums;
  font-size: 12px;
}
:deep(.cl-mono-strong) { color: var(--st-text-pri); }
:deep(.cl-dim) { color: var(--st-text-ter); }
:deep(.cl-warn) { color: var(--st-warning); }
:deep(.cl-cost) { color: var(--st-text-pri); }
:deep(.cl-key) {
  color: var(--st-text-pri);
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
:deep(.cl-tokens) { white-space: nowrap; }
:deep(.cl-chip) {
  font-family: var(--st-font-mono);
  font-size: 11px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 3px;
  color: var(--st-text-pri);
}

/* status atag */
:deep(.cl-atag) {
  display: inline-flex; align-items: center; gap: 4px;
  height: 18px; padding: 0 6px;
  border-radius: 2px;
  font-size: 10px; font-weight: 600;
  letter-spacing: 0.06em;
  font-family: var(--st-font-mono);
}
:deep(.cl-atag-dot) { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
:deep(.cl-atag--ok) { color: var(--st-success); background: rgba(11, 212, 112, 0.10); }
:deep(.cl-atag--ok .cl-atag-dot) { background: var(--st-success); }
:deep(.cl-atag--err) { color: var(--st-error); background: rgba(255, 77, 77, 0.10); }
:deep(.cl-atag--err .cl-atag-dot) { background: var(--st-error); }

/* compact NDataTable to match admin v6 atable density */
.cl-table :deep(.n-data-table-th) {
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
.cl-table :deep(.n-data-table-td) {
  height: 36px !important;
  padding: 0 12px !important;
  font-size: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.cl-table :deep(.n-data-table-tr:hover .n-data-table-td) {
  background: rgba(255, 255, 255, 0.04) !important;
}
.cl-table :deep(.n-data-table) {
  background: transparent !important;
}
</style>
