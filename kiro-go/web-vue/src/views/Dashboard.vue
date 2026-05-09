<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from 'vue'
let echarts = null
import { api } from '../api/admin'
import { formatNum } from '../utils/format'
import { useWorldTheme } from '../stores/worldTheme'
import { useAuthStore } from '../stores/auth'
import { useToast } from '../composables/useToast'
import {
  Users, Zap, Crown, CreditCard, Clock,
  Copy, Terminal, Globe, AlertTriangle, Moon, RefreshCw,
  DollarSign, BarChart3, TrendingUp, Activity
} from 'lucide-vue-next'
import { copyToClipboard } from '../utils/clipboard'
import WorldCard from '../components/world/WorldCard.vue'
import WorldStat from '../components/world/WorldStat.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldProgress from '../components/world/WorldProgress.vue'
import WorldLoader from '../components/world/WorldLoader.vue'
import WorldTable from '../components/world/WorldTable.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldButton from '../components/world/WorldButton.vue'

const { success } = useToast()
const stats = ref({
  accounts: 0, totalRequests: 0, successRequests: 0, failedRequests: 0,
  totalTokens: 0, totalCredits: 0, uptime: 0,
  freePool: { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 },
  proPool:  { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 },
})
const version = ref('')
const loading = ref(true)
const profit = ref(null)
const auth = useAuthStore()

async function fetchProfit() {
  try {
    const res = await fetch('/admin/api/profit', {
      headers: { 'X-Admin-Password': auth.password, 'Content-Type': 'application/json' },
    })
    if (res.ok) profit.value = await res.json()
  } catch (e) { console.error('fetchProfit failed:', e) }
}

const theme = useWorldTheme()
const chartRef = ref(null)
let chart = null
const requestHistory = ref([])
const chartIncrements = ref([0, 0, 0, 0, 0, 0, 0, 0, 0, 0])

async function loadEcharts() {
  if (echarts) return
  const [echartsModule, abyssTheme] = await Promise.all([
    import('echarts'),
    import('@/lib/echarts-abyss-theme.json'),
  ])
  echarts = echartsModule
  echarts.registerTheme('abyss', abyssTheme.default || abyssTheme)
}

async function initChart() {
  if (!chartRef.value) return
  await loadEcharts()
  chart = echarts.init(chartRef.value, theme.currentWorld === 'daogui' ? 'abyss' : null)
  updateChart()
  window.addEventListener('resize', () => chart?.resize())
}

watch(() => theme.currentWorld, async (newVal) => {
  if (chart && echarts) {
    chart.dispose()
    chart = echarts.init(chartRef.value, newVal === 'daogui' ? 'abyss' : null)
    updateChart()
  }
})

function updateChart() {
  if (!chart) return
  const isDaogui = theme.currentWorld === 'daogui'
  const accentColor = isDaogui ? 'rgba(196, 30, 58, 1)' : 'rgba(2, 132, 199, 1)'
  const shadowColor = isDaogui ? 'rgba(196, 30, 58, 0.5)' : 'rgba(2, 132, 199, 0.5)'
  const areaStart   = isDaogui ? 'rgba(196, 30, 58, 0.3)' : 'rgba(2, 132, 199, 0.3)'
  const areaEnd     = isDaogui ? 'rgba(196, 30, 58, 0)'   : 'rgba(2, 132, 199, 0)'
  const textColor   = isDaogui ? '#a08766' : '#475569'
  const tooltipBg   = isDaogui ? 'rgba(10, 10, 10, 0.92)' : 'rgba(255, 255, 255, 0.95)'
  const tooltipText = isDaogui ? '#e7d7c1' : '#0f172a'
  const tooltipBorder = isDaogui ? '#b8860b' : '#e2e8f0'
  const splitLineColor = isDaogui ? 'rgba(184, 134, 11, 0.18)' : 'rgba(226, 232, 240, 1)'

  chart.setOption({
    grid: { top: 10, right: 10, bottom: 25, left: 40 },
    xAxis: {
      type: 'category',
      data: Array.from({ length: 10 }, (_, i) => `${(9 - i) * 5}s`),
      axisLabel: { color: textColor, fontSize: 10 },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: textColor, fontSize: 10 },
      splitLine: { lineStyle: { type: 'dashed', color: splitLineColor } },
    },
    series: [{
      type: 'line',
      data: chartIncrements.value,
      smooth: true,
      showSymbol: false,
      itemStyle: { color: accentColor },
      lineStyle: { width: 3, color: accentColor, shadowBlur: 10, shadowColor, shadowOffsetX: 2, shadowOffsetY: 2 },
      areaStyle: {
        color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [{ offset: 0, color: areaStart }, { offset: 1, color: areaEnd }] },
      },
    }],
    tooltip: {
      trigger: 'axis',
      backgroundColor: tooltipBg,
      borderColor: tooltipBorder,
      textStyle: { color: tooltipText, fontSize: 12 },
    },
  })
}

async function loadStats() {
  try {
    const res = await api('/status')
    if (res.ok) processStats(await res.json())
  } catch {}
}

function processStats(newStats) {
  requestHistory.value.push(newStats.totalRequests || 0)
  if (requestHistory.value.length > 10) requestHistory.value.shift()

  const increments = []
  for (let i = 1; i < requestHistory.value.length; i++) {
    increments.push(Math.max(0, requestHistory.value[i] - requestHistory.value[i - 1]))
  }
  while (increments.length < 10) increments.unshift(0)
  chartIncrements.value = increments

  stats.value = newStats
  loading.value = false
  nextTick(() => { if (!chart) initChart(); else updateChart() })
}

let sseSource = null
let pollTimer = null

function connectStatsSSE() {
  const password = localStorage.getItem('admin_password') || ''
  const url = `${location.origin}/admin/api/sse/stats?password=${encodeURIComponent(password)}`
  sseSource = new EventSource(url)
  sseSource.addEventListener('stats', (e) => {
    try { processStats(JSON.parse(e.data)) } catch {}
  })
  sseSource.onerror = () => {
    sseSource.close(); sseSource = null
    loadStats()
    if (!pollTimer) pollTimer = setInterval(loadStats, 5000)
    setTimeout(() => {
      if (!sseSource) {
        if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
        connectStatsSSE()
      }
    }, 5000)
  }
}

async function loadVersion() {
  try {
    const res = await api('/version')
    if (res.ok) { version.value = (await res.json()).version || '' }
  } catch {}
}

// === 沉睡用户 ===
const inactiveDays = ref(30)
const inactiveDayOptions = [
  { value: 30, label: '30 天+' },
  { value: 60, label: '60 天+' },
  { value: 90, label: '90 天+' },
]
const inactiveData = ref({ count: 0, keys: [] })
const inactiveLoading = ref(false)

async function loadInactive() {
  inactiveLoading.value = true
  try {
    const res = await api(`/inactive-keys?days=${inactiveDays.value}`)
    if (res.ok) inactiveData.value = await res.json()
  } catch {}
  inactiveLoading.value = false
}

watch(inactiveDays, loadInactive)

const inactiveColumns = [
  { key: 'note',     label: '备注' },
  { key: 'daysIdle', label: '闲置天数', align: 'right' },
  { key: 'balance',  label: '余额',     align: 'right', mono: true },
  { key: 'giftBalance', label: '赠金',  align: 'right', mono: true },
  { key: 'requests', label: '总请求',   align: 'right', mono: true },
  { key: 'lastUsed', label: '末次使用', align: 'left' },
]

function formatLastUsed(ts) {
  if (!ts) return '从未使用'
  return new Date(ts * 1000).toLocaleDateString('zh-CN')
}

function formatUptime(s) {
  if (!s) return '0s'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

function copy(text) {
  copyToClipboard(text)
  success('已复制到剪贴板')
}

const base = location.origin

onMounted(async () => {
  await loadVersion()
  connectStatsSSE()
  loadInactive()
  fetchProfit()
  // 营收数据每 30s 刷一次（admin 看趋势）
  setInterval(fetchProfit, 30000)
})

onUnmounted(() => {
  if (sseSource) { sseSource.close(); sseSource = null }
  if (pollTimer) clearInterval(pollTimer)
  chart?.dispose()
  window.removeEventListener('resize', () => chart?.resize())
})

const successRate = computed(() => {
  if (!stats.value.totalRequests) return 100
  return (stats.value.successRequests / stats.value.totalRequests) * 100
})
const isErrorHigh = computed(() => (100 - successRate.value) > 5)

const freeAvailable = computed(() => stats.value.freePool?.available || 0)
const proAvailable  = computed(() => stats.value.proPool?.available || 0)
</script>

<template>
  <div class="dashboard-page" v-if="!loading">
    <!-- Header -->
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">控制台</div>
        <h1 class="page-title">数据面板 <span class="page-version">v{{ version }}</span></h1>
      </div>
      <div class="status-row">
        <WorldChip variant="success" :dot="true" :pulse="true">系统在线</WorldChip>
        <WorldChip variant="neutral" size="sm"><Clock :size="11" /> 运行 {{ formatUptime(stats.uptime) }}</WorldChip>
      </div>
    </header>

    <!-- 营收概览 4 卡 -->
    <div class="stats-row" v-if="profit">
      <WorldStat
        label="总收入" unit="CNY"
        :value="`¥${(profit.revenue_cny || 0).toFixed(2)}`"
        :hint="`面值 $${(profit.revenue_usd || 0).toFixed(2)}`"
        :icon="DollarSign" variant="success"
      />
      <WorldStat
        label="总成本" unit="CNY"
        :value="`¥${(profit.total_cost_cny || 0).toFixed(2)}`"
        :hint="`PRO ¥${(profit.pro_cost_cny || 0).toFixed(2)} · FREE ¥${(profit.free_cost_cny || 0).toFixed(2)}`"
        :icon="BarChart3" variant="danger"
      />
      <WorldStat
        label="净利润" unit="CNY"
        :value="`¥${(profit.profit_cny || 0).toFixed(2)}`"
        :hint="(profit.profit_cny || 0) >= 0 ? '盈利中' : '亏损中'"
        :icon="TrendingUp"
        :variant="(profit.profit_cny || 0) >= 0 ? 'success' : 'danger'"
      />
      <WorldStat
        label="利润率" unit="%"
        :value="(profit.margin_percent || 0).toFixed(1)"
        :hint="(profit.margin_percent || 0) >= 30 ? '健康' : ((profit.margin_percent || 0) >= 0 ? '偏低' : '亏损')"
        :icon="Activity"
        :variant="(profit.margin_percent || 0) >= 30 ? 'info' : ((profit.margin_percent || 0) >= 0 ? 'warning' : 'danger')"
      />
    </div>

    <!-- 4 Stats -->
    <div class="stats-row">
      <WorldStat
        label="FREE 号池"
        :value="stats.freePool?.total || 0"
        :hint="`${freeAvailable} 个可用`"
        :icon="Users"
        variant="success"
      />
      <WorldStat
        label="PRO 号池"
        :value="stats.proPool?.total || 0"
        :hint="`${proAvailable} 个可用`"
        :icon="Crown"
        variant="info"
      />
      <WorldStat
        label="总请求数"
        :value="formatNum(stats.totalRequests || 0)"
        :hint="`${stats.successRequests} 成功 / ${stats.failedRequests} 失败`"
        :icon="Zap"
        :variant="isErrorHigh ? 'danger' : 'primary'"
      />
      <WorldStat
        label="成功率"
        :value="successRate.toFixed(1)"
        unit="%"
        :hint="isErrorHigh ? '错误率偏高，请检查' : '运行正常'"
        :icon="AlertTriangle"
        :variant="isErrorHigh ? 'danger' : 'success'"
      />
    </div>

    <!-- Pool detail with progress -->
    <div class="pool-grid">
      <WorldCard padding="md">
        <h3 class="section-title">FREE 池配额</h3>
        <WorldProgress
          v-if="stats.freePool?.trialLimit > 0"
          :value="stats.freePool.trialCurrent || 0"
          :max="stats.freePool.trialLimit"
          variant="warning"
          :show-label="true"
          label="试用配额"
          :hint="`${(stats.freePool.trialCurrent || 0).toFixed(0)} / ${stats.freePool.trialLimit.toFixed(0)}`"
        />
        <WorldProgress
          v-if="stats.freePool?.usageLimit > 0"
          :value="stats.freePool.usageCurrent || 0"
          :max="stats.freePool.usageLimit"
          variant="success"
          :show-label="true"
          label="主配额"
          :hint="`${(stats.freePool.usageCurrent || 0).toFixed(0)} / ${stats.freePool.usageLimit.toFixed(0)}`"
        />
        <div v-if="stats.freePrediction?.sufficient" class="pred-line">
          <Clock :size="12" />
          <span v-if="stats.freePrediction.remainingDays >= 1">预计可用 {{ Math.floor(stats.freePrediction.remainingDays) }} 天</span>
          <span v-else-if="stats.freePrediction.remainingHours >= 1">活跃可用 {{ Math.floor(stats.freePrediction.remainingHours) }}h</span>
          <span v-else>额度即将用尽</span>
        </div>
      </WorldCard>

      <WorldCard padding="md">
        <h3 class="section-title">PRO 池配额</h3>
        <WorldProgress
          v-if="stats.proPool?.trialLimit > 0"
          :value="stats.proPool.trialCurrent || 0"
          :max="stats.proPool.trialLimit"
          variant="warning"
          :show-label="true"
          label="试用配额"
          :hint="`${(stats.proPool.trialCurrent || 0).toFixed(0)} / ${stats.proPool.trialLimit.toFixed(0)}`"
        />
        <WorldProgress
          v-if="stats.proPool?.usageLimit > 0"
          :value="stats.proPool.usageCurrent || 0"
          :max="stats.proPool.usageLimit"
          variant="primary"
          :show-label="true"
          label="主配额"
          :hint="`${(stats.proPool.usageCurrent || 0).toFixed(0)} / ${stats.proPool.usageLimit.toFixed(0)}`"
        />
        <div v-if="stats.proPrediction?.sufficient" class="pred-line">
          <Clock :size="12" />
          <span v-if="stats.proPrediction.remainingDays >= 1">预计可用 {{ Math.floor(stats.proPrediction.remainingDays) }} 天</span>
          <span v-else-if="stats.proPrediction.remainingHours >= 1">活跃可用 {{ Math.floor(stats.proPrediction.remainingHours) }}h</span>
          <span v-else>额度即将用尽</span>
        </div>
      </WorldCard>
    </div>

    <!-- API endpoint quick reference -->
    <WorldCard padding="md">
      <header class="section-head">
        <h3>
          <Terminal :size="16" />
          <span>API 接口快查</span>
        </h3>
      </header>
      <div class="ep-grid">
        <button
          v-for="ep in [
            { label: 'Claude Messages', path: '/v1/messages' },
            { label: 'OpenAI Chat',     path: '/v1/chat/completions' },
            { label: '模型发现',        path: '/v1/models' },
            { label: '服务状态',        path: '/health' },
          ]"
          :key="ep.path"
          class="ep-btn"
          @click="copy(base + ep.path)"
        >
          <span class="ep-label">{{ ep.label }}</span>
          <span class="ep-url">
            <span class="ep-host">{{ base.replace(/https?:\/\//, '') }}</span><span class="ep-path">{{ ep.path }}</span>
          </span>
          <Copy :size="14" class="ep-copy" />
        </button>
      </div>
    </WorldCard>

    <!-- Real-time chart -->
    <WorldCard padding="md">
      <header class="section-head">
        <h3>
          <Globe :size="16" />
          <span>实时流量</span>
        </h3>
        <WorldChip size="sm" variant="info" :dot="true">请求增量 / 5s</WorldChip>
      </header>
      <div ref="chartRef" class="chart-canvas" />
    </WorldCard>

    <!-- 沉睡用户 -->
    <WorldCard padding="md">
      <header class="section-head">
        <h3>
          <Moon :size="16" />
          <span>沉睡用户</span>
          <WorldChip v-if="inactiveData.count > 0" size="sm" variant="warning">{{ inactiveData.count }}</WorldChip>
        </h3>
        <div class="inactive-controls">
          <WorldSegment v-model="inactiveDays" :options="inactiveDayOptions" size="sm" />
          <WorldButton variant="secondary" size="sm" @click="loadInactive" :loading="inactiveLoading">
            <RefreshCw :size="13" />
          </WorldButton>
        </div>
      </header>
      <p class="section-hint">
        长期未使用的 API Key（含从未发起请求的）。可用于追踪潜在僵尸账户。
      </p>
      <WorldTable
        :columns="inactiveColumns"
        :rows="inactiveData.keys"
        empty-text="暂无沉睡用户"
        max-height="420px"
      >
        <template #cell-note="{ row }">
          <span class="note-cell">
            {{ row.note || '—' }}
            <WorldChip v-if="row.neverUsed" size="sm" variant="neutral">从未使用</WorldChip>
            <WorldChip v-else-if="!row.enabled" size="sm" variant="danger">已禁用</WorldChip>
          </span>
        </template>
        <template #cell-daysIdle="{ row }">
          <span :class="['days-cell', row.daysIdle >= 90 && 'is-grave', row.daysIdle >= 60 && 'is-warn']">
            {{ row.daysIdle }} 天
          </span>
        </template>
        <template #cell-balance="{ row }">
          <span class="mono">${{ Number(row.balance || 0).toFixed(2) }}</span>
        </template>
        <template #cell-giftBalance="{ row }">
          <span class="mono">${{ Number(row.giftBalance || 0).toFixed(2) }}</span>
        </template>
        <template #cell-requests="{ row }">
          <span class="mono">{{ row.requests || 0 }}</span>
        </template>
        <template #cell-lastUsed="{ row }">
          <span class="mono small">{{ formatLastUsed(row.lastUsed) }}</span>
        </template>
      </WorldTable>
    </WorldCard>
  </div>

  <div v-else class="loading-wrap">
    <WorldLoader :size="56" label="载入数据中" />
  </div>
</template>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  font-family: var(--world-font-display);
  font-size: 1.75rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}
.page-version {
  font-family: var(--world-font-mono);
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--world-text-mute);
}
.status-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 920px) { .stats-row { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 480px) { .stats-row { grid-template-columns: 1fr; } }

.pool-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
@media (max-width: 768px) { .pool-grid { grid-template-columns: 1fr; } }

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.875rem;
  font-weight: 800;
  margin: 0 0 14px;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
[data-world="daogui"] .section-title { color: var(--world-paper-aged); }

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.section-head h3 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 0.875rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}

.pred-line {
  margin-top: 10px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.7rem;
  color: var(--world-text-mute);
}

.ep-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}
@media (max-width: 600px) { .ep-grid { grid-template-columns: 1fr; } }

.ep-btn {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  cursor: pointer;
  font-family: var(--world-font-sans);
  text-align: left;
  transition: all 220ms ease;
}
.ep-btn:hover {
  border-color: var(--world-accent);
  transform: translateY(-1px);
}
.ep-label {
  flex: 0 0 130px;
  font-size: 0.75rem;
  font-weight: 800;
  color: var(--world-text-primary);
}
.ep-url {
  flex: 1;
  font-family: var(--world-font-mono);
  font-size: 0.72rem;
  color: var(--world-text-mute);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: flex;
  min-width: 0;
}
.ep-host { opacity: 0.5; }
.ep-path { color: var(--world-accent); font-weight: 700; }
.ep-copy {
  color: var(--world-text-dim);
  opacity: 0;
  transition: opacity 200ms ease;
}
.ep-btn:hover .ep-copy { opacity: 1; }

.chart-canvas {
  height: 280px;
  width: 100%;
}

.loading-wrap {
  min-height: 60vh;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 沉睡用户 */
.inactive-controls { display: flex; gap: 8px; align-items: center; }
.section-hint {
  margin: -6px 0 12px;
  font-size: 0.78rem;
  color: var(--world-text-mute);
}
.note-cell {
  display: inline-flex; align-items: center; gap: 8px;
  font-weight: 700;
}
.days-cell {
  font-family: var(--world-font-mono);
  font-weight: 800;
  color: var(--world-text-primary);
}
.days-cell.is-warn  { color: var(--world-warning); }
.days-cell.is-grave { color: var(--world-error); }
.mono {
  font-family: var(--world-font-mono);
  font-weight: 700;
  color: var(--world-text-primary);
}
.mono.small { font-size: 0.78rem; color: var(--world-text-mute); }
</style>
