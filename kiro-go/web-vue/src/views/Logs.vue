<script setup>
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { 
  RotateCw, 
  Trash2, 
  Clock, 
  Zap, 
  ArrowRight, 
  AlertCircle, 
  CheckCircle2,
  Filter,
  Layers,
  Search,
  ChevronLeft,
  ChevronRight,
  Monitor,
  Cpu,
  History
} from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const logs = ref([])
const loading = ref(false)
const searchQuery = ref('')

async function loadLogs() {
  loading.value = true
  try {
    const res = await api('/logs')
    if (res.ok) {
      const d = await res.json()
      logs.value = d.logs || []
    }
  } catch {
    toastError('无法加载日志')
  }
  loading.value = false
}

async function clearLogs() {
  if (!confirm('确认清空所有历史调用日志？此操作不可撤销。')) return
  try {
    await api('/logs', { method: 'DELETE' })
    logs.value = []
    success('日志已清空')
  } catch {
    toastError('清空失败')
  }
}

const filteredLogs = computed(() => {
  if (!searchQuery.value) return logs.value
  const q = searchQuery.value.toLowerCase()
  return logs.value.filter(l => 
    l.actual_model?.toLowerCase().includes(q) || 
    l.original_model?.toLowerCase().includes(q) ||
    l.account?.toLowerCase().includes(q) ||
    l.error?.toLowerCase().includes(q)
  )
})

onMounted(loadLogs)
</script>

<template>
  <div class="space-y-8 max-w-[1600px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-6">
      <div class="space-y-1">
        <h1 class="text-3xl font-black tracking-tighter text-[var(--text)]">审计日志</h1>
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
           <History class="w-3.5 h-3.5 text-indigo-500" />
           追踪全局 API 调用记录、模型转换及实时消耗
        </p>
      </div>
      <div class="flex items-center gap-3">
        <button @click="loadLogs" :disabled="loading" class="flex items-center gap-2 px-5 py-2.5 bg-[var(--card)] border border-[var(--border)] rounded-2xl font-bold text-sm hover:bg-[var(--bg)] shadow-sm transition-all active:scale-95">
          <RotateCw class="w-4 h-4 text-primary" :class="{ 'animate-spin': loading }" /> 刷新
        </button>
        <button @click="clearLogs" class="flex items-center gap-2 px-5 py-2.5 bg-rose-500/10 text-rose-500 rounded-2xl font-black text-sm hover:bg-rose-500 hover:text-white transition-all shadow-lg shadow-rose-500/5">
          <Trash2 class="w-4 h-4" /> 清空记录
        </button>
      </div>
    </div>

    <!-- Filter Tool Bar -->
    <div class="bg-[var(--card)]/60 backdrop-blur-2xl border border-[var(--border)] rounded-[24px] p-3 flex flex-col md:flex-row items-center gap-4 shadow-xl shadow-black/5 sticky top-20 z-20">
      <div class="relative flex-1 w-full group">
        <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-secondary)] group-focus-within:text-primary transition-colors" />
        <input 
          v-model="searchQuery"
          type="text" 
          placeholder="搜索模型、账号或错误快照..."
          class="w-full h-12 pl-12 pr-4 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-sm outline-none focus:ring-4 focus:ring-primary/10 focus:border-primary transition-all font-medium"
        />
      </div>
      <div class="flex gap-2 w-full md:w-auto">
        <button class="flex-1 md:flex-none h-12 px-5 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-[11px] font-black uppercase tracking-widest flex items-center justify-center gap-2 hover:bg-[var(--card)] transition-colors">
          <Filter class="w-3.5 h-3.5 opacity-40" /> Status
        </button>
        <button class="flex-1 md:flex-none h-12 px-5 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-[11px] font-black uppercase tracking-widest flex items-center justify-center gap-2 hover:bg-[var(--card)] transition-colors">
          <Layers class="w-3.5 h-3.5 opacity-40" /> Type
        </button>
      </div>
    </div>

    <!-- Modern Audit Table -->
    <div class="modern-card overflow-hidden shadow-2xl shadow-black/5 bg-gradient-to-b from-[var(--card)] to-[var(--bg)]">
      <div class="overflow-x-auto custom-scrollbar">
        <table class="w-full text-left border-collapse min-w-[1000px]">
          <thead>
            <tr class="bg-[var(--bg)]/50 border-b border-[var(--border)]">
              <th class="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-[var(--text-secondary)] opacity-50">Timestamp</th>
              <th class="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-[var(--text-secondary)] opacity-50">Status</th>
              <th class="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-[var(--text-secondary)] opacity-50">Model Pipeline</th>
              <th class="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-[var(--text-secondary)] opacity-50">Account Instance</th>
              <th class="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-[var(--text-secondary)] opacity-50 text-right">Resource Usage</th>
              <th class="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-[var(--text-secondary)] opacity-50">Payload Meta</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[var(--border)]/50">
            <tr v-for="log in filteredLogs" :key="log.time" class="hover:bg-primary/[0.02] transition-colors group">
              <!-- Time Column -->
              <td class="px-8 py-6 whitespace-nowrap">
                <div class="flex flex-col">
                  <span class="text-xs font-black text-[var(--text)]">{{ log.time.split(' ')[1] }}</span>
                  <span class="text-[9px] font-bold text-[var(--text-secondary)] opacity-50 uppercase">{{ log.time.split(' ')[0] }}</span>
                </div>
              </td>

              <!-- Status Column -->
              <td class="px-8 py-6">
                <div v-if="log.error" class="flex items-center gap-2 px-3 py-1 rounded-xl bg-rose-500/10 text-rose-500 text-[10px] font-black uppercase tracking-wider w-fit border border-rose-500/10">
                  <div class="w-1.5 h-1.5 rounded-full bg-rose-500 animate-pulse"></div>
                  Rejected
                </div>
                <div v-else class="flex items-center gap-2 px-3 py-1 rounded-xl bg-emerald-500/10 text-emerald-500 text-[10px] font-black uppercase tracking-wider w-fit border border-emerald-500/10">
                  <div class="w-1.5 h-1.5 rounded-full bg-emerald-500"></div>
                  Authorized
                </div>
              </td>

              <!-- Model Pipeline Column -->
              <td class="px-8 py-6">
                <div class="flex items-center gap-3">
                  <div class="flex flex-col min-w-0">
                    <span class="text-[10px] font-mono text-[var(--text-secondary)] opacity-50 truncate">{{ log.original_model }}</span>
                    <div class="flex items-center gap-2 mt-0.5">
                      <div class="w-3 h-[1px] bg-primary/30"></div>
                      <span class="text-xs font-black text-primary truncate tracking-tight">{{ log.actual_model }}</span>
                    </div>
                  </div>
                </div>
              </td>

              <!-- Account Column -->
              <td class="px-8 py-6">
                <div class="flex items-center gap-3">
                  <div class="w-8 h-8 rounded-xl bg-indigo-500/10 flex items-center justify-center border border-indigo-500/10">
                    <Monitor class="w-4 h-4 text-indigo-500" />
                  </div>
                  <div class="flex flex-col">
                    <span class="text-xs font-black truncate max-w-[120px]">{{ log.account?.split('@')[0] }}</span>
                    <span class="text-[10px] font-bold text-[var(--text-secondary)] opacity-40 uppercase tracking-tighter">{{ log.api_type || 'REST' }}</span>
                  </div>
                </div>
              </td>

              <!-- Usage Column -->
              <td class="px-8 py-6 text-right">
                <div class="flex flex-col items-end gap-0.5">
                  <div class="flex items-center gap-1.5">
                    <Cpu class="w-3 h-3 text-amber-500" />
                    <span class="text-sm font-black tracking-tight">{{ log.total_tokens?.toLocaleString() }}</span>
                  </div>
                  <div class="flex items-center gap-1 text-[9px] font-bold text-[var(--text-secondary)] opacity-40 uppercase">
                    <span>In: {{ log.input_tokens }}</span>
                    <span>/</span>
                    <span>Out: {{ log.output_tokens }}</span>
                  </div>
                </div>
              </td>

              <!-- Meta Column -->
              <td class="px-8 py-6">
                <div v-if="log.error" class="group/err relative cursor-help">
                  <div class="text-xs text-rose-500 font-medium max-w-[200px] truncate italic">
                    {{ log.error }}
                  </div>
                  <div class="absolute bottom-full left-0 mb-2 p-3 bg-slate-900 text-white text-[10px] rounded-xl shadow-2xl w-64 opacity-0 group-hover/err:opacity-100 transition-opacity z-50 pointer-events-none border border-white/10">
                    {{ log.error }}
                  </div>
                </div>
                <div v-else class="flex items-center gap-3">
                  <div class="flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-slate-100 dark:bg-slate-800 text-[10px] font-black text-[var(--text-secondary)] uppercase tracking-tighter">
                    <Zap class="w-3 h-3 text-amber-500" />
                    {{ log.stream ? 'Stream' : 'Block' }}
                  </div>
                  <div class="text-[10px] font-bold text-primary/50 uppercase tracking-widest">
                    {{ (log.duration || 0).toFixed(0) }}ms
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination / Footer Stats -->
      <div class="px-8 py-6 bg-[var(--bg)]/50 border-t border-[var(--border)] flex flex-col sm:flex-row justify-between items-center gap-4">
        <div class="flex items-center gap-4 text-[10px] font-black uppercase tracking-[0.2em] text-[var(--text-secondary)] opacity-60">
           <span>Total Logs: {{ filteredLogs.length }}</span>
           <span class="w-1 h-1 rounded-full bg-[var(--border)]"></span>
           <span>Filtered View</span>
        </div>
        
        <div class="flex items-center gap-3">
          <button class="w-10 h-10 rounded-xl bg-[var(--card)] border border-[var(--border)] flex items-center justify-center hover:bg-primary hover:text-white hover:border-primary transition-all shadow-sm">
            <ChevronLeft class="w-4 h-4" />
          </button>
          <div class="px-4 py-2 bg-primary/10 text-primary rounded-xl text-xs font-black border border-primary/10">1 / 1</div>
          <button class="w-10 h-10 rounded-xl bg-[var(--card)] border border-[var(--border)] flex items-center justify-center hover:bg-primary hover:text-white hover:border-primary transition-all shadow-sm">
            <ChevronRight class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar { height: 6px; }
.custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: var(--border); border-radius: 10px; }
.custom-scrollbar::-webkit-scrollbar-thumb:hover { background: var(--text-secondary); }
</style>

<style scoped>
/* 滚动条美化 */
.overflow-x-auto::-webkit-scrollbar {
  height: 4px;
}
</style>
