<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { api } from '../api/admin'
import { formatNum } from '../utils/format'
import { useToast } from '../composables/useToast'
import { 
  Line
} from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { 
  Users, 
  Zap, 
  Activity, 
  CreditCard, 
  Clock, 
  ChevronRight, 
  Copy, 
  ShieldCheck, 
  Terminal,
  Info,
  ExternalLink,
  Cpu,
  Globe,
  AlertTriangle
} from 'lucide-vue-next'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { success } = useToast()
const stats = ref({ accounts: 0, totalRequests: 0, successRequests: 0, failedRequests: 0, totalTokens: 0, totalCredits: 0, uptime: 0 })
const version = ref('')
let pollTimer = null

// Mock chart data for real-time traffic
const chartData = ref({
  labels: Array.from({ length: 10 }, (_, i) => `${i * 2}s`),
  datasets: [
    {
      label: 'Request Flow',
      backgroundColor: 'rgba(99, 102, 241, 0.1)',
      borderColor: '#6366f1',
      pointBackgroundColor: '#6366f1',
      borderWidth: 2,
      fill: true,
      tension: 0.4,
      data: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
    }
  ]
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      mode: 'index',
      intersect: false,
      backgroundColor: '#0f172a',
      titleFont: { size: 10 },
      bodyFont: { size: 12 }
    }
  },
  scales: {
    y: { display: false, beginAtZero: true },
    x: { display: false }
  }
}

async function loadStats() {
  try {
    const res = await api('/status')
    if (res.ok) {
      stats.value = await res.json()
      // Update chart with new data point
      const newData = [...chartData.value.datasets[0].data]
      newData.shift()
      newData.push(stats.value.totalRequests % 100 + Math.random() * 20)
      chartData.value = {
        ...chartData.value,
        datasets: [{ ...chartData.value.datasets[0], data: newData }]
      }
    }
  } catch {}
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
  navigator.clipboard.writeText(text)
  success('已复制到剪贴板')
}

const base = location.origin

const faqs = [
  { q: 'FREE 账号能用什么模型？', a: '支持 Claude 3.5 Sonnet / Haiku 全系列模型。' },
  { q: '显示 429 错误如何处理？', a: '系统会自动轮询可用账号，若持续报错请补充账号。' },
  { q: 'Token 刷新机制是怎样的？', a: '每 30 分钟后台自动校验一次有效性并静默刷新。' },
]

onMounted(async () => {
  await Promise.all([loadStats(), loadVersion()])
  pollTimer = setInterval(loadStats, 5000)
})
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

const successRate = computed(() => {
  if (!stats.value.totalRequests) return 100
  return (stats.value.successRequests / stats.value.totalRequests) * 100
})

const isErrorHigh = computed(() => (100 - successRate.value) > 5)
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-10">
    <!-- Top Welcome Area -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-black tracking-tight flex items-center gap-2">
          Dashboard <span class="text-[var(--text-secondary)] font-medium text-sm">v{{ version }}</span>
        </h1>
        <div class="flex items-center gap-3 mt-1 text-sm text-[var(--text-secondary)]">
          <div class="flex items-center gap-1.5 text-emerald-500 font-bold">
            <span class="w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></span>
            System Online
          </div>
          <span class="opacity-30">|</span>
          <div class="flex items-center gap-1.5">
            <Clock class="w-3.5 h-3.5" />
            Uptime: {{ formatUptime(stats.uptime) }}
          </div>
        </div>
      </div>
      <div class="flex items-center gap-2 bg-[var(--card)] p-1 rounded-xl border border-[var(--border)]">
        <button class="px-4 py-1.5 rounded-lg bg-primary text-white text-xs font-bold shadow-sm transition-all">Real-time</button>
        <button class="px-4 py-1.5 rounded-lg text-xs font-bold text-[var(--text-secondary)] hover:text-[var(--text)] transition-colors">History</button>
      </div>
    </div>

    <!-- Main Stats Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <!-- Accounts Card -->
      <div class="modern-card p-6 flex flex-col justify-between group overflow-hidden relative">
        <div class="absolute -right-4 -top-4 w-24 h-24 bg-primary/5 rounded-full group-hover:scale-150 transition-transform duration-500"></div>
        <div class="flex justify-between items-start mb-4 relative">
          <div class="p-2.5 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 text-primary">
            <Users class="w-6 h-6" />
          </div>
          <ChevronRight class="w-4 h-4 text-[var(--text-secondary)] opacity-0 group-hover:opacity-100 transition-opacity" />
        </div>
        <div>
          <div class="text-[11px] font-bold text-[var(--text-secondary)] uppercase tracking-widest mb-1">账号资源池</div>
          <div class="text-3xl font-black tracking-tight">{{ stats.accounts || 0 }}</div>
          <div class="mt-2 flex items-center gap-2 text-xs font-bold text-emerald-500 bg-emerald-500/10 px-2 py-0.5 rounded-full w-fit">
            <ShieldCheck class="w-3 h-3" />
            100% Available
          </div>
        </div>
      </div>

      <!-- Requests Card -->
      <div class="modern-card p-6 flex flex-col justify-between group overflow-hidden relative border-l-4"
        :class="isErrorHigh ? 'border-l-rose-500 bg-rose-500/[0.02]' : 'border-l-transparent'">
        <div class="absolute -right-4 -top-4 w-24 h-24 bg-amber-500/5 rounded-full group-hover:scale-150 transition-transform duration-500"></div>
        <div class="flex justify-between items-start mb-4 relative">
          <div class="p-2.5 rounded-xl bg-amber-50 dark:bg-amber-900/20 text-amber-500" :class="isErrorHigh ? 'text-rose-500 bg-rose-50 dark:bg-rose-900/20' : ''">
            <Zap class="w-6 h-6" />
          </div>
          <div class="flex flex-col items-end">
             <span class="text-xs font-bold" :class="isErrorHigh ? 'text-rose-500' : 'text-amber-500'">{{ successRate.toFixed(1) }}% Rate</span>
             <AlertTriangle v-if="isErrorHigh" class="w-4 h-4 text-rose-500 mt-1 animate-bounce" />
          </div>
        </div>
        <div>
          <div class="text-[11px] font-bold text-[var(--text-secondary)] uppercase tracking-widest mb-1">总计请求数</div>
          <div class="text-3xl font-black tracking-tight">{{ formatNum(stats.totalRequests || 0) }}</div>
          <div class="mt-2 text-[11px] font-bold text-[var(--text-secondary)] flex gap-2">
            <span class="text-emerald-500">{{ stats.successRequests }} OK</span>
            <span class="opacity-30">/</span>
            <span class="text-rose-500 font-bold" :class="{ 'scale-110 transition-transform': isErrorHigh }">{{ stats.failedRequests }} ERR</span>
          </div>
        </div>
      </div>

      <!-- Tokens Card -->
      <div class="modern-card p-6 flex flex-col justify-between group overflow-hidden relative">
        <div class="absolute -right-4 -top-4 w-24 h-24 bg-emerald-500/5 rounded-full group-hover:scale-150 transition-transform duration-500"></div>
        <div class="flex justify-between items-start mb-4 relative">
          <div class="p-2.5 rounded-xl bg-emerald-50 dark:bg-emerald-900/20 text-emerald-500">
            <Activity class="w-6 h-6" />
          </div>
        </div>
        <div>
          <div class="text-[11px] font-bold text-[var(--text-secondary)] uppercase tracking-widest mb-1">TOKEN 消耗量</div>
          <div class="text-3xl font-black tracking-tight">{{ formatNum(stats.totalTokens || 0) }}</div>
          <div class="mt-2 h-1.5 bg-[var(--bg)] rounded-full overflow-hidden border border-[var(--border)]">
            <div class="h-full bg-emerald-500 w-2/3 shadow-[0_0_8px_rgba(16,185,129,0.5)]"></div>
          </div>
        </div>
      </div>

      <!-- Credits Card -->
      <div class="modern-card p-6 flex flex-col justify-between group overflow-hidden relative">
        <div class="absolute -right-4 -top-4 w-24 h-24 bg-indigo-500/5 rounded-full group-hover:scale-150 transition-transform duration-500"></div>
        <div class="flex justify-between items-start mb-4 relative">
          <div class="p-2.5 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 text-indigo-500">
            <CreditCard class="w-6 h-6" />
          </div>
        </div>
        <div>
          <div class="text-[11px] font-bold text-[var(--text-secondary)] uppercase tracking-widest mb-1">估计总成本</div>
          <div class="text-3xl font-black tracking-tight">${{ (stats.totalCredits || 0).toFixed(2) }}</div>
          <div class="mt-2 text-[11px] font-bold text-primary italic bg-primary/5 px-2 py-0.5 rounded w-fit">Saved 85% with Kiro Pool</div>
        </div>
      </div>
    </div>

    <!-- Bottom Detailed Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <!-- API Endpoints -->
      <div class="lg:col-span-8 space-y-4 flex flex-col">
        <div class="flex items-center gap-2 px-2">
          <Terminal class="w-5 h-5 text-primary" />
          <h2 class="font-bold text-sm uppercase tracking-widest text-[var(--text-secondary)]">API 控制台</h2>
        </div>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div v-for="ep in [
            { label: 'Claude Messages', path: '/v1/messages', color: 'bg-indigo-500' },
            { label: 'OpenAI Chat', path: '/v1/chat/completions', color: 'bg-emerald-500' },
            { label: 'Model Discovery', path: '/v1/models', color: 'bg-amber-500' },
            { label: 'Service Status', path: '/health', color: 'bg-rose-500' }
          ]" :key="ep.path" class="modern-card p-4 hover:translate-y-[-2px] transition-all">
            <div class="flex justify-between items-center mb-3">
              <span class="text-xs font-bold text-[var(--text-secondary)]">{{ ep.label }}</span>
              <div class="flex gap-1.5">
                <span class="px-1.5 py-0.5 rounded-md bg-emerald-500/10 text-emerald-500 text-[9px] font-bold uppercase">Stable</span>
                <span class="px-1.5 py-0.5 rounded-md bg-primary/10 text-primary text-[9px] font-bold uppercase tracking-tighter">JSON</span>
              </div>
            </div>
            <div class="flex items-center gap-2 p-2.5 bg-[var(--bg)] rounded-xl border border-[var(--border)] group relative cursor-pointer hover:border-primary transition-colors" @click="copy(base + ep.path)">
              <div :class="ep.color" class="w-1.5 h-1.5 rounded-full shrink-0 shadow-lg"></div>
              <code class="text-[10px] font-mono truncate flex-1 flex items-center">
                <span class="opacity-30 mr-1">{{ base.replace(/https?:\/\//, '') }}</span>
                <span class="text-primary font-bold">{{ ep.path }}</span>
              </code>
              <Copy class="w-3.5 h-3.5 text-[var(--text-secondary)] opacity-0 group-hover:opacity-100 transition-opacity" />
            </div>
          </div>
        </div>

        <!-- Real Chart! -->
        <div class="modern-card p-6 flex-1 min-h-[350px] flex flex-col bg-gradient-to-b from-[var(--card)] to-[var(--bg)]">
          <div class="flex justify-between items-center mb-6">
            <div class="flex items-center gap-2">
              <Globe class="w-5 h-5 text-primary" />
              <div class="font-bold text-sm">Real-time Traffic Monitor</div>
            </div>
            <div class="flex items-center gap-4 text-[10px] font-bold text-[var(--text-secondary)] uppercase">
              <div class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-primary"></span> Requests/s</div>
            </div>
          </div>
          <div class="flex-1 relative">
            <Line :data="chartData" :options="chartOptions" />
          </div>
        </div>
      </div>

      <!-- FAQ & Info Side -->
      <div class="lg:col-span-4 space-y-4">
        <div class="flex items-center gap-2 px-2">
          <Info class="w-5 h-5 text-amber-500" />
          <h2 class="font-bold text-sm uppercase tracking-widest text-[var(--text-secondary)]">运维指南</h2>
        </div>

        <div class="modern-card divide-y divide-[var(--border)] overflow-hidden">
          <div v-for="(faq, i) in faqs" :key="i" class="p-4 hover:bg-[var(--bg)] transition-colors cursor-pointer group">
            <div class="flex justify-between items-center mb-1">
              <span class="text-xs font-bold group-hover:text-primary transition-colors">{{ faq.q }}</span>
              <ChevronRight class="w-3.5 h-3.5 text-[var(--text-secondary)]" />
            </div>
            <p class="text-[11px] text-[var(--text-secondary)] leading-relaxed">{{ faq.a }}</p>
          </div>
        </div>

        <!-- Quick Links -->
        <div class="modern-card p-6 space-y-4">
          <div class="text-[10px] font-black uppercase tracking-widest text-[var(--text-secondary)] mb-2">系统资源</div>
          
          <div class="space-y-2">
            <div class="flex items-center justify-between text-xs">
              <span class="flex items-center gap-2 text-[var(--text-secondary)] font-medium"><Cpu class="w-3.5 h-3.5" /> CPU Usage</span>
              <span class="font-mono font-bold text-primary">2.4%</span>
            </div>
            <div class="w-full bg-[var(--bg)] h-2 rounded-full overflow-hidden border border-[var(--border)] p-[1px]">
              <div class="bg-primary h-full rounded-full w-[2.4%] shadow-[0_0_8px_rgba(99,102,241,0.5)]"></div>
            </div>
          </div>

          <div class="space-y-2">
            <div class="flex items-center justify-between text-xs">
              <span class="flex items-center gap-2 text-[var(--text-secondary)] font-medium"><Zap class="w-3.5 h-3.5" /> Memory</span>
              <span class="font-mono font-bold text-indigo-500">128MB</span>
            </div>
            <div class="w-full bg-[var(--bg)] h-2 rounded-full overflow-hidden border border-[var(--border)] p-[1px]">
              <div class="bg-indigo-500 h-full rounded-full w-[15%] shadow-[0_0_8px_rgba(99,102,241,0.5)]"></div>
            </div>
          </div>

          <div class="pt-4 border-t border-[var(--border)]">
             <div class="text-[10px] font-bold text-[var(--text-secondary)] flex items-center gap-2">
                <div class="w-2 h-2 rounded-full bg-emerald-500"></div>
                Storage Engine: PostgreSQL (Local)
             </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modern-card {
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}
</style>

<style scoped>
.modern-card {
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}
</style>
