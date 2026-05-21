<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { NButton, NDataTable, NInput, NSwitch, NTag, NPopconfirm, NSpace, useMessage, type DataTableColumns, type DataTableRowKey } from 'naive-ui'
import { Search, RefreshCw, Trash2, CheckCircle2, XCircle, Download, Plus } from 'lucide-vue-next'
import Toolbar from '../../common/Toolbar.vue'
import MonoValue from '../../common/MonoValue.vue'
import EmptyState from '../../common/EmptyState.vue'
import BatchActionBar from '../../common/BatchActionBar.vue'
import KiroAddDrawer from './KiroAddDrawer.vue'
import { listAccounts, updateAccount, deleteAccount, refreshAccount, type KiroAccount } from '../../../api/admin/accounts'
import { useTablePagination } from '../../../composables/useTablePagination'
import { useRowClickToggle } from '../../../composables/useRowClickToggle'

const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const accounts = ref<KiroAccount[]>([])
const search = ref('')
const refreshingId = ref('')
const togglingId = ref('')
const checkedRowKeys = ref<DataTableRowKey[]>([])
const batchRunning = ref(false)
const addShow = ref(false)
const rowProps = useRowClickToggle<KiroAccount>(checkedRowKeys, r => r.id)

const selectedAccounts = computed(() => accounts.value.filter(a => checkedRowKeys.value.includes(a.id)))

async function runBatch(items: KiroAccount[], op: (a: KiroAccount) => Promise<unknown>, label: string) {
  if (!items.length) return
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

async function bulkEnable() {
  await runBatch(selectedAccounts.value.filter(a => !a.enabled), a => updateAccount(a.id, { enabled: true }), '批量启用')
}
async function bulkDisable() {
  await runBatch(selectedAccounts.value.filter(a => a.enabled), a => updateAccount(a.id, { enabled: false }), '批量禁用')
}
async function bulkRefresh() {
  await runBatch(selectedAccounts.value, a => refreshAccount(a.id), '批量刷新 token')
}
async function bulkDelete() {
  await runBatch(selectedAccounts.value, a => deleteAccount(a.id), '批量删除')
}
function exportCsv() {
  const rows = (selectedAccounts.value.length ? selectedAccounts.value : accounts.value)
  const header = ['email', 'subscription', 'daysRemaining', 'usagePercent', 'enabled', 'weight', 'requests', 'tokens']
  const lines = [header.join(',')]
  for (const a of rows) {
    lines.push([
      a.email || a.id, a.subscriptionTitle || '', a.daysRemaining ?? 0,
      a.usagePercent ?? 0, a.enabled ? 1 : 0, a.weight ?? 100,
      a.requestCount ?? 0, a.totalTokens ?? 0,
    ].join(','))
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `kiro-accounts-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
  message.success(`已导出 ${rows.length} 条`)
}

async function reload() {
  loading.value = true
  try {
    accounts.value = await listAccounts()
  } catch (e: any) {
    message.error(e?.message || '加载账号池失败')
  } finally {
    loading.value = false
  }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return accounts.value
  return accounts.value.filter(a => [a.email, a.nickname, a.id, a.subscriptionTitle].some(v => String(v || '').toLowerCase().includes(q)))
})

const stats = computed(() => {
  const total = accounts.value.length
  const enabled = accounts.value.filter(a => a.enabled).length
  const pro = accounts.value.filter(a => /PRO/i.test(a.subscriptionTitle || '')).length
  return { total, enabled, pro }
})

function subscriptionTone(a: KiroAccount): 'success' | 'info' | 'warning' | 'default' {
  const t = (a.subscriptionTitle || '').toUpperCase()
  if (t.includes('PRO')) return 'success'
  if (t.includes('FREE')) return 'info'
  if (t.includes('TRIAL')) return 'warning'
  return 'default'
}

function fmtUsage(a: KiroAccount) {
  if (a.usageLimit == null) return '-'
  return `${a.usageCurrent ?? 0} / ${a.usageLimit} (${(a.usagePercent ?? 0).toFixed(0)}%)`
}

function fmtTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}

async function toggle(row: KiroAccount, v: boolean) {
  togglingId.value = row.id
  try {
    await updateAccount(row.id, { enabled: v })
    row.enabled = v
    message.success(v ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '操作失败')
  } finally {
    togglingId.value = ''
  }
}

async function doRefresh(row: KiroAccount) {
  refreshingId.value = row.id
  try {
    await refreshAccount(row.id)
    message.success('已刷新 token')
    reload()
  } catch (e: any) {
    message.error(e?.message || '刷新失败')
  } finally {
    refreshingId.value = ''
  }
}

async function doDelete(row: KiroAccount) {
  try {
    await deleteAccount(row.id)
    message.success('已删除')
    reload()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

const columns: DataTableColumns<KiroAccount> = [
  { type: 'selection' },
  {
    title: '账号',
    key: 'email',
    width: 280,
    ellipsis: { tooltip: true },
    render: r => h('div', { class: 'acc-cell' }, [
      h('span', { class: 'acc-email' }, r.email || r.nickname || r.id),
      r.banStatus && r.banStatus !== '' ? h('span', { class: 'acc-ban' }, `⚠ ${r.banStatus}`) : null,
    ]),
  },
  {
    title: '套餐',
    key: 'subscription',
    width: 140,
    align: 'center',
    render: r => r.subscriptionTitle
      ? h(NTag, { size: 'small', bordered: false, type: subscriptionTone(r) }, () => r.subscriptionTitle!)
      : h('span', { class: 'dim' }, '-'),
  },
  { title: '剩余天数', key: 'daysRemaining', width: 110, align: 'center', render: r => h('span', { class: 'mono' }, r.daysRemaining != null ? `${r.daysRemaining}d` : '-') },
  { title: '用量', key: 'usage', width: 220, ellipsis: { tooltip: true }, render: r => h('span', { class: 'mono dim' }, fmtUsage(r)) },
  { title: '权重', key: 'weight', width: 90, align: 'center', render: r => h('span', { class: 'mono' }, r.weight ?? 100) },
  { title: '请求数', key: 'requests', width: 110, align: 'center', render: r => h('span', { class: 'mono' }, (r.requestCount ?? 0).toLocaleString()) },
  { title: 'Token', key: 'tokens', width: 110, align: 'center', render: r => h('span', { class: 'mono' }, fmtToken(r.totalTokens)) },
  {
    title: '启用',
    key: 'enabled',
    width: 80,
    align: 'center',
    render: r => h(NSwitch, { size: 'small', value: r.enabled, loading: r.id === togglingId.value, onUpdateValue: (v: boolean) => toggle(r, v) }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 170,
    align: 'center',
    render: row => h(NSpace, { size: 4, justify: 'center' }, () => [
      h(NButton, { size: 'tiny', quaternary: true, loading: row.id === refreshingId.value, onClick: () => doRefresh(row) },
        { default: () => '刷新', icon: () => h(RefreshCw, { size: 13 }) }),
      h(NPopconfirm, {
        onPositiveClick: () => doDelete(row),
        positiveText: '删除',
        negativeText: '取消',
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { icon: () => h(Trash2, { size: 13 }) }),
        default: () => `从账号池移除「${row.email || row.id}」？该账号将不再被调度。`,
      }),
    ]),
  },
]

function fmtToken(t?: number) {
  if (!t) return '-'
  if (t >= 1e6) return `${(t / 1e6).toFixed(1)}M`
  if (t >= 1e3) return `${(t / 1e3).toFixed(1)}K`
  return String(t)
}

onMounted(reload)
defineExpose({ reload })
</script>

<template>
  <section>
    <section class="hero-row">
      <div class="hero-row__item">
        <span class="hero-row__label">总数</span>
        <span class="hero-row__value mono">{{ stats.total }}</span>
      </div>
      <div class="hero-row__item">
        <span class="hero-row__label">启用中</span>
        <span class="hero-row__value mono accent">{{ stats.enabled }}</span>
      </div>
      <div class="hero-row__item">
        <span class="hero-row__label">PRO 套餐</span>
        <span class="hero-row__value mono">{{ stats.pro }}</span>
      </div>
    </section>

    <Toolbar>
      <template #left>
        <n-input v-model:value="search" clearable size="small" placeholder="搜索邮箱 / 备注 / 套餐" style="width: 320px">
          <template #prefix><Search :size="14" /></template>
        </n-input>
      </template>
      <template #right>
        <n-button type="primary" size="small" @click="addShow = true">
          <template #icon><Plus :size="14" /></template>添加账号
        </n-button>
        <n-button size="small" quaternary @click="exportCsv">
          <template #icon><Download :size="14" /></template>导出 CSV
        </n-button>
        <n-button size="small" :loading="loading" @click="reload">
          <template #icon><RefreshCw :size="14" /></template>刷新列表
        </n-button>
      </template>
    </Toolbar>

    <KiroAddDrawer v-model:show="addShow" @added="reload" />

    <BatchActionBar :count="checkedRowKeys.length" @clear="checkedRowKeys = []">
      <n-button size="small" :loading="batchRunning" @click="bulkEnable">
        <template #icon><CheckCircle2 :size="13" /></template>批量启用
      </n-button>
      <n-button size="small" :loading="batchRunning" @click="bulkDisable">
        <template #icon><XCircle :size="13" /></template>批量禁用
      </n-button>
      <n-button size="small" :loading="batchRunning" @click="bulkRefresh">
        <template #icon><RefreshCw :size="13" /></template>批量刷新 token
      </n-button>
      <n-popconfirm @positive-click="bulkDelete" positive-text="删除" negative-text="取消">
        <template #trigger>
          <n-button size="small" type="error" :loading="batchRunning">
            <template #icon><Trash2 :size="13" /></template>批量删除
          </n-button>
        </template>
        移除选中的 {{ checkedRowKeys.length }} 个账号？这些账号将不再被调度。
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
      :scroll-x="1530"
      size="small"
      striped
    />
    <EmptyState v-else icon="○" title="账号池为空" desc="去「上游账号」页（legacy/accounts）添加 Kiro 账号" />
  </section>
</template>

<style scoped>
.hero-row {
  display: flex;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 6px;
  background: #0a0a0a;
  margin-bottom: 12px;
}
.hero-row__item { display: flex; align-items: baseline; gap: 8px; padding-right: 24px; border-right: 1px solid rgba(255,255,255,0.06); }
.hero-row__item:last-child { border-right: none; }
.hero-row__label { font-size: 11px; color: #707070; text-transform: uppercase; letter-spacing: 0.06em; }
.hero-row__value { font-size: 18px; font-weight: 600; color: #ededed; }

.acc-cell { display: flex; flex-direction: column; gap: 2px; }
.acc-email { color: #ededed; font-weight: 500; font-size: 13px; }
.acc-ban { color: #ff7a7a; font-size: 11px; }

.mono { font-family: "Geist Mono", ui-monospace, monospace; color: #ededed; }
.dim { color: #707070; }
.accent { color: #0bd470; }
</style>
