<script setup>
import { onMounted, computed, ref, watch } from 'vue'
import { useWorldTheme } from '../stores/worldTheme'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Filler
} from 'chart.js'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../composables/useToast'
import { api } from '../api/admin'
import {
  Plus,
  Search,
  RotateCw,
  Users,
  Activity,
  ShieldAlert,
  CheckCircle2,
  PackageSearch,
  LayoutGrid,
  List,
  Sparkles,
  X,
  Cpu,
  Crown,
  Trash2,
  ChevronLeft,
  ChevronRight
} from 'lucide-vue-next'
import AccountCard from '../components/AccountCard.vue'
import BatchBar from '../components/BatchBar.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const store = useAccountsStore()
const theme = useWorldTheme()
const { success, error } = useToast()
const isRefreshing = ref(false)
const viewMode = ref('grid')
const currentPage = ref(1)
const pageSize = 50

const showAddDialog = ref(false)
const isAdding = ref(false)
const jsonText = ref('')

// 导入进度状态
const importStatus = ref({ phase: 'idle', total: 0, imported: 0, failed: 0, elapsed: 0, message: '', details: null })
let elapsedTimer = null

// 号池统计数据（从 /status API 获取）
const poolStats = ref({
  freePool: { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 },
  proPool: { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 }
})

async function loadPoolStats() {
  try {
    const res = await api('/status')
    if (res.ok) {
      const d = await res.json()
      const def = { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 }
      poolStats.value = {
        freePool: { ...def, ...d.freePool },
        proPool: { ...def, ...d.proPool }
      }
    }
  } catch {}
}

const stats = computed(() => {
  const all = store.accounts.length
  const freeAccs = store.accounts.filter(a => !a.subscriptionType || a.subscriptionType === 'FREE')
  const proAccs = store.accounts.filter(a => a.subscriptionType === 'PRO' || a.subscriptionType === 'PRO_PLUS' || a.subscriptionType === 'POWER')
  const banned = store.accounts.filter(a => a.banStatus && a.banStatus !== 'ACTIVE').length
  const fp = poolStats.value.freePool
  const pp = poolStats.value.proPool
  return {
    all,
    freeCount: freeAccs.length,
    proCount: proAccs.length,
    banned,
    freeAvailable: fp.available,
    proAvailable: pp.available,
    freeUsed: fp.usageCurrent + fp.trialCurrent,
    freeTotal: fp.usageLimit + fp.trialLimit,
    proUsed: pp.usageCurrent + pp.trialCurrent,
    proTotal: pp.usageLimit + pp.trialLimit
  }
})

onMounted(() => {
  store.load()
  loadPoolStats()
})

const totalPages = computed(() => Math.max(1, Math.ceil(store.filtered.length / pageSize)))
const paginatedAccounts = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return store.filtered.slice(start, start + pageSize)
})
// 过滤条件变化时重置页码
watch(() => [store.filterKeyword, store.filterStatus, store.filterTier], () => { currentPage.value = 1 })

const refreshAll = async () => {
  isRefreshing.value = true
  try {
    await store.load()
    success('已刷新账号列表')
  } finally {
    isRefreshing.value = false
  }
}

const isDeletingBanned = ref(false)
async function deleteBanned() {
  const banned = store.accounts.filter(a => a.banStatus && a.banStatus !== 'ACTIVE')
  if (!banned.length) { error('没有封禁账号'); return }
  if (!confirm(`确定删除 ${banned.length} 个封禁/限制账号？此操作不可恢复！`)) return
  isDeletingBanned.value = true
  try {
    const res = await api('/accounts/batch', {
      method: 'POST',
      body: JSON.stringify({ ids: banned.map(a => a.id), action: 'delete' })
    })
    const data = await res.json()
    if (data.success) {
      success(`已删除 ${data.deleted} 个封禁账号`)
      store.filterStatus = 'all'
      await store.load()
    } else {
      error(data.error || '删除失败')
    }
  } catch (e) {
    error('删除失败: ' + e.message)
  }
  isDeletingBanned.value = false
}

function resetAddDialog() {
  jsonText.value = ''
  importStatus.value = { phase: 'idle', total: 0, imported: 0, failed: 0, elapsed: 0, message: '', details: null }
  if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
}

async function submitImport() {
  if (!jsonText.value.trim()) { error('请粘贴账号 JSON 数据'); return }
  let parsed
  try { parsed = JSON.parse(jsonText.value.trim()) } catch { error('JSON 格式错误，请检查内容'); return }

  const items = Array.isArray(parsed) ? parsed : [parsed]
  if (!items.length) { error('JSON 数组为空'); return }

  // 预处理每个账号的字段
  const accounts = items.map(item => {
    let authMethod = item.authMethod || ''
    if (!authMethod) {
      const provider = (item.provider || '').toLowerCase()
      if (provider === 'google' || provider === 'github') authMethod = 'social'
      else if (item.clientId || item.clientID) authMethod = 'idc'
      else authMethod = 'social'
    }
    return {
      accessToken: item.accessToken || item.access_token || '',
      refreshToken: item.refreshToken || item.refresh_token || '',
      clientId: item.clientId || item.clientID || item.client_id || '',
      clientSecret: item.clientSecret || item.client_secret || '',
      authMethod,
      provider: item.provider || '',
      region: item.region || 'us-east-1',
      email: item.email || '',
      userId: item.userId || item.user_id || '',
      profileArn: item.profileArn || '',
      machineId: item.machineId || '',
      usageData: item.usageData || null,
    }
  }).filter(a => a.refreshToken || a.accessToken)

  if (!accounts.length) { error('没有有效的账号（缺少 token）'); return }

  isAdding.value = true
  importStatus.value = { phase: 'importing', total: accounts.length, done: 0, imported: 0, failed: 0, elapsed: 0, message: '', details: null }

  const startMs = Date.now()
  elapsedTimer = setInterval(() => {
    importStatus.value.elapsed = ((Date.now() - startMs) / 1000).toFixed(1)
  }, 200)

  try {
    const auth = (await import('../stores/auth')).useAuthStore()
    const res = await fetch('/admin/api/auth/credentials/batch', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Password': auth.password
      },
      body: JSON.stringify({ accounts, concurrency: 20 })
    })

    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }

    // 读取 SSE 流
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() // 保留不完整的行

      let currentEvent = ''
      for (const line of lines) {
        if (line.startsWith('event: ')) {
          currentEvent = line.slice(7).trim()
        } else if (line.startsWith('data: ')) {
          const dataStr = line.slice(6)
          try {
            const data = JSON.parse(dataStr)
            if (currentEvent === 'progress') {
              importStatus.value.done = data.done || 0
              importStatus.value.imported = data.ok || 0
              importStatus.value.failed = data.fail || 0
            } else if (currentEvent === 'done') {
              clearInterval(elapsedTimer); elapsedTimer = null
              importStatus.value = {
                phase: 'done',
                total: accounts.length,
                done: accounts.length,
                imported: data.imported || 0,
                failed: data.failed || 0,
                elapsed: data.elapsed_sec?.toFixed(1) || importStatus.value.elapsed,
                message: data.message,
                details: data.details
              }
              setTimeout(() => store.load(), 1500)
            }
          } catch {}
          currentEvent = ''
        }
      }
    }

    // 如果流结束了但没收到 done 事件
    if (importStatus.value.phase === 'importing') {
      clearInterval(elapsedTimer); elapsedTimer = null
      importStatus.value.phase = 'done'
      importStatus.value.message = `导入完成: ${importStatus.value.imported} 成功, ${importStatus.value.failed} 失败`
      setTimeout(() => store.load(), 1500)
    }
  } catch (e) {
    clearInterval(elapsedTimer); elapsedTimer = null
    importStatus.value = { ...importStatus.value, phase: 'error', message: '请求失败: ' + e.message }
  }

  isAdding.value = false
}
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div class="space-y-1">
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">账号管理</h1>
        <p class="text-sm text-[var(--text)]-secondary font-medium flex items-center gap-2">
          <Sparkles class="w-3.5 h-3.5 text-[var(--world-accent-alt)]" />
          管理您的所有 AI 账号资产
        </p>
      </div>
      <button @click="showAddDialog = true" class="flex items-center gap-2 px-5 py-2.5 bg-[var(--primary)] text-white rounded-xl font-bold text-sm shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-[0.98] transition-all blood-glow-hover">
        <Plus class="w-4 h-4" /> 添加账号
      </button>
    </div>

    <!-- Pool Stats -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <!-- FREE 号池 -->
      <div class="modern-card p-4 stat-bronze">
        <div class="flex items-center gap-3">
          <div class="stat-icon-box">
            <Users class="w-4 h-4" />
          </div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ stats.freeCount }}</div>
            <div class="text-[10px] font-bold text-[var(--text)]-secondary uppercase tracking-wider">FREE 号池</div>
            <div class="text-[10px] text-green-500 font-bold mt-0.5">{{ stats.freeAvailable }} 可用 · {{ stats.freeUsed }}/{{ stats.freeTotal }} credits</div>
          </div>
        </div>
      </div>

      <!-- PRO 号池 -->
      <div class="modern-card p-4 stat-blood">
        <div class="flex items-center gap-3">
          <div class="stat-icon-box">
            <Crown class="w-4 h-4" />
          </div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ stats.proCount }}</div>
            <div class="text-[10px] font-bold text-[var(--text)]-secondary uppercase tracking-wider">PRO 号池</div>
            <div class="text-[10px] text-purple-500 font-bold mt-0.5">{{ stats.proAvailable }} 可用 · {{ stats.proUsed }}/{{ stats.proTotal }} credits</div>
          </div>
        </div>
      </div>

      <!-- 全部账号 -->
      <div class="modern-card p-4 stat-amber">
        <div class="flex items-center gap-3">
          <div class="stat-icon-box">
            <CheckCircle2 class="w-4 h-4" />
          </div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ stats.all }}</div>
            <div class="text-[10px] font-bold text-[var(--text)]-secondary uppercase tracking-wider">全部账号</div>
          </div>
        </div>
      </div>

      <!-- 封禁/限制 -->
      <div class="modern-card p-4 stat-rose">
        <div class="flex items-center gap-3">
          <div class="stat-icon-box">
            <ShieldAlert class="w-4 h-4" />
          </div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ stats.banned }}</div>
            <div class="text-[10px] font-bold text-[var(--text)]-secondary uppercase tracking-wider">封禁/限制</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Pool Credits Overview -->
    <div class="modern-card p-6">
      <div class="flex items-center gap-2 mb-4">
        <Cpu class="w-5 h-5 text-[var(--primary)]" />
        <h2 class="font-bold text-[10px] uppercase tracking-[0.2em] text-[var(--world-accent-alt)]">号池用量概览 (Credits)</h2>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <!-- FREE Pool -->
        <div class="p-4 rounded-xl bg-green-500/5 border border-green-500/10">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-bold text-green-500 flex items-center gap-1.5">
              <Users class="w-3.5 h-3.5" /> 普通号池
            </span>
            <span class="text-[9px] font-bold text-green-500 bg-green-500/10 px-2 py-0.5 rounded-full">FREE</span>
          </div>
          <div class="text-3xl font-black text-[var(--text)]">{{ stats.freeUsed }} / {{ stats.freeTotal }}</div>
          <div class="mt-2 text-[10px] text-[var(--text)]-secondary">{{ stats.freeCount }} 个账号 · {{ stats.freeAvailable }} 可用</div>
          <div class="mt-2 text-[10px] text-[var(--text)]-secondary opacity-60">模型: sonnet-4.5, haiku-4.5, opus-4.5</div>
        </div>
        <!-- PRO Pool -->
        <div class="p-4 rounded-xl bg-purple-500/5 border border-purple-500/10">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-bold text-purple-500 flex items-center gap-1.5">
              <Crown class="w-3.5 h-3.5" /> PRO 号池
            </span>
            <span class="text-[9px] font-bold text-purple-500 bg-purple-500/10 px-2 py-0.5 rounded-full">PRO</span>
          </div>
          <div class="text-3xl font-black text-[var(--text)]">{{ stats.proUsed }} / {{ stats.proTotal }}</div>
          <div class="mt-2 text-[10px] text-[var(--text)]-secondary">{{ stats.proCount }} 个账号 · {{ stats.proAvailable }} 可用</div>
          <div class="mt-2 text-[10px] text-[var(--text)]-secondary opacity-60">模型: sonnet-4.6, opus-4.6</div>
        </div>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
      <div class="relative flex-1 group">
        <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text)]-secondary group-focus-within:text-[var(--primary)] transition-colors" />
        <input
          v-model="store.filterKeyword"
          type="text"
          placeholder="搜索 Email、ID..."
          class="w-full h-10 pl-11 pr-4 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-[var(--primary)] transition-all"
        />
      </div>

      <div class="flex items-center gap-2">
        <!-- Pool Tier Filter -->
        <div class="flex items-center bg-[var(--card)] border border-[var(--border)] rounded-xl p-0.5">
          <button v-for="f in [{v:'all',l:'全部'},{v:'free',l:'FREE',c:'text-green-500'},{v:'pro',l:'PRO',c:'text-purple-500'}]" :key="f.v"
            @click="store.filterTier = f.v"
            class="px-3 py-1.5 rounded-lg text-xs font-bold transition-all"
            :class="store.filterTier === f.v ? 'bg-[var(--primary)] text-white shadow-sm' : (f.c || 'text-[var(--text)]-secondary') + ' hover:text-[var(--text)]'"
          >{{ f.l }}</button>
        </div>

        <!-- Status Filter Buttons -->
        <div class="flex items-center bg-[var(--card)] border border-[var(--border)] rounded-xl p-0.5">
          <button v-for="f in [{v:'all',l:'全部'},{v:'enabled',l:'启用'},{v:'disabled',l:'禁用'},{v:'banned',l:'封禁'}]" :key="f.v"
            @click="store.filterStatus = f.v"
            class="px-3 py-1.5 rounded-lg text-xs font-bold transition-all"
            :class="store.filterStatus === f.v ? 'bg-[var(--primary)] text-white shadow-sm' : 'text-[var(--text)]-secondary hover:text-[var(--text)]'"
          >{{ f.l }}</button>
        </div>

        <!-- View Switcher -->
        <div class="flex items-center bg-[var(--card)] border border-[var(--border)] rounded-xl p-0.5">
          <button @click="viewMode = 'grid'" class="p-2 rounded-lg transition-all" :class="viewMode === 'grid' ? 'bg-[var(--primary)] text-white shadow-sm' : 'text-[var(--text)]-secondary'">
            <LayoutGrid class="w-4 h-4" />
          </button>
          <button @click="viewMode = 'list'" class="p-2 rounded-lg transition-all" :class="viewMode === 'list' ? 'bg-[var(--primary)] text-white shadow-sm' : 'text-[var(--text)]-secondary'">
            <List class="w-4 h-4" />
          </button>
        </div>

        <button @click="refreshAll" :disabled="isRefreshing"
          class="p-2.5 bg-[var(--card)] border border-[var(--border)] rounded-xl hover:bg-[var(--bg)] transition-all disabled:opacity-50">
          <RotateCw class="w-4 h-4 text-[var(--text)]-secondary" :class="{ 'animate-spin': isRefreshing }" />
        </button>

        <button v-if="stats.banned > 0" @click="deleteBanned" :disabled="isDeletingBanned"
          class="flex items-center gap-1.5 px-3 py-2 bg-red-500/10 border border-red-500/20 text-red-500 rounded-xl text-xs font-bold hover:bg-red-500/20 transition-all disabled:opacity-50">
          <Trash2 class="w-3.5 h-3.5" />
          删除封禁 ({{ stats.banned }})
        </button>
      </div>
    </div>

    <!-- Batch Bar -->
    <BatchBar />

    <!-- Account Grid -->
    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      <div v-for="i in 8" :key="i" class="h-64 bg-[var(--card)] rounded-2xl animate-pulse border border-[var(--border)]"></div>
    </div>

    <div v-else-if="store.filtered.length > 0">
      <div :class="viewMode === 'grid' ? 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4' : 'flex flex-col gap-3'">
        <AccountCard v-for="account in paginatedAccounts" :key="account.id" :account="account" :horizontal="viewMode === 'list'" />
      </div>
      <!-- 分页 -->
      <div v-if="totalPages > 1" class="flex items-center justify-center gap-2 mt-6">
        <button @click="currentPage = Math.max(1, currentPage - 1)" :disabled="currentPage <= 1"
          class="p-2 rounded-xl bg-[var(--card)] border border-[var(--border)] disabled:opacity-30 hover:bg-[var(--bg)] transition-all">
          <ChevronLeft class="w-4 h-4" />
        </button>
        <template v-for="p in totalPages" :key="p">
          <button v-if="p === 1 || p === totalPages || (p >= currentPage - 2 && p <= currentPage + 2)"
            @click="currentPage = p"
            class="w-9 h-9 rounded-xl text-xs font-bold transition-all"
            :class="currentPage === p ? 'bg-[var(--primary)] text-white shadow-sm' : 'bg-[var(--card)] border border-[var(--border)] hover:bg-[var(--bg)]'">
            {{ p }}
          </button>
          <span v-else-if="p === currentPage - 3 || p === currentPage + 3" class="text-[var(--text)]-secondary text-xs">...</span>
        </template>
        <button @click="currentPage = Math.min(totalPages, currentPage + 1)" :disabled="currentPage >= totalPages"
          class="p-2 rounded-xl bg-[var(--card)] border border-[var(--border)] disabled:opacity-30 hover:bg-[var(--bg)] transition-all">
          <ChevronRight class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="flex flex-col items-center justify-center py-20 bg-[var(--card)] rounded-2xl border-2 border-dashed border-[var(--border)]">
      <PackageSearch class="w-12 h-12 text-[var(--text)]-secondary opacity-15 mb-4" />
      <h3 class="text-lg font-black mb-2">未找到匹配账号</h3>
      <p class="text-sm text-[var(--text)]-secondary mb-6">调整过滤条件或添加新账号</p>
      <button @click="store.filterKeyword = ''; store.filterStatus = 'all'; store.filterTier = 'all'" class="px-6 py-2 bg-[var(--primary)] text-white rounded-xl font-bold text-sm shadow-lg shadow-[var(--primary)]/20">
        重置过滤器
      </button>
    </div>

    <!-- Footer -->
    <div class="flex items-center justify-between pt-6 border-t border-[var(--border)] text-xs font-bold text-[var(--text)]-secondary">
      <span>显示 {{ store.filtered.length > 0 ? (currentPage - 1) * pageSize + 1 : 0 }}-{{ Math.min(currentPage * pageSize, store.filtered.length) }} / {{ store.filtered.length }} 个账号（共 {{ store.accounts.length }}）</span>
      <span v-if="store.selectedIds.size">已选 {{ store.selectedIds.size }} 个</span>
    </div>
  </div>

  <!-- Add Account Dialog -->
  <Teleport to="body">
    <div v-if="showAddDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
         @click.self="!isAdding && (showAddDialog = false, resetAddDialog())">
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl shadow-2xl w-full max-w-lg p-6 space-y-5">

        <!-- 导入中：进度面板 -->
        <template v-if="importStatus.phase === 'importing'">
          <div class="flex flex-col items-center py-8 gap-4">
            <div class="w-12 h-12 border-3 border-[var(--primary)]/20 border-t-[var(--primary)] rounded-full animate-spin"></div>
            <h3 class="text-lg font-black">正在导入账号...</h3>
            <div class="text-3xl font-black text-[var(--primary)] tabular-nums">{{ importStatus.done }} <span class="text-base font-bold text-[var(--text)]-secondary">/ {{ importStatus.total }}</span></div>
            <!-- 进度条 -->
            <div class="w-full h-2 bg-[var(--border)] rounded-full overflow-hidden">
              <div class="h-full bg-[var(--primary)] rounded-full transition-all duration-300"
                   :style="{ width: (importStatus.total > 0 ? (importStatus.done / importStatus.total * 100) : 0) + '%' }"></div>
            </div>
            <div class="flex items-center gap-4 text-xs font-bold">
              <span class="text-green-500">✓ {{ importStatus.imported }}</span>
              <span class="text-red-500" v-if="importStatus.failed > 0">✗ {{ importStatus.failed }}</span>
              <span class="text-[var(--text)]-secondary font-mono tabular-nums">{{ importStatus.elapsed }}s</span>
            </div>
          </div>
        </template>

        <!-- 导入完成：结果面板 -->
        <template v-else-if="importStatus.phase === 'done'">
          <div class="flex flex-col items-center py-6 gap-4">
            <CheckCircle2 class="w-14 h-14 text-green-500" />
            <h3 class="text-lg font-black">导入完成</h3>
            <div class="flex items-center gap-6">
              <div class="text-center">
                <div class="text-3xl font-black text-green-500">{{ importStatus.imported }}</div>
                <div class="text-[10px] font-bold text-[var(--text)]-secondary uppercase">成功</div>
              </div>
              <div class="text-center" v-if="importStatus.failed > 0">
                <div class="text-3xl font-black text-red-500">{{ importStatus.failed }}</div>
                <div class="text-[10px] font-bold text-[var(--text)]-secondary uppercase">失败</div>
              </div>
              <div class="text-center">
                <div class="text-3xl font-black text-[var(--text)]-secondary">{{ importStatus.elapsed }}s</div>
                <div class="text-[10px] font-bold text-[var(--text)]-secondary uppercase">耗时</div>
              </div>
            </div>
            <!-- 失败详情 -->
            <div v-if="importStatus.details?.failed?.length" class="w-full max-h-32 overflow-y-auto bg-red-500/5 border border-red-500/10 rounded-xl p-3 text-xs space-y-1">
              <div v-for="(f, i) in importStatus.details.failed" :key="i" class="text-red-400 truncate">{{ f.email || '未知' }}: {{ f.error }}</div>
            </div>
            <button @click="showAddDialog = false; resetAddDialog()" class="w-full h-10 rounded-xl bg-[var(--primary)] text-white text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-[0.98] transition-all">确定</button>
          </div>
        </template>

        <!-- 导入失败：错误面板 -->
        <template v-else-if="importStatus.phase === 'error'">
          <div class="flex flex-col items-center py-6 gap-4">
            <ShieldAlert class="w-14 h-14 text-red-500" />
            <h3 class="text-lg font-black">导入失败</h3>
            <p class="text-sm text-red-400 text-center">{{ importStatus.message }}</p>
            <button @click="importStatus.phase = 'idle'" class="w-full h-10 rounded-xl border border-[var(--border)] text-sm font-bold hover:bg-[var(--bg)] transition-all">返回重试</button>
          </div>
        </template>

        <!-- 默认：输入面板 -->
        <template v-else>
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-black text-[var(--text)]">导入账号</h2>
            <button @click="showAddDialog = false; resetAddDialog()" class="p-2 rounded-xl hover:bg-[var(--bg)] transition-all">
              <X class="w-4 h-4" />
            </button>
          </div>

          <p class="text-[11px] text-[var(--text)]-secondary leading-relaxed">
            粘贴 Kiro Account Manager 导出的 JSON（单个或数组），自动解析 accessToken / refreshToken / provider 等字段。
          </p>

          <textarea v-model="jsonText" rows="14"
            placeholder="粘贴 kiro-account-manager 导出的 JSON&#10;&#10;支持单个对象或数组批量导入 [...]"
            class="w-full px-3 py-2.5 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-xs font-mono outline-none focus:ring-2 focus:ring-primary/20 focus:border-[var(--primary)] transition-all resize-none leading-relaxed" />

          <div class="flex gap-3">
            <button @click="showAddDialog = false; resetAddDialog()" class="flex-1 h-10 rounded-xl border border-[var(--border)] text-sm font-bold hover:bg-[var(--bg)] transition-all">取消</button>
            <button @click="submitImport" :disabled="isAdding" class="flex-1 h-10 rounded-xl bg-[var(--primary)] text-white text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50">
              解析并导入
            </button>
          </div>
        </template>

      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.no-scrollbar::-webkit-scrollbar { display: none; }
.no-scrollbar { -ms-overflow-style: none; scrollbar-width: none; }

.stat-blood .stat-icon-box { background: rgba(196, 30, 58, 0.12); color: #c41e3a; padding: 0.5rem; border-radius: 0.75rem; }
.stat-bronze .stat-icon-box { background: rgba(184, 134, 11, 0.12); color: #b8860b; padding: 0.5rem; border-radius: 0.75rem; }
.stat-amber .stat-icon-box { background: rgba(245, 158, 11, 0.1); color: #f59e0b; padding: 0.5rem; border-radius: 0.75rem; }
.stat-rose .stat-icon-box { background: rgba(244, 63, 94, 0.1); color: #f43f5e; padding: 0.5rem; border-radius: 0.75rem; }
</style>
