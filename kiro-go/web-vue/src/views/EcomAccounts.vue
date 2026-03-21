<script setup>
import { onMounted, computed, ref } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import {
  Globe, Plus, Trash2, Power, PowerOff, RotateCw,
  Sparkles, X, CheckCircle2, ShieldAlert, Activity, Zap, RefreshCw
} from 'lucide-vue-next'

const { success, error } = useToast()
const accounts = ref([])
const stats = ref({ total: 0, available: 0 })
const loading = ref(true)
const refreshing = ref(false)
const showImportDialog = ref(false)
const jsonText = ref('')
const isImporting = ref(false)

async function loadAccounts() {
  loading.value = true
  try {
    const [accRes, statsRes] = await Promise.all([
      api('/ecom/accounts'),
      api('/ecom/stats')
    ])
    accounts.value = await accRes.json()
    stats.value = await statsRes.json()
  } catch (e) {
    error('加载失败: ' + e.message)
  }
  loading.value = false
}

async function refreshUpstream() {
  refreshing.value = true
  try {
    const res = await api('/ecom/refresh', { method: 'POST' })
    const data = await res.json()
    if (data.success) {
      success(`刷新完成：${data.refreshed} 成功，${data.failed} 失败`)
      await loadAccounts()
    } else {
      error('刷新失败')
    }
  } catch (e) {
    error('刷新失败: ' + e.message)
  }
  refreshing.value = false
}

onMounted(loadAccounts)

const summary = computed(() => {
  const all = accounts.value.length
  const enabled = accounts.value.filter(a => a.enabled).length
  const withToken = accounts.value.filter(a => a.hasAccessToken).length
  const totalRequests = accounts.value.reduce((s, a) => s + (a.requestCount || 0), 0)
  return { all, enabled, withToken, totalRequests }
})

async function importAccounts() {
  if (!jsonText.value.trim()) { error('请粘贴 EcomAgent 账号 JSON'); return }
  let parsed
  try { parsed = JSON.parse(jsonText.value.trim()) } catch { error('JSON 格式错误'); return }
  if (!Array.isArray(parsed)) parsed = [parsed]

  isImporting.value = true
  try {
    const res = await api('/ecom/accounts', {
      method: 'POST',
      body: JSON.stringify(parsed)
    })
    const data = await res.json()
    if (data.success) {
      success(`导入完成：${data.imported} 成功，${data.skipped} 跳过，${data.filtered} 过滤`)
      showImportDialog.value = false
      jsonText.value = ''
      await loadAccounts()
    } else {
      error(data.error || '导入失败')
    }
  } catch (e) {
    error('导入失败: ' + e.message)
  }
  isImporting.value = false
}

async function toggleAccount(id) {
  try {
    await api(`/ecom/accounts/${id}/toggle`, { method: 'POST' })
    success('状态已切换')
    await loadAccounts()
  } catch (e) {
    error('操作失败: ' + e.message)
  }
}

async function deleteAccount(id) {
  if (!confirm('确定删除这个 EcomAgent 账号？')) return
  try {
    await api(`/ecom/accounts/${id}`, { method: 'DELETE' })
    success('已删除')
    await loadAccounts()
  } catch (e) {
    error('删除失败: ' + e.message)
  }
}

function formatTime(ts) {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function formatTokens(n) {
  if (!n) return '0'
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

// Parse token limit like "10M" to a number
function parseLimit(str) {
  if (!str) return 0
  str = String(str).trim().toUpperCase()
  if (str.endsWith('M')) return parseFloat(str) * 1000000
  if (str.endsWith('K')) return parseFloat(str) * 1000
  return parseInt(str) || 0
}

// Calculate usage percentage
function usagePercent(used, limitStr) {
  const limit = parseLimit(limitStr)
  if (!limit) return null
  return Math.min(100, (used / limit * 100))
}

// Get progress bar color
function progressColor(pct) {
  if (pct === null) return 'bg-gray-300'
  if (pct >= 90) return 'bg-red-500'
  if (pct >= 70) return 'bg-amber-500'
  return 'bg-green-500'
}
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div class="space-y-1">
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">EcomAgent 号池</h1>
        <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
          <Sparkles class="w-3.5 h-3.5 text-amber-500" />
          独立号池 · 透传至 api.ecomagent.in
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button @click="refreshUpstream" :disabled="refreshing"
          class="flex items-center gap-2 px-4 py-2.5 bg-[var(--card)] border border-[var(--border)] rounded-xl hover:bg-[var(--bg)] transition-all disabled:opacity-50 text-sm font-bold">
          <RefreshCw class="w-4 h-4 text-amber-500" :class="{ 'animate-spin': refreshing }" />
          {{ refreshing ? '刷新中...' : '刷新额度' }}
        </button>
        <button @click="loadAccounts" :disabled="loading"
          class="p-2.5 bg-[var(--card)] border border-[var(--border)] rounded-xl hover:bg-[var(--bg)] transition-all disabled:opacity-50">
          <RotateCw class="w-4 h-4 text-[var(--text-secondary)]" :class="{ 'animate-spin': loading }" />
        </button>
        <button @click="showImportDialog = true"
          class="flex items-center gap-2 px-5 py-2.5 bg-[var(--primary)] text-white rounded-xl font-bold text-sm shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-[0.98] transition-all">
          <Plus class="w-4 h-4" /> 导入账号
        </button>
      </div>
    </div>

    <!-- Stats Cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <div class="modern-card p-4">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-blue-500/10"><Globe class="w-4 h-4 text-blue-500" /></div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ summary.all }}</div>
            <div class="text-[10px] font-bold text-[var(--text-secondary)] uppercase tracking-wider">总账号</div>
          </div>
        </div>
      </div>
      <div class="modern-card p-4">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-green-500/10"><CheckCircle2 class="w-4 h-4 text-green-500" /></div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ summary.enabled }}</div>
            <div class="text-[10px] font-bold text-[var(--text-secondary)] uppercase tracking-wider">启用</div>
          </div>
        </div>
      </div>
      <div class="modern-card p-4">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-amber-500/10"><Zap class="w-4 h-4 text-amber-500" /></div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ summary.withToken }}</div>
            <div class="text-[10px] font-bold text-[var(--text-secondary)] uppercase tracking-wider">有Token</div>
          </div>
        </div>
      </div>
      <div class="modern-card p-4">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-purple-500/10"><Activity class="w-4 h-4 text-purple-500" /></div>
          <div>
            <div class="text-2xl font-black leading-tight">{{ summary.totalRequests }}</div>
            <div class="text-[10px] font-bold text-[var(--text-secondary)] uppercase tracking-wider">本地请求</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Account Table -->
    <div class="modern-card overflow-hidden">
      <div v-if="loading" class="p-12 flex items-center justify-center">
        <div class="w-8 h-8 border-3 border-[var(--primary)]/20 border-t-[var(--primary)] rounded-full animate-spin"></div>
      </div>

      <div v-else-if="accounts.length === 0" class="flex flex-col items-center justify-center py-16">
        <Globe class="w-12 h-12 text-[var(--text-secondary)] opacity-15 mb-4" />
        <h3 class="text-lg font-black mb-2">暂无 EcomAgent 账号</h3>
        <p class="text-sm text-[var(--text-secondary)] mb-4">点击「导入账号」添加 EcomAgent 账号到号池</p>
      </div>

      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm min-w-[1100px]">
          <thead>
            <tr class="border-b border-[var(--border)] bg-[var(--bg)]/50">
              <th class="text-left px-4 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">Email</th>
              <th class="text-center px-4 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">状态</th>
              <th class="text-center px-4 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">套餐</th>
              <th class="text-center px-3 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">请求用量</th>
              <th class="text-center px-3 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">Token用量</th>
              <th class="text-center px-4 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">本地请求</th>
              <th class="text-center px-4 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">刷新时间</th>
              <th class="text-center px-4 py-3 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="acc in accounts" :key="acc.id"
                class="border-b border-[var(--border)]/50 hover:bg-[var(--primary)]/3 transition-colors">
              <!-- Email -->
              <td class="px-4 py-3">
                <div class="text-xs font-mono truncate max-w-[180px]">{{ acc.email }}</div>
                <div class="text-[10px] text-[var(--text-secondary)] font-mono">{{ acc.apiKey }}</div>
              </td>
              <!-- Status -->
              <td class="px-4 py-3 text-center">
                <span class="px-2 py-0.5 rounded-full text-[10px] font-bold"
                      :class="acc.enabled ? 'bg-green-500/10 text-green-500' : 'bg-red-500/10 text-red-500'">
                  {{ acc.enabled ? '启用' : '禁用' }}
                </span>
              </td>
              <!-- Plan -->
              <td class="px-4 py-3 text-center">
                <span v-if="acc.upstreamPlan" class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-blue-500/10 text-blue-500">
                  {{ acc.upstreamPlan }}
                </span>
                <span v-else class="text-[var(--text-secondary)] text-xs">—</span>
              </td>
              <!-- Request Usage: used/limit with progress bar -->
              <td class="px-3 py-3">
                <div v-if="acc.requestLimit" class="space-y-1">
                  <div class="text-xs font-bold tabular-nums text-center">
                    {{ acc.upstreamRequests || 0 }} / {{ acc.requestLimit }}
                  </div>
                  <div class="w-full h-1.5 rounded-full bg-[var(--border)] overflow-hidden">
                    <div class="h-full rounded-full transition-all duration-300"
                         :class="progressColor(usagePercent(acc.upstreamRequests || 0, acc.requestLimit))"
                         :style="{ width: (usagePercent(acc.upstreamRequests || 0, acc.requestLimit) || 0) + '%' }"></div>
                  </div>
                  <div class="text-[10px] text-center tabular-nums" :class="(usagePercent(acc.upstreamRequests||0, acc.requestLimit)||0) >= 90 ? 'text-red-500 font-bold' : 'text-[var(--text-secondary)]'">
                    {{ (usagePercent(acc.upstreamRequests || 0, acc.requestLimit) || 0).toFixed(0) }}%
                  </div>
                </div>
                <div v-else class="text-xs text-[var(--text-secondary)] text-center">—</div>
              </td>
              <!-- Token Usage: used/limit with progress bar -->
              <td class="px-3 py-3">
                <div v-if="acc.tokenLimit" class="space-y-1">
                  <div class="text-xs font-bold tabular-nums text-center">
                    {{ formatTokens(acc.upstreamTokens || 0) }} / {{ acc.tokenLimit }}
                  </div>
                  <div class="w-full h-1.5 rounded-full bg-[var(--border)] overflow-hidden">
                    <div class="h-full rounded-full transition-all duration-300"
                         :class="progressColor(usagePercent(acc.upstreamTokens || 0, acc.tokenLimit))"
                         :style="{ width: (usagePercent(acc.upstreamTokens || 0, acc.tokenLimit) || 0) + '%' }"></div>
                  </div>
                  <div class="text-[10px] text-center tabular-nums" :class="(usagePercent(acc.upstreamTokens||0, acc.tokenLimit)||0) >= 90 ? 'text-red-500 font-bold' : 'text-[var(--text-secondary)]'">
                    {{ (usagePercent(acc.upstreamTokens || 0, acc.tokenLimit) || 0).toFixed(1) }}%
                  </div>
                </div>
                <div v-else class="text-xs text-[var(--text-secondary)] text-center">—</div>
              </td>
              <!-- Local proxy request count -->
              <td class="px-4 py-3 text-center text-xs font-bold tabular-nums">{{ acc.requestCount || 0 }}</td>
              <!-- Last refresh time -->
              <td class="px-4 py-3 text-center text-xs text-[var(--text-secondary)]">{{ formatTime(acc.lastRefresh) }}</td>
              <!-- Actions -->
              <td class="px-4 py-3 text-center">
                <div class="flex items-center justify-center gap-1">
                  <button @click="toggleAccount(acc.id)"
                    class="p-1.5 rounded-lg hover:bg-[var(--primary)]/10 transition-all"
                    :title="acc.enabled ? '禁用' : '启用'">
                    <Power v-if="acc.enabled" class="w-3.5 h-3.5 text-green-500" />
                    <PowerOff v-else class="w-3.5 h-3.5 text-red-500" />
                  </button>
                  <button @click="deleteAccount(acc.id)"
                    class="p-1.5 rounded-lg hover:bg-red-500/10 transition-all" title="删除">
                    <Trash2 class="w-3.5 h-3.5 text-red-500" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Footer -->
    <div class="text-xs text-[var(--text-secondary)] opacity-50 text-center">
      EcomAgent 号池与 Kiro 号池完全隔离 · 点击「刷新额度」获取上游实时使用量
    </div>
  </div>

  <!-- Import Dialog -->
  <Teleport to="body">
    <div v-if="showImportDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
         @click.self="!isImporting && (showImportDialog = false)">
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl shadow-2xl w-full max-w-lg p-6 space-y-5">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-black text-[var(--text)]">导入 EcomAgent 账号</h2>
          <button @click="showImportDialog = false" class="p-2 rounded-xl hover:bg-[var(--bg)] transition-all">
            <X class="w-4 h-4" />
          </button>
        </div>

        <p class="text-[11px] text-[var(--text-secondary)] leading-relaxed">
          粘贴注册工具导出的 JSON 数组。格式：<code class="bg-[var(--bg)] px-1 rounded text-[10px]">[{"email","api_key","account_id","access_token","status":"success",...}]</code>
          <br>只会导入 status=success 且有 api_key 的账号，重复 account_id 自动跳过。
          <br><strong>注意</strong>：需要 access_token 字段才能查询上游额度。
        </p>

        <textarea v-model="jsonText" rows="12"
          placeholder='[&#10;  {"email":"...","api_key":"sk-...","account_id":"...","access_token":"eyJ...","status":"success"}&#10;]'
          class="w-full px-3 py-2.5 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-xs font-mono outline-none focus:ring-2 focus:ring-primary/20 focus:border-[var(--primary)] transition-all resize-none leading-relaxed" />

        <div class="flex gap-3">
          <button @click="showImportDialog = false" class="flex-1 h-10 rounded-xl border border-[var(--border)] text-sm font-bold hover:bg-[var(--bg)] transition-all">取消</button>
          <button @click="importAccounts" :disabled="isImporting"
            class="flex-1 h-10 rounded-xl bg-[var(--primary)] text-white text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50">
            {{ isImporting ? '导入中...' : '导入' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
