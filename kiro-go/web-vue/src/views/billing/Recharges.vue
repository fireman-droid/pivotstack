<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  NDataTable, NInput, NSelect, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { Search, RefreshCw } from 'lucide-vue-next'
import CopyableText from '../../components/common/CopyableText.vue'
import { listAdminRecharges, type AdminRechargeRecord, type AdminRechargesSummary } from '../../api/admin/recharges'
import { useTablePagination } from '../../composables/useTablePagination'

const message = useMessage()
const pagination = useTablePagination(50)
const loading = ref(false)
const rows = ref<AdminRechargeRecord[]>([])
const total = ref(0)
const summary = ref<AdminRechargesSummary>({ todayCNY: 0, monthCNY: 0, avgCNY: 0, returningRate: 0 })
const search = ref('')
const typeFilter = ref<string>('all')

const typeOptions = [
  { label: '全部类型', value: 'all' },
  { label: '兑换码', value: 'code_redeem' },
  { label: '兑换码（天卡）', value: 'code_redeem_days' },
  { label: 'admin 充值', value: 'admin_balance' },
  { label: 'admin 赠送', value: 'admin_gift' },
  { label: 'admin 调整', value: 'admin_adjust' },
]
const typeLabel: Record<string, string> = {
  code_redeem: '兑换码',
  code_redeem_days: '天卡',
  admin_balance: '充值',
  admin_gift: '赠送',
  admin_adjust: '调整',
}
const typeColor: Record<string, string> = {
  code_redeem: 'r-chip--green',
  code_redeem_days: 'r-chip--blue',
  admin_balance: 'r-chip--green',
  admin_gift: 'r-chip--blue',
  admin_adjust: 'r-chip--warn',
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(r => {
    if (typeFilter.value !== 'all' && r.type !== typeFilter.value) return false
    if (!q) return true
    return [r.key_note, r.code, r.note, r.key_id].some(v => String(v || '').toLowerCase().includes(q))
  })
})

function fmtTime(row: AdminRechargeRecord) {
  if (row.time) return row.time
  return row.timestamp ? new Date(row.timestamp * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}
function fmtMoney(v?: number, sign = false) {
  if (v == null) return '-'
  const s = sign ? (v >= 0 ? '+' : '') : ''
  return `${s}¥${v.toFixed(2)}`
}

const columns: DataTableColumns<AdminRechargeRecord> = [
  { title: '时间', key: 'time', width: 160, align: 'center', render: row => h('span', { class: 'r-mono r-dim' }, fmtTime(row)) },
  {
    title: '用户',
    key: 'key',
    width: 340,
    render: row => h('div', null, [
      h('div', { class: 'r-key-note' }, row.key_note || '(无备注)'),
      h(CopyableText, { text: row.key_id, mono: true, mask: false }, () => null),
    ]),
  },
  {
    title: '类型',
    key: 'type',
    width: 90,
    align: 'center',
    render: row => h('span', { class: ['r-chip', typeColor[row.type] || ''] }, typeLabel[row.type] || row.type),
  },
  {
    title: 'code',
    key: 'code',
    width: 160,
    render: row => row.code
      ? h(CopyableText, { text: row.code, mono: true, mask: false })
      : h('span', { class: 'r-dim' }, '-'),
  },
  {
    title: '金额',
    key: 'amount_cny',
    width: 110,
    align: 'center',
    render: row => h('span', { class: ['r-mono r-amount', row.amount_cny < 0 ? 'r-amount--down' : 'r-amount--up'] }, fmtMoney(row.amount_cny, true)),
  },
  {
    title: '余额变化',
    key: 'delta',
    width: 220,
    align: 'center',
    render: row => {
      // 后端 v2+ 返回 *_cny（已按 PivotStackDollarsPerYuan 换算）；
      // 老数据无 *_cny 时仍 fallback raw（兼容但单位会错，需后端 refresh）。
      const before = row.balance_before_cny ?? row.balance_before
      const after = row.balance_after_cny ?? row.balance_after
      return h('span', { class: 'r-mono r-dim' }, `${fmtMoney(before)} → ${fmtMoney(after)}`)
    },
  },
]

async function reload() {
  loading.value = true
  try {
    const res = await listAdminRecharges({ limit: 500 })
    rows.value = res.records || []
    total.value = res.total || 0
    summary.value = res.summary || { todayCNY: 0, monthCNY: 0, avgCNY: 0, returningRate: 0 }
  } catch (e: any) {
    message.error(e?.message || '加载充值流水失败')
  } finally {
    loading.value = false
  }
}

onMounted(reload)
</script>

<template>
  <div class="admin-page">
    <header class="page-head">
      <div>
        <div class="page-head__crumb"><b>BILLING</b> / 充值流水</div>
        <div class="page-head__title">
          <div class="t-display-admin">充值流水</div>
          <div class="page-head__sub">全平台入账记录 · 共 {{ total }} 条 · 谁用兑换码 / admin 调余额</div>
        </div>
      </div>
      <div class="page-head__right">
        <button class="r-btn r-btn--ghost" :disabled="loading" @click="reload">
          <RefreshCw :size="14" :class="{ 'is-spinning': loading }" />
          刷新
        </button>
      </div>
    </header>

    <section class="metric-strip">
      <div class="metric-tile">
        <div class="metric-tile__label">今日入账</div>
        <div class="metric-tile__num">¥{{ summary.todayCNY.toFixed(2) }}</div>
        <div class="metric-tile__delta"><span class="t-meta">code_redeem + admin_balance</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">本月入账</div>
        <div class="metric-tile__num">¥{{ summary.monthCNY.toFixed(2) }}</div>
        <div class="metric-tile__delta"><span class="t-meta">从本月 1 日起</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">平均充值</div>
        <div class="metric-tile__num">¥{{ summary.avgCNY.toFixed(2) }}</div>
        <div class="metric-tile__delta"><span class="t-meta">单笔均值</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">复充率</div>
        <div class="metric-tile__num">{{ (summary.returningRate * 100).toFixed(0) }}%</div>
        <div class="metric-tile__delta"><span class="t-meta">充过 ≥2 次的 key</span></div>
      </div>
    </section>

    <div class="r-filter">
      <NInput v-model:value="search" clearable size="small" placeholder="搜索 key 备注 / code / 备注" style="width:320px">
        <template #prefix><Search :size="14" /></template>
      </NInput>
      <NSelect v-model:value="typeFilter" :options="typeOptions" size="small" style="width:160px" />
    </div>

    <NDataTable
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="row => `${row.timestamp}-${row.key_id}-${row.type}`"
      :pagination="pagination"
      :scroll-x="1080"
      :bordered="false"
      size="small"
      class="r-table"
    />
  </div>
</template>

<style scoped>
.r-btn {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--st-border);
  border-radius: 4px;
  color: var(--st-text-pri);
  font-size: 12px; font-family: inherit; cursor: pointer;
}
.r-btn--ghost { background: transparent; }
.r-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); }
.is-spinning { animation: r-spin 0.8s linear infinite; }
@keyframes r-spin { to { transform: rotate(360deg); } }

.r-filter { display: flex; gap: 8px; margin-bottom: 16px; }

:deep(.r-mono) { font-family: var(--st-font-mono); font-variant-numeric: tabular-nums; font-size: 12px; }
:deep(.r-dim) { color: var(--st-text-ter); font-size: 12px; }
:deep(.r-key-note) { color: var(--st-text-pri); font-size: 13px; font-weight: 500; }

:deep(.r-chip) {
  display: inline-flex; align-items: center;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 500;
}
:deep(.r-chip--green) { color: var(--st-success); background: rgba(11, 212, 112, 0.10); }
:deep(.r-chip--blue) { color: var(--st-info); background: rgba(82, 168, 255, 0.10); }
:deep(.r-chip--warn) { color: var(--st-warning); background: rgba(245, 166, 35, 0.10); }

:deep(.r-amount) { font-weight: 500; }
:deep(.r-amount--up) { color: var(--st-success); }
:deep(.r-amount--down) { color: var(--st-error); }

.r-table :deep(.n-data-table-th) {
  font-size: 11px !important; font-weight: 500 !important;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter) !important;
  background: transparent !important;
  height: 32px !important; padding: 0 12px !important;
  border-bottom: 1px solid var(--st-border) !important;
}
.r-table :deep(.n-data-table-td) {
  height: 44px !important; padding: 6px 12px !important;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.r-table :deep(.n-data-table-tr:hover .n-data-table-td) { background: rgba(255, 255, 255, 0.04) !important; }
.r-table :deep(.n-data-table) { background: transparent !important; }
</style>
