<script setup>
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { 
  RotateCw, 
  Trash2, 
  Zap, 
  AlertCircle, 
  Search,
  Monitor,
  Cpu,
  History,
  ChevronDown,
  X
} from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const logs = ref([])
const loading = ref(false)
const searchQuery = ref('')
const expandedIndex = ref(-1)
const statusFilter = ref('all')

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
    expandedIndex.value = -1
    success('日志已清空')
  } catch {
    toastError('清空失败')
  }
}

function toggleExpand(i) {
  expandedIndex.value = expandedIndex.value === i ? -1 : i
}

const filteredLogs = computed(() => {
  let result = logs.value
  if (statusFilter.value === 'error') {
    result = result.filter(l => l.error)
  } else if (statusFilter.value === 'success') {
    result = result.filter(l => !l.error)
  }
  if (!searchQuery.value) return result
  const q = searchQuery.value.toLowerCase()
  return result.filter(l => 
    l.actual_model?.toLowerCase().includes(q) || 
    l.original_model?.toLowerCase().includes(q) ||
    l.account?.toLowerCase().includes(q) ||
    l.error?.toLowerCase().includes(q)
  )
})

const errorCount = computed(() => logs.value.filter(l => l.error).length)

onMounted(loadLogs)
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div class="space-y-1">
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">使用日志</h1>
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
          <History class="w-3.5 h-3.5 text-indigo-500" />
          API 调用记录 · 共 {{ logs.length }} 条
          <span v-if="errorCount" class="text-rose-500">· {{ errorCount }} 个错误</span>
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button @click="loadLogs" :disabled="loading" class="flex items-center gap-2 px-4 py-2 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm font-bold hover:bg-[var(--bg)] transition-all active:scale-95">
          <RotateCw class="w-4 h-4 text-primary" :class="{ 'animate-spin': loading }" /> 刷新
        </button>
        <button @click="clearLogs" class="flex items-center gap-2 px-4 py-2 bg-rose-500/10 text-rose-500 rounded-xl text-sm font-bold hover:bg-rose-500 hover:text-white transition-all">
          <Trash2 class="w-4 h-4" /> 清空
        </button>
      </div>
    </div>

    <!-- Filter Bar -->
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
      <div class="relative flex-1 group">
        <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-secondary)] group-focus-within:text-primary transition-colors" />
        <input 
          v-model="searchQuery"
          type="text" 
          placeholder="搜索模型、账号或错误信息..."
          class="w-full h-10 pl-11 pr-4 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all"
        />
      </div>
      <div class="flex items-center bg-[var(--card)] border border-[var(--border)] rounded-xl p-0.5">
        <button v-for="f in [{v:'all',l:'全部'},{v:'success',l:'成功'},{v:'error',l:'失败'}]" :key="f.v"
          @click="statusFilter = f.v"
          class="px-4 py-1.5 rounded-lg text-xs font-bold transition-all"
          :class="statusFilter === f.v ? 'bg-primary text-white shadow-sm' : 'text-[var(--text-secondary)] hover:text-[var(--text)]'"
        >{{ f.l }}</button>
      </div>
    </div>

    <!-- Log Table -->
    <div class="modern-card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse min-w-[900px]">
          <thead>
            <tr class="bg-[var(--bg)]/50 border-b border-[var(--border)]">
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">时间</th>
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">状态</th>
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">模型</th>
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">账号</th>
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] text-right">Token</th>
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] text-right">耗时</th>
              <th class="px-6 py-4 w-10"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[var(--border)]/50">
            <template v-for="(log, i) in filteredLogs" :key="i">
              <!-- Main Row -->
              <tr
                class="transition-colors cursor-pointer"
                :class="log.error ? 'hover:bg-rose-500/[0.03]' : 'hover:bg-primary/[0.02]'"
                @click="toggleExpand(i)"
              >
                <td class="px-6 py-4 whitespace-nowrap">
                  <div class="text-xs font-bold">{{ log.time?.split(' ')[1] }}</div>
                  <div class="text-[9px] text-[var(--text-secondary)]">{{ log.time?.split(' ')[0] }}</div>
                </td>

                <td class="px-6 py-4">
                  <span v-if="log.error"
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-rose-500/10 text-rose-500 text-[10px] font-bold border border-rose-500/10">
                    <AlertCircle class="w-3 h-3" /> 失败
                  </span>
                  <span v-else
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-500 text-[10px] font-bold border border-emerald-500/10">
                    <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span> 成功
                  </span>
                </td>

                <td class="px-6 py-4">
                  <div class="text-xs font-bold text-primary">{{ log.actual_model }}</div>
                  <div v-if="log.original_model !== log.actual_model" class="text-[9px] text-[var(--text-secondary)]">← {{ log.original_model }}</div>
                </td>

                <td class="px-6 py-4">
                  <div class="flex items-center gap-2">
                    <div class="w-6 h-6 rounded-lg bg-indigo-500/10 flex items-center justify-center">
                      <Monitor class="w-3 h-3 text-indigo-500" />
                    </div>
                    <div>
                      <div class="text-xs font-bold truncate max-w-[120px]">{{ log.account?.split('@')[0] }}</div>
                      <div class="text-[9px] text-[var(--text-secondary)]">{{ log.api_type || 'REST' }}</div>
                    </div>
                  </div>
                </td>

                <td class="px-6 py-4 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <Cpu class="w-3 h-3 text-amber-500" />
                    <span class="text-xs font-bold">{{ log.total_tokens?.toLocaleString() || '-' }}</span>
                  </div>
                  <div class="text-[9px] text-[var(--text-secondary)]">{{ log.input_tokens || 0 }} / {{ log.output_tokens || 0 }}</div>
                </td>

                <td class="px-6 py-4 text-right">
                  <div class="flex items-center justify-end gap-1.5">
                    <Zap class="w-3 h-3 text-amber-500" />
                    <span class="text-xs font-bold">{{ (log.duration || 0).toFixed(0) }}ms</span>
                  </div>
                  <div class="text-[9px] text-[var(--text-secondary)]">{{ log.stream ? 'Stream' : 'Block' }}</div>
                </td>

                <td class="px-6 py-4">
                  <ChevronDown
                    class="w-4 h-4 text-[var(--text-secondary)] transition-transform duration-200"
                    :class="{ 'rotate-180': expandedIndex === i }"
                  />
                </td>
              </tr>

              <!-- Expanded Detail Row -->
              <tr v-if="expandedIndex === i">
                <td colspan="7" class="px-6 py-0">
                  <div class="py-4 space-y-3">
                    <!-- Error Detail -->
                    <div v-if="log.error" class="p-4 bg-rose-500/5 border border-rose-500/10 rounded-xl">
                      <div class="flex items-center gap-2 mb-2">
                        <AlertCircle class="w-4 h-4 text-rose-500" />
                        <span class="text-xs font-bold text-rose-500">错误详情</span>
                      </div>
                      <pre class="text-xs text-rose-400 font-mono whitespace-pre-wrap break-all leading-relaxed">{{ log.error }}</pre>
                    </div>

                    <!-- Request Detail -->
                    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">请求模型</div>
                        <div class="text-xs font-bold font-mono">{{ log.original_model }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">实际模型</div>
                        <div class="text-xs font-bold font-mono text-primary">{{ log.actual_model }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">完整账号</div>
                        <div class="text-xs font-bold font-mono">{{ log.account || '-' }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">请求时间</div>
                        <div class="text-xs font-bold font-mono">{{ log.time }}</div>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <!-- Empty State -->
      <div v-if="!filteredLogs.length" class="px-6 py-16 text-center">
        <History class="w-10 h-10 text-[var(--text-secondary)] opacity-20 mx-auto mb-3" />
        <div class="text-sm font-bold text-[var(--text-secondary)]">{{ searchQuery || statusFilter !== 'all' ? '没有匹配的日志' : '暂无调用记录' }}</div>
      </div>

      <!-- Footer -->
      <div class="px-6 py-4 bg-[var(--bg)]/50 border-t border-[var(--border)] flex justify-between items-center text-[10px] font-bold text-[var(--text-secondary)]">
        <span>显示 {{ filteredLogs.length }} / {{ logs.length }} 条记录</span>
        <span>点击任意行查看详情</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overflow-x-auto::-webkit-scrollbar { height: 4px; }
.overflow-x-auto::-webkit-scrollbar-thumb { background: var(--border); border-radius: 10px; }
</style>
