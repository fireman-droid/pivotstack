<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import {
  NButton, NDataTable, NSelect, NSwitch, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { RefreshCw, AlertTriangle } from 'lucide-vue-next'
import KpiCard from '../../components/admin/business/KpiCard.vue'
import {
  getBusinessBoard,
  type BoardPeriod,
  type BusinessBoardResponse,
  type ChannelRow,
  type ModelRow,
} from '../../api/admin/businessBoard'

const message = useMessage()
const loading = ref(false)
const data = ref<BusinessBoardResponse | null>(null)
const period = ref<BoardPeriod>('today')
const includeGift = ref(false)
const channelFilter = ref<string | null>(null)

const periodOptions: { label: string; value: BoardPeriod }[] = [
  { label: '今日', value: 'today' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

const channelOptions = computed(() => {
  const rows = data.value?.channels || []
  return [
    { label: '全部渠道', value: null },
    ...rows.map(r => ({
      label: `${r.alias || r.channel_id} · ${r.channel_type}`,
      value: r.channel_id,
    })),
  ]
})

const kpi = computed(() => data.value?.kpi)
const trend = computed(() => data.value?.trend || [])
const warnings = computed(() => data.value?.warnings || [])

const profitTone = computed<'positive' | 'negative' | 'neutral'>(() => {
  const p = kpi.value?.profit_cny ?? 0
  if (p > 0) return 'positive'
  if (p < 0) return 'negative'
  return 'neutral'
})
const marginTone = computed<'positive' | 'negative' | 'warning' | 'neutral'>(() => {
  const m = kpi.value?.margin_percent ?? 0
  if (m >= 30) return 'positive'
  if (m >= 0) return 'warning'
  return 'negative'
})

function fmtCNY(v: number): string {
  return `¥${(v ?? 0).toFixed(2)}`
}
function fmtPercent(v: number): string {
  return `${(v ?? 0).toFixed(1)}%`
}
function fmtTokens(v: number): string {
  if (v >= 1e6) return `${(v / 1e6).toFixed(2)}M`
  if (v >= 1e3) return `${(v / 1e3).toFixed(1)}k`
  return String(v ?? 0)
}

const channelColumns: DataTableColumns<ChannelRow> = [
  { title: '渠道', key: 'alias', minWidth: 180,
    render: row => h('div', { class: 'bb-cell' }, [
      h('div', { class: 'bb-cell__name' }, row.alias || row.channel_id || '(未知)'),
      h('div', { class: 'bb-cell__sub' }, row.channel_type || ''),
    ]),
  },
  { title: '请求', key: 'requests', width: 90, align: 'right',
    render: row => h('span', { class: 'bb-mono' }, `${row.requests} / ${row.errors}`),
  },
  { title: 'Tokens', key: 'tokens', width: 130, align: 'right',
    render: row => h('span', { class: 'bb-mono bb-mono--dim' },
      `${fmtTokens(row.tokens_in)} → ${fmtTokens(row.tokens_out)}`),
  },
  { title: '用户支出', key: 'charged_cny', width: 110, align: 'right',
    render: row => h('span', { class: 'bb-mono' }, fmtCNY(row.charged_cny)),
  },
  { title: '真实成本', key: 'cost_cny', width: 110, align: 'right',
    render: row => h('span', { class: 'bb-mono' }, fmtCNY(row.cost_cny)),
  },
  { title: '收入分摊', key: 'revenue_share_cny', width: 110, align: 'right',
    render: row => h('span', { class: 'bb-mono bb-mono--dim' }, fmtCNY(row.revenue_share_cny)),
  },
  { title: '利润', key: 'profit_cny', width: 110, align: 'right',
    render: row => h('span', {
      class: ['bb-mono', row.profit_cny >= 0 ? 'bb-mono--up' : 'bb-mono--down'],
    }, fmtCNY(row.profit_cny)),
  },
  { title: '毛利率', key: 'margin_percent', width: 90, align: 'right',
    render: row => h('span', { class: 'bb-mono' }, fmtPercent(row.margin_percent)),
  },
]

const modelColumns: DataTableColumns<ModelRow> = [
  { title: '模型', key: 'model', minWidth: 200,
    render: row => h('span', { class: 'bb-cell__name' }, row.model || '(未知)'),
  },
  { title: '主渠道', key: 'channel_id', width: 160,
    render: row => h('span', { class: 'bb-cell__sub' }, row.channel_id || '—'),
  },
  { title: '请求', key: 'requests', width: 80, align: 'right',
    render: row => h('span', { class: 'bb-mono' }, String(row.requests)),
  },
  { title: 'Tokens', key: 'tokens', width: 130, align: 'right',
    render: row => h('span', { class: 'bb-mono bb-mono--dim' },
      `${fmtTokens(row.tokens_in)} → ${fmtTokens(row.tokens_out)}`),
  },
  { title: '用户支出', key: 'charged_cny', width: 110, align: 'right',
    render: row => h('span', { class: 'bb-mono' }, fmtCNY(row.charged_cny)),
  },
  { title: '成本', key: 'cost_cny', width: 110, align: 'right',
    render: row => h('span', { class: 'bb-mono' }, fmtCNY(row.cost_cny)),
  },
]

async function reload() {
  loading.value = true
  try {
    data.value = await getBusinessBoard({
      period: period.value,
      includeGift: includeGift.value,
      channel: channelFilter.value || undefined,
      topN: 10,
    })
  } catch (e: any) {
    message.error(e?.message || '加载经营看板失败')
  } finally {
    loading.value = false
  }
}

function switchPeriod(p: BoardPeriod) {
  period.value = p
  reload()
}

onMounted(reload)
</script>

<template>
  <div class="bb-page">
    <header class="bb-head">
      <div>
        <div class="bb-crumb"><b>OPS</b> / 经营看板</div>
        <h1 class="bb-title">经营看板</h1>
        <div class="bb-sub">多渠道盈亏 · 收入 / 成本 / 利润 · 现金入账口径收入 + 调用发生口径成本</div>
      </div>
      <div class="bb-actions">
        <div class="bb-seg">
          <button v-for="opt in periodOptions" :key="opt.value"
            :class="['bb-seg__btn', { 'is-on': period === opt.value }]"
            :disabled="loading"
            @click="switchPeriod(opt.value)">
            {{ opt.label }}
          </button>
        </div>
        <NSelect
          v-model:value="channelFilter"
          :options="channelOptions"
          size="small"
          style="width: 220px"
          placeholder="全部渠道"
          @update:value="reload"
        />
        <div class="bb-toggle">
          <NSwitch v-model:value="includeGift" size="small" @update:value="reload" />
          <span>含赠送</span>
        </div>
        <NButton size="small" :loading="loading" @click="reload">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </NButton>
      </div>
    </header>

    <section class="bb-kpis">
      <KpiCard label="收入" :value="kpi?.revenue_cny ?? 0" :formatter="fmtCNY"
        :trend="trend" trend-key="revenue_cny" tone="positive" />
      <KpiCard label="真实成本" :value="kpi?.cost_cny ?? 0" :formatter="fmtCNY"
        :trend="trend" trend-key="cost_cny" tone="warning" />
      <KpiCard label="净利润" :value="kpi?.profit_cny ?? 0" :formatter="fmtCNY"
        :trend="trend" trend-key="profit" :tone="profitTone" />
      <KpiCard label="毛利率" :value="kpi?.margin_percent ?? 0" unit="%"
        :formatter="(v) => v.toFixed(1)" :tone="marginTone" />
    </section>

    <section v-if="warnings.length" class="bb-warn">
      <div class="bb-warn__head">
        <AlertTriangle :size="14" />
        数据提示
      </div>
      <ul class="bb-warn__list">
        <li v-for="(w, i) in warnings" :key="i">{{ w }}</li>
      </ul>
    </section>

    <section class="bb-section">
      <h2 class="bb-section__title">渠道盈亏</h2>
      <NDataTable
        :columns="channelColumns"
        :data="data?.channels || []"
        :loading="loading"
        :row-key="(row: ChannelRow) => row.channel_id"
        :bordered="false"
        size="small"
        class="bb-table"
      />
    </section>

    <section class="bb-section">
      <h2 class="bb-section__title">模型 Top</h2>
      <NDataTable
        :columns="modelColumns"
        :data="data?.models || []"
        :loading="loading"
        :row-key="(row: ModelRow) => `${row.model}::${row.channel_id}`"
        :bordered="false"
        size="small"
        class="bb-table"
      />
    </section>
  </div>
</template>

<style scoped>
.bb-page { padding: 24px 32px; background: #000; min-height: 100vh; color: var(--st-text-pri); }
.bb-head { display: flex; justify-content: space-between; align-items: flex-end;
  margin-bottom: 20px; padding-bottom: 14px; border-bottom: 1px solid var(--st-border); gap: 16px; }
.bb-crumb { font-size: 11px; letter-spacing: 0.06em; color: var(--st-text-ter); text-transform: uppercase; }
.bb-crumb b { color: var(--st-text-pri); font-weight: 600; }
.bb-title { font-size: 18px; font-weight: 600; letter-spacing: -0.01em; margin: 4px 0; }
.bb-sub { font-size: 12px; color: var(--st-text-ter); }
.bb-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.bb-toggle { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--st-text-sec); }

.bb-seg { display: inline-flex; background: rgba(255,255,255,0.04); border: 1px solid var(--st-border); border-radius: 4px; padding: 2px; }
.bb-seg__btn { background: transparent; border: 0; padding: 4px 10px; font-size: 12px; color: var(--st-text-sec);
  cursor: pointer; border-radius: 3px; font-family: inherit; }
.bb-seg__btn.is-on { background: rgba(255,255,255,0.08); color: var(--st-text-pri); }
.bb-seg__btn:hover:not(.is-on):not(:disabled) { background: rgba(255,255,255,0.04); }

.bb-kpis { display: flex; gap: 12px; margin-bottom: 16px; }

.bb-warn { padding: 10px 14px; margin-bottom: 16px;
  background: rgba(245, 166, 35, 0.06); border: 1px solid rgba(245, 166, 35, 0.2); border-radius: 4px; }
.bb-warn__head { display: inline-flex; align-items: center; gap: 6px; font-size: 12px;
  color: var(--st-warning); margin-bottom: 4px; }
.bb-warn__list { margin: 0; padding-left: 18px; font-size: 12px; color: var(--st-text-sec); line-height: 1.6; }

.bb-section { margin-top: 22px; }
.bb-section__title { font-size: 12px; letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter); margin: 0 0 10px 0; font-weight: 500; }

:deep(.bb-cell__name) { color: var(--st-text-pri); font-size: 13px; font-weight: 500; }
:deep(.bb-cell__sub) { color: var(--st-text-ter); font-size: 11px; }
:deep(.bb-mono) { font-family: var(--st-font-mono, ui-monospace); font-variant-numeric: tabular-nums; font-size: 12px; }
:deep(.bb-mono--dim) { color: var(--st-text-ter); }
:deep(.bb-mono--up) { color: var(--st-success); }
:deep(.bb-mono--down) { color: var(--st-error); }

.bb-table :deep(.n-data-table-th) {
  font-size: 11px !important; font-weight: 500 !important;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter) !important; background: transparent !important;
  height: 32px !important; padding: 0 12px !important;
  border-bottom: 1px solid var(--st-border) !important;
}
.bb-table :deep(.n-data-table-td) {
  height: 40px !important; padding: 4px 12px !important;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.bb-table :deep(.n-data-table-tr:hover .n-data-table-td) { background: rgba(255, 255, 255, 0.04) !important; }
.bb-table :deep(.n-data-table) { background: transparent !important; }
</style>
