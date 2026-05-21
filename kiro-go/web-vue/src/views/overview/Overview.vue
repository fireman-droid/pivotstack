<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { RefreshCw, Download, TrendingUp, TrendingDown, UserPlus } from 'lucide-vue-next'
import { api } from '../../api/admin'

// ─── data sources ───────────────────────────────────────────
interface StatusResp {
  accounts: number
  available: number
  totalRequests: number
  successRequests: number
  failedRequests: number
  totalTokens: number
  totalCredits: number
  uptime: number
}
interface CallLog {
  time?: string
  timestamp?: number
  request_id?: string
  original_model?: string
  api_key_id?: string
  channel_alias?: string
  channel_id?: string
  channel_type?: string
  input_tokens?: number
  output_tokens?: number
  cost_usd?: number
  status?: string
  error?: string
  duration_ms?: number
}

const router = useRouter()
const loading = ref(true)
const lastSyncAt = ref<number>(0)
const status = ref<StatusResp | null>(null)
const logs = ref<CallLog[]>([])
const keyNoteMap = ref<Record<string, string>>({})

const formatTime = (ts: number) =>
  ts ? new Date(ts * 1000).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' }) : '--:--'

async function reload() {
  loading.value = true
  try {
    const [statusRes, logsRes, keysRes] = await Promise.all([
      api('/status').then(r => r.json()).catch(() => null),
      api('/logs?limit=500').then(r => r.json()).catch(() => ({ logs: [] })),
      api('/apikeys').then(r => r.json()).catch(() => []),
    ])
    status.value = statusRes as StatusResp
    logs.value = (logsRes?.logs || []) as CallLog[]
    const map: Record<string, string> = {}
    for (const k of (keysRes || []) as Array<{ id: string; note: string }>) {
      if (k.id) map[k.id] = k.note || ''
    }
    keyNoteMap.value = map
    lastSyncAt.value = Math.floor(Date.now() / 1000)
  } catch {
    /* silent */
  } finally {
    loading.value = false
    await nextTick()
    renderCharts()
  }
}

// ─── derived metrics ────────────────────────────────────────
const todayStart = computed(() => {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return Math.floor(d.getTime() / 1000)
})

const todayLogs = computed(() =>
  logs.value.filter(l => (l.timestamp || 0) >= todayStart.value),
)

const todayCallCount = computed(() => todayLogs.value.length)
const todayCost = computed(() =>
  todayLogs.value.reduce((s, l) => s + (l.cost_usd || 0), 0),
)
const errorRate = computed(() => {
  const t = todayLogs.value.length
  if (!t) return 0
  const errs = todayLogs.value.filter(l => l.error || l.status === 'error').length
  return errs / t
})

const channelsOnline = computed(() => status.value?.available ?? 0)
const channelsTotal = computed(() => status.value?.accounts ?? 0)

// ─── revenue 7d ─────────────────────────────────────────────
const revenue7d = computed(() => {
  const buckets: Record<string, number> = {}
  const now = new Date()
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now)
    d.setHours(0, 0, 0, 0)
    d.setDate(d.getDate() - i)
    buckets[d.toISOString().slice(0, 10)] = 0
  }
  for (const l of logs.value) {
    if (!l.timestamp) continue
    const key = new Date(l.timestamp * 1000).toISOString().slice(0, 10)
    if (key in buckets) buckets[key] += (l.cost_usd || 0) * 7.2 // 粗略 ¥ 换算
  }
  return Object.entries(buckets).map(([date, val]) => ({ date, val }))
})

const revenueTotal7d = computed(() =>
  revenue7d.value.reduce((s, p) => s + p.val, 0),
)

// ─── calls + errors dual ────────────────────────────────────
const dual7d = computed(() => {
  const calls: Record<string, number> = {}
  const errs: Record<string, number> = {}
  const now = new Date()
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now)
    d.setHours(0, 0, 0, 0)
    d.setDate(d.getDate() - i)
    const k = d.toISOString().slice(0, 10)
    calls[k] = 0; errs[k] = 0
  }
  for (const l of logs.value) {
    if (!l.timestamp) continue
    const k = new Date(l.timestamp * 1000).toISOString().slice(0, 10)
    if (!(k in calls)) continue
    calls[k] += 1
    if (l.error || l.status === 'error') errs[k] += 1
  }
  const dates = Object.keys(calls)
  return { dates, calls: dates.map(d => calls[d]), errors: dates.map(d => errs[d]) }
})

// ─── channel distribution ───────────────────────────────────
// v7：从"按模型聚合"改为"按渠道聚合"——admin 关心哪条上游在扛大头，
// 而不是"用户调了什么模型"（那个在 ops/profit 的模型表已经能看到）。
// 渠道名优先 channel_alias，fallback channel_id；都没有就归 (legacy)。
const modelDist = computed(() => {
  const m: Record<string, number> = {}
  for (const l of logs.value) {
    const k = (l as any).channel_alias || (l as any).channel_id || '(legacy)'
    m[k] = (m[k] || 0) + 1
  }
  return Object.entries(m)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([name, count]) => ({ name, count }))
})

const modelTotal = computed(() => modelDist.value.reduce((s, p) => s + p.count, 0))

// ─── 24×7 heatmap ───────────────────────────────────────────
const heatmap = computed(() => {
  // [hour, weekday] count，weekday 周一=0 周日=6
  const data: number[][] = []
  const grid: number[][] = Array.from({ length: 24 }, () => Array(7).fill(0))
  for (const l of logs.value) {
    if (!l.timestamp) continue
    const d = new Date(l.timestamp * 1000)
    const h = d.getHours()
    const wd = (d.getDay() + 6) % 7
    grid[h][wd] += 1
  }
  for (let h = 0; h < 24; h++) {
    for (let w = 0; w < 7; w++) {
      data.push([w, h, grid[h][w]])
    }
  }
  return data
})

// ─── upstream channel health ────────────────────────────────
const channelHealth = computed(() => {
  // 暂时按 channel_alias 聚合 logs 算最近调用 + 错误率，不依赖后端 health endpoint
  const m: Record<string, { calls: number; errors: number; lastMs: number }> = {}
  for (const l of logs.value) {
    const name = l.channel_alias || l.channel_id || '(legacy)'
    if (!m[name]) m[name] = { calls: 0, errors: 0, lastMs: 0 }
    m[name].calls += 1
    if (l.error || l.status === 'error') m[name].errors += 1
    if (l.duration_ms && l.duration_ms > m[name].lastMs) m[name].lastMs = l.duration_ms
  }
  return Object.entries(m)
    .map(([name, v]) => ({
      name,
      latency: v.lastMs,
      errPct: v.calls ? v.errors / v.calls : 0,
      calls: v.calls,
    }))
    .sort((a, b) => b.calls - a.calls)
    .slice(0, 5)
})

// ─── TOP 5 users ────────────────────────────────────────────
const topUsers = computed(() => {
  const m: Record<string, { cost: number; calls: number }> = {}
  for (const l of todayLogs.value) {
    if (!l.api_key_id) continue
    if (!m[l.api_key_id]) m[l.api_key_id] = { cost: 0, calls: 0 }
    m[l.api_key_id].cost += l.cost_usd || 0
    m[l.api_key_id].calls += 1
  }
  return Object.entries(m)
    .map(([id, v]) => ({
      id,
      name: keyNoteMap.value[id] || id.slice(0, 10),
      cost: v.cost,
      calls: v.calls,
    }))
    .sort((a, b) => b.cost - a.cost)
    .slice(0, 5)
})

// ─── anomalies ─────────────────────────────────────────────
const anomalies = computed(() => {
  return logs.value
    .filter(l => l.error || l.status === 'error')
    .slice(0, 10)
    .map(l => ({
      time: l.timestamp ? new Date(l.timestamp * 1000).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' }) : '--:--',
      level: (l.error || '').toLowerCase().includes('rate') ? 'high' : 'med',
      message: l.error || '调用失败',
      ref: l.api_key_id || l.channel_alias || '-',
    }))
})

// ─── ECharts refs + render ──────────────────────────────────
const elRev = ref<HTMLDivElement | null>(null)
const elDual = ref<HTMLDivElement | null>(null)
const elHeat = ref<HTMLDivElement | null>(null)
let chRev: echarts.ECharts | null = null
let chDual: echarts.ECharts | null = null
let chHeat: echarts.ECharts | null = null

function disposeCharts() {
  ;[chRev, chDual, chHeat].forEach(c => c?.dispose())
  chRev = chDual = chHeat = null
}

function renderCharts() {
  disposeCharts()
  if (elRev.value) {
    chRev = echarts.init(elRev.value, 'stellar')
    chRev.setOption({
      grid: { left: 8, right: 8, top: 8, bottom: 20, containLabel: true },
      xAxis: {
        type: 'category',
        data: revenue7d.value.map(p => p.date.slice(5)),
        axisLabel: { color: '#707070', fontSize: 10 },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: '#707070', fontSize: 10, formatter: (v: number) => `¥${v.toFixed(0)}` },
        splitLine: { lineStyle: { color: 'rgba(255,255,255,0.04)' } },
      },
      series: [{
        type: 'line', smooth: true, symbol: 'circle', symbolSize: 4,
        data: revenue7d.value.map(p => Number(p.val.toFixed(2))),
        lineStyle: { width: 1.5, color: '#0bd470' },
        itemStyle: { color: '#0bd470' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(11,212,112,0.28)' },
            { offset: 1, color: 'rgba(11,212,112,0)' },
          ]),
        },
      }],
      tooltip: { trigger: 'axis' },
    })
  }
  if (elDual.value) {
    chDual = echarts.init(elDual.value, 'stellar')
    const { dates, calls, errors } = dual7d.value
    const errPcts = calls.map((c, i) => c ? (errors[i] / c) * 100 : 0)
    chDual.setOption({
      grid: { left: 8, right: 24, top: 24, bottom: 20, containLabel: true },
      legend: {
        data: ['调用量', '错误率%'],
        top: 0,
        textStyle: { color: '#a1a1a1', fontSize: 11 },
        icon: 'circle', itemWidth: 6, itemHeight: 6, itemGap: 16,
      },
      xAxis: { type: 'category', data: dates.map(d => d.slice(5)), axisLabel: { color: '#707070', fontSize: 10 } },
      yAxis: [
        { type: 'value', name: '', axisLabel: { color: '#707070', fontSize: 10 }, splitLine: { lineStyle: { color: 'rgba(255,255,255,0.04)' } } },
        { type: 'value', axisLabel: { color: '#707070', fontSize: 10, formatter: '{value}%' }, splitLine: { show: false } },
      ],
      tooltip: { trigger: 'axis' },
      series: [
        { name: '调用量', type: 'line', smooth: true, data: calls, lineStyle: { color: '#0bd470', width: 1.5 }, itemStyle: { color: '#0bd470' }, symbol: 'circle', symbolSize: 3 },
        { name: '错误率%', type: 'line', smooth: true, yAxisIndex: 1, data: errPcts.map(v => Number(v.toFixed(2))), lineStyle: { color: '#ff4d4d', width: 1.5 }, itemStyle: { color: '#ff4d4d' }, symbol: 'circle', symbolSize: 3 },
      ],
    })
  }
  if (elHeat.value) {
    chHeat = echarts.init(elHeat.value, 'stellar')
    const max = Math.max(1, ...heatmap.value.map(d => d[2]))
    chHeat.setOption({
      tooltip: {
        formatter: (p: any) => `${['一', '二', '三', '四', '五', '六', '日'][p.data[0]]} ${String(p.data[1]).padStart(2, '0')}:00 — ${p.data[2]} calls`,
      },
      grid: { left: 24, right: 8, top: 8, bottom: 24, containLabel: true },
      xAxis: {
        type: 'category',
        data: ['一', '二', '三', '四', '五', '六', '日'],
        splitArea: { show: false },
        axisLine: { show: false }, axisTick: { show: false },
        axisLabel: { color: '#707070', fontSize: 10 },
      },
      yAxis: {
        type: 'category',
        data: Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0')),
        splitArea: { show: false },
        axisLine: { show: false }, axisTick: { show: false },
        axisLabel: { color: '#707070', fontSize: 9 },
        inverse: true,
      },
      visualMap: {
        show: false,
        min: 0, max,
        inRange: { color: ['rgba(11,212,112,0.05)', 'rgba(11,212,112,0.25)', 'rgba(11,212,112,0.55)', '#0bd470'] },
      },
      series: [{
        type: 'heatmap',
        data: heatmap.value,
        itemStyle: { borderRadius: 2 },
        emphasis: { itemStyle: { shadowBlur: 6, shadowColor: 'rgba(11,212,112,0.6)' } },
      }],
    })
  }
}

function onResize() {
  ;[chRev, chDual, chHeat].forEach(c => c?.resize())
}

watch([revenue7d, dual7d, modelDist, heatmap], () => {
  if (!loading.value) nextTick(renderCharts)
})

onMounted(() => {
  reload()
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  disposeCharts()
  window.removeEventListener('resize', onResize)
})

// ─── helpers ───────────────────────────────────────────────
const colors5 = ['#0bd470', '#52a8ff', '#f5a623', '#ff7a7a', '#a1a1a1']
function topModelColor(i: number) { return colors5[i % colors5.length] }

function fmtMs(ms: number) {
  if (!ms) return '--'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}
function fmtPct(v: number) {
  return `${(v * 100).toFixed(2)}%`
}
function fmtMoneyUSD(v: number) {
  return `$${v.toFixed(2)}`
}
function gotoLogs() { router.push('/ops/call-logs') }
function gotoKeys() { router.push('/billing/keys') }
function exportCsv() {
  // 占位，预留接口
}
</script>

<template>
  <div class="admin-page">
    <!-- Page head -->
    <header class="page-head">
      <div>
        <div class="page-head__crumb"><b>DASHBOARD</b> / 总览</div>
        <div class="page-head__title">
          <div class="t-display-admin">运营总览</div>
          <div class="page-head__sub">所有上游渠道 · 所有 API key · 实时数据</div>
        </div>
      </div>
      <div class="page-head__right">
        <span class="page-head__live">
          <span class="dot dot--green dot--pulse" />
          <span class="t-label">LIVE</span>
          <span class="t-meta">· {{ formatTime(lastSyncAt) }} last sync</span>
        </span>
        <button class="ax-btn ax-btn--ghost" :disabled="loading" @click="reload">
          <RefreshCw :size="14" :class="{ 'is-spinning': loading }" />
          刷新
        </button>
        <button class="ax-btn ax-btn--ghost" @click="exportCsv">
          <Download :size="14" />
          导出
        </button>
      </div>
    </header>

    <!-- Metric strip -->
    <section class="metric-strip">
      <div class="metric-tile">
        <div class="metric-tile__label">今日调用</div>
        <div class="metric-tile__num">{{ todayCallCount.toLocaleString() }}</div>
        <div class="metric-tile__delta">
          <span class="ax-chip ax-chip--up">
            <TrendingUp :size="11" /> 实时
          </span>
          <span class="t-meta">vs 昨日</span>
        </div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">今日成本</div>
        <div class="metric-tile__num">{{ fmtMoneyUSD(todayCost) }}</div>
        <div class="metric-tile__delta">
          <span class="t-meta">≈ ¥{{ (todayCost * 7.2).toFixed(2) }} · 今日已扣</span>
        </div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">活跃 Keys</div>
        <div class="metric-tile__num">
          {{ topUsers.length }}<span class="sub" style="margin-left:6px">/ {{ Object.keys(keyNoteMap).length }}</span>
        </div>
        <div class="metric-tile__delta">
          <span class="ax-chip ax-chip--up"><UserPlus :size="11" />今日活跃</span>
        </div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">错误率</div>
        <div class="metric-tile__num">{{ fmtPct(errorRate) }}</div>
        <div class="metric-tile__delta">
          <span class="ax-chip" :class="errorRate < 0.01 ? 'ax-chip--up' : 'ax-chip--down'">
            <component :is="errorRate < 0.01 ? TrendingDown : TrendingUp" :size="11" />
            {{ errorRate < 0.01 ? '健康' : '需关注' }}
          </span>
          <span class="t-meta">今日平均</span>
        </div>
      </div>
    </section>

    <!-- 3-col grid -->
    <section class="admin-grid-3">
      <!-- 列 1：收入趋势 / 模型分布 -->
      <div class="col">
        <div class="tile tile--admin">
          <div class="tile__head tile__head--split">
            <div>
              <div class="t-h-admin">收入趋势</div>
              <div class="t-label tertiary">LAST 7 DAYS</div>
            </div>
            <div style="text-align:right">
              <div class="mono t-body-strong">¥{{ revenueTotal7d.toFixed(0) }}</div>
              <div class="t-label tertiary">7d 累计</div>
            </div>
          </div>
          <div ref="elRev" style="height:160px" />
        </div>

        <div class="tile tile--admin">
          <div class="tile__head"><span class="t-h-admin">渠道分布</span><span class="t-label tertiary">TOP 5 · 最近 500 调用</span></div>
          <div class="ch-bars">
            <div v-for="(m, i) in modelDist" :key="m.name" class="ch-bar">
              <span class="ch-bar__name" :title="m.name">{{ m.name }}</span>
              <span class="ch-bar__track">
                <span class="ch-bar__fill" :style="{
                  width: modelTotal ? `${(m.count / modelTotal) * 100}%` : '0%',
                  background: topModelColor(i),
                }" />
              </span>
              <span class="ch-bar__pct">{{ modelTotal ? Math.round((m.count / modelTotal) * 100) : 0 }}%</span>
              <span class="ch-bar__count">{{ m.count.toLocaleString() }}</span>
            </div>
            <div v-if="!modelDist.length" class="t-label tertiary" style="padding:24px 4px;text-align:center">暂无数据</div>
          </div>
        </div>
      </div>

      <!-- 列 2：调用量+错误率 / 24×7 热力 -->
      <div class="col">
        <div class="tile tile--admin">
          <div class="tile__head tile__head--split">
            <div>
              <div class="t-h-admin">调用量 + 错误率</div>
              <div class="t-label tertiary">7D · DUAL AXIS</div>
            </div>
          </div>
          <div ref="elDual" style="height:200px" />
        </div>

        <div class="tile tile--admin">
          <div class="tile__head tile__head--split">
            <div>
              <div class="t-h-admin">24h × 7d 调用热力</div>
              <div class="t-label tertiary">绿色深 = 高峰</div>
            </div>
            <div class="heatmap-legend">
              <span>少</span>
              <span class="heatmap-legend__swatches">
                <i style="background:rgba(11,212,112,0.05)" />
                <i style="background:rgba(11,212,112,0.18)" />
                <i style="background:rgba(11,212,112,0.40)" />
                <i style="background:rgba(11,212,112,0.70)" />
                <i style="background:#0bd470" />
              </span>
              <span>多</span>
            </div>
          </div>
          <div ref="elHeat" style="height:280px" />
        </div>
      </div>

      <!-- 列 3：上游 Channel 健康 / TOP 5 用户 -->
      <div class="col">
        <div class="tile tile--admin">
          <div class="tile__head tile__head--split">
            <div>
              <div class="t-h-admin">上游 Channel 健康</div>
              <div class="t-label tertiary">REAL-TIME · {{ channelHealth.length }} ACTIVE</div>
            </div>
          </div>
          <div class="health-list">
            <div v-for="h in channelHealth" :key="h.name" class="health-row">
              <span class="dot" :class="h.errPct < 0.01 ? 'dot--green' : h.errPct < 0.05 ? 'dot--warn' : 'dot--err'" />
              <span class="health-row__name">{{ h.name }}</span>
              <span class="health-row__val">{{ fmtMs(h.latency) }}</span>
              <span class="health-row__val" :style="h.errPct >= 0.05 ? 'color:#ff7a7a' : ''">{{ fmtPct(h.errPct) }}</span>
              <span class="health-row__val">{{ h.calls }}</span>
            </div>
            <div v-if="!channelHealth.length" class="t-label tertiary" style="padding:12px 4px">暂无调用</div>
          </div>
        </div>

        <div class="tile tile--admin">
          <div class="tile__head tile__head--split">
            <div>
              <div class="t-h-admin">TOP 5 用户</div>
              <div class="t-label tertiary">今日花费</div>
            </div>
            <button class="ax-btn ax-btn--ghost ax-btn--sm" @click="gotoKeys">全部 →</button>
          </div>
          <div class="topuser-list">
            <div v-for="(u, i) in topUsers" :key="u.id" class="topuser">
              <span class="topuser__rank">{{ i + 1 }}</span>
              <span class="topuser__name">{{ u.name }}</span>
              <span class="topuser__cost">{{ fmtMoneyUSD(u.cost) }}</span>
              <span class="topuser__sub">{{ u.calls.toLocaleString() }} calls</span>
            </div>
            <div v-if="!topUsers.length" class="t-label tertiary" style="padding:12px 4px">今日无调用</div>
          </div>
        </div>
      </div>
    </section>

    <!-- Anomalies (wide) -->
    <section class="tile tile--wide">
      <div class="tile__head tile__head--split">
        <div>
          <div class="t-h-admin">异常事件</div>
          <div class="t-label tertiary">24H · {{ anomalies.length }} EVENTS</div>
        </div>
        <button class="ax-btn ax-btn--ghost ax-btn--sm" @click="gotoLogs">全部异常 →</button>
      </div>
      <div class="atable">
        <div class="atable__head">
          <div style="width:60px">时间</div>
          <div style="width:60px">级别</div>
          <div style="flex:1">事件</div>
          <div style="width:160px">关联</div>
        </div>
        <div class="atable__body">
          <div v-for="(a, i) in anomalies" :key="i" class="atable__row" @click="gotoLogs">
            <div class="mono" style="width:60px;color:var(--st-text-ter)">{{ a.time }}</div>
            <div style="width:60px">
              <span class="atag" :class="a.level === 'high' ? 'atag--err' : 'atag--warn'">
                <span class="atag__dot" />{{ a.level.toUpperCase() }}
              </span>
            </div>
            <div style="flex:1">{{ a.message }}</div>
            <div style="width:160px">
              <span class="ax-chip ax-chip--mono">{{ a.ref }}</span>
            </div>
          </div>
          <div v-if="!anomalies.length" class="t-label tertiary" style="padding:16px 12px">最近 24h 无异常事件</div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* ax-btn 是 admin 专用按钮（避免污染全局），保持紧凑、不滥用 naive-ui */
.ax-btn {
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
.ax-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.08);
  border-color: var(--st-border-strong);
}
.ax-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.ax-btn--ghost {
  background: transparent;
  border-color: var(--st-border);
}
.ax-btn--ghost:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.06);
}
.ax-btn--sm { height: 26px; padding: 0 10px; font-size: 11px; }

.is-spinning { animation: ax-spin 0.8s linear infinite; }
@keyframes ax-spin { to { transform: rotate(360deg); } }

/* 渠道分布水平条状表 */
.ch-bars { display: flex; flex-direction: column; gap: 10px; padding: 4px 0; }
.ch-bar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(100px, 2fr) 40px 48px;
  align-items: center;
  gap: 10px;
  font-size: 12px;
}
.ch-bar__name {
  color: var(--st-text-pri);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ch-bar__track {
  display: block;
  width: 100%;
  height: 8px;
  background: rgba(255, 255, 255, 0.04);
  border-radius: 4px;
  overflow: hidden;
}
.ch-bar__fill {
  display: block;
  height: 100%;
  border-radius: 4px;
  transition: width 280ms ease;
}
.ch-bar__pct {
  font-family: var(--st-font-mono);
  font-variant-numeric: tabular-nums;
  text-align: right;
  color: var(--st-text-pri);
}
.ch-bar__count {
  font-family: var(--st-font-mono);
  font-variant-numeric: tabular-nums;
  text-align: right;
  color: var(--st-text-ter);
  font-size: 11px;
}
</style>
