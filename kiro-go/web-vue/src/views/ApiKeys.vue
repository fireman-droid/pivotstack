<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import {
  Plus, Trash2, Copy, Eye, EyeOff, Key,
  ToggleLeft, ToggleRight, Pencil, Search,
  ChevronDown, X, Check, Save, Clock, Wallet
} from 'lucide-vue-next'

const { success, error: toastError } = useToast()

const keys = ref([])
const loading = ref(false)
const showCreate = ref(false)
const showKeyId = ref(null)
const expandedId = ref(null)
const searchQuery = ref('')
const editingId = ref(null)
const editForm = reactive({ note: '', balance: 0, expiresAt: 0 })

const form = ref({ note: '' })

async function loadKeys() {
  loading.value = true
  try {
    const res = await api('/apikeys')
    if (res.ok) keys.value = await res.json()
  } catch { toastError('加载失败') }
  loading.value = false
}

async function createKey() {
  try {
    const res = await api('/apikeys', {
      method: 'POST',
      body: JSON.stringify({ note: form.value.note })
    })
    if (res.ok) {
      const newKey = await res.json()
      keys.value.unshift(newKey)
      showCreate.value = false
      showKeyId.value = newKey.id
      form.value = { note: '' }
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
  if (!confirm(`确认删除 Key "${k.note || k.id.slice(0, 8)}"？此操作不可撤销。`)) return
  try {
    await api(`/apikeys/${k.id}`, { method: 'DELETE' })
    keys.value = keys.value.filter(x => x.id !== k.id)
    success('已删除')
  } catch { toastError('删除失败') }
}

function startEdit(k) {
  editingId.value = k.id
  editForm.note = k.note || ''
  editForm.balance = k.balance || 0
  editForm.expiresAt = k.expiresAt || 0
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(k) {
  try {
    const body = {
      note: editForm.note,
      balance: Number(editForm.balance),
      expiresAt: Number(editForm.expiresAt),
    }
    const res = await api(`/apikeys/${k.id}`, {
      method: 'PUT', body: JSON.stringify(body)
    })
    if (res.ok) {
      k.note = editForm.note
      k.balance = body.balance
      k.expiresAt = body.expiresAt
      editingId.value = null
      success('已保存')
    }
  } catch { toastError('保存失败') }
}

function toggleExpand(k) {
  expandedId.value = expandedId.value === k.id ? null : k.id
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
  return new Date(ts * 1000).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function timeRemaining(expiresAt) {
  if (!expiresAt) return { text: '永不过期', class: 'ok' }
  const diff = expiresAt - Date.now() / 1000
  if (diff <= 0) return { text: '已过期', class: 'danger' }
  const days = Math.floor(diff / 86400)
  const hours = Math.floor((diff % 86400) / 3600)
  const mins = Math.max(1, Math.ceil((diff % 3600) / 60))
  let text = ''
  if (days > 0) text += `${days}天`
  if (hours > 0) text += `${hours}小时`
  if (days === 0 && mins > 0) text += `${mins}分钟`
  const cls = days < 3 ? (days < 1 ? 'danger' : 'warning') : 'ok'
  return { text: text || '1分钟', class: cls }
}

function subscriptionInfo(k) {
  const parts = []
  if (k.expiresAt) {
    const tr = timeRemaining(k.expiresAt)
    parts.push({ label: '剩余', value: tr.text, class: tr.class })
  }
  if (k.balance !== undefined && k.balance !== null) {
    const cls = k.balance < 1 ? 'danger' : 'ok'
    parts.push({ label: '余额', value: `¥${k.balance.toFixed(2)}`, class: cls })
  }
  return parts
}

// 编辑 expiresAt 辅助
function expiresAtDisplay(ts) {
  if (!ts) return '未设置'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

function addTime(amount) {
  const now = Math.floor(Date.now() / 1000)
  const base = editForm.expiresAt > now ? editForm.expiresAt : now
  editForm.expiresAt = Math.max(now, base + amount)
}

const filteredKeys = computed(() => {
  if (!searchQuery.value) return keys.value
  const q = searchQuery.value.toLowerCase()
  return keys.value.filter(k =>
    k.note?.toLowerCase().includes(q) ||
    k.key?.toLowerCase().includes(q) ||
    k.id?.toLowerCase().includes(q)
  )
})

onMounted(loadKeys)
</script>

<template>
  <div class="space-y-5 max-w-[1400px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div class="space-y-1">
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">API Key 管理</h1>
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
          <Key class="w-3.5 h-3.5 text-[var(--primary)]" />
          共 {{ keys.length }} 个 · {{ keys.filter(k => k.enabled).length }} 个活跃
        </p>
      </div>
      <button @click="showCreate = true"
        class="flex items-center gap-2 px-5 py-2.5 bg-[var(--primary)] text-white rounded-xl text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-95 transition-all">
        <Plus class="w-4 h-4" /> 创建 Key
      </button>
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
          <div class="p-6 space-y-4">
            <p class="text-xs text-[var(--text-secondary)] leading-relaxed">
              创建后需通过兑换激活码来充值时间或余额，才能开始使用。
            </p>
            <div class="space-y-2">
              <label class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">备注（用户名）</label>
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
          <!-- Icon -->
          <div class="shrink-0">
            <div class="w-10 h-10 rounded-xl bg-[var(--primary)]/10 flex items-center justify-center">
              <Key class="w-5 h-5 text-[var(--primary)]" />
            </div>
          </div>

          <!-- Info -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span class="text-sm font-bold text-[var(--text)] truncate">{{ k.note || 'Unnamed Key' }}</span>
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

          <!-- Subscription Info -->
          <div class="hidden md:flex items-center gap-4 shrink-0">
            <template v-for="(item, idx) in subscriptionInfo(k)" :key="idx">
              <div class="text-center min-w-[60px]">
                <div class="text-xs font-bold"
                  :class="{
                    'text-emerald-500': item.class === 'ok',
                    'text-amber-500': item.class === 'warning',
                    'text-rose-500': item.class === 'danger'
                  }">{{ item.value }}</div>
                <div class="text-[9px] text-[var(--text-secondary)]">{{ item.label }}</div>
              </div>
            </template>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1 shrink-0">
            <button @click.stop="toggleKey(k)" class="p-2 rounded-lg hover:bg-[var(--bg)] transition-colors" :title="k.enabled ? '禁用' : '启用'">
              <ToggleRight v-if="k.enabled" class="w-4 h-4 text-emerald-500" />
              <ToggleLeft v-else class="w-4 h-4 text-[var(--text-secondary)]" />
            </button>
            <button @click.stop="startEdit(k); expandedId = k.id" class="p-2 rounded-lg hover:bg-[var(--bg)] transition-colors" title="编辑">
              <Pencil class="w-4 h-4 text-[var(--text-secondary)]" />
            </button>
            <button @click.stop="deleteKey(k)" class="p-2 rounded-lg hover:bg-rose-500/10 transition-colors" title="删除">
              <Trash2 class="w-4 h-4 text-rose-500" />
            </button>
            <ChevronDown class="w-4 h-4 text-[var(--text-secondary)] transition-transform" :class="{ 'rotate-180': expandedId === k.id }" />
          </div>
        </div>

        <!-- Expanded Detail / Edit -->
        <div v-if="expandedId === k.id" class="border-t border-[var(--border)]">
          <!-- Edit Mode -->
          <div v-if="editingId === k.id" class="p-5 space-y-4 bg-[var(--bg)]/50">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-[var(--primary)]">✏️ 编辑信息</span>
              <div class="flex gap-2">
                <button @click="cancelEdit" class="px-3 py-1.5 text-xs font-bold text-[var(--text-secondary)] hover:text-[var(--text)] rounded-lg hover:bg-[var(--card)]">取消</button>
                <button @click="saveEdit(k)" class="px-3 py-1.5 text-xs font-bold text-white bg-[var(--primary)] rounded-lg hover:scale-[1.02] active:scale-95 transition-all flex items-center gap-1">
                  <Save class="w-3 h-3" /> 保存
                </button>
              </div>
            </div>

            <!-- Note -->
            <div class="space-y-1">
              <label class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">备注</label>
              <input v-model="editForm.note" class="w-full h-9 px-3 bg-[var(--card)] border border-[var(--border)] rounded-lg text-sm outline-none focus:border-[var(--primary)]" />
            </div>

            <!-- Balance -->
            <div class="space-y-1">
              <label class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">余额 (¥)</label>
              <input v-model.number="editForm.balance" type="number" step="0.01"
                class="w-full h-9 px-3 bg-[var(--card)] border border-[var(--border)] rounded-lg text-sm outline-none focus:border-[var(--primary)]" />
            </div>

            <!-- ExpiresAt -->
            <div class="space-y-2">
              <label class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">到期时间</label>
              <div class="text-xs text-[var(--text-secondary)] mb-1">
                当前：{{ editForm.expiresAt ? expiresAtDisplay(editForm.expiresAt) : '未设置（永不过期）' }}
              </div>
              <div class="flex flex-wrap gap-2">
                <button @click="addTime(3600)" class="time-btn">+1小时</button>
                <button @click="addTime(86400)" class="time-btn">+1天</button>
                <button @click="addTime(3 * 86400)" class="time-btn">+3天</button>
                <button @click="addTime(7 * 86400)" class="time-btn">+7天</button>
                <button @click="addTime(30 * 86400)" class="time-btn">+30天</button>
                <button @click="editForm.expiresAt = Math.floor(Date.now()/1000)" class="time-btn danger">重置为0</button>
                <button @click="editForm.expiresAt = 0" class="time-btn danger">永不过期</button>
              </div>
            </div>
          </div>

          <!-- View Mode -->
          <div v-else class="p-5 bg-[var(--bg)]/50">
            <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div class="info-cell">
                <div class="info-label">余额</div>
                <div class="info-value" :class="(k.balance || 0) < 1 ? 'text-rose-500' : 'text-emerald-500'">
                  ¥{{ (k.balance || 0).toFixed(2) }}
                </div>
              </div>
              <div class="info-cell">
                <div class="info-label">到期时间</div>
                <div class="info-value" :class="{
                  'text-emerald-500': timeRemaining(k.expiresAt).class === 'ok',
                  'text-amber-500': timeRemaining(k.expiresAt).class === 'warning',
                  'text-rose-500': timeRemaining(k.expiresAt).class === 'danger'
                }">
                  {{ timeRemaining(k.expiresAt).text }}
                </div>
                <div v-if="k.expiresAt" class="text-[9px] text-[var(--text-secondary)] mt-0.5">{{ formatDate(k.expiresAt) }}</div>
              </div>
              <div class="info-cell">
                <div class="info-label">创建时间</div>
                <div class="info-value">{{ formatDate(k.createdAt) }}</div>
              </div>
              <div class="info-cell">
                <div class="info-label">最后使用</div>
                <div class="info-value">{{ k.lastUsed ? formatDate(k.lastUsed) : '从未' }}</div>
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

<style scoped>
.time-btn {
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s;
}
.time-btn:hover {
  border-color: var(--primary);
  color: var(--primary);
  background: rgba(99, 102, 241, 0.05);
}
.time-btn.danger {
  color: #ef4444;
}
.time-btn.danger:hover {
  border-color: #ef4444;
  background: rgba(239, 68, 68, 0.05);
}

.info-cell {
  padding: 0.75rem;
  background: var(--card);
  border-radius: 10px;
}
.info-label {
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin-bottom: 0.25rem;
}
.info-value {
  font-size: 0.875rem;
  font-weight: 700;
}
</style>
