<script setup>
import { ref } from 'vue'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../composables/useToast'
import { api } from '../api/admin'
import { formatNum, formatTokenExpiry, maskEmail, getSubBadge } from '../utils/format'
import { 
  RefreshCw, 
  Copy, 
  Trash2, 
  Power, 
  ShieldCheck, 
  ShieldAlert, 
  Activity, 
  Clock, 
  Zap,
  Weight,
  MoreVertical,
  Check
} from 'lucide-vue-next'
import { copyToClipboard } from '../utils/clipboard'

const props = defineProps({ account: Object })
const store = useAccountsStore()
const { success, error } = useToast()
const refreshing = ref(false)
const copied = ref(false)
const privacyMode = ref(localStorage.getItem('privacyMode') !== 'false')

const isSelected = () => store.selectedIds.has(props.account.id)
const sub = () => getSubBadge(props.account.subscriptionType)
const isBanned = () => props.account.banStatus && props.account.banStatus !== 'ACTIVE'

async function refresh() {
  refreshing.value = true
  try {
    const res = await api(`/accounts/${props.account.id}/refresh`, { method: 'POST' })
    const d = await res.json()
    if (d.success) { success('刷新成功'); await store.load() }
    else error('刷新失败: ' + (d.error || ''))
  } catch { error('网络错误') }
  refreshing.value = false
}

async function toggle() {
  try {
    await api(`/accounts/${props.account.id}`, {
      method: 'PUT', body: JSON.stringify({ enabled: !props.account.enabled })
    })
    await store.load()
    success(props.account.enabled ? '已禁用' : '已启用')
  } catch { error('操作失败') }
}

async function del() {
  if (!confirm('确定彻底删除该账号？此操作不可恢复。')) return
  try {
    await api(`/accounts/${props.account.id}`, { method: 'DELETE' })
    await store.load()
    success('已删除')
  } catch { error('删除失败') }
}

async function copyJSON() {
  try {
    const res = await api(`/accounts/${props.account.id}/full`)
    const data = await res.json()
    const { clientId, clientSecret, accessToken, refreshToken } = data
    const text = JSON.stringify({ clientId, clientSecret, accessToken, refreshToken }, null, 2)
    await copyToClipboard(text)
    copied.value = true
    success('配置已复制到剪贴板')
    setTimeout(() => copied.value = false, 1500)
  } catch { error('获取配置失败') }
}

function displayEmail() {
  const raw = props.account.email || (props.account.id?.substring(0, 12) + '...')
  return maskEmail(raw, privacyMode.value)
}

function formatAuth(method) {
  if (!method) return '默认'
  const map = { 'idc': '企业账号', 'social': '社交账号', 'email': '邮箱直连' }
  return map[method] || method
}

const getUsageColor = (percent) => {
  if (percent > 0.9) return 'bg-rose-500'
  if (percent > 0.7) return 'bg-amber-500'
  return 'bg-[var(--primary)]'
}
</script>

<template>
  <div class="modern-card p-5 group relative flex flex-col h-full overflow-hidden"
    :class="{ 
      'ring-2 ring-primary border-transparent': isSelected(),
      'opacity-75': !account.enabled || isBanned()
    }">
    
    <!-- Status Indicator Top-Right -->
    <div class="absolute top-0 right-0 p-3">
      <div v-if="isBanned()" class="flex items-center gap-1.5 px-2 py-1 rounded-full bg-rose-100 dark:bg-rose-900/30 text-rose-600 dark:text-rose-400 text-[10px] font-bold uppercase tracking-wider">
        <ShieldAlert class="w-3 h-3" />
        {{ account.banStatus === 'BANNED' ? '已封禁' : '已暂停' }}
      </div>
      <div v-else-if="!account.enabled" class="flex items-center gap-1.5 px-2 py-1 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-500 text-[10px] font-bold uppercase tracking-wider">
        <Power class="w-3 h-3" />
        已禁用
      </div>
      <div v-else class="flex items-center gap-1.5 px-2 py-1 rounded-full bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400 text-[10px] font-bold uppercase tracking-wider">
        <span class="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse"></span>
        运行中
      </div>
    </div>

    <!-- Header Section -->
    <div class="flex items-start gap-4 mb-5">
      <div class="relative shrink-0 pt-1">
        <input type="checkbox" :checked="isSelected()" @change="store.toggleSelect(account.id)"
          class="w-5 h-5 cursor-pointer rounded border-[var(--border)] text-[var(--primary)] focus:ring-primary transition-all" />
      </div>
      
      <div class="flex-1 min-w-0 pr-16">
        <h3 class="font-bold text-sm text-[var(--text)] truncate mb-1 group-hover:text-[var(--primary)] transition-colors">
          {{ displayEmail() }}
        </h3>
        <div class="flex flex-wrap gap-1.5">
          <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-tight text-white"
            :class="{
              'bg-amber-500': sub().color === 'amber',
              'bg-indigo-500': sub().color === 'violet',
              'bg-blue-500': sub().color === 'blue',
              'bg-slate-500': sub().color === 'gray',
            }">{{ sub().label }}</span>
          <span v-if="account.trialStatus === 'ACTIVE'" class="px-2 py-0.5 rounded bg-emerald-500 text-white text-[10px] font-bold uppercase">试用</span>
          <span class="px-2 py-0.5 rounded bg-[var(--bg)] border border-[var(--border)] text-[var(--text-secondary)] text-[10px] font-medium">
            {{ formatAuth(account.provider || account.authMethod) }}
          </span>
        </div>
      </div>
    </div>

    <!-- Usage Section -->
    <div class="flex-1 mb-5">
      <!-- 如果有试用 + 主配额，先显示总量 -->
      <div v-if="(account.trialUsageLimit > 0) || (account.usageLimit > 0)">
        <!-- 总配额概览 -->
        <div v-if="account.trialUsageLimit > 0 && account.usageLimit > 0" class="mb-3 p-2 bg-[var(--bg)] rounded-lg border border-[var(--border)]">
          <div class="flex justify-between text-[10px] font-bold">
            <span class="text-[var(--text)]">总配额</span>
            <span class="text-[var(--primary)]">{{ ((account.trialUsageCurrent || 0) + (account.usageCurrent || 0)).toFixed(0) }} / {{ ((account.trialUsageLimit || 0) + (account.usageLimit || 0)).toFixed(0) }}</span>
          </div>
        </div>
        <!-- 试用配额 -->
        <div v-if="account.trialUsageLimit > 0 && account.trialStatus === 'ACTIVE'" class="mb-2">
          <div class="flex justify-between items-end mb-1">
            <span class="text-[10px] font-semibold text-[var(--text-secondary)] flex items-center gap-1">
              <Activity class="w-3 h-3" /> 试用配额
            </span>
            <span class="text-[10px] font-bold" :class="(account.trialUsagePercent||0) > 0.8 ? 'text-rose-500' : 'text-emerald-500'">
              {{ (account.trialUsageCurrent || 0).toFixed(0) }} / {{ (account.trialUsageLimit || 0).toFixed(0) }}
            </span>
          </div>
          <div class="h-1.5 bg-[var(--bg)] rounded-full overflow-hidden border border-[var(--border)] p-[1px]">
            <div class="h-full rounded-full transition-all duration-500"
              :style="{ width: (account.trialUsagePercent || 0) * 100 + '%' }"
              :class="getUsageColor(account.trialUsagePercent||0)" />
          </div>
        </div>
        <!-- 主配额 -->
        <div v-if="account.usageLimit > 0" class="mb-2">
          <div class="flex justify-between items-end mb-1">
            <span class="text-[10px] font-semibold text-[var(--text-secondary)] flex items-center gap-1">
              <Activity class="w-3 h-3" /> 主配额
            </span>
            <span class="text-[10px] font-bold" :class="(account.usagePercent||0) > 0.8 ? 'text-rose-500' : 'text-[var(--primary)]'">
              {{ (account.usageCurrent || 0).toFixed(0) }} / {{ (account.usageLimit || 0).toFixed(0) }}
            </span>
          </div>
          <div class="h-1.5 bg-[var(--bg)] rounded-full overflow-hidden border border-[var(--border)] p-[1px]">
            <div class="h-full rounded-full transition-all duration-500"
              :style="{ width: (account.usagePercent || 0) * 100 + '%' }"
              :class="getUsageColor(account.usagePercent||0)" />
          </div>
        </div>
      </div>
      <div v-else class="h-full flex flex-col justify-center items-center py-4 border-2 border-dashed border-[var(--border)] rounded-xl opacity-40">
        <Clock class="w-5 h-5 mb-1" />
        <span class="text-xs">无额度信息</span>
      </div>
    </div>

    <!-- Stats Grid -->
    <div class="grid grid-cols-3 gap-3 mb-5 p-3 bg-[var(--bg)] rounded-xl border border-[var(--border)]">
      <div class="space-y-0.5">
        <div class="text-[10px] uppercase font-bold text-[var(--text-secondary)] tracking-wider">总请求</div>
        <div class="text-sm font-bold flex items-center gap-1.5">
          <Zap class="w-3.5 h-3.5 text-amber-500" /> {{ account.requestCount || 0 }}
        </div>
      </div>
      <div class="space-y-0.5">
        <div class="text-[10px] uppercase font-bold text-[var(--text-secondary)] tracking-wider">Token 消耗</div>
        <div class="text-sm font-bold">{{ formatNum(account.totalTokens || 0) }}</div>
      </div>
      <div class="space-y-0.5">
        <div class="text-[10px] uppercase font-bold text-[var(--text-secondary)] tracking-wider">并发</div>
        <div class="text-sm font-bold flex items-center gap-1.5">
          <span v-if="account.inFlight > 0" class="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse"></span>
          <span :class="account.inFlight > 5 ? 'text-rose-500' : account.inFlight > 0 ? 'text-amber-500' : ''">
            {{ account.inFlight || 0 }}/10
          </span>
        </div>
      </div>
      <div class="space-y-0.5">
        <div class="text-[10px] uppercase font-bold text-[var(--text-secondary)] tracking-wider">到期时间</div>
        <div class="text-sm font-bold flex items-center gap-1.5">
          <Clock class="w-3.5 h-3.5 text-blue-500" /> {{ formatTokenExpiry(account.expiresAt) }}
        </div>
      </div>
      <div class="space-y-0.5">
        <div class="text-[10px] uppercase font-bold text-[var(--text-secondary)] tracking-wider">权重</div>
        <div class="flex items-center gap-2">
          <select :value="account.weight || 0"
            @change="async e => { try { await api(`/accounts/${account.id}`, { method: 'PUT', body: JSON.stringify({ weight: +e.target.value }) }); await store.load(); success('权重已更新') } catch { error('更新失败') } }"
            class="text-xs font-bold py-0.5 px-1 border-none bg-transparent hover:bg-white dark:hover:bg-slate-800 rounded transition-colors cursor-pointer outline-none text-[var(--primary)]">
            <option v-for="w in [0,1,2,3,4,5]" :key="w" :value="w">权重 {{ w }}</option>
          </select>
        </div>
      </div>
      <div class="space-y-0.5">
        <div class="text-[10px] uppercase font-bold text-[var(--text-secondary)] tracking-wider">错误数</div>
        <div class="text-sm font-bold" :class="account.errorCount > 0 ? 'text-rose-500' : ''">{{ account.errorCount || 0 }}</div>
      </div>
    </div>

    <!-- Action Toolbar -->
    <div class="flex items-center justify-between gap-2 pt-1">
      <div class="flex gap-1">
        <button @click="refresh" :disabled="refreshing" 
          class="p-2 rounded-lg bg-[var(--bg)] border border-[var(--border)] hover:border-[var(--primary)] hover:text-[var(--primary)] transition-all" title="刷新状态">
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': refreshing }" />
        </button>
        <button @click="copyJSON" 
          class="p-2 rounded-lg bg-[var(--bg)] border border-[var(--border)] hover:border-[var(--primary)] hover:text-[var(--primary)] transition-all" title="复制配置">
          <Check v-if="copied" class="w-4 h-4 text-emerald-500" />
          <Copy v-else class="w-4 h-4" />
        </button>
      </div>

      <div class="flex gap-2">
        <button v-if="!isBanned()" @click="toggle"
          class="flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-bold transition-all"
          :class="account.enabled 
            ? 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-200' 
            : 'bg-[var(--primary)] text-white hover:bg-[var(--primary)]-hover shadow-lg shadow-[var(--primary)]/20'">
          <Power class="w-3.5 h-3.5" />
          {{ account.enabled ? '禁用' : '启用' }}
        </button>
        <button @click="del" class="p-2 rounded-lg bg-rose-50 dark:bg-rose-900/20 text-rose-600 hover:bg-rose-500 hover:text-white transition-all">
          <Trash2 class="w-4 h-4" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 隐藏原生 Select 箭头在某些浏览器 */
select {
  appearance: none;
  background-image: none;
}
</style>
