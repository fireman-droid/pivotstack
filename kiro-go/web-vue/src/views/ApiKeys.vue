<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { formatNum } from '../utils/format'
import {
  Plus, Trash2, Copy, Eye, EyeOff, Key, Shield, Crown,
  Clock, ToggleLeft, ToggleRight, Pencil, Search,
  Activity, FileText, ChevronDown, X, Check, Ban
} from 'lucide-vue-next'

const { success, error: toastError } = useToast()

const keys = ref([])
const loading = ref(false)
const showCreate = ref(false)
const showKeyId = ref(null)
const expandedId = ref(null)
const searchQuery = ref('')
const keyLogs = ref({})
const keyLogsLoading = ref({})

// Create form
const form = ref({ tier: 'normal', plan: 'timed', durationDays: 30, customDate: '', balance: 0, note: '' })
const durationPresets = [
  { label: '1天', days: 1 },
  { label: '3天', days: 3 },
  { label: '7天', days: 7 },
  { label: '15天', days: 15 },
  { label: '30天', days: 30 },
  { label: '自选', days: -1 },
]

async function loadKeys() {
  loading.value = true
  try {
    const res = await api('/apikeys')
    if (res.ok) keys.value = await res.json()
  } catch { toastError('加载失败') }
  loading.value = false
}

async function createKey() {
  let expiresAt = 0
  if (form.value.durationDays === -1) {
    if (!form.value.customDate) return toastError('请选择到期日期')
    expiresAt = Math.floor(new Date(form.value.customDate).getTime() / 1000)
  } else if (form.value.durationDays > 0) {
    expiresAt = Math.floor(Date.now() / 1000) + form.value.durationDays * 86400
  }
  try {
    const res = await api('/apikeys', {
      method: 'POST',
      body: JSON.stringify({ tier: form.value.tier, plan: form.value.plan, expiresAt, balance: form.value.balance, note: form.value.note })
    })
    if (res.ok) {
      const newKey = await res.json()
      keys.value.unshift(newKey)
      showCreate.value = false
      showKeyId.value = newKey.id
      form.value = { tier: 'normal', plan: 'timed', durationDays: 30, customDate: '', balance: 0, note: '' }
      success('API Key 已创建')
    }
  } catch { toastError('创建失败') }
}

async function toggleKey(k) {
  try {
    await api(`/apikeys/${k.id}`, {
      method: 'PUT', body: JSON.stringify({ enabled: !k.enabled })
    })
    k.enabled = !k.enabled
    success(k.enabled ? '已启用' : '已禁用')
  } catch { toastError('操作失败') }
}

async function deleteKey(k) {
  if (!confirm(`确认删除 Key "${k.note || k.id.slice(0, 8)}"？`)) return
  try {
    await api(`/apikeys/${k.id}`, { method: 'DELETE' })
    keys.value = keys.value.filter(x => x.id !== k.id)
    success('已删除')
  } catch { toastError('删除失败') }
}

async function updateTier(k, tier) {
  try {
    await api(`/apikeys/${k.id}`, { method: 'PUT', body: JSON.stringify({ tier }) })
    k.tier = tier
    success('等级已更新')
  } catch { toastError('更新失败') }
}

async function loadKeyLogs(keyId) {
  if (keyLogs.value[keyId]) return
  keyLogsLoading.value[keyId] = true
  try {
    const res = await api(`/apikeys/${keyId}/logs`)
    if (res.ok) {
      const d = await res.json()
      keyLogs.value[keyId] = d.logs || []
    }
  } catch {}
  keyLogsLoading.value[keyId] = false
}

function toggleExpand(k) {
  if (expandedId.value === k.id) {
    expandedId.value = null
  } else {
    expandedId.value = k.id
    loadKeyLogs(k.id)
  }
}

function copyText(text) {
  navigator.clipboard?.writeText(text)
  success('已复制')
}

function maskKey(key) {
  if (!key) return ''
  return key.slice(0, 7) + '••••••••' + key.slice(-4)
}

function formatDate(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function expiryStatus(k) {
  if (!k.expiresAt) return { text: '永不过期', color: 'emerald' }
  const diff = k.expiresAt - Date.now() / 1000
  if (diff <= 0) return { text: '已过期', color: 'rose' }
  if (diff < 86400) return { text: Math.floor(diff / 3600) + 'h', color: 'amber' }
  return { text: Math.floor(diff / 86400) + '天', color: 'sky' }
}

const filteredKeys = computed(() => {
  if (!searchQuery.value) return keys.value
  const q = searchQuery.value.toLowerCase()
  return keys.value.filter(k =>
    k.note?.toLowerCase().includes(q) ||
    k.key?.toLowerCase().includes(q) ||
    k.tier?.toLowerCase().includes(q) ||
    k.id?.toLowerCase().includes(q)
  )
})

const stats = computed(() => {
  const all = keys.value
  return {
    total: all.length,
    active: all.filter(k => k.enabled).length,
    normal: all.filter(k => k.tier === 'normal').length,
    pro: all.filter(k => k.tier === 'pro').length,
    totalReqs: all.reduce((s, k) => s + (k.requests || 0), 0),
    totalCredits: all.reduce((s, k) => s + (k.credits || 0), 0),
  }
})

onMounted(loadKeys)
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div class="space-y-1">
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">API Key 管理</h1>
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
          <Key class="w-3.5 h-3.5 text-[var(--primary)]" />
          商业密钥发放 · 共 {{ stats.total }} 个 · {{ stats.active }} 个活跃
        </p>
      </div>
      <button @click="showCreate = true"
        class="flex items-center gap-2 px-5 py-2.5 bg-[var(--primary)] text-white rounded-xl text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-95 transition-all">
        <Plus class="w-4 h-4" /> 创建 Key
      </button>
    </div>

    <!-- Stats Row -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="modern-card p-4">
        <div class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] mb-1">普通卡</div>
        <div class="text-xl font-black text-[var(--text)]">{{ stats.normal }}</div>
      </div>
      <div class="modern-card p-4">
        <div class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] mb-1">Pro 卡</div>
        <div class="text-xl font-black text-amber-500">{{ stats.pro }}</div>
      </div>
      <div class="modern-card p-4">
        <div class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] mb-1">总请求</div>
        <div class="text-xl font-black text-[var(--text)]">{{ formatNum(stats.totalReqs) }}</div>
      </div>
      <div class="modern-card p-4">
        <div class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] mb-1">总 Credits</div>
        <div class="text-xl font-black text-emerald-500">{{ stats.totalCredits.toFixed(1) }}</div>
      </div>
    </div>

    <!-- Search -->
    <div class="relative">
      <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-secondary)]" />
      <input v-model="searchQuery" placeholder="搜索备注、Key 或 ID..."
        class="w-full h-10 pl-11 pr-4 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-[var(--primary)] transition-all" />
    </div>

    <!-- Create Modal -->
    <Teleport to="body">
      <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center p-4" @click.self="showCreate = false">
        <div class="fixed inset-0 bg-black/50 backdrop-blur-sm"></div>
        <div class="relative w-full max-w-lg bg-[var(--card)] border border-[var(--border)] rounded-2xl shadow-2xl overflow-hidden">
          <div class="px-6 py-4 border-b border-[var(--border)] flex items-center justify-between">
            <h3 class="text-sm font-black text-[var(--text)]">创建 API Key</h3>
            <button @click="showCreate = false" class="p-1 hover:bg-[var(--bg)] rounded-lg"><X class="w-4 h-4" /></button>
          </div>
          <div class="p-6 space-y-5">
            <!-- Tier -->
            <div class="space-y-2">
              <label class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">等级</label>
              <div class="grid grid-cols-2 gap-3">
                <button @click="form.tier = 'normal'"
                  class="p-4 rounded-xl border-2 text-left transition-all"
                  :class="form.tier === 'normal' ? 'border-[var(--primary)] bg-[var(--primary)]/5' : 'border-[var(--border)] hover:border-[var(--primary)]/30'">
                  <div class="flex items-center gap-2 mb-1">
                    <Shield class="w-4 h-4 text-sky-500" />
                    <span class="text-sm font-bold">普通卡</span>
                  </div>
                  <p class="text-[10px] text-[var(--text-secondary)]">仅限 Claude 4.5 Sonnet</p>
                </button>
                <button @click="form.tier = 'pro'"
                  class="p-4 rounded-xl border-2 text-left transition-all"
                  :class="form.tier === 'pro' ? 'border-amber-500 bg-amber-500/5' : 'border-[var(--border)] hover:border-amber-500/30'">
                  <div class="flex items-center gap-2 mb-1">
                    <Crown class="w-4 h-4 text-amber-500" />
                    <span class="text-sm font-bold">Pro 卡</span>
                  </div>
                  <p class="text-[10px] text-[var(--text-secondary)]">全部模型可用</p>
                </button>
              </div>
            </div>

            <!-- Plan -->
            <div class="space-y-2">
              <label class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">计费模式</label>
              <div class="grid grid-cols-3 gap-2">
                <button @click="form.plan = 'timed'"
                  class="px-3 py-2 rounded-lg text-xs font-bold transition-all text-center"
                  :class="form.plan === 'timed' ? 'bg-sky-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)] hover:text-[var(--text)]'">
                  ⏱ 时间制
                </button>
                <button @click="form.plan = 'credit'"
                  class="px-3 py-2 rounded-lg text-xs font-bold transition-all text-center"
                  :class="form.plan === 'credit' ? 'bg-emerald-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)] hover:text-[var(--text)]'">
                  💰 计量制
                </button>
                <button @click="form.plan = 'hybrid'"
                  class="px-3 py-2 rounded-lg text-xs font-bold transition-all text-center"
                  :class="form.plan === 'hybrid' ? 'bg-purple-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)] hover:text-[var(--text)]'">
                  🔀 混合制
                </button>
              </div>
            </div>

            <!-- Duration (for timed/hybrid) -->
            <div v-if="form.plan !== 'credit'" class="space-y-2">
              <label class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">有效期</label>
              <div class="flex flex-wrap gap-2">
                <button v-for="p in durationPresets" :key="p.days" @click="form.durationDays = p.days"
                  class="px-3 py-1.5 rounded-lg text-xs font-bold transition-all"
                  :class="form.durationDays === p.days ? 'bg-[var(--primary)] text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)] hover:text-[var(--text)]'">
                  {{ p.label }}
                </button>
              </div>
              <input v-if="form.durationDays === -1" v-model="form.customDate" type="datetime-local"
                class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)] mt-2" />
            </div>

            <!-- Balance (for credit/hybrid) -->
            <div v-if="form.plan !== 'timed'" class="space-y-2">
              <label class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">初始余额 (¥)</label>
              <input v-model.number="form.balance" type="number" step="0.01" min="0" placeholder="0.00"
                class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
            </div>

            <!-- Note -->
            <div class="space-y-2">
              <label class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">备注</label>
              <input v-model="form.note" placeholder="用户名 / 用途说明"
                class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
            </div>
          </div>
          <div class="px-6 py-4 border-t border-[var(--border)] flex justify-end gap-3">
            <button @click="showCreate = false" class="px-4 py-2 text-sm font-bold text-[var(--text-secondary)] hover:text-[var(--text)]">取消</button>
            <button @click="createKey" class="px-5 py-2 bg-[var(--primary)] text-white rounded-xl text-sm font-bold hover:scale-[1.02] active:scale-95 transition-all">确认创建</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Key List -->
    <div class="space-y-3">
      <div v-for="k in filteredKeys" :key="k.id" class="modern-card overflow-hidden transition-all"
        :class="{ 'opacity-50': !k.enabled }">
        <!-- Main Row -->
        <div class="p-5 flex items-center gap-4 cursor-pointer" @click="toggleExpand(k)">
          <!-- Tier Badge -->
          <div class="shrink-0">
            <div v-if="k.tier === 'pro'" class="w-10 h-10 rounded-xl bg-amber-500/10 flex items-center justify-center">
              <Crown class="w-5 h-5 text-amber-500" />
            </div>
            <div v-else class="w-10 h-10 rounded-xl bg-sky-500/10 flex items-center justify-center">
              <Shield class="w-5 h-5 text-sky-500" />
            </div>
          </div>

          <!-- Info -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span class="text-sm font-bold text-[var(--text)] truncate">{{ k.note || 'Unnamed Key' }}</span>
              <span class="px-1.5 py-0.5 rounded text-[9px] font-bold uppercase"
                :class="k.tier === 'pro' ? 'bg-amber-500/10 text-amber-500' : 'bg-sky-500/10 text-sky-500'">
                {{ k.tier }}
              </span>
              <span class="px-1.5 py-0.5 rounded text-[9px] font-bold"
                :class="{ 'bg-sky-500/10 text-sky-400': k.plan === 'timed', 'bg-emerald-500/10 text-emerald-400': k.plan === 'credit', 'bg-purple-500/10 text-purple-400': k.plan === 'hybrid' }">
                {{ k.plan === 'timed' ? '时间制' : k.plan === 'credit' ? '计量制' : k.plan === 'hybrid' ? '混合制' : k.plan }}
              </span>
              <span v-if="k.plan !== 'timed' && k.balance !== undefined" class="px-1.5 py-0.5 rounded text-[9px] font-bold"
                :class="k.balance < 1 ? 'bg-rose-500/10 text-rose-400' : 'bg-emerald-500/10 text-emerald-400'">
                ¥{{ (k.balance || 0).toFixed(2) }}
              </span>
              <span v-if="!k.enabled" class="px-1.5 py-0.5 rounded bg-rose-500/10 text-rose-500 text-[9px] font-bold">禁用</span>
            </div>
            <div class="flex items-center gap-3 text-[10px] text-[var(--text-secondary)]">
              <span class="font-mono">{{ showKeyId === k.id ? k.key : maskKey(k.key) }}</span>
              <button @click.stop="showKeyId = showKeyId === k.id ? null : k.id" class="hover:text-[var(--primary)]">
                <Eye v-if="showKeyId !== k.id" class="w-3 h-3" />
                <EyeOff v-else class="w-3 h-3" />
              </button>
              <button @click.stop="copyText(k.key)" class="hover:text-[var(--primary)]">
                <Copy class="w-3 h-3" />
              </button>
            </div>
          </div>

          <!-- Stats -->
          <div class="hidden md:flex items-center gap-6 text-xs shrink-0">
            <div class="text-center">
              <div class="font-bold text-[var(--text)]">{{ formatNum(k.requests || 0) }}</div>
              <div class="text-[9px] text-[var(--text-secondary)]">请求</div>
            </div>
            <div class="text-center">
              <div class="font-bold text-[var(--text)]">{{ formatNum(k.tokens || 0) }}</div>
              <div class="text-[9px] text-[var(--text-secondary)]">Token</div>
            </div>
            <div class="text-center">
              <div class="font-bold" :class="`text-${expiryStatus(k).color}-500`">{{ expiryStatus(k).text }}</div>
              <div class="text-[9px] text-[var(--text-secondary)]">到期</div>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1 shrink-0">
            <button @click.stop="toggleKey(k)" class="p-2 rounded-lg hover:bg-[var(--bg)] transition-colors" :title="k.enabled ? '禁用' : '启用'">
              <ToggleRight v-if="k.enabled" class="w-4 h-4 text-emerald-500" />
              <ToggleLeft v-else class="w-4 h-4 text-[var(--text-secondary)]" />
            </button>
            <button @click.stop="deleteKey(k)" class="p-2 rounded-lg hover:bg-rose-500/10 transition-colors">
              <Trash2 class="w-4 h-4 text-rose-500" />
            </button>
            <ChevronDown class="w-4 h-4 text-[var(--text-secondary)] transition-transform" :class="{ 'rotate-180': expandedId === k.id }" />
          </div>
        </div>

        <!-- Expanded Detail -->
        <div v-if="expandedId === k.id" class="border-t border-[var(--border)]">
          <!-- Quick Stats -->
          <div class="grid grid-cols-2 md:grid-cols-5 gap-3 p-5 bg-[var(--bg)]/50">
            <div class="p-3 bg-[var(--card)] rounded-xl">
              <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">请求数</div>
              <div class="text-sm font-black">{{ (k.requests || 0).toLocaleString() }}</div>
            </div>
            <div class="p-3 bg-[var(--card)] rounded-xl">
              <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">错误数</div>
              <div class="text-sm font-black text-rose-500">{{ (k.errors || 0).toLocaleString() }}</div>
            </div>
            <div class="p-3 bg-[var(--card)] rounded-xl">
              <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">Token</div>
              <div class="text-sm font-black">{{ formatNum(k.tokens || 0) }}</div>
            </div>
            <div class="p-3 bg-[var(--card)] rounded-xl">
              <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">Credits</div>
              <div class="text-sm font-black text-emerald-500">{{ (k.credits || 0).toFixed(2) }}</div>
            </div>
            <div v-if="k.plan !== 'timed'" class="p-3 bg-[var(--card)] rounded-xl">
              <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">余额</div>
              <div class="text-sm font-black" :class="k.balance < 1 ? 'text-rose-500' : 'text-emerald-500'">¥{{ (k.balance || 0).toFixed(2) }}</div>
            </div>
            <div class="p-3 bg-[var(--card)] rounded-xl">
              <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">创建时间</div>
              <div class="text-sm font-black">{{ formatDate(k.createdAt) }}</div>
            </div>
          </div>

          <!-- Model Usage -->
          <div v-if="k.models && Object.keys(k.models).length" class="px-5 py-3 border-t border-[var(--border)]">
            <div class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] mb-2">模型使用分布</div>
            <div class="flex flex-wrap gap-2">
              <span v-for="(count, model) in k.models" :key="model"
                class="px-2 py-1 bg-[var(--bg)] rounded-lg text-[10px] font-bold">
                {{ model }} <span class="text-[var(--primary)]">{{ count }}</span>
              </span>
            </div>
          </div>

          <!-- Tier Switch -->
          <div class="px-5 py-3 border-t border-[var(--border)] flex items-center gap-3">
            <span class="text-[10px] font-bold text-[var(--text-secondary)]">切换等级:</span>
            <button @click="updateTier(k, 'normal')"
              class="px-3 py-1 rounded-lg text-[10px] font-bold transition-all"
              :class="k.tier === 'normal' ? 'bg-sky-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)] hover:text-[var(--text)]'">
              普通
            </button>
            <button @click="updateTier(k, 'pro')"
              class="px-3 py-1 rounded-lg text-[10px] font-bold transition-all"
              :class="k.tier === 'pro' ? 'bg-amber-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)] hover:text-[var(--text)]'">
              Pro
            </button>
          </div>

          <!-- Recent Logs -->
          <div class="px-5 py-3 border-t border-[var(--border)]">
            <div class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] mb-2">最近调用</div>
            <div v-if="keyLogsLoading[k.id]" class="text-xs text-[var(--text-secondary)] py-2">加载中...</div>
            <div v-else-if="!keyLogs[k.id]?.length" class="text-xs text-[var(--text-secondary)] py-2">暂无记录</div>
            <div v-else class="space-y-1 max-h-48 overflow-y-auto">
              <div v-for="(log, li) in keyLogs[k.id].slice(0, 20)" :key="li"
                class="flex items-center gap-3 py-1.5 text-[10px]">
                <span class="font-mono text-[var(--text-secondary)] w-24 shrink-0">{{ log.time }}</span>
                <span :class="log.status === 'error' ? 'text-rose-500' : 'text-emerald-500'" class="w-8 font-bold shrink-0">
                  {{ log.status === 'error' ? '失败' : '成功' }}
                </span>
                <span class="font-bold text-[var(--primary)] truncate">{{ log.actual_model }}</span>
                <span class="ml-auto text-[var(--text-secondary)] shrink-0">{{ (log.total_tokens || 0).toLocaleString() }} tok</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty -->
    <div v-if="!loading && !filteredKeys.length" class="text-center py-16">
      <Key class="w-10 h-10 text-[var(--text-secondary)] opacity-20 mx-auto mb-3" />
      <div class="text-sm font-bold text-[var(--text-secondary)]">{{ searchQuery ? '没有匹配的 Key' : '还没有 API Key' }}</div>
      <button v-if="!searchQuery" @click="showCreate = true" class="mt-3 text-sm text-[var(--primary)] font-bold hover:underline">创建第一个 Key</button>
    </div>
  </div>
</template>
