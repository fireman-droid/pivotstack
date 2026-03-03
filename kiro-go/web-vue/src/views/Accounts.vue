<script setup>
import { onMounted, computed, ref } from 'vue'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../composables/useToast'
import { api } from '../api/admin'
import { 
  Plus, 
  Database, 
  Search, 
  RotateCw, 
  Filter, 
  Users, 
  Activity, 
  ShieldAlert,
  ArrowUpDown,
  CheckCircle2,
  AlertCircle,
  XCircle,
  PackageSearch,
  LayoutGrid,
  List,
  Sparkles
} from 'lucide-vue-next'
import AccountCard from '../components/AccountCard.vue'
import BatchBar from '../components/BatchBar.vue'

const store = useAccountsStore()
const { success, error } = useToast()
const isRefreshing = ref(false)
const viewMode = ref('grid')

onMounted(() => store.load())

const refreshAll = async () => {
  isRefreshing.value = true
  await store.load()
  isRefreshing.value = false
  success('已刷新账号列表')
}

// 统计计算
const stats = computed(() => {
  const all = store.accounts.length
  const active = store.accounts.filter(a => a.enabled && (!a.banStatus || a.banStatus === 'ACTIVE')).length
  const lowQuota = store.accounts.filter(a => a.usagePercent > 0.8).length
  const banned = store.accounts.filter(a => a.banStatus && a.banStatus !== 'ACTIVE').length
  return { all, active, lowQuota, banned }
})

const setFilter = (status) => {
  store.filterStatus = status
}
</script>

<template>
  <div class="space-y-8 max-w-[1600px] mx-auto pb-20">
    <!-- Header Section -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-6">
      <div class="space-y-1">
        <div class="flex items-center gap-2">
           <h1 class="text-3xl font-black tracking-tighter text-[var(--text)]">账号资源池</h1>
           <span class="px-2 py-0.5 rounded-full bg-primary/10 text-primary text-[10px] font-black uppercase">Alpha</span>
        </div>
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
           <Sparkles class="w-3.5 h-3.5 text-amber-500" />
           监控、调度并管理您的所有 AI 账号资产
        </p>
      </div>
      <div class="flex items-center gap-3">
        <button class="flex items-center gap-2 px-5 py-2.5 bg-primary text-white rounded-2xl font-black text-sm shadow-xl shadow-primary/20 hover:scale-[1.02] active:scale-[0.98] transition-all">
          <Plus class="w-4 h-4" /> 添加账号
        </button>
        <button class="flex items-center gap-2 px-5 py-2.5 bg-[var(--card)] border border-[var(--border)] rounded-2xl font-bold text-sm hover:bg-[var(--bg)] shadow-sm transition-all">
          <Database class="w-4 h-4 text-emerald-500" /> 数据库补给
        </button>
      </div>
    </div>

    <!-- Quick Stats Grid -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div v-for="s in [
        { key: 'all', label: '全部账号', val: stats.all, icon: Users, color: 'indigo', sub: `+${stats.all} Total` },
        { key: 'enabled', label: '运行正常', val: stats.active, icon: CheckCircle2, color: 'emerald', sub: `${((stats.active/stats.all || 0)*100).toFixed(0)}%` },
        { key: 'usage', label: '配额紧缺', val: stats.lowQuota, icon: Activity, color: 'amber', sub: 'Alert' },
        { key: 'banned', label: '封禁/限制', val: stats.banned, icon: ShieldAlert, color: 'rose', sub: 'Banned' }
      ]" :key="s.key" @click="s.key === 'usage' ? store.filterKeyword = 'usage > 80' : setFilter(s.key)" 
        class="modern-card p-5 cursor-pointer group relative overflow-hidden"
        :class="store.filterStatus === s.key ? `ring-2 ring-${s.color}-500 border-transparent` : ''">
        <div class="absolute -right-4 -top-4 w-20 h-20 rounded-full transition-transform group-hover:scale-150" :class="`bg-${s.color}-500/5`" />
        <div class="flex justify-between items-start mb-4 relative z-10">
          <div :class="`p-2.5 rounded-xl bg-${s.color}-50 dark:bg-${s.color}-900/20 text-${s.color}-600 dark:text-${s.color}-400`">
            <component :is="s.icon" class="w-5 h-5" />
          </div>
          <span :class="`text-xs font-black text-${s.color}-500 bg-${s.color}-500/10 px-2 py-0.5 rounded-full`">{{ s.sub }}</span>
        </div>
        <div class="text-3xl font-black mb-1 relative z-10" :class="`text-${s.color}-500`" v-if="s.key !== 'all'">{{ s.val }}</div>
        <div class="text-3xl font-black mb-1 relative z-10" v-else>{{ s.val }}</div>
        <div class="text-[10px] font-black text-[var(--text-secondary)] uppercase tracking-[0.15em] opacity-60">{{ s.label }}</div>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="sticky top-20 z-20">
      <div class="bg-[var(--card)]/80 backdrop-blur-2xl border border-[var(--border)] rounded-[24px] p-3 shadow-2xl shadow-black/5 flex flex-col lg:flex-row items-center gap-4">
        <!-- Search -->
        <div class="relative flex-1 w-full group">
          <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-secondary)] group-focus-within:text-primary transition-colors" />
          <input 
            v-model="store.filterKeyword"
            type="text" 
            placeholder="通过 Email、ID 或状态过滤..."
            class="w-full h-12 pl-12 pr-4 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-sm outline-none focus:ring-4 focus:ring-primary/10 focus:border-primary transition-all font-medium"
          />
        </div>

        <!-- Filters -->
        <div class="flex items-center gap-3 w-full lg:w-auto overflow-x-auto no-scrollbar py-1">
          <div class="flex items-center bg-[var(--bg)] border border-[var(--border)] rounded-2xl px-4 h-12 shrink-0">
            <Filter class="w-4 h-4 text-[var(--text-secondary)] mr-3" />
            <select v-model="store.filterStatus" class="bg-transparent border-none outline-none text-xs font-black text-[var(--text)] pr-4 uppercase tracking-tighter cursor-pointer">
              <option value="all">全部状态</option>
              <option value="enabled">已启用</option>
              <option value="disabled">已禁用</option>
              <option value="banned">已封禁</option>
            </select>
          </div>

          <!-- View Switcher -->
          <div class="flex items-center bg-[var(--bg)] border border-[var(--border)] rounded-2xl p-1 h-12 shrink-0">
            <button @click="viewMode = 'grid'" class="p-2 rounded-xl transition-all" :class="viewMode === 'grid' ? 'bg-[var(--card)] shadow-sm text-primary' : 'text-[var(--text-secondary)]'">
               <LayoutGrid class="w-4 h-4" />
            </button>
            <button @click="viewMode = 'list'" class="p-2 rounded-xl transition-all" :class="viewMode === 'list' ? 'bg-[var(--card)] shadow-sm text-primary' : 'text-[var(--text-secondary)]'">
               <List class="w-4 h-4" />
            </button>
          </div>

          <button 
            @click="refreshAll" 
            :disabled="isRefreshing"
            class="h-12 w-12 flex items-center justify-center bg-[var(--bg)] border border-[var(--border)] rounded-2xl hover:bg-[var(--card)] active:scale-90 transition-all disabled:opacity-50 shrink-0"
          >
            <RotateCw class="w-4 h-4 text-[var(--text-secondary)]" :class="{ 'animate-spin': isRefreshing }" />
          </button>
        </div>

        <div class="w-px h-8 bg-[var(--border)] hidden lg:block mx-1"></div>

        <!-- Batch Operations -->
        <div class="w-full lg:w-auto">
          <BatchBar />
        </div>
      </div>
    </div>

    <!-- Main Grid -->
    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
      <div v-for="i in 8" :key="i" class="h-80 bg-[var(--card)] rounded-[32px] animate-pulse border border-[var(--border)] shadow-sm"></div>
    </div>
    
    <div v-else-if="store.filtered.length > 0" 
         :class="viewMode === 'grid' ? 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6' : 'flex flex-col gap-4'">
      <AccountCard 
        v-for="account in store.filtered" 
        :key="account.id" 
        :account="account" 
        :horizontal="viewMode === 'list'"
      />
    </div>

    <!-- Empty State -->
    <div v-else class="flex flex-col items-center justify-center py-24 bg-[var(--card)] rounded-[40px] border-2 border-dashed border-[var(--border)] transition-all">
      <div class="w-24 h-24 bg-[var(--bg)] rounded-3xl flex items-center justify-center mb-8 shadow-inner">
        <PackageSearch class="w-12 h-12 text-[var(--text-secondary)] opacity-10" />
      </div>
      <h3 class="text-2xl font-black mb-3 tracking-tight">未发现匹配账号</h3>
      <p class="text-[var(--text-secondary)] mb-10 max-w-sm text-center font-medium leading-relaxed px-6">
        尝试调整您的过滤条件或关键词，或者通过顶部按钮添加一个新账号。
      </p>
      <button @click="store.filterKeyword = ''; store.filterStatus = 'all'" class="px-8 py-3 bg-primary text-white rounded-2xl font-black text-sm shadow-2xl shadow-primary/30 hover:scale-[1.05] active:scale-95 transition-all">
        重置过滤器
      </button>
    </div>

    <!-- Footer Stats -->
    <div class="flex flex-col md:flex-row items-center justify-between pt-10 border-t border-[var(--border)] gap-6">
      <div class="flex items-center gap-8 px-4 py-3 bg-[var(--card)] rounded-2xl border border-[var(--border)]">
        <div class="text-center">
          <div class="text-lg font-black leading-tight">{{ store.filtered.length }}</div>
          <div class="text-[9px] uppercase font-black text-[var(--text-secondary)] tracking-widest opacity-40">DISPLAYED</div>
        </div>
        <div class="w-px h-8 bg-[var(--border)]"></div>
        <div class="text-center">
          <div class="text-lg font-black leading-tight text-primary">{{ store.selectedIds.size }}</div>
          <div class="text-[9px] uppercase font-black text-[var(--text-secondary)] tracking-widest opacity-40">SELECTED</div>
        </div>
      </div>
      
      <div class="flex items-center gap-2.5">
        <button v-for="i in [1]" :key="i" 
          class="w-10 h-10 rounded-xl flex items-center justify-center text-xs font-black border transition-all"
          :class="i === 1 ? 'bg-primary border-primary text-white shadow-lg shadow-primary/20' : 'border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--card)]'">
          {{ i }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.no-scrollbar::-webkit-scrollbar { display: none; }
.no-scrollbar { -ms-overflow-style: none; scrollbar-width: none; }

/* Tailwind V4 Colors hack for select text */
.text-indigo-500 { color: #6366f1; }
.text-emerald-500 { color: #10b981; }
.text-amber-500 { color: #f59e0b; }
.text-rose-500 { color: #f43f5e; }

.bg-indigo-500\/10 { background-color: rgba(99, 102, 241, 0.1); }
.bg-emerald-500\/10 { background-color: rgba(16, 185, 129, 0.1); }
.bg-amber-500\/10 { background-color: rgba(245, 158, 11, 0.1); }
.bg-rose-500\/10 { background-color: rgba(244, 63, 94, 0.1); }

.ring-indigo-500 { --tw-ring-color: #6366f1; }
.ring-emerald-500 { --tw-ring-color: #10b981; }
.ring-amber-500 { --tw-ring-color: #f59e0b; }
.ring-rose-500 { --tw-ring-color: #f43f5e; }
</style>

<style scoped>
.no-scrollbar::-webkit-scrollbar {
  display: none;
}
.no-scrollbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
</style>
