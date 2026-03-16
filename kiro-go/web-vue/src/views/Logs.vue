<script setup>
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
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
  X,
  Radio,
  Key
} from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const route = useRoute()
const router = useRouter()
const logs = ref([])
const loading = ref(false)
const searchQuery = ref('')
const expandedIndex = ref(-1)
const statusFilter = ref(route.query.status || 'all')
const keyFilter = ref(route.query.key || 'all')
const apiKeys = ref([])
const sseConnected = ref(false)
const currentPage = ref(parseInt(route.query.page) || 1)
const totalLogs = ref(0)
const pageSize = ref(50)
let eventSource = null

// Persist filters to URL
watch([statusFilter, keyFilter, currentPage], ([s, k, p]) => {
  const q = {}
  if (s !== 'all') q.status = s
  if (k !== 'all') q.key = k
  if (p > 1) q.page = p
  router.replace({ query: q }).catch(() => {})
})

// 通过 SSE 实时接收日志
function connectSSE() {
  const password = document.cookie.match(/admin_password=([^;]+)/)?.[1] || ''
  const url = `${location.origin}/admin/api/sse/logs?password=${encodeURIComponent(password)}`
  
  eventSource = new EventSource(url)
  
  eventSource.addEventListener('log', (e) => {
    try {
      const entry = JSON.parse(e.data)
      
      // 过滤不需要显示的 SSE 日志
      if (statusFilter.value === 'error' && !entry.error && entry.status !== 'error') return
      if (statusFilter.value === 'success' && (entry.error || entry.status === 'error')) return
      if (keyFilter.value !== 'all' && entry.api_key_id !== keyFilter.value) return
      if (searchQuery.value) {
        const q = searchQuery.value.toLowerCase()
        const matches = 
          entry.actual_model?.toLowerCase().includes(q) ||
          entry.original_model?.toLowerCase().includes(q) ||
          entry.account?.toLowerCase().includes(q) ||
          entry.error?.toLowerCase().includes(q) ||
          entry.request_id?.toLowerCase().includes(q) ||
          entry.stop_reason?.toLowerCase().includes(q)
        if (!matches) return
      }

      totalLogs.value++
      if (currentPage.value !== 1) return
      const exists = logs.value.some(l => 
        l.time === entry.time && l.actual_model === entry.actual_model && l.account === entry.account
      )
      if (!exists) {
        logs.value.unshift(entry)
        if (logs.value.length > pageSize.value) {
          logs.value = logs.value.slice(0, pageSize.value)
        }
      }
    } catch {}
  })
  
  eventSource.onopen = () => {
    sseConnected.value = true
  }
  
  eventSource.onerror = () => {
    sseConnected.value = false
    // 3 秒后重连
    setTimeout(() => {
      if (eventSource) {
        eventSource.close()
        connectSSE()
      }
    }, 3000)
  }
}

// 加载 API Keys 列表（用于筛选下拉）
async function loadApiKeys() {
  try {
    const res = await api('/apikeys')
    if (res.ok) apiKeys.value = await res.json()
  } catch {}
}

// 分页加载日志
async function loadLogs(page = 1) {
  loading.value = true
  try {
    const params = new URLSearchParams({
      page,
      limit: pageSize.value
    })
    if (statusFilter.value !== 'all') params.append('status', statusFilter.value)
    if (keyFilter.value !== 'all') params.append('key', keyFilter.value)
    if (searchQuery.value) params.append('search', searchQuery.value)

    const res = await api(`/logs?${params.toString()}`)
    if (res.ok) {
      const d = await res.json()
      logs.value = d.logs || []
      totalLogs.value = d.total || 0
      currentPage.value = d.page || 1
    }
  } catch {
    toastError('无法加载日志')
  }
  loading.value = false
}

const totalPages = computed(() => Math.max(1, Math.ceil(totalLogs.value / pageSize.value)))

function goToPage(page) {
  if (page < 1 || page > totalPages.value) return
  expandedIndex.value = -1
  loadLogs(page)
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

function formatDuration(ms) {
  if (!ms && ms !== 0) return '-'
  if (ms < 1000) return ms + 'ms'
  return (ms / 1000).toFixed(1) + 's'
}

function getApiKeyDisplay(keyId) {
  if (!keyId) return '未关联'
  const keyObj = apiKeys.value.find(k => k.id === keyId)
  if (keyObj) {
    return keyObj.note || keyObj.key?.slice(0, 10) + '...'
  }
  return keyId
}

const filteredLogs = computed(() => logs.value)

let searchTimeout = null
watch(searchQuery, () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    goToPage(1)
  }, 500)
})

watch([statusFilter, keyFilter], () => {
  goToPage(1)
})

const errorCount = computed(() => totalLogs.value > 0 ? logs.value.filter(l => l.error || l.status === 'error').length : 0)

onMounted(async () => {
  await Promise.all([loadLogs(), loadApiKeys()])
  connectSSE()
})

onUnmounted(() => {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
})
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div class="space-y-1">
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)] flex items-center gap-3">
          使用日志
          <span v-if="sseConnected" class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-500 text-[10px] font-bold">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span> 实时
          </span>
          <span v-else class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-amber-500/10 text-amber-500 text-[10px] font-bold">
            <span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span> 离线
          </span>
        </h1>
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
          <History class="w-3.5 h-3.5 text-indigo-500" />
          API 调用记录 · 共 {{ logs.length }} 条
          <span v-if="errorCount" class="text-rose-500">· {{ errorCount }} 个错误</span>
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button @click="loadLogs(currentPage)" :disabled="loading" class="flex items-center gap-2 px-4 py-2 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm font-bold hover:bg-[var(--bg)] transition-all active:scale-95">
          <RotateCw class="w-4 h-4 text-[var(--primary)]" :class="{ 'animate-spin': loading }" /> 刷新
        </button>
        <button @click="clearLogs" class="flex items-center gap-2 px-4 py-2 bg-rose-500/10 text-rose-500 rounded-xl text-sm font-bold hover:bg-rose-500 hover:text-white transition-all">
          <Trash2 class="w-4 h-4" /> 清空
        </button>
      </div>
    </div>

    <!-- Filter Bar -->
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
      <div class="relative flex-1 group">
        <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-secondary)] group-focus-within:text-[var(--primary)] transition-colors" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="搜索模型、账号或错误信息..."
          class="w-full h-10 pl-11 pr-4 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-[var(--primary)] transition-all"
        />
      </div>
      <div v-if="apiKeys.length" class="relative">
        <Key class="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[var(--text-secondary)] pointer-events-none" />
        <select v-model="keyFilter"
          class="h-10 pl-9 pr-8 bg-[var(--card)] border border-[var(--border)] rounded-xl text-xs font-bold outline-none appearance-none cursor-pointer hover:border-[var(--primary)] transition-colors"
          style="background-image: url(&quot;data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke='currentColor'%3E%3Cpath stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M19 9l-7 7-7-7'%3E%3C/path%3E%3C/svg%3E&quot;); background-repeat: no-repeat; background-position: right 0.5rem center; background-size: 1em;"
        >
          <option value="all">全部 Key</option>
          <option v-for="k in apiKeys" :key="k.id" :value="k.id">
            {{ k.note || k.key?.slice(0, 10) + '...' }}
          </option>
        </select>
      </div>
      <div class="flex items-center bg-[var(--card)] border border-[var(--border)] rounded-xl p-0.5">
        <button v-for="f in [{v:'all',l:'全部'},{v:'success',l:'成功'},{v:'error',l:'失败'}]" :key="f.v"
          @click="statusFilter = f.v"
          class="px-4 py-1.5 rounded-lg text-xs font-bold transition-all"
          :class="statusFilter === f.v ? 'bg-[var(--primary)] text-white shadow-sm' : 'text-[var(--text-secondary)] hover:text-[var(--text)]'"
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
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">使用方</th>
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] text-right">Credit</th>
              <th class="px-6 py-4 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] text-right">耗时</th>
              <th class="px-6 py-4 w-10"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[var(--border)]/50">
            <template v-for="(log, i) in filteredLogs" :key="i">
              <!-- Main Row -->
              <tr
                class="transition-colors cursor-pointer"
                :class="log.error ? 'hover:bg-rose-500/[0.03]' : 'hover:bg-[var(--primary)]/[0.02]'"
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
                  <span v-if="log.stop_reason" class="ml-1 px-1.5 py-0.5 rounded text-[9px] font-bold"
                    :class="{
                      'bg-sky-500/10 text-sky-400': log.stop_reason === 'end_turn' || log.stop_reason === 'stop',
                      'bg-amber-500/10 text-amber-400': log.stop_reason === 'tool_use' || log.stop_reason === 'tool_calls',
                      'bg-rose-500/10 text-rose-400': log.stop_reason === 'max_tokens'
                    }">
                    {{ log.stop_reason }}
                  </span>
                </td>

                <td class="px-6 py-4">
                  <div class="text-xs font-bold text-[var(--primary)]">{{ log.actual_model }}</div>
                  <div v-if="log.original_model !== log.actual_model" class="text-[9px] text-[var(--text-secondary)]">← {{ log.original_model }}</div>
                </td>

                <td class="px-6 py-4">
                  <div class="flex items-center gap-2">
                    <div class="w-6 h-6 shrink-0 rounded-lg bg-indigo-500/10 flex items-center justify-center">
                      <Monitor class="w-3 h-3 text-indigo-500" />
                    </div>
                    <div class="min-w-0 flex flex-col gap-0.5">
                      <div class="text-xs font-bold truncate max-w-[150px]" :title="log.account">{{ log.account?.split('@')[0] }}</div>
                      <div class="flex items-center gap-1 text-[9px] text-[var(--text-secondary)]" :title="log.api_type + ' | Key: ' + getApiKeyDisplay(log.api_key_id)">
                        <Key class="w-2.5 h-2.5 shrink-0" v-if="log.api_key_id" />
                        <span class="truncate max-w-[130px] font-mono">{{ log.api_key_id ? getApiKeyDisplay(log.api_key_id) : (log.api_type || 'REST') }}</span>
                      </div>
                    </div>
                  </div>
                </td>

                <td class="px-6 py-4 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <Cpu class="w-3 h-3 text-amber-500" />
                    <span class="text-xs font-bold">{{ log.credits?.toFixed(4) || '0' }}</span>
                  </div>
                  <div class="text-[9px] text-[var(--text-secondary)]">${{ (log.cost_usd || 0).toFixed(4) }}</div>
                </td>

                <td class="px-6 py-4 text-right">
                  <div class="flex items-center justify-end gap-1.5">
                    <Zap class="w-3 h-3 text-amber-500" />
                    <span class="text-xs font-bold">{{ formatDuration(log.duration_ms) }}</span>
                  </div>
                  <div class="text-[9px] text-[var(--text-secondary)]">
                    {{ log.stream ? 'Stream' : 'Block' }}
                    <span v-if="log.request_id" class="ml-1 opacity-50">#{{ log.request_id }}</span>
                  </div>
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
                        <div class="text-xs font-bold font-mono text-[var(--primary)]">{{ log.actual_model }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">完整账号</div>
                        <div class="text-xs font-bold font-mono">{{ log.account || '-' }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">请求时间</div>
                        <div class="text-xs font-bold">{{ log.time }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">来源 API Key</div>
                        <div class="text-xs font-bold">{{ apiKeys.find(k => k.id === log.api_key_id)?.note || log.api_key_id || '未关联/内置' }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl opacity-50">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">Request ID</div>
                        <div class="text-xs font-bold font-mono text-sky-400">{{ log.request_id || '-' }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">Stop Reason</div>
                        <div class="text-xs font-bold font-mono" :class="{
                          'text-emerald-400': log.stop_reason === 'end_turn' || log.stop_reason === 'stop',
                          'text-amber-400': log.stop_reason === 'tool_use' || log.stop_reason === 'tool_calls',
                          'text-rose-400': log.stop_reason === 'max_tokens'
                        }">{{ log.stop_reason || '-' }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">耗时</div>
                        <div class="text-xs font-bold font-mono">{{ formatDuration(log.duration_ms) }}</div>
                      </div>
                      <div class="p-3 bg-[var(--bg)] rounded-xl">
                        <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">Credits</div>
                        <div class="text-xs font-bold font-mono text-amber-400">{{ log.credits?.toFixed(2) || '0' }}</div>
                      </div>
                    </div>
                    <!-- API Key Info -->
                    <div v-if="log.api_key_id" class="flex items-center gap-3 p-3 bg-[var(--bg)] rounded-xl">
                      <Key class="w-3.5 h-3.5 text-[var(--text-secondary)]" />
                      <span class="text-[10px] font-bold text-[var(--text-secondary)]">API Key:</span>
                      <span class="text-[10px] font-bold font-mono text-[var(--text)]">{{ log.api_key_id.slice(0, 8) }}...</span>
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

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="px-6 py-4 bg-[var(--bg)]/50 border-t border-[var(--border)] flex items-center justify-between">
        <span class="text-[10px] font-bold text-[var(--text-secondary)]">
          共 {{ totalLogs }} 条 · 第 {{ currentPage }}/{{ totalPages }} 页
        </span>
        <div class="flex items-center gap-1">
          <button @click="goToPage(1)" :disabled="currentPage <= 1"
            class="px-2.5 py-1 rounded-lg text-[10px] font-bold transition-all"
            :class="currentPage <= 1 ? 'text-[var(--text-secondary)]/30 cursor-not-allowed' : 'text-[var(--text-secondary)] hover:bg-[var(--card)] hover:text-[var(--text)]'">
            首页
          </button>
          <button @click="goToPage(currentPage - 1)" :disabled="currentPage <= 1"
            class="px-2.5 py-1 rounded-lg text-[10px] font-bold transition-all"
            :class="currentPage <= 1 ? 'text-[var(--text-secondary)]/30 cursor-not-allowed' : 'text-[var(--text-secondary)] hover:bg-[var(--card)] hover:text-[var(--text)]'">
            ‹ 上一页
          </button>
          <template v-for="p in totalPages" :key="p">
            <button v-if="p === 1 || p === totalPages || (p >= currentPage - 2 && p <= currentPage + 2)"
              @click="goToPage(p)"
              class="w-7 h-7 rounded-lg text-[10px] font-bold transition-all"
              :class="p === currentPage ? 'bg-[var(--primary)] text-white shadow-sm' : 'text-[var(--text-secondary)] hover:bg-[var(--card)]'">
              {{ p }}
            </button>
            <span v-else-if="p === currentPage - 3 || p === currentPage + 3" class="text-[var(--text-secondary)]/30 text-xs px-1">…</span>
          </template>
          <button @click="goToPage(currentPage + 1)" :disabled="currentPage >= totalPages"
            class="px-2.5 py-1 rounded-lg text-[10px] font-bold transition-all"
            :class="currentPage >= totalPages ? 'text-[var(--text-secondary)]/30 cursor-not-allowed' : 'text-[var(--text-secondary)] hover:bg-[var(--card)] hover:text-[var(--text)]'">
            下一页 ›
          </button>
          <button @click="goToPage(totalPages)" :disabled="currentPage >= totalPages"
            class="px-2.5 py-1 rounded-lg text-[10px] font-bold transition-all"
            :class="currentPage >= totalPages ? 'text-[var(--text-secondary)]/30 cursor-not-allowed' : 'text-[var(--text-secondary)] hover:bg-[var(--card)] hover:text-[var(--text)]'">
            末页
          </button>
        </div>
      </div>

      <!-- Footer -->
      <div class="px-6 py-3 bg-[var(--bg)]/50 border-t border-[var(--border)] flex justify-between items-center text-[10px] font-bold text-[var(--text-secondary)]">
        <span>显示 {{ filteredLogs.length }} / {{ totalLogs }} 条记录</span>
        <span>点击任意行查看详情</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overflow-x-auto::-webkit-scrollbar { height: 4px; }
.overflow-x-auto::-webkit-scrollbar-thumb { background: var(--border); border-radius: 10px; }
</style>
