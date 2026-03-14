<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from 'vue'
import * as echarts from 'echarts'
import abyssTheme from '@/lib/echarts-abyss-theme.json'
import { api } from '../api/admin'
import { formatNum } from '../utils/format'
import { useWorldTheme } from '../stores/worldTheme'
import { useToast } from '../composables/useToast'
import CopperCoinLoader from '../components/ui/CopperCoinLoader.vue'
import BloodSplashButton from '../components/ui/BloodSplashButton.vue'
import { 
  Users, Zap, Activity, CreditCard, Clock, 
  Copy, Terminal, Globe, AlertTriangle, Crown
} from 'lucide-vue-next'

// 注册 ECharts abyss 主题
echarts.registerTheme('abyss', abyssTheme)

const { success } = useToast()
const stats = ref({ accounts: 0, totalRequests: 0, successRequests: 0, failedRequests: 0, totalTokens: 0, totalCredits: 0, uptime: 0, freePool: { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 }, proPool: { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 } })
const version = ref('')
const loading = ref(true)

const theme = useWorldTheme()

// ECharts 实例
const chartRef = ref(null)
let chart = null
const requestHistory = ref([])
const chartIncrements = ref([0, 0, 0, 0, 0, 0, 0, 0, 0, 0])

function initChart() {
  if (!chartRef.value) return
  chart = echarts.init(chartRef.value, theme.currentWorld === 'daogui' ? 'abyss' : null)
  updateChart()
  window.addEventListener('resize', () => chart?.resize())
}

watch(() => theme.currentWorld, (newVal) => {
  if (chart) {
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
  const areaStart = isDaogui ? 'rgba(196, 30, 58, 0.3)' : 'rgba(2, 132, 199, 0.3)'
  const areaEnd = isDaogui ? 'rgba(196, 30, 58, 0)' : 'rgba(2, 132, 199, 0)'
  const textColor = isDaogui ? '#9ca3af' : '#475569'
  const tooltipBg = isDaogui ? 'rgba(10, 10, 10, 0.9)' : 'rgba(255, 255, 255, 0.9)'
  const tooltipText = isDaogui ? '#e5e5e5' : '#0f172a'
  const tooltipBorder = isDaogui ? '#b8860b' : '#e2e8f0'
  const splitLineColor = isDaogui ? 'rgba(74, 26, 74, 0.3)' : 'rgba(226, 232, 240, 1)'

  chart.setOption({
    grid: { top: 10, right: 10, bottom: 25, left: 40 },
    xAxis: {
      type: 'category',
      data: Array.from({ length: 10 }, (_, i) => `${(9 - i) * 5}s`),
      axisLabel: { color: textColor, fontSize: 10 }
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: textColor, fontSize: 10 },
      splitLine: { lineStyle: { type: 'dashed', color: splitLineColor } }
    },
    series: [{
      type: 'line',
      data: chartIncrements.value,
      smooth: true,
      showSymbol: false,
      itemStyle: { color: accentColor },
      lineStyle: {
        width: 3,
        color: accentColor,
        shadowBlur: 10,
        shadowColor: shadowColor,
        shadowOffsetX: 2,
        shadowOffsetY: 2
      },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: areaStart },
            { offset: 1, color: areaEnd }
          ]
        }
      }
    }],
    tooltip: {
      trigger: 'axis',
      backgroundColor: tooltipBg,
      borderColor: tooltipBorder,
      textStyle: { color: tooltipText, fontSize: 12 }
    }
  })
}

async function loadStats() {
  try {
    const res = await api('/status')
    if (res.ok) {
      const newStats = await res.json()
      processStats(newStats)
    }
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
  nextTick(() => {
    if (!chart) initChart()
    else updateChart()
  })
}

let sseSource = null
let pollTimer = null

function connectStatsSSE() {
  const password = document.cookie.match(/admin_password=([^;]+)/)?.[1] || ''
  const url = `${location.origin}/admin/api/sse/stats?password=${encodeURIComponent(password)}`
  sseSource = new EventSource(url)
  
  sseSource.addEventListener('stats', (e) => {
    try {
      const newStats = JSON.parse(e.data)
      processStats(newStats)
    } catch {}
  })
  
  sseSource.onerror = () => {
    // SSE 断开，回退到 HTTP 轮询
    sseSource.close()
    sseSource = null
    if (!pollTimer) {
      pollTimer = setInterval(loadStats, 5000)
    }
    // 5 秒后尝试重连 SSE
    setTimeout(() => {
      if (!sseSource) {
        if (pollTimer) {
          clearInterval(pollTimer)
          pollTimer = null
        }
        connectStatsSSE()
      }
    }, 5000)
  }
}

async function loadVersion() {
  try {
    const res = await api('/version')
    if (res.ok) { const d = await res.json(); version.value = d.version || '' }
  } catch {}
}

function formatUptime(s) {
  if (!s) return '0s'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

function copy(text) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text)
  } else {
    const ta = document.createElement('textarea')
    ta.value = text; ta.style.cssText = 'position:fixed;left:-9999px'
    document.body.appendChild(ta); ta.select()
    document.execCommand('copy'); document.body.removeChild(ta)
  }
  success('已复制到剪贴板')
}

const base = location.origin

onMounted(async () => {
  await loadVersion()
  connectStatsSSE()
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
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-10">
    <!-- Loading -->
    <CopperCoinLoader v-if="loading" />

    <template v-else>
      <!-- Top -->
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 class="text-2xl font-black tracking-tight flex items-center gap-2 text-[var(--text)]">
            控制台 <span class="text-[var(--text)]-secondary font-medium text-sm">v{{ version }}</span>
          </h1>
          <div class="flex items-center gap-3 mt-1 text-sm text-[var(--text)]-secondary">
            <div class="flex items-center gap-1.5 text-[var(--world-accent-alt)] font-bold">
              <span class="w-2 h-2 bg-[var(--world-accent-alt)] rounded-full animate-pulse shadow-md"></span>
              系统在线
            </div>
            <span class="opacity-20 text-[var(--text)]-secondary">|</span>
            <div class="flex items-center gap-1.5">
              <Clock class="w-3.5 h-3.5" />
              运行时长: {{ formatUptime(stats.uptime) }}
            </div>
          </div>
        </div>
      </div>

      <!-- Stats Grid -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <!-- FREE 号池 -->
        <div class="modern-card p-6 flex flex-col justify-between group blood-glow-hover">
          <div class="flex justify-between items-start mb-4">
            <div class="p-2.5 rounded-xl bg-[var(--world-accent-alt)]/10 text-[var(--world-accent-alt)]">
              <Users class="w-6 h-6" />
            </div>
            <span class="text-[9px] font-bold tracking-[0.15em] text-green-500 bg-green-500/10 px-2 py-0.5 rounded-full uppercase">FREE</span>
          </div>
          <div>
            <div class="text-[10px] font-bold text-[var(--world-accent-alt)] uppercase tracking-[0.2em] mb-1">普通号池</div>
            <div class="text-3xl font-black tracking-tight text-[var(--text)]">{{ stats.freePool?.total || 0 }}</div>
            <div class="mt-2 text-[11px] font-bold flex gap-2">
              <span class="text-[var(--world-accent-alt)]">{{ stats.freePool?.available || 0 }} 可用</span>
              <span class="opacity-20 text-[var(--text)]-secondary">|</span>
              <span class="text-[var(--primary)]">{{ (stats.freePool?.usageCurrent || 0) + (stats.freePool?.trialCurrent || 0) }}/{{ (stats.freePool?.usageLimit || 0) + (stats.freePool?.trialLimit || 0) }}</span>
            </div>
          </div>
        </div>

        <!-- PRO 号池 -->
        <div class="modern-card p-6 flex flex-col justify-between group blood-glow-hover">
          <div class="flex justify-between items-start mb-4">
            <div class="p-2.5 rounded-xl bg-purple-500/10 text-purple-500">
              <Crown class="w-6 h-6" />
            </div>
            <span class="text-[9px] font-bold tracking-[0.15em] text-purple-500 bg-purple-500/10 px-2 py-0.5 rounded-full uppercase">PRO</span>
          </div>
          <div>
            <div class="text-[10px] font-bold text-purple-400 uppercase tracking-[0.2em] mb-1">PRO 号池</div>
            <!-- 试用配额（如果有） -->
            <div v-if="stats.proPool?.trialLimit > 0" class="mb-2">
              <div class="flex justify-between text-[10px] mb-0.5">
                <span class="font-bold text-amber-400">✨ 试用配额</span>
                <span class="text-[var(--text)]/50">{{ (stats.proPool?.trialCurrent || 0).toFixed(0) }} / {{ (stats.proPool?.trialLimit || 0).toFixed(0) }}</span>
              </div>
              <div class="w-full h-1.5 bg-[var(--border)] rounded-full overflow-hidden">
                <div class="h-full bg-gradient-to-r from-amber-400 to-orange-400 rounded-full transition-all"
                     :style="{ width: stats.proPool?.trialLimit ? `${Math.min((stats.proPool.trialCurrent / stats.proPool.trialLimit) * 100, 100)}%` : '0%' }"></div>
              </div>
            </div>
            <!-- 主配额 -->
            <div v-if="stats.proPool?.usageLimit > 0" class="mb-2">
              <div class="flex justify-between text-[10px] mb-0.5">
                <span class="font-bold text-purple-400">主配额</span>
                <span class="text-[var(--text)]/50">{{ (stats.proPool?.usageCurrent || 0).toFixed(0) }} / {{ (stats.proPool?.usageLimit || 0).toFixed(0) }}</span>
              </div>
              <div class="w-full h-1.5 bg-[var(--border)] rounded-full overflow-hidden">
                <div class="h-full bg-gradient-to-r from-purple-500 to-blue-500 rounded-full transition-all"
                     :style="{ width: stats.proPool?.usageLimit ? `${Math.min((stats.proPool.usageCurrent / stats.proPool.usageLimit) * 100, 100)}%` : '0%' }"></div>
              </div>
            </div>
            <div class="text-[11px] font-bold flex gap-2">
              <span class="text-purple-400">{{ stats.proPool?.available || 0 }} 可用</span>
              <span class="opacity-20 text-[var(--text)]-secondary">|</span>
              <span class="text-purple-400">{{ stats.proPool?.total || 0 }} 总号</span>
            </div>
            <div v-if="stats.prediction?.sufficient" class="mt-2 text-[10px] font-bold flex items-center gap-1.5 text-purple-300/80">
              <span>⏱</span>
              <span v-if="stats.prediction.remainingDays >= 1">预计可用 {{ Math.floor(stats.prediction.remainingDays) }}天</span>
              <span v-else-if="stats.prediction.remainingHours >= 1">活跃可用 {{ Math.floor(stats.prediction.remainingHours) }}h{{ Math.floor((stats.prediction.remainingHours % 1) * 60) }}m</span>
              <span v-else>额度即将用尽!</span>
              <span class="opacity-40 ml-1">({{ stats.prediction.avgPerRequest?.toFixed(2) }} cr/次)</span>
            </div>
            <div v-else class="mt-2 text-[10px] opacity-40">⏱ 需要更多请求数据用于预测</div>
          </div>
        </div>

        <!-- 请求统计 -->
        <div class="modern-card p-6 flex flex-col justify-between group"
          :class="isErrorHigh ? 'border-l-2 border-l-[var(--primary)]' : ''">
          <div class="flex justify-between items-start mb-4">
            <div class="p-2.5 rounded-xl" :class="isErrorHigh ? 'bg-[var(--primary)]/15 text-[var(--primary)]' : 'bg-[var(--world-accent-alt)]/10 text-[var(--world-accent-alt)]'">
              <Zap class="w-6 h-6" />
            </div>
            <div class="flex flex-col items-end">
              <span class="text-xs font-bold" :class="isErrorHigh ? 'text-[var(--primary)]' : 'text-[var(--world-accent-alt)]'">{{ successRate.toFixed(1) }}% 成功率</span>
              <AlertTriangle v-if="isErrorHigh" class="w-4 h-4 text-[var(--primary)] mt-1 animate-bounce" />
            </div>
          </div>
          <div>
            <div class="text-[10px] font-bold text-[var(--world-accent-alt)] uppercase tracking-[0.2em] mb-1">总计请求数</div>
            <div class="text-3xl font-black tracking-tight text-[var(--text)]">{{ formatNum(stats.totalRequests || 0) }}</div>
            <div class="mt-2 text-[11px] font-bold flex gap-2">
              <span class="text-[var(--world-accent-alt)]">{{ stats.successRequests }} 成功</span>
              <span class="opacity-20 text-[var(--text)]-secondary">/</span>
              <span class="text-[var(--primary)] font-bold">{{ stats.failedRequests }} 失败</span>
            </div>
          </div>
        </div>

        <!-- 总成本概览 -->
        <div class="modern-card p-6 flex flex-col justify-between group">
          <div class="flex justify-between items-start mb-4">
            <div class="p-2.5 rounded-xl bg-text-secondary/20 text-[var(--primary)]">
              <CreditCard class="w-6 h-6" />
            </div>
          </div>
          <div>
            <div class="text-[10px] font-bold text-[var(--world-accent-alt)] uppercase tracking-[0.2em] mb-1">总 Credits 概览</div>
            <div class="text-3xl font-black tracking-tight text-[var(--text)]">{{ (stats.freePool?.usageCurrent || 0) + (stats.freePool?.trialCurrent || 0) + (stats.proPool?.usageCurrent || 0) + (stats.proPool?.trialCurrent || 0) }} / {{ (stats.freePool?.usageLimit || 0) + (stats.freePool?.trialLimit || 0) + (stats.proPool?.usageLimit || 0) + (stats.proPool?.trialLimit || 0) }}</div>
            <div class="mt-2 text-[10px] font-bold flex gap-3">
              <span class="text-green-400">FREE {{ (stats.freePool?.usageCurrent || 0) + (stats.freePool?.trialCurrent || 0) }}/{{ (stats.freePool?.usageLimit || 0) + (stats.freePool?.trialLimit || 0) }}</span>
              <span class="text-purple-400">PRO {{ (stats.proPool?.usageCurrent || 0) + (stats.proPool?.trialCurrent || 0) }}/{{ (stats.proPool?.usageLimit || 0) + (stats.proPool?.trialLimit || 0) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Bottom -->
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
        <div class="lg:col-span-12 space-y-4 flex flex-col">
          <div class="flex items-center gap-2 px-2">
            <Terminal class="w-5 h-5 text-[var(--primary)]" />
            <h2 class="font-bold text-[10px] uppercase tracking-[0.2em] text-[var(--world-accent-alt)]">API 控 制 台</h2>
          </div>
          
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div v-for="ep in [
              { label: 'Claude Messages', path: '/v1/messages', color: 'bg-[var(--primary)]' },
              { label: 'OpenAI Chat', path: '/v1/chat/completions', color: 'bg-[var(--world-accent-alt)]' },
              { label: '模型发现', path: '/v1/models', color: 'bg-text-secondary' },
              { label: '服务状态', path: '/health', color: 'bg-[var(--primary)]' }
            ]" :key="ep.path" class="modern-card p-4 hover:translate-y-[-2px] transition-all">
              <div class="flex justify-between items-center mb-3">
                <span class="text-xs font-bold text-[var(--text)]-secondary">{{ ep.label }}</span>
                <span class="px-1.5 py-0.5 rounded-md bg-[var(--world-accent-alt)]/10 text-[var(--world-accent-alt)] text-[9px] font-bold uppercase">稳</span>
              </div>
              <div class="flex items-center gap-2 p-2.5 bg-[var(--bg)]/60 rounded-xl border border-[var(--border)] group relative cursor-pointer hover:border-[var(--primary)]/30 transition-colors" @click="copy(base + ep.path)">
                <div :class="ep.color" class="w-1.5 h-1.5 rounded-full shrink-0 shadow-lg"></div>
                <code class="text-[10px] font-mono truncate flex-1 flex items-center text-[var(--text)]-secondary">
                  <span class="opacity-30 mr-1">{{ base.replace(/https?:\/\//, '') }}</span>
                  <span class="text-[var(--primary)] font-bold">{{ ep.path }}</span>
                </code>
                <Copy class="w-3.5 h-3.5 text-[var(--text)]-secondary opacity-0 group-hover:opacity-100 transition-opacity" />
              </div>
            </div>
          </div>

          <!-- ECharts 实时流量监控 -->
          <div class="modern-card p-6 flex-1 min-h-[350px] flex flex-col">
            <div class="flex justify-between items-center mb-6">
              <div class="flex items-center gap-2">
                <Globe class="w-5 h-5 text-[var(--primary)]" />
                <div class="font-bold text-sm text-[var(--text)]">实时流量监控</div>
              </div>
              <div class="flex items-center gap-4 text-[10px] font-bold text-[var(--text)]-secondary uppercase">
                <div class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-[var(--primary)]"></span> 请求增量</div>
              </div>
            </div>
            <div ref="chartRef" class="flex-1 min-h-[280px]"></div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
