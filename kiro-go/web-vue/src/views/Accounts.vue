<script setup>
import { onMounted, computed, ref } from 'vue'
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
  Crown
} from 'lucide-vue-next'
import AccountCard from '../components/AccountCard.vue'
import BatchBar from '../components/BatchBar.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const store = useAccountsStore()
const theme = useWorldTheme()
const { success, error } = useToast()
const isRefreshing = ref(false)
const viewMode = ref('grid')

const showAddDialog = ref(false)
const isAdding = ref(false)
const jsonText = ref('')

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

const refreshAll = async () => {
  isRefreshing.value = true
  try {
    await store.load()
    success('已刷新账号列表')
  } finally {
    isRefreshing.value = false
  }
}

function resetAddDialog() {
  jsonText.value = ''
}

async function submitImport() {
  if (!jsonText.value.trim()) { error('请粘贴账号 JSON 数据'); return }
  let parsed
  try { parsed = JSON.parse(jsonText.value.trim()) } catch { error('JSON 格式错误，请检查内容'); return }

  const items = Array.isArray(parsed) ? parsed : [parsed]
  if (!items.length) { error('JSON 数组为空'); return }

  isAdding.value = true
  let successCount = 0
  let failCount = 0
  const errors = []

  for (const item of items) {
    // 自动检测 authMethod
    let authMethod = item.authMethod || ''
    if (!authMethod) {
      const provider = (item.provider || '').toLowerCase()
      if (provider === 'google' || provider === 'github') authMethod = 'social'
      else if (item.clientId || item.clientID) authMethod = 'idc'
      else authMethod = 'social'
    }
    const payload = {
      accessToken: item.accessToken || item.access_token || '',
      refreshToken: item.refreshToken || item.refresh_token || '',
      clientId: item.clientId || item.clientID || item.client_id || '',
      clientSecret: item.clientSecret || item.client_secret || '',
      authMethod: authMethod,
      provider: item.provider || '',
      region: item.region || 'us-east-1',
      // 额外字段：kiro-account-manager 导出的完整信息
      email: item.email || '',
      userId: item.userId || item.user_id || '',
      profileArn: item.profileArn || '',
      machineId: item.machineId || '',
      usageData: item.usageData || null,
    }
    if (!payload.refreshToken && !payload.accessToken) {
      failCount++
      errors.push(`第 ${successCount + failCount} 条: 缺少 token`)
      continue
    }
    try {
      await api('/auth/credentials', { method: 'POST', body: JSON.stringify(payload) })
      successCount++
    } catch (e) {
      failCount++
      errors.push(`第 ${successCount + failCount} 条: ${e.message}`)
    }
  }

  if (successCount > 0) {
    success(`成功导入 ${successCount} 个账号${failCount > 0 ? `，${failCount} 个失败` : ''}，正在后台刷新配额...`)
    showAddDialog.value = false
    resetAddDialog()
    setTimeout(() => store.load(), 4000)
  } else {
    error(`全部导入失败：${errors[0]}`)
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
      </div>
    </div>

    <!-- Batch Bar -->
    <BatchBar />

    <!-- Account Grid -->
    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      <div v-for="i in 8" :key="i" class="h-64 bg-[var(--card)] rounded-2xl animate-pulse border border-[var(--border)]"></div>
    </div>

    <div v-else-if="store.filtered.length > 0"
         :class="viewMode === 'grid' ? 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4' : 'flex flex-col gap-3'">
      <AccountCard v-for="account in store.filtered" :key="account.id" :account="account" :horizontal="viewMode === 'list'" />
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
      <span>显示 {{ store.filtered.length }} / {{ store.accounts.length }} 个账号</span>
      <span v-if="store.selectedIds.size">已选 {{ store.selectedIds.size }} 个</span>
    </div>
  </div>

  <!-- Add Account Dialog -->
  <Teleport to="body">
    <div v-if="showAddDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
         @click.self="showAddDialog = false; resetAddDialog()">
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl shadow-2xl w-full max-w-lg p-6 space-y-5">
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
            {{ isAdding ? '导入中...' : '解析并导入' }}
          </button>
        </div>
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
