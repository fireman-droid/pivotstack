<script setup>
import { ref } from 'vue'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../composables/useToast'
import { api } from '../api/admin'
import { formatNum, formatTokenExpiry, maskEmail, getSubBadge } from '../utils/format'
import {
  RefreshCw, Copy, Trash2, Power, ShieldAlert, Activity,
  Clock, Zap, Check
} from 'lucide-vue-next'
import { copyToClipboard } from '../utils/clipboard'
import WorldCard from './world/WorldCard.vue'
import WorldChip from './world/WorldChip.vue'
import WorldButton from './world/WorldButton.vue'
import Switch from './ui/Switch.vue'

const props = defineProps({ account: Object })
const store = useAccountsStore()
const { success, error } = useToast()
const refreshing = ref(false)
const copied = ref(false)
const privacyMode = ref(localStorage.getItem('privacyMode') !== 'false')

const isSelected = () => store.selectedIds.has(props.account.id)
const sub = () => getSubBadge(props.account.subscriptionType)
const isBanned = () => props.account.banStatus && props.account.banStatus !== 'ACTIVE'
const isPro = () => /pro/i.test(props.account.subscriptionType || '')

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

async function toggleOverQuota() {
  const next = !props.account.allowOverQuota
  try {
    await api(`/accounts/${props.account.id}`, {
      method: 'PUT', body: JSON.stringify({ allowOverQuota: next })
    })
    await store.load()
    success(next ? '已开启超开' : '已关闭超开')
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

async function changeWeight(e) {
  try {
    await api(`/accounts/${props.account.id}`, {
      method: 'PUT', body: JSON.stringify({ weight: +e.target.value })
    })
    await store.load()
    success('权重已更新')
  } catch { error('更新失败') }
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

function getUsageVariant(percent) {
  if (percent > 0.9) return 'danger'
  if (percent > 0.7) return 'warning'
  return 'primary'
}
</script>

<template>
  <WorldCard
    padding="md"
    :elevated="false"
    class="account-card"
    :class="{ 'is-selected': isSelected(), 'is-disabled': !account.enabled || isBanned() }"
  >
    <!-- 状态徽章（右上） -->
    <div class="card-status">
      <WorldChip v-if="isBanned()" variant="danger" :dot="true" size="sm">
        <ShieldAlert :size="11" />
        {{ account.banStatus === 'BANNED' ? '已封禁' : '已暂停' }}
      </WorldChip>
      <WorldChip v-else-if="!account.enabled" variant="neutral" :dot="false" size="sm">
        <Power :size="11" />已禁用
      </WorldChip>
      <WorldChip v-else variant="success" :dot="true" size="sm" :pulse="true">运行中</WorldChip>
      <span v-if="account.allowOverQuota" class="overquota-badge" title="已开启超开">+</span>
    </div>

    <!-- 头部 -->
    <div class="card-head">
      <input type="checkbox" :checked="isSelected()" @change="store.toggleSelect(account.id)" class="select-cb" />
      <div class="head-info">
        <h3 class="account-email">{{ displayEmail() }}</h3>
        <div class="badge-row">
          <span class="sub-badge" :class="`sub-${sub().color}`">{{ sub().label }}</span>
          <span v-if="account.trialStatus === 'ACTIVE'" class="sub-badge sub-emerald">试用</span>
          <span class="auth-badge">{{ formatAuth(account.provider || account.authMethod) }}</span>
        </div>
      </div>
    </div>

    <!-- 配额 -->
    <div class="usage-section">
      <div v-if="(account.trialUsageLimit > 0) || (account.usageLimit > 0)">
        <div v-if="account.trialUsageLimit > 0 && account.usageLimit > 0" class="total-row">
          <span class="total-label">总配额</span>
          <span class="total-val">
            {{ ((account.trialUsageCurrent || 0) + (account.usageCurrent || 0)).toFixed(0) }}
            /
            {{ ((account.trialUsageLimit || 0) + (account.usageLimit || 0)).toFixed(0) }}
          </span>
        </div>

        <div v-if="account.trialUsageLimit > 0 && account.trialStatus === 'ACTIVE'" class="quota-row">
          <div class="quota-head">
            <span class="quota-label"><Activity :size="11" /> 试用配额</span>
            <span class="quota-val" :class="`v-${getUsageVariant(account.trialUsagePercent || 0)}`">
              {{ (account.trialUsageCurrent || 0).toFixed(0) }} / {{ (account.trialUsageLimit || 0).toFixed(0) }}
            </span>
          </div>
          <div class="quota-bar">
            <div class="quota-fill" :class="`v-${getUsageVariant(account.trialUsagePercent || 0)}`"
                 :style="{ width: ((account.trialUsagePercent || 0) * 100) + '%' }" />
          </div>
        </div>

        <div v-if="account.usageLimit > 0" class="quota-row">
          <div class="quota-head">
            <span class="quota-label"><Activity :size="11" /> 主配额</span>
            <span class="quota-val" :class="`v-${getUsageVariant(account.usagePercent || 0)}`">
              {{ (account.usageCurrent || 0).toFixed(0) }} / {{ (account.usageLimit || 0).toFixed(0) }}
            </span>
          </div>
          <div class="quota-bar">
            <div class="quota-fill" :class="`v-${getUsageVariant(account.usagePercent || 0)}`"
                 :style="{ width: ((account.usagePercent || 0) * 100) + '%' }" />
          </div>
        </div>
      </div>
      <div v-else class="quota-empty">
        <Clock :size="14" />
        <span>无额度信息</span>
      </div>
    </div>

    <!-- 数据网格 -->
    <div class="stats-grid">
      <div class="stat-cell">
        <div class="stat-label">总请求</div>
        <div class="stat-val"><Zap :size="13" class="ic-warning" />{{ account.requestCount || 0 }}</div>
      </div>
      <div class="stat-cell">
        <div class="stat-label">Token 消耗</div>
        <div class="stat-val">{{ formatNum(account.totalTokens || 0) }}</div>
      </div>
      <div class="stat-cell">
        <div class="stat-label">并发</div>
        <div class="stat-val" :class="{ 'is-busy': account.inFlight > 5, 'is-mid': account.inFlight > 0 && account.inFlight <= 5 }">
          <span v-if="account.inFlight > 0" class="busy-dot" />
          {{ account.inFlight || 0 }}/10
        </div>
      </div>
      <div class="stat-cell">
        <div class="stat-label">到期时间</div>
        <div class="stat-val"><Clock :size="13" class="ic-info" />{{ formatTokenExpiry(account.expiresAt) }}</div>
      </div>
      <div class="stat-cell">
        <div class="stat-label">权重</div>
        <select :value="account.weight || 0" @change="changeWeight" class="weight-sel">
          <option v-for="w in [0,1,2,3,4,5]" :key="w" :value="w">权重 {{ w }}</option>
        </select>
      </div>
      <div class="stat-cell">
        <div class="stat-label">错误数</div>
        <div class="stat-val" :class="{ 'is-error': account.errorCount > 0 }">{{ account.errorCount || 0 }}</div>
      </div>
    </div>

    <!-- 超开开关（仅 PRO 显示） -->
    <div v-if="isPro()" class="overquota-row">
      <div class="oq-label">
        <Zap :size="13" />
        <span>超开</span>
        <span class="oq-hint" title="额度耗尽后仍可调用，按 OVERAGE 计费">?</span>
      </div>
      <Switch :modelValue="account.allowOverQuota" @update:modelValue="toggleOverQuota" />
    </div>

    <!-- 操作栏 -->
    <div class="actions">
      <div class="action-group">
        <button class="ic-btn" @click="refresh" :disabled="refreshing" title="刷新状态">
          <RefreshCw :size="14" :class="{ spin: refreshing }" />
        </button>
        <button class="ic-btn" @click="copyJSON" title="复制配置">
          <Check v-if="copied" :size="14" class="ok" />
          <Copy v-else :size="14" />
        </button>
      </div>
      <div class="action-group">
        <WorldButton v-if="!isBanned()" :variant="account.enabled ? 'secondary' : 'primary'" size="sm" @click="toggle">
          <Power :size="13" />
          <span>{{ account.enabled ? '禁用' : '启用' }}</span>
        </WorldButton>
        <button class="ic-btn danger" @click="del" title="删除"><Trash2 :size="14" /></button>
      </div>
    </div>
  </WorldCard>
</template>

<style scoped>
.account-card {
  position: relative;
  height: 100%;
  display: flex;
  flex-direction: column;
}
.account-card.is-selected {
  outline: 2px solid var(--world-accent);
  outline-offset: -2px;
}
.account-card.is-disabled { opacity: 0.7; }

/* === 头部状态徽章 === */
.card-status {
  position: absolute;
  top: 12px;
  right: 14px;
  display: flex;
  align-items: center;
  gap: 6px;
  z-index: 2;
}
.overquota-badge {
  width: 22px; height: 22px;
  border-radius: 50%;
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: white;
  font-weight: 800;
  font-size: 0.85rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 0 12px rgba(245, 158, 11, 0.5);
}

/* === 头部信息 === */
.card-head {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 16px;
  padding-right: 80px;
}
.select-cb {
  width: 18px; height: 18px;
  margin-top: 2px;
  accent-color: var(--world-accent);
  cursor: pointer;
}
.head-info { flex: 1; min-width: 0; }
.account-email {
  font-size: 0.875rem;
  font-weight: 800;
  color: var(--world-text-primary);
  margin: 0 0 6px;
  truncate: ellipsis;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badge-row { display: flex; flex-wrap: wrap; gap: 5px; }
.sub-badge {
  padding: 2px 7px;
  border-radius: var(--world-radius-sm);
  font-size: 0.65rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: white;
}
.sub-amber  { background: #f59e0b; }
.sub-violet { background: #8b5cf6; }
.sub-blue   { background: #3b82f6; }
.sub-gray   { background: #64748b; }
.sub-emerald{ background: #10b981; }

.auth-badge {
  padding: 2px 7px;
  border-radius: var(--world-radius-sm);
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  font-size: 0.65rem;
  font-weight: 600;
}

/* === 配额 === */
.usage-section { margin-bottom: 14px; }
.total-row {
  display: flex;
  justify-content: space-between;
  padding: 6px 10px;
  margin-bottom: 8px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  font-size: 0.7rem;
  font-weight: 800;
}
.total-label { color: var(--world-text-primary); }
.total-val { color: var(--world-accent); }

.quota-row { margin-bottom: 6px; }
.quota-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.quota-label {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 0.65rem;
  font-weight: 700;
  color: var(--world-text-mute);
}
.quota-val {
  font-size: 0.65rem;
  font-weight: 800;
}
.quota-val.v-primary { color: var(--world-accent); }
.quota-val.v-warning { color: var(--world-warning); }
.quota-val.v-danger  { color: var(--world-error); }

.quota-bar {
  height: 5px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-full);
  padding: 1px;
  overflow: hidden;
}
.quota-fill {
  height: 100%;
  border-radius: inherit;
  transition: width 540ms cubic-bezier(0.4, 0, 0.2, 1);
}
.quota-fill.v-primary { background: var(--world-accent); }
.quota-fill.v-warning { background: var(--world-warning); }
.quota-fill.v-danger  { background: var(--world-error); }

.quota-empty {
  height: 60px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  border: 2px dashed var(--world-glass-border);
  border-radius: var(--world-radius-md);
  color: var(--world-text-dim);
  font-size: 0.7rem;
  opacity: 0.6;
}

/* === Stats grid === */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  padding: 12px;
  margin-bottom: 14px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
}
.stat-cell { display: flex; flex-direction: column; gap: 3px; }
.stat-label {
  font-size: 0.6rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--world-text-mute);
}
.stat-val {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 0.8125rem;
  font-weight: 800;
  color: var(--world-text-primary);
}
.stat-val.is-error { color: var(--world-error); }
.stat-val.is-busy  { color: var(--world-error); }
.stat-val.is-mid   { color: var(--world-warning); }
.busy-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--world-warning);
  animation: chip-pulse 1.4s ease-in-out infinite;
}
@keyframes chip-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(0.85); }
}
.ic-warning { color: var(--world-warning); }
.ic-info { color: var(--world-info); }

.weight-sel {
  appearance: none;
  font-size: 0.7rem;
  font-weight: 700;
  padding: 1px 0;
  background: transparent;
  border: none;
  color: var(--world-accent);
  cursor: pointer;
}

/* === 超开 toggle === */
.overquota-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 10px;
  margin-bottom: 12px;
  background: rgba(245, 158, 11, 0.08);
  border: 1px solid rgba(245, 158, 11, 0.22);
  border-radius: var(--world-radius-md);
}
.oq-label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--world-warning);
}
.oq-hint {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px; height: 14px;
  border-radius: 50%;
  background: rgba(245, 158, 11, 0.18);
  color: var(--world-warning);
  font-size: 0.6rem;
  font-weight: 800;
  cursor: help;
}

/* === 操作栏 === */
.actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-top: auto;
}
.action-group { display: flex; gap: 6px; }
.ic-btn {
  width: 30px; height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--world-radius-sm);
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  cursor: pointer;
  transition: all 200ms ease;
}
.ic-btn:hover { color: var(--world-accent); border-color: var(--world-accent); }
.ic-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.ic-btn.danger:hover { color: white; background: var(--world-error); border-color: var(--world-error); }
.ic-btn .ok { color: var(--world-success); }
.spin { animation: rotate 0.8s linear infinite; }
@keyframes rotate { to { transform: rotate(360deg); } }
</style>
