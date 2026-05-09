<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import * as echarts from 'echarts'
import { userApi } from '../../api/user'
import {
  FileX, Database, Coins, Timer, ChevronLeft, ChevronRight,
  CheckCircle2, XCircle, Activity, Sparkles, Calendar, TrendingUp,
} from 'lucide-vue-next'
import WorldCard from '../../components/world/WorldCard.vue'
import WorldChip from '../../components/world/WorldChip.vue'
import WorldStat from '../../components/world/WorldStat.vue'
import WorldTimeline from '../../components/world/WorldTimeline.vue'
import WorldButton from '../../components/world/WorldButton.vue'

// ===== 7 天活跃度 =====
const activity = ref({
  daily: [],
  totalCalls: 0,
  totalErrors: 0,
  totalTokens: 0,
  totalCostUSD: 0,
  promotion: { active: false },
})
const activityLoading = ref(true)

// ===== 当天/某日日志 =====
const logs = ref([])
const loading = ref(true)
const selectedDate = ref('today') // 'today' | 'YYYY-MM-DD' | 'all'
const page = ref(1)
const limit = ref(50)
const total = ref(0)
const respDate = ref('') // 后端返回的当前查询日期

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))
const hasMore = computed(() => page.value < totalPages.value)

// ===== ECharts =====
const chartEl = ref(null)
let chartInstance = null

function pickCss(name, fallback = '#000') {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name)
  return (v && v.trim()) || fallback
}

function buildChartOption() {
  const daily = activity.value.daily || []
  const dates = daily.map(d => d.date.slice(5)) // "MM-DD"
  const calls = daily.map(d => d.calls || 0)
  const errors = daily.map(d => d.errors || 0)
  const tokensK = daily.map(d => Math.round((d.tokens || 0) / 100) / 10) // 1 dec K

  const accent = pickCss('--world-accent', '#7c3aed')
  const success = pickCss('--world-success', '#10b981')
  const error = pickCss('--world-error', '#ef4444')
  const textMute = pickCss('--world-text-mute', '#9ca3af')
  const textDim = pickCss('--world-text-dim', '#6b7280')
  const border = pickCss('--world-glass-border', 'rgba(0,0,0,0.08)')
  const bg = pickCss('--world-glass-bg', 'rgba(255,255,255,0.6)')
  const isDaogui = document.documentElement.getAttribute('data-world') === 'daogui'

  return {
    backgroundColor: 'transparent',
    grid: { left: 12, right: 16, top: 36, bottom: 28, containLabel: true },
    legend: {
      top: 0,
      right: 8,
      itemWidth: 12,
      itemHeight: 12,
      itemGap: 14,
      textStyle: { color: textMute, fontSize: 11, fontWeight: 600 },
      icon: 'roundRect',
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: bg,
      borderColor: border,
      borderWidth: 1,
      padding: [8, 12],
      textStyle: { color: pickCss('--world-text-primary', '#111'), fontSize: 12 },
      extraCssText: 'backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); border-radius: 10px;',
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLine: { lineStyle: { color: border } },
      axisTick: { show: false },
      axisLabel: {
        color: textMute, fontSize: 11, fontWeight: 600,
        fontFamily: 'var(--world-font-mono)',
      },
    },
    yAxis: [
      {
        type: 'value',
        name: '调用',
        nameTextStyle: { color: textDim, fontSize: 10, fontWeight: 600, padding: [0, 0, 0, -20] },
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { lineStyle: { color: border, type: 'dashed' } },
        axisLabel: { color: textDim, fontSize: 10 },
        minInterval: 1,
      },
      {
        type: 'value',
        name: 'K Tokens',
        nameTextStyle: { color: textDim, fontSize: 10, fontWeight: 600, padding: [0, -20, 0, 0] },
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { color: textDim, fontSize: 10 },
      },
    ],
    series: [
      {
        name: '成功',
        type: 'bar',
        stack: 'calls',
        data: calls,
        barMaxWidth: 28,
        itemStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: accent },
              { offset: 1, color: isDaogui ? '#7a1f3a' : accent + 'aa' },
            ],
          },
          borderRadius: [4, 4, 0, 0],
        },
        emphasis: { itemStyle: { color: success } },
      },
      {
        name: '失败',
        type: 'bar',
        stack: 'calls',
        data: errors,
        barMaxWidth: 28,
        itemStyle: {
          color: error,
          borderRadius: [4, 4, 0, 0],
          opacity: 0.7,
        },
      },
      {
        name: 'Tokens (K)',
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        symbol: 'circle',
        symbolSize: 7,
        data: tokensK,
        lineStyle: { width: 2.5, color: success },
        itemStyle: { color: success, borderColor: bg, borderWidth: 2 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: success + '40' },
              { offset: 1, color: success + '00' },
            ],
          },
        },
      },
    ],
  }
}

function renderChart() {
  if (!chartEl.value) return
  if (!chartInstance) chartInstance = echarts.init(chartEl.value)
  chartInstance.setOption(buildChartOption(), { notMerge: true })
}

function handleResize() { chartInstance && chartInstance.resize() }

// ===== 数据加载 =====
async function loadActivity() {
  activityLoading.value = true
  try {
    const data = await userApi('/activity?days=7')
    activity.value = data
  } catch {}
  activityLoading.value = false
  await nextTick()
  renderChart()
}

async function loadLogs() {
  loading.value = true
  try {
    const dateParam = selectedDate.value === 'today' ? '' :
      (selectedDate.value === 'all' ? 'all' : selectedDate.value)
    const qs = `page=${page.value}&limit=${limit.value}` +
      (dateParam ? `&date=${dateParam}` : '')
    const data = await userApi('/logs?' + qs)
    logs.value = data.logs || []
    total.value = data.total || 0
    respDate.value = data.date || ''
  } catch {}
  loading.value = false
}

// ===== 日期切换 =====
const dateChips = computed(() => {
  const out = [{ key: 'today', label: '今日', sub: '' }]
  const today = new Date()
  for (let i = 1; i < 7; i++) {
    const d = new Date(today)
    d.setDate(d.getDate() - i)
    const ymd = d.toISOString().slice(0, 10)
    out.push({
      key: ymd,
      label: ymd.slice(5).replace('-', '/'),
      sub: ['日','一','二','三','四','五','六'][d.getDay()],
    })
  }
  out.push({ key: 'all', label: '全部', sub: '' })
  return out
})

function pickDate(key) {
  if (selectedDate.value === key) return
  selectedDate.value = key
  page.value = 1
  loadLogs()
}

// ===== 分页 =====
function prevPage() { if (page.value > 1) { page.value--; loadLogs() } }
function nextPage() { if (hasMore.value) { page.value++; loadLogs() } }
function gotoPage(p) { page.value = p; loadLogs() }

// ===== Lifecycle =====
onMounted(() => {
  loadActivity()
  loadLogs()
  window.addEventListener('resize', handleResize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chartInstance && chartInstance.dispose()
})

// 主题切换时重画
const themeObserver = ref(null)
onMounted(() => {
  themeObserver.value = new MutationObserver(() => renderChart())
  themeObserver.value.observe(document.documentElement, { attributes: true, attributeFilter: ['data-world'] })
})
onBeforeUnmount(() => themeObserver.value && themeObserver.value.disconnect())

watch(() => activity.value.daily, () => nextTick(renderChart), { deep: true })

// ===== 渲染辅助 =====
function fmtTime(ts) {
  if (!ts) return '-'
  const ms = typeof ts === 'number' ? ts * 1000 : ts
  return new Date(ms).toLocaleString('zh-CN', {
    month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
    hour12: false,
  })
}

// 反穿帮硬约束：必须保留 actual_model || original_model fallback
function modelOf(log) {
  return log.actual_model || log.original_model || '-'
}

function creditToUSD(credits, model) {
  if (!credits) return '0.00'
  const m = (model || '').toLowerCase()
  const isProPool = m.includes('opus') || (m.includes('sonnet') && (m.includes('4.6') || m.includes('4-6')))
  const pricePerCredit = isProPool ? 0.20 : 0.04
  return (credits * pricePerCredit).toFixed(4)
}

const items = computed(() => logs.value.map((log, i) => ({
  id: log.request_id || `log-${i}`,
  time: fmtTime(log.timestamp),
  status: log.status === 'error' ? 'danger' : 'success',
  raw: log,
})))

// 活跃度状态描述
const activityHeadline = computed(() => {
  const a = activity.value
  if (!a || activityLoading.value) return { value: '—', hint: '' }
  return {
    value: a.totalCalls,
    hint: a.totalErrors > 0 ? `含 ${a.totalErrors} 次失败` : '7 天累计',
  }
})

const tokenHeadline = computed(() => {
  const t = activity.value.totalTokens || 0
  if (t < 1000) return { value: t.toString(), unit: 'tk' }
  if (t < 1_000_000) return { value: (t / 1000).toFixed(1), unit: 'K' }
  return { value: (t / 1_000_000).toFixed(2), unit: 'M' }
})

const promoStatus = computed(() => {
  const p = activity.value.promotion
  if (!p || !p.active) return null
  const need = p.minRecentCalls
  const have = p.recentCalls
  const pct = need > 0 ? Math.min(100, Math.round((have / need) * 100)) : 0
  return {
    name: p.name || '当期活动',
    eligible: p.eligible,
    whitelisted: p.whitelisted,
    need, have, pct,
    days: p.recentCallsDays,
    proPrice: p.proPoolPriceUSD,
    freePrice: p.freePoolPriceUSD,
  }
})

const titleOfDate = computed(() => {
  if (selectedDate.value === 'today') return '今日调用'
  if (selectedDate.value === 'all') return '全部记录'
  return `${selectedDate.value} 调用`
})
</script>

<template>
  <div class="logs-page">
    <!-- 顶部 -->
    <header class="page-head">
      <div class="title-row">
        <div class="title-wrap">
          <div class="eyebrow">用户中心 · 日志</div>
          <h1 class="page-title">活跃度 &amp; 调用记录</h1>
        </div>
        <WorldChip variant="info" :dot="true">
          <Activity :size="11" />
          {{ activity.totalCalls }} 次 / 7 天
        </WorldChip>
      </div>
    </header>

    <!-- 顶部数据卡 -->
    <div class="stat-grid">
      <WorldStat
        label="7 天调用"
        :value="activityHeadline.value"
        unit="次"
        :hint="activityHeadline.hint"
        :icon="Activity"
        variant="primary"
      />
      <WorldStat
        label="7 天 Tokens"
        :value="tokenHeadline.value"
        :unit="tokenHeadline.unit"
        hint="input + output 累计"
        :icon="Database"
        variant="info"
      />
      <WorldStat
        label="7 天花费"
        :value="`$${(activity.totalCostUSD || 0).toFixed(2)}`"
        :hint="`≈ ¥${((activity.totalCostUSD || 0) * 0.05).toFixed(2)}（1¥=20$）`"
        :icon="Coins"
        variant="warning"
      />
      <WorldStat
        v-if="promoStatus"
        :label="`活动门槛 · ${promoStatus.name}`"
        :value="promoStatus.eligible ? '已达标' : `${promoStatus.have}/${promoStatus.need}`"
        :hint="promoStatus.eligible ?
          (promoStatus.whitelisted ? '白名单永久享受' : `${promoStatus.days} 天内调用满足`) :
          `还差 ${Math.max(0, promoStatus.need - promoStatus.have)} 次（${promoStatus.days} 天内）`"
        :icon="Sparkles"
        :variant="promoStatus.eligible ? 'success' : 'warning'"
      />
      <WorldStat
        v-else
        label="活动状态"
        value="未开放"
        hint="管理员开启活动后自动显示"
        :icon="Sparkles"
        variant="info"
      />
    </div>

    <!-- ECharts 7 天趋势 -->
    <WorldCard padding="md" class="chart-card">
      <div class="chart-head">
        <div class="chart-title">
          <TrendingUp :size="16" />
          <span>近 7 天活跃度趋势</span>
        </div>
        <div class="chart-meta">
          <span class="meta-pill"><span class="dot d-accent"></span>调用次数</span>
          <span class="meta-pill"><span class="dot d-success"></span>Tokens (K)</span>
        </div>
      </div>
      <div ref="chartEl" class="chart-canvas"></div>
    </WorldCard>

    <!-- 日期切换 -->
    <WorldCard padding="md" class="date-card">
      <div class="date-head">
        <Calendar :size="14" />
        <span>{{ titleOfDate }}</span>
        <span class="date-total">{{ total }} 条</span>
      </div>
      <div class="date-chips">
        <button
          v-for="c in dateChips"
          :key="c.key"
          @click="pickDate(c.key)"
          :class="['date-chip', { active: selectedDate === c.key, all: c.key === 'all' }]"
        >
          <span class="cl">{{ c.label }}</span>
          <span v-if="c.sub" class="cs">周{{ c.sub }}</span>
        </button>
      </div>
    </WorldCard>

    <!-- 日志列表 -->
    <WorldCard padding="md" v-if="loading">
      <div class="loading-state">
        <div v-for="i in 4" :key="i" class="skeleton-line" />
      </div>
    </WorldCard>

    <WorldCard padding="lg" v-else-if="!logs.length">
      <div class="empty-state">
        <div class="empty-icon"><FileX :size="32" /></div>
        <h4>{{ selectedDate === 'today' ? '今天还没有调用记录' : '当前日期没有调用记录' }}</h4>
        <p>{{ selectedDate === 'today' ? '快试试发起第一次请求吧' : '试试切换其他日期查看' }}</p>
      </div>
    </WorldCard>

    <WorldCard v-else padding="md">
      <WorldTimeline :items="items" empty-text="没有查询到日志">
        <template #title="{ item }">
          <span class="model-name">{{ modelOf(item.raw) }}</span>
        </template>
        <template #body="{ item }">
          <div class="log-meta">
            <span class="meta-item"><Database :size="12" />
              {{ (((item.raw.input_tokens || 0) + (item.raw.output_tokens || 0)) / 1000).toFixed(1) }}K Tokens
            </span>
            <span class="meta-item"><Coins :size="12" />
              ${{ item.raw.cost_usd ? item.raw.cost_usd.toFixed(4) : creditToUSD(item.raw.credits, modelOf(item.raw)) }}
            </span>
            <span class="meta-item" v-if="item.raw.duration_ms">
              <Timer :size="12" />{{ item.raw.duration_ms }}ms
            </span>
            <WorldChip
              :variant="item.raw.status === 'error' ? 'danger' : 'success'"
              :dot="true"
              size="sm"
            >
              <component :is="item.raw.status === 'error' ? XCircle : CheckCircle2" :size="11" />
              {{ item.raw.status === 'error' ? 'ERROR' : (item.raw.stop_reason || 'SUCCESS') }}
            </WorldChip>
          </div>
          <div v-if="item.raw.error" class="error-detail">{{ item.raw.error }}</div>
        </template>
      </WorldTimeline>
    </WorldCard>

    <!-- 分页 -->
    <div v-if="total > limit && !loading" class="pagination">
      <WorldButton variant="secondary" size="sm" :disabled="page <= 1" @click="prevPage">
        <ChevronLeft :size="14" /><span>上一页</span>
      </WorldButton>
      <span class="pg-info">第 {{ page }} / {{ totalPages }} 页 · 共 {{ total }} 条</span>
      <WorldButton variant="secondary" size="sm" :disabled="!hasMore" @click="nextPage">
        <span>下一页</span><ChevronRight :size="14" />
      </WorldButton>
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* === 头部 === */
.page-head { display: flex; flex-direction: column; gap: 12px; }
.title-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}

/* === 统计卡片网格 === */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 920px) { .stat-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 480px) { .stat-grid { grid-template-columns: 1fr; } }

/* === ECharts 卡片 === */
.chart-card { padding: 18px 20px; }
.chart-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 8px;
}
.chart-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 800;
  font-size: 0.95rem;
  color: var(--world-text-primary);
  letter-spacing: -0.01em;
}
.chart-title svg { color: var(--world-accent); }
.chart-meta {
  display: inline-flex;
  align-items: center;
  gap: 14px;
}
.meta-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--world-text-mute);
  letter-spacing: 0.04em;
}
.dot { width: 8px; height: 8px; border-radius: 2px; }
.d-accent  { background: var(--world-accent); }
.d-success { background: var(--world-success); }
.chart-canvas {
  width: 100%;
  height: 280px;
}
@media (max-width: 480px) { .chart-canvas { height: 220px; } }

/* === 日期切换器 === */
.date-card { padding: 14px 16px; }
.date-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
  font-size: 0.85rem;
  color: var(--world-text-primary);
  margin-bottom: 10px;
}
.date-head svg { color: var(--world-text-mute); }
.date-total {
  margin-left: auto;
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--world-text-mute);
  font-family: var(--world-font-mono);
  background: var(--world-overlay-light);
  padding: 3px 10px;
  border-radius: var(--world-radius-full);
}
.date-chips {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.date-chip {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1px;
  min-width: 56px;
  padding: 8px 10px;
  background: transparent;
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  color: var(--world-text-mute);
  font-family: var(--world-font-mono);
  cursor: pointer;
  transition: all 200ms ease;
}
.date-chip .cl { font-size: 0.78rem; font-weight: 800; letter-spacing: -0.01em; }
.date-chip .cs { font-size: 0.62rem; opacity: 0.7; letter-spacing: 0.06em; }
.date-chip:hover {
  border-color: var(--world-accent);
  color: var(--world-text-primary);
  transform: translateY(-1px);
}
.date-chip.active {
  background: var(--world-accent);
  border-color: var(--world-accent);
  color: #fff;
  box-shadow: 0 4px 14px -4px var(--world-accent);
}
.date-chip.active .cs { opacity: 0.85; }
.date-chip.all {
  background: transparent;
  border-style: dashed;
}
.date-chip.all.active {
  background: var(--world-text-primary);
  border-color: var(--world-text-primary);
  color: var(--world-bg-primary, #fff);
}

/* === 分页 === */
.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 6px 0;
}
.pg-info {
  font-size: 0.78rem;
  color: var(--world-text-mute);
  font-family: var(--world-font-mono);
}

/* === loading skeleton === */
.loading-state { display: flex; flex-direction: column; gap: 14px; padding: 4px 0; }
.skeleton-line {
  height: 60px;
  border-radius: var(--world-radius-md);
  background: linear-gradient(
    90deg,
    var(--world-overlay-light) 0%,
    var(--world-overlay-medium) 50%,
    var(--world-overlay-light) 100%
  );
  background-size: 200% 100%;
  animation: shimmer 1.4s linear infinite;
}
@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* === empty === */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 24px 12px;
  gap: 12px;
}
.empty-icon {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--world-overlay-light);
  color: var(--world-text-mute);
}
.empty-state h4 {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
  color: var(--world-text-primary);
}
.empty-state p {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
}

/* === log entries === */
.model-name {
  font-family: var(--world-font-mono);
  font-size: 0.875rem;
  font-weight: 700;
  color: var(--world-text-primary);
  letter-spacing: 0.02em;
}
[data-world="daogui"] .model-name {
  color: var(--world-paper-aged);
}
.log-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 4px;
}
.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.75rem;
  color: var(--world-text-mute);
  font-family: var(--world-font-mono);
}
.error-detail {
  margin-top: 8px;
  padding: 8px 10px;
  background: rgba(239, 68, 68, 0.08);
  border-left: 2px solid var(--world-error);
  border-radius: var(--world-radius-sm);
  font-size: 0.75rem;
  color: var(--world-error);
  font-family: var(--world-font-mono);
  word-break: break-all;
}
[data-world="daogui"] .error-detail {
  background: rgba(196, 30, 58, 0.10);
  color: #f5707f;
}
</style>
