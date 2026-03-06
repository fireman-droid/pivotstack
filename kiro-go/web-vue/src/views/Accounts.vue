<script setup>
import { onMounted, computed, ref } from 'vue'
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
  Cpu
} from 'lucide-vue-next'
import AccountCard from '../components/AccountCard.vue'
import BatchBar from '../components/BatchBar.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const store = useAccountsStore()
const { success, error } = useToast()
const isRefreshing = ref(false)
const viewMode = ref('grid')

const showAddDialog = ref(false)
const addTab = ref('token')
const isAdding = ref(false)

const jsonText = ref('')
const addForm = ref({ refreshToken: '', region: 'us-east-1', authMethod: 'social' })

const usageChartData = computed(() => {
  const accs = store.accounts.filter(a => a.usageLimit > 0 || a.trialUsageLimit > 0)
  if (!accs.length) return { labels: [], datasets: [] }
  const currentAvg = +(accs.reduce((s, a) => s + (a.usagePercent || 0), 0) / accs.length * 100).toFixed(1)
  const mockHistory = [35, 42, 38, 55, 62, 58]
  return {
    labels: ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00', '现在'],
    datasets: [{
      label: '全池平均配额使用 %',
      borderColor: '#6366f1',
      backgroundColor: 'rgba(99, 102, 241, 0.1)',
      borderWidth: 3,
      tension: 0.4,
      fill: true,
      pointRadius: 4,
      pointHoverRadius: 6,
      data: [...mockHistory, currentAvg]
    }]
  }
})

const usageChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { intersect: false, mode: 'index' },
  plugins: {
    legend: { display: false },
    tooltip: { callbacks: { label: (ctx) => `${ctx.raw}% 平均使用率` } }
  },
  scales: {
    y: { beginAtZero: true, max: 100, ticks: { callback: v => v + '%', font: { size: 10 } }, grid: { color: 'rgba(0,0,0,0.05)' } },
    x: { ticks: { font: { size: 10 } }, grid: { display: false } }
  }
}

const hasUsageData = computed(() => store.accounts.some(a => a.usageLimit > 0 || a.trialUsageLimit > 0))

const stats = computed(() => {
  const all = store.accounts.length
  const active = store.accounts.filter(a => a.enabled && (!a.banStatus || a.banStatus === 'ACTIVE')).length
  const lowQuota = store.accounts.filter(a => a.usagePercent > 0.8).length
  const banned = store.accounts.filter(a => a.banStatus && a.banStatus !== 'ACTIVE').length
  return { all, active, lowQuota, banned }
})

onMounted(() => store.load())

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
  addForm.value = { refreshToken: '', region: 'us-east-1', authMethod: 'social' }
}

async function submitAddByToken() {
  if (!addForm.value.refreshToken.trim()) { error('请填写 Refresh Token'); return }
  isAdding.value = true
  try {
    await api('/auth/credentials', { method: 'POST', body: JSON.stringify(addForm.value) })
    success('账号添加成功，正在后台刷新配额...')
    showAddDialog.value = false
    resetAddDialog()
    setTimeout(() => store.load(), 4000)
  } catch (e) {
    error('添加失败：' + e.message)
  } finally {
    isAdding.value = false
  }
}

async function submitAddByJson() {
  if (!jsonText.value.trim()) { error('请粘贴 JSON 数据'); return }
  let parsed
  try { parsed = JSON.parse(jsonText.value.trim()) } catch { error('JSON 格式错误，请检查内容'); return }

  const items = Array.isArray(parsed) ? parsed : [parsed]
  if (!items.length) { error('JSON 数组为空'); return }

  isAdding.value = true
  let successCount = 0
  let failCount = 0
  const errors = []

  for (const item of items) {
    const payload = {
      accessToken: item.accessToken || item.access_token || '',
      refreshToken: item.refreshToken || item.refresh_token || '',
      clientId: item.clientId || item.clientID || item.client_id || '',
      clientSecret: item.clientSecret || item.client_secret || '',
      authMethod: item.authMethod || 'idc',
      region: item.region || 'us-east-1',
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
    success(`成功导入 ${successCount} 个账号${failCount > 0 ? `，${failCount} 个失败` : ''}`)
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
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
          <Sparkles class="w-3.5 h-3.5 text-amber-500" />
          管理您的所有 AI 账号资产
        </p>
      </div>
      <button @click="showAddDialog = true" class="flex items-center gap-2 px-5 py-2.5 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20 hover:scale-[1.02] active:scale-[0.98] transition-all">
        <Plus class="w-4 h-4" /> 添加账号
      </button>
    </div>

    <!-- Stats (display only, not clickable) -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <div v-for="s in [
        { label: '全部账号', val: stats.all, icon: Users, colorClass: 'stat-indigo' },
        { label: '运行正常', val: stats.active, icon: CheckCircle2, colorClass: 'stat-emerald' },
        { label: '配额紧缺', val: stats.lowQuota, icon: Activity, colorClass: 'stat-amber' },
        { label: '封禁/限制', val: stats.banned, icon: ShieldAlert, colorClass: 'stat-rose' }
      ]" :key="s.label" class="modern-card p-4" :class="s.colorClass">
        <div class="flex items-center gap-3">
          <div class="stat-icon-box">
            <component :is="s.icon" class="w-4 h-4" />
          </div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ s.val }}</div>
            <div class="text-[10px] font-bold text-[var(--text-secondary)] uppercase tracking-wider">{{ s.label }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Usage Chart -->
    <div v-if="hasUsageData" class="modern-card p-6">
      <div class="flex items-center gap-2 mb-4">
        <Cpu class="w-5 h-5 text-indigo-500" />
        <h2 class="font-bold text-sm uppercase tracking-widest text-[var(--text-secondary)]">配额使用概览</h2>
      </div>
      <div class="h-[180px]">
        <Line :data="usageChartData" :options="usageChartOptions" />
      </div>
    </div>

    <!-- Toolbar -->
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
      <div class="relative flex-1 group">
        <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-secondary)] group-focus-within:text-primary transition-colors" />
        <input
          v-model="store.filterKeyword"
          type="text"
          placeholder="搜索 Email、ID..."
          class="w-full h-10 pl-11 pr-4 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all"
        />
      </div>

      <div class="flex items-center gap-2">
        <!-- Status Filter Buttons -->
        <div class="flex items-center bg-[var(--card)] border border-[var(--border)] rounded-xl p-0.5">
          <button v-for="f in [{v:'all',l:'全部'},{v:'enabled',l:'启用'},{v:'disabled',l:'禁用'},{v:'banned',l:'封禁'}]" :key="f.v"
            @click="store.filterStatus = f.v"
            class="px-3 py-1.5 rounded-lg text-xs font-bold transition-all"
            :class="store.filterStatus === f.v ? 'bg-primary text-white shadow-sm' : 'text-[var(--text-secondary)] hover:text-[var(--text)]'"
          >{{ f.l }}</button>
        </div>

        <!-- View Switcher -->
        <div class="flex items-center bg-[var(--card)] border border-[var(--border)] rounded-xl p-0.5">
          <button @click="viewMode = 'grid'" class="p-2 rounded-lg transition-all" :class="viewMode === 'grid' ? 'bg-primary text-white shadow-sm' : 'text-[var(--text-secondary)]'">
            <LayoutGrid class="w-4 h-4" />
          </button>
          <button @click="viewMode = 'list'" class="p-2 rounded-lg transition-all" :class="viewMode === 'list' ? 'bg-primary text-white shadow-sm' : 'text-[var(--text-secondary)]'">
            <List class="w-4 h-4" />
          </button>
        </div>

        <button @click="refreshAll" :disabled="isRefreshing"
          class="p-2.5 bg-[var(--card)] border border-[var(--border)] rounded-xl hover:bg-[var(--bg)] transition-all disabled:opacity-50">
          <RotateCw class="w-4 h-4 text-[var(--text-secondary)]" :class="{ 'animate-spin': isRefreshing }" />
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
      <PackageSearch class="w-12 h-12 text-[var(--text-secondary)] opacity-15 mb-4" />
      <h3 class="text-lg font-black mb-2">未找到匹配账号</h3>
      <p class="text-sm text-[var(--text-secondary)] mb-6">调整过滤条件或添加新账号</p>
      <button @click="store.filterKeyword = ''; store.filterStatus = 'all'" class="px-6 py-2 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20">
        重置过滤器
      </button>
    </div>

    <!-- Footer -->
    <div class="flex items-center justify-between pt-6 border-t border-[var(--border)] text-xs font-bold text-[var(--text-secondary)]">
      <span>显示 {{ store.filtered.length }} / {{ store.accounts.length }} 个账号</span>
      <span v-if="store.selectedIds.size">已选 {{ store.selectedIds.size }} 个</span>
    </div>
  </div>

  <!-- Add Account Dialog -->
  <Teleport to="body">
    <div v-if="showAddDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
         @click.self="showAddDialog = false; resetAddDialog()">
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl shadow-2xl w-full max-w-md p-6 space-y-5">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-black">添加账号</h2>
          <button @click="showAddDialog = false; resetAddDialog()" class="p-2 rounded-xl hover:bg-[var(--bg)] transition-all">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Tab Switch -->
        <div class="flex bg-[var(--bg)] rounded-xl p-0.5 border border-[var(--border)]">
          <button v-for="tab in [{k:'token',l:'Token'},{k:'json',l:'JSON 导入'}]" :key="tab.k"
            @click="addTab = tab.k"
            class="flex-1 py-2 rounded-lg text-xs font-bold transition-all"
            :class="addTab === tab.k ? 'bg-[var(--card)] text-primary shadow-sm' : 'text-[var(--text-secondary)]'">
            {{ tab.l }}
          </button>
        </div>

        <!-- Tab: Refresh Token -->
        <div v-if="addTab === 'token'" class="space-y-4">
          <div class="space-y-1.5">
            <label class="text-xs font-bold text-[var(--text-secondary)]">认证方式</label>
            <select v-model="addForm.authMethod" class="w-full h-10 px-3 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all">
              <option value="social">社交账号（GitHub / Google）</option>
              <option value="idc">企业 SSO（AWS IdC）</option>
            </select>
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-bold text-[var(--text-secondary)]">Refresh Token <span class="text-rose-500">*</span></label>
            <textarea v-model="addForm.refreshToken" rows="4"
              placeholder="粘贴 Refresh Token..."
              class="w-full px-3 py-2 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm font-mono outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all resize-none" />
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-bold text-[var(--text-secondary)]">区域</label>
            <select v-model="addForm.region" class="w-full h-10 px-3 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all">
              <option value="us-east-1">us-east-1（美东）</option>
              <option value="us-west-2">us-west-2（美西）</option>
              <option value="eu-west-1">eu-west-1（欧洲）</option>
              <option value="ap-southeast-1">ap-southeast-1（亚太）</option>
            </select>
          </div>
          <div class="flex gap-3 pt-2">
            <button @click="showAddDialog = false; resetAddDialog()" class="flex-1 h-10 rounded-xl border border-[var(--border)] text-sm font-bold hover:bg-[var(--bg)] transition-all">取消</button>
            <button @click="submitAddByToken" :disabled="isAdding" class="flex-1 h-10 rounded-xl bg-primary text-white text-sm font-bold shadow-lg shadow-primary/20 hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50">
              {{ isAdding ? '添加中...' : '确认添加' }}
            </button>
          </div>
        </div>

        <!-- Tab: JSON Import (supports array) -->
        <div v-if="addTab === 'json'" class="space-y-4">
          <p class="text-[11px] text-[var(--text-secondary)] leading-relaxed">
            粘贴单个 JSON 对象或 JSON 数组来批量导入多个账号。
          </p>
          <textarea v-model="jsonText" rows="10"
            :placeholder="'单个账号:\n{&quot;refreshToken&quot;:&quot;...&quot;, &quot;region&quot;:&quot;us-east-1&quot;}\n\n批量导入:\n[\n  {&quot;refreshToken&quot;:&quot;...&quot;},\n  {&quot;refreshToken&quot;:&quot;...&quot;}\n]'"
            class="w-full px-3 py-2 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-xs font-mono outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all resize-none" />
          <div class="flex gap-3">
            <button @click="showAddDialog = false; resetAddDialog()" class="flex-1 h-10 rounded-xl border border-[var(--border)] text-sm font-bold hover:bg-[var(--bg)] transition-all">取消</button>
            <button @click="submitAddByJson" :disabled="isAdding" class="flex-1 h-10 rounded-xl bg-indigo-600 text-white text-sm font-bold shadow-lg hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50">
              {{ isAdding ? '导入中...' : '解析并导入' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.no-scrollbar::-webkit-scrollbar { display: none; }
.no-scrollbar { -ms-overflow-style: none; scrollbar-width: none; }

.stat-indigo .stat-icon-box { background: rgba(99, 102, 241, 0.1); color: #6366f1; padding: 0.5rem; border-radius: 0.75rem; }
.stat-emerald .stat-icon-box { background: rgba(16, 185, 129, 0.1); color: #10b981; padding: 0.5rem; border-radius: 0.75rem; }
.stat-amber .stat-icon-box { background: rgba(245, 158, 11, 0.1); color: #f59e0b; padding: 0.5rem; border-radius: 0.75rem; }
.stat-rose .stat-icon-box { background: rgba(244, 63, 94, 0.1); color: #f43f5e; padding: 0.5rem; border-radius: 0.75rem; }
</style>
