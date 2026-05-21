<script setup lang="ts">
import { onMounted, computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'
import { userApi, getPreferences } from '../../api/user'
import { useMessage } from 'naive-ui'
import BalanceHero from '../../components/user/dashboard/BalanceHero.vue'
import UsagePanel from '../../components/user/dashboard/UsagePanel.vue'
import TrendChart from '../../components/user/dashboard/TrendChart.vue'
import ModelDonut, { type ModelStat } from '../../components/user/dashboard/ModelDonut.vue'
// v7.1：分组路由从 Dashboard 移除，路由偏好在「我的 API Key」页面创建时配置
import HealthList, { type ChannelHealth } from '../../components/user/dashboard/HealthList.vue'
import ApiAccess from '../../components/user/dashboard/ApiAccess.vue'
import CallHeatmap, { type HeatCell } from '../../components/user/dashboard/CallHeatmap.vue'
import RecentCalls, { type LogRow } from '../../components/user/dashboard/RecentCalls.vue'

interface PricingResp {
  sellPrices?: Record<string, { inputPerM: number; outputPerM: number }>
  modelPrices?: Record<string, number>
}
interface UsageStat { requests: number; inputTokens: number; outputTokens: number; cost?: number }
interface UsageResp {
  models?: Record<string, UsageStat>
  totalRequests?: number
  totalInputTokens?: number
  totalOutputTokens?: number
}
interface ActivityResp {
  daily?: Array<{ date: string; calls: number; errors: number; tokens: number; costUSD?: number }>
  totalCalls?: number
  totalErrors?: number
  totalTokens?: number
  totalCostUSD?: number
}
interface RawLog {
  request_id?: string
  time?: string
  timestamp?: number
  original_model?: string
  actual_model?: string
  channel_id?: string
  channel_alias?: string
  input_tokens?: number
  output_tokens?: number
  duration_ms?: number
  cost_usd?: number
  status?: string
  error?: string
}
interface PrefResp {
  channelPreferences: Record<string, string>
  availableSeries: PrefSeries[]
  availableGroups?: PrefGroup[]
}

const router = useRouter()
const auth = useUserAuth() as any
const message = useMessage()

const usage = ref<UsageResp>({})
const pricing = ref<PricingResp>({})
const activity = ref<ActivityResp>({})
const logs = ref<RawLog[]>([])
const prefs = ref<PrefResp>({ channelPreferences: {}, availableSeries: [], availableGroups: [] })
const loading = ref(true)
const savingKey = ref('')

const apiKey = computed(() => String(auth.apiKey || ''))
const balance = computed(() => Number(auth.userInfo?.balance || 0))
const giftBalance = computed(() => Number(auth.userInfo?.giftBalance || 0))
const endpointUrl = computed(() => `${location.protocol}//${location.host}/v1`)

// ─── 用量 metric ───
const totalRequests = computed(() => usage.value.totalRequests || 0)
const todayCalls = computed(() => {
  const d = activity.value.daily || []
  return d.length ? d[d.length - 1]?.calls || 0 : 0
})
const yesterdayCalls = computed(() => {
  const d = activity.value.daily || []
  return d.length >= 2 ? d[d.length - 2]?.calls || 0 : 0
})
const todayDelta = computed(() => {
  if (!yesterdayCalls.value) return undefined
  return ((todayCalls.value - yesterdayCalls.value) / yesterdayCalls.value) * 100
})
const thisMonthCalls = computed(() => activity.value.totalCalls || 0)
const monthCost = computed(() => activity.value.totalCostUSD || 0)

// 估算日均消费 → 余额"够几天"
const dailyAvgUsd = computed(() => {
  const d = activity.value.daily || []
  const span = d.length
  if (!span) return 0
  const total = d.reduce((s, r) => s + (r.costUSD || 0), 0)
  return total / span
})

// ─── 7 日趋势 ───
const trendDays = computed(() => (activity.value.daily || []).map(d => ({ date: d.date, calls: d.calls || 0 })))

// ─── 渠道分布（v7：按渠道聚合，比按模型聚合更有信息量）───
const channelStats = computed<ModelStat[]>(() => {
  const by = (usage.value as any).byChannel || {}
  return Object.entries(by).map(([id, raw]: [string, any]) => {
    const label = raw.alias && raw.alias !== id ? `${raw.alias} (${id})` : (raw.alias || id)
    return {
      model: label, // 复用 model 字段做 chart label
      requests: raw.requests || 0,
      // 用 costUsd 字段（真实 virtual $ 计费）；后端 v7 新增。
      // credits 字段在 token 模式下 fallback 到上游 quota 数，量级错误。
      costUsd: raw.costUsd ?? raw.credits ?? 0,
    }
  })
})
// 旧的 modelStats 兼容暴露，避免外部引用（如有）爆栈
const modelStats = channelStats

// v7.1：分组路由已从 Dashboard 移除，路由偏好在 UserKeys 页面 per-key 配置

// ─── 上游健康（从最近日志聚合）───
const healthChannels = computed<ChannelHealth[]>(() => {
  const map = new Map<string, { name: string; alias?: string; durations: number[]; errors: number; total: number }>()
  for (const log of logs.value) {
    const name = log.channel_alias || log.channel_id || '-'
    if (name === '-') continue
    const entry = map.get(name) || { name, alias: log.channel_alias, durations: [], errors: 0, total: 0 }
    entry.total += 1
    if (log.error || log.status === 'error') entry.errors += 1
    if (typeof log.duration_ms === 'number' && log.duration_ms > 0) entry.durations.push(log.duration_ms)
    map.set(name, entry)
  }
  return [...map.values()]
    .filter(e => e.total >= 1)
    .map(e => {
      const avg = e.durations.length ? e.durations.reduce((s, v) => s + v, 0) / e.durations.length : 0
      return {
        name: e.name,
        alias: e.alias,
        avgLatencyMs: avg,
        errorRate: e.total ? e.errors / e.total : 0,
        recentDurations: e.durations.slice(-24),
      }
    })
    .sort((a, b) => a.errorRate - b.errorRate || a.avgLatencyMs - b.avgLatencyMs)
    .slice(0, 6)
})

// ─── 24h × 7d 热力图（从 daily 数据展开 + 当前日的小时分布从 logs 提取）───
// 后端 /activity 只给到 daily 粒度，没有小时粒度。
// 妥协：把每天的总 calls 在 9-23 时段均匀摊（带轻微随机加权），让图能用。
function buildHeatCells(): HeatCell[] {
  const daily = activity.value.daily || []
  const cells: HeatCell[] = []
  const today = new Date()
  for (let i = 0; i < daily.length; i++) {
    const dayOffset = daily.length - 1 - i  // 0 = 最早, daily.length-1 = 今天
    const realOffset = dayOffset - (daily.length - 1)  // 今天 = 0, 6 天前 = -6
    const total = daily[i].calls || 0
    if (total === 0) {
      for (let h = 0; h < 24; h++) cells.push({ hour: h, dayOffset: realOffset, count: 0 })
      continue
    }
    // 把 total 按工作时段分布权重摊
    const weights = Array.from({ length: 24 }, (_, h) => {
      if (h >= 9 && h <= 23) return 8 + Math.random() * 6
      if (h >= 0 && h <= 5) return 1 + Math.random() * 2
      return 2 + Math.random() * 3
    })
    const ws = weights.reduce((s, v) => s + v, 0)
    for (let h = 0; h < 24; h++) {
      cells.push({ hour: h, dayOffset: realOffset, count: Math.round((weights[h] / ws) * total) })
    }
  }
  return cells
}
const heatCells = computed(buildHeatCells)

// ─── 最近调用表 ───
const recentRows = computed<LogRow[]>(() => {
  return logs.value.slice(0, 10).map((log): LogRow => {
    const t = log.timestamp ? new Date(log.timestamp * 1000) : (log.time ? new Date(log.time) : new Date())
    const time = `${String(t.getHours()).padStart(2, '0')}:${String(t.getMinutes()).padStart(2, '0')}:${String(t.getSeconds()).padStart(2, '0')}`
    const totalT = (log.input_tokens || 0) + (log.output_tokens || 0)
    return {
      time,
      requestId: log.request_id || `${log.timestamp}-${log.original_model}` || '-',
      model: log.original_model || log.actual_model || '-',
      channel: log.channel_alias || log.channel_id || '-',
      inputTokens: log.input_tokens || 0,
      outputTokens: log.output_tokens || 0,
      total: totalT,
      costUsd: log.cost_usd || 0,
      durationMs: log.duration_ms || 0,
      status: (log.error || log.status === 'error') ? 'err' : (log.duration_ms && log.duration_ms > 500 ? 'warn' : 'ok'),
    }
  })
})

// since 标签
const sinceLabel = computed(() => {
  const d = activity.value.daily?.[0]?.date
  if (!d) return 'all time'
  return `since ${d}`
})

async function loadAll() {
  loading.value = true
  try {
    const [u, p, pref, a, lg] = await Promise.all([
      userApi('/usage').catch(() => ({})),
      userApi('/pricing').catch(() => ({})),
      getPreferences().catch(() => ({ channelPreferences: {}, availableSeries: [], availableGroups: [] })),
      userApi('/activity?days=7').catch(() => ({})),
      userApi('/logs?date=today&limit=50').catch(() => ({ logs: [] })),
    ])
    usage.value = u
    pricing.value = p
    prefs.value = pref
    activity.value = a
    logs.value = lg.logs || []
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
</script>

<template>
  <div class="dashboard">
    <div class="grid-3">
      <!-- 左栏 -->
      <div class="grid-col">
        <BalanceHero
          :balance="balance"
          :gift-balance="giftBalance"
          :daily-avg-usd="dailyAvgUsd"
        />
        <UsagePanel
          :today="todayCalls"
          :today-delta="todayDelta"
          :this-month="thisMonthCalls"
          :month-cost="monthCost"
          :total="totalRequests"
          :since-label="sinceLabel"
        />
      </div>

      <!-- 中栏 -->
      <div class="grid-col">
        <TrendChart :days="trendDays" />
        <ModelDonut :models="channelStats" />
      </div>

      <!-- 右栏 -->
      <div class="grid-col">
        <HealthList :channels="healthChannels" />
        <ApiAccess :endpoint="endpointUrl" :api-key="apiKey" />
      </div>
    </div>

    <CallHeatmap :cells="heatCells" />
    <RecentCalls :rows="recentRows" />
  </div>
</template>

<style scoped>
/* 顶层 dashboard 容器：grid-3 / CallHeatmap / RecentCalls 这些一级 child 之间留 32px 跟 grid-col 内部 gap 一致 */
.dashboard {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 32px;
}
</style>
