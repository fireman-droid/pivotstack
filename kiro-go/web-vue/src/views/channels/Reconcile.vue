<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { NButton, NDataTable, NTabs, NTabPane, NSpace, useMessage, type DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import KPIBlock from '../../components/common/KPIBlock.vue'
import StatusBadge from '../../components/common/StatusBadge.vue'
import MonoValue from '../../components/common/MonoValue.vue'
import EmptyState from '../../components/common/EmptyState.vue'
import { getReconcileStatus, retryReconcileRequest, type ReconcileEvent, type ReconcileStatus } from '../../api/admin/reconcile'
import { useTablePagination } from '../../composables/useTablePagination'

const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const tab = ref<'recent' | 'errors'>('recent')
const status = ref<ReconcileStatus>({})

// 后端真实 shape：{ providers: [{ providerId, pendingCount?, errorCount?, recentEvents[] }], globalDebtUsd }
// KPI 从 providers 派生；事件列表从 providers.recentEvents 合并。
const allEvents = computed(() => {
  return (status.value.providers || []).flatMap(p => p.recentEvents || [])
})
const pendingCount = computed(() => allEvents.value.filter(e => e.status === 'pending').length)
const successCount = computed(() => allEvents.value.filter(e => e.status === 'success').length)
const errorCount = computed(() => allEvents.value.filter(e => e.status === 'error' || e.status === 'failed').length)
const retryCount = computed(() => allEvents.value.filter(e => e.status === 'retrying').length)

const events = computed(() => {
  if (tab.value === 'errors') {
    return allEvents.value.filter(e => e.status === 'error' || e.status === 'failed')
  }
  return allEvents.value
})

const columns: DataTableColumns<ReconcileEvent> = [
  { title: '时间', key: 'time', width: 170, align: 'center', render: row => h(MonoValue, { value: eventTime(row) }) },
  { title: '渠道 / Provider', key: 'channelAlias', width: 240, ellipsis: { tooltip: true }, render: row => row.channelAlias || row.channelId || row.providerId || '-' },
  { title: '上游 quota', key: 'upstreamQuota', width: 130, align: 'center', render: row => h('span', { class: 'mono' }, quota((row as any).upstreamQuota ?? row.upstreamCost)) },
  { title: '本地估算', key: 'estimatedQuota', width: 130, align: 'center', render: row => h('span', { class: 'mono' }, quota((row as any).estimatedQuota ?? row.localCost)) },
  { title: '差额', key: 'diff', width: 130, align: 'center', render: row => h('span', { class: 'mono' }, money(diffOf(row))) },
  {
    title: '状态',
    key: 'status',
    width: 110,
    align: 'center',
    render: row => h(StatusBadge, { status: statusTone(row.status), label: row.status || 'pending' }),
  },
  {
    title: '操作',
    key: 'action',
    width: 110,
    align: 'center',
    render: row => h(NSpace, { size: 4, justify: 'center' }, () => [
      h(NButton, {
        size: 'tiny',
        quaternary: true,
        disabled: !row.requestId,
        onClick: () => retry(row),
      }, { default: () => '重试' }),
    ]),
  },
]

function statusTone(v?: string): 'success' | 'error' | 'pending' | 'warning' {
  if (v === 'success') return 'success'
  if (v === 'failed' || v === 'error') return 'error'
  if (v === 'retrying') return 'pending'
  return 'warning'
}
function eventTime(row: ReconcileEvent) {
  if (row.time) return row.time
  const ts = (row as any).timestamp ?? row.createdAt
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}
function quota(v?: number) {
  return v == null ? '-' : v.toLocaleString()
}
function diffOf(row: ReconcileEvent) {
  const d = (row as any).paidUsdDelta ?? (row as any).debtUsdAdded ?? row.diff
  return d
}
function money(v?: number) {
  return v == null ? '-' : `$${Number(v).toFixed(6)}`
}
async function reload() {
  loading.value = true
  try {
    status.value = await getReconcileStatus()
  } catch (e: any) {
    message.error(e?.message || '加载对账状态失败')
  } finally {
    loading.value = false
  }
}
async function retry(row: ReconcileEvent) {
  if (!row.requestId) return
  await retryReconcileRequest(row.requestId)
  message.success('已加入重试')
  reload()
}

onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="渠道 · 对账"
      :kicker-dot="'#707070'"
      title="对账监控"
      desc="上游 quota ↔ 本地 ledger 差异检测与回滚"
    >
      <template #actions>
        <n-button size="small" :loading="loading" @click="reload">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </n-button>
      </template>
    </PageHeader>

    <section class="kpi-grid">
      <div class="kpi-cell"><KPIBlock label="待处理" :value="pendingCount" /></div>
      <div class="kpi-cell"><KPIBlock label="成功" :value="successCount" /></div>
      <div class="kpi-cell"><KPIBlock label="失败" :value="errorCount" /></div>
      <div class="kpi-cell">
        <KPIBlock
          label="全局欠款"
          :value="`$${Number((status as any).globalDebtUsd ?? 0).toFixed(4)}`"
        />
      </div>
    </section>

    <n-tabs v-model:value="tab" size="small" class="sub-tabs">
      <n-tab-pane name="recent" tab="最近事件" />
      <n-tab-pane name="errors" tab="异常" />
    </n-tabs>

    <n-data-table
      v-if="events.length || loading"
      :columns="columns"
      :data="events"
      :loading="loading"
      :row-key="row => row.id || row.requestId || `${row.providerId}-${(row as any).timestamp || row.createdAt}`"
      :pagination="pagination"
      :scroll-x="1020"
      size="small"
      striped
    />
    <EmptyState v-else icon="✓" title="当前没有对账事件" desc="队列为空，最近没有需要处理的差异" />
  </PageContainer>
</template>

<style scoped>
.kpi-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 24px; }
.kpi-cell { border: 1px solid rgba(255,255,255,0.06); border-radius: 6px; padding: 16px; background: #0a0a0a; }
.sub-tabs { margin-bottom: 16px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; color: #ededed; }
@media (max-width: 900px) { .kpi-grid { grid-template-columns: repeat(2, 1fr); } }
</style>
