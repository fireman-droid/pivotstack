<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'
import { userApi } from '../../api/user'
import PlanStatusBadge from '../../components/user/PlanStatusBadge.vue'
import {
  AlertTriangle, Gift, Wallet, Clock, Activity,
  Copy, Check, LayoutGrid
} from 'lucide-vue-next'

const router = useRouter()
const auth = useUserAuth()
const usage = ref(null)
const pricing = ref(null)
const loading = ref(true)
const copied = ref(false)

onMounted(async () => {
  try {
    const [usageData, pricingData] = await Promise.all([
      userApi('/usage'),
      userApi('/pricing'),
    ])
    usage.value = usageData
    pricing.value = pricingData
  } catch {}
  loading.value = false
})

const info = computed(() => auth.userInfo || {})
const isActivated = computed(() => !!info.value.plan)
const isTimedPlan = computed(() => info.value.plan === 'timed' || info.value.plan === 'hybrid')
const isCreditPlan = computed(() => info.value.plan === 'credit' || info.value.plan === 'hybrid')
const balanceValue = computed(() => Number(info.value.balance || 0))

const timeRemaining = computed(() => {
  if (!isTimedPlan.value) return { label: '-', unit: '' }
  if (!info.value.expiresAt || info.value.expiresAt === 0) return { label: '∞', unit: '' }
  const diff = Math.max(0, info.value.expiresAt - Date.now() / 1000)
  if (diff <= 0) return { label: '已过期', unit: '' }
  const days = Math.floor(diff / 86400)
  const hours = Math.floor((diff % 86400) / 3600)
  const mins = Math.max(1, Math.ceil((diff % 3600) / 60))
  let text = ''
  if (days > 0) text += `${days}天`
  if (hours > 0) text += `${hours}小时`
  if (days === 0 && mins > 0) text += `${mins}分钟`
  return { label: text || '1分钟', unit: '' }
})

const expiryDate = computed(() => {
  if (!info.value.expiresAt || info.value.expiresAt === 0) return '永久有效'
  const diff = Math.max(0, info.value.expiresAt - Date.now() / 1000)
  if (diff < 86400) {
    // 不到1天，显示具体时间
    return '到期：' + new Date(info.value.expiresAt * 1000).toLocaleString('zh-CN', { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' })
  }
  return '到期：' + new Date(info.value.expiresAt * 1000).toLocaleDateString('zh-CN')
})

const timeClass = computed(() => {
  if (!isTimedPlan.value || !info.value.expiresAt) return 'ok'
  const diff = Math.max(0, info.value.expiresAt - Date.now() / 1000)
  if (diff <= 0) return 'danger'
  if (diff < 3 * 86400) return 'danger'
  if (diff < 7 * 86400) return 'warning'
  return 'ok'
})

const statusColor = computed(() => {
  const s = info.value.status
  if (s === 'active') return '#22c55e'
  if (s === 'key_expired') return '#ef4444'
  if (s === 'insufficient_balance') return '#f59e0b'
  return '#64748b'
})

const topModels = computed(() => {
  if (!usage.value?.models) return []
  return Object.entries(usage.value.models)
    .sort((a, b) => b[1].requests - a[1].requests)
    .slice(0, 5)
})

const baseUrl = computed(() => `${location.protocol}//${location.host}`)

function copyUrl() {
  navigator.clipboard.writeText(baseUrl.value)
  copied.value = true
  setTimeout(() => copied.value = false, 2000)
}

function goRecharge() {
  router.push('/user/recharge')
}
</script>

<template>
  <div class="dashboard" v-if="!loading">
    <!-- Header -->
    <div class="header-section">
      <h2 class="page-title">账户概览</h2>
    </div>

    <!-- Activation Banner -->
    <div v-if="!info.plan" class="activate-banner glass">
      <div class="banner-content">
        <div class="icon-box">
          <AlertTriangle :size="24" color="#818cf8" />
        </div>
        <div class="text-box">
          <h3>账号尚未激活</h3>
          <p>请兑换激活码来获取余额或时间，开始使用 API 服务</p>
        </div>
      </div>
      <button @click="goRecharge" class="activate-btn">
        <Gift :size="16" style="margin-right:6px" />前往充值
      </button>
    </div>

    <!-- Stat Cards -->
    <div class="stat-cards">
      <!-- Balance -->
      <div v-if="isCreditPlan" class="stat-card glass">
        <div class="stat-header">
          <span class="stat-label">当前余额</span>
          <Wallet :size="18" class="stat-icon" />
        </div>
        <div class="stat-value balance">¥{{ balanceValue.toFixed(2) }}</div>
        <div class="stat-sub" :style="{ color: balanceValue < 1 ? '#ef4444' : '#22c55e' }">
          {{ balanceValue < 1 ? '⚠ 余额不足' : '✓ 账户正常' }}
        </div>
      </div>

      <!-- Time Remaining -->
      <div v-if="isTimedPlan" class="stat-card glass">
        <div class="stat-header">
          <span class="stat-label">剩余时间</span>
          <Clock :size="18" class="stat-icon" />
        </div>
        <div class="stat-value">{{ timeRemaining.label }}<small v-if="timeRemaining.unit">{{ timeRemaining.unit }}</small></div>
        <div class="stat-sub" :class="timeClass">{{ expiryDate }}</div>
      </div>

      <!-- Requests -->
      <div class="stat-card glass">
        <div class="stat-header">
          <span class="stat-label">累计请求</span>
          <Activity :size="18" class="stat-icon" />
        </div>
        <div class="stat-value">{{ (info.requests || 0).toLocaleString() }}</div>
        <div class="stat-sub muted">消耗 Token：{{ ((info.tokens || 0) / 1000).toFixed(1) }}K</div>
      </div>

      <!-- Status -->
      <div class="stat-card glass">
        <div class="stat-header">
          <span class="stat-label">服务状态</span>
          <div class="status-dot-pulse" :style="{ '--color': statusColor }"></div>
        </div>
        <div class="stat-value" :style="{ color: statusColor, fontSize: '1.25rem' }">
          {{ info.status === 'active' ? '正常运行' : info.statusMessage || info.status || '未知' }}
        </div>
        <PlanStatusBadge :plan="info.plan" :tier="info.tier" :balance="info.balance" :expires-at="info.expiresAt" style="margin-top:8px" />
      </div>
    </div>

    <!-- Model Usage + API Access -->
    <div class="grid-layout">
      <div class="section glass usage-section" v-if="topModels.length > 0">
        <h3 class="section-title">
          <LayoutGrid :size="16" style="margin-right:8px" />模型消耗排行
        </h3>
        <div class="table-container">
          <table class="data-table">
            <thead>
              <tr>
                <th>模型</th>
                <th class="text-right">请求</th>
                <th class="text-right">输入(K)</th>
                <th class="text-right">输出(K)</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="[model, stats] in topModels" :key="model">
                <td class="model-name">{{ model }}</td>
                <td class="text-right">{{ stats.requests }}</td>
                <td class="text-right">{{ (stats.inputTokens / 1000).toFixed(1) }}</td>
                <td class="text-right">{{ (stats.outputTokens / 1000).toFixed(1) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="section glass access-section">
        <h3 class="section-title">API 接入配置</h3>
        <div class="api-config">
          <div class="config-item">
            <label>接口地址 (Base URL)</label>
            <div class="copy-box">
              <code>{{ baseUrl }}</code>
              <button @click="copyUrl" class="copy-btn" :class="{ copied }">
                <Check v-if="copied" :size="14" />
                <Copy v-else :size="14" />
              </button>
            </div>
          </div>
          <div class="config-item">
            <label>协议兼容性</label>
            <div class="tag-cloud">
              <span class="tech-tag">OpenAI</span>
              <span class="tech-tag">Claude</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Pricing -->
    <div class="section glass pricing-section" v-if="pricing && pricing.models">
      <h3 class="section-title">实时定价表</h3>
      <div class="table-container">
        <table class="data-table">
          <thead>
            <tr>
              <th>模型名称</th>
              <th class="text-right">输入 (¥/M)</th>
              <th class="text-right">输出 (¥/M)</th>
              <th class="text-right">费率倍率</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(p, model) in pricing.models" :key="model">
              <td class="model-name">{{ model }}</td>
              <td class="text-right">¥{{ p.inputPricePerM }}</td>
              <td class="text-right">¥{{ p.outputPricePerM }}</td>
              <td class="text-right">×{{ p.multiplier }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>

  <div v-else class="loading-state">
    <div class="spinner"></div>
    <span>载入数据中...</span>
  </div>


</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.header-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.page-title {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
  color: #f8fafc;
}



.glass {
  background: rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
}

.activate-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.5rem 2rem;
  border-color: rgba(99, 102, 241, 0.2);
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.05) 0%, rgba(168, 85, 247, 0.05) 100%);
  gap: 1rem;
  flex-wrap: wrap;
}

.banner-content {
  display: flex;
  align-items: center;
  gap: 1.5rem;
}

.icon-box {
  width: 52px;
  height: 52px;
  background: rgba(99, 102, 241, 0.1);
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.text-box h3 {
  margin: 0 0 0.25rem;
  font-size: 1.125rem;
  font-family: 'Space Grotesk', sans-serif;
  color: #f8fafc;
}

.text-box p {
  margin: 0;
  font-size: 0.875rem;
  color: #94a3b8;
}

.activate-btn {
  display: flex;
  align-items: center;
  padding: 0.75rem 1.5rem;
  background: #6366f1;
  color: #fff;
  border: none;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.activate-btn:hover {
  background: #4f46e5;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1.5rem;
}

.stat-card {
  padding: 1.5rem;
}

.stat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.stat-label {
  font-size: 0.875rem;
  font-weight: 600;
  color: #94a3b8;
}

.stat-icon {
  color: #64748b;
}

.stat-value {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 2rem;
  font-weight: 700;
  color: #f8fafc;
  margin-bottom: 0.5rem;
}

.stat-value.balance {
  background: linear-gradient(135deg, #22c55e, #10b981);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.stat-value small {
  font-size: 1rem;
  margin-left: 0.25rem;
  color: #64748b;
  -webkit-text-fill-color: #64748b;
}

.stat-sub {
  font-size: 0.8125rem;
}

.stat-sub.ok { color: #22c55e; }
.stat-sub.warning { color: #f59e0b; }
.stat-sub.danger { color: #ef4444; }
.stat-sub.muted { color: #64748b; }

.status-dot-pulse {
  width: 10px;
  height: 10px;
  background: var(--color);
  border-radius: 50%;
  position: relative;
}

.status-dot-pulse::after {
  content: '';
  position: absolute;
  top: 0; left: 0;
  width: 100%; height: 100%;
  background: var(--color);
  border-radius: 50%;
  animation: ping 1.5s cubic-bezier(0, 0, 0.2, 1) infinite;
}

@keyframes ping {
  75%, 100% { transform: scale(2.5); opacity: 0; }
}

.grid-layout {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 1.5rem;
}

.section {
  padding: 1.5rem;
}

.section-title {
  display: flex;
  align-items: center;
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1rem;
  font-weight: 700;
  color: #f8fafc;
  margin: 0 0 1.5rem;
}

.table-container {
  overflow-x: auto;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
}

.data-table th {
  text-align: left;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #64748b;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.data-table td {
  padding: 0.75rem 1rem;
  font-size: 0.875rem;
  color: #cbd5e1;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.data-table tr:last-child td {
  border-bottom: none;
}

.data-table tr:hover td {
  background: rgba(255, 255, 255, 0.02);
}

.model-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: #a855f7;
  font-weight: 600;
}

.text-right { text-align: right; }

.api-config {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.config-item label {
  display: block;
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.copy-box {
  display: flex;
  align-items: center;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 0.25rem 0.25rem 0.25rem 1rem;
}

.copy-box code {
  flex: 1;
  font-family: ui-monospace, monospace;
  font-size: 0.8125rem;
  color: #22c55e;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.copy-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.04);
  border: none;
  border-radius: 6px;
  color: #94a3b8;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.copy-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
}

.copy-btn.copied {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.1);
}

.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.tech-tag {
  padding: 0.25rem 0.625rem;
  border-radius: 6px;
  background: rgba(99, 102, 241, 0.1);
  color: #818cf8;
  font-size: 0.75rem;
  font-weight: 600;
  border: 1px solid rgba(99, 102, 241, 0.2);
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 400px;
  color: #64748b;
  gap: 1rem;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid rgba(99, 102, 241, 0.1);
  border-top-color: #6366f1;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 1024px) {
  .grid-layout { grid-template-columns: 1fr; }
}

@media (max-width: 640px) {
  .activate-banner { padding: 1.25rem; }
  .banner-content { flex-direction: column; text-align: center; align-items: center; }
  .activate-btn { width: 100%; justify-content: center; }
  .stat-cards { grid-template-columns: 1fr; }
}
</style>
