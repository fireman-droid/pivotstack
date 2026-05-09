<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'
import { userApi } from '../../api/user'
import PlanStatusBadge from '../../components/user/PlanStatusBadge.vue'
import {
  AlertTriangle, Gift, Wallet, Clock, Activity,
  Copy, Check, LayoutGrid, ServerCog
} from 'lucide-vue-next'
import { copyToClipboard } from '../../utils/clipboard'
import WorldCard from '../../components/world/WorldCard.vue'
import WorldStat from '../../components/world/WorldStat.vue'
import WorldTable from '../../components/world/WorldTable.vue'
import WorldButton from '../../components/world/WorldButton.vue'
import WorldChip from '../../components/world/WorldChip.vue'
import WorldLoader from '../../components/world/WorldLoader.vue'

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
const isChildKey = computed(() => !!info.value.isChildKey)
const isTimedPlan  = computed(() => info.value.plan === 'timed' || info.value.plan === 'hybrid')
const isCreditPlan = computed(() => info.value.plan === 'credit' || info.value.plan === 'hybrid' || (!info.value.plan && totalBalanceValue.value > 0))
const balanceValue     = computed(() => Number(info.value.balance || 0))
const giftBalanceValue = computed(() => Number(info.value.giftBalance || 0))
const totalBalanceValue= computed(() => balanceValue.value + giftBalanceValue.value)

const timeRemaining = computed(() => {
  if (!isTimedPlan.value) return ''
  if (!info.value.expiresAt || info.value.expiresAt === 0) return '∞'
  const diff = Math.max(0, info.value.expiresAt - Date.now() / 1000)
  if (diff <= 0) return '已过期'
  const days = Math.floor(diff / 86400)
  const hours = Math.floor((diff % 86400) / 3600)
  const mins = Math.max(1, Math.ceil((diff % 3600) / 60))
  let text = ''
  if (days > 0) text += `${days}天`
  if (hours > 0) text += `${hours}小时`
  if (days === 0 && mins > 0) text += `${mins}分钟`
  return text || '1分钟'
})

const expiryDate = computed(() => {
  if (!info.value.expiresAt || info.value.expiresAt === 0) return '永久有效'
  const diff = Math.max(0, info.value.expiresAt - Date.now() / 1000)
  if (diff < 86400) {
    return '到期：' + new Date(info.value.expiresAt * 1000).toLocaleString('zh-CN', { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' })
  }
  return '到期：' + new Date(info.value.expiresAt * 1000).toLocaleDateString('zh-CN')
})

const timeVariant = computed(() => {
  if (!isTimedPlan.value || !info.value.expiresAt) return 'primary'
  const diff = Math.max(0, info.value.expiresAt - Date.now() / 1000)
  if (diff <= 0) return 'danger'
  if (diff < 3 * 86400) return 'danger'
  if (diff < 7 * 86400) return 'warning'
  return 'success'
})

const balanceVariant = computed(() => totalBalanceValue.value < 1 ? 'danger' : 'success')

const statusVariant = computed(() => {
  const s = info.value.status
  if (s === 'active') return 'success'
  if (s === 'key_expired') return 'danger'
  if (s === 'insufficient_balance') return 'warning'
  return 'primary'
})

const statusText = computed(() => {
  const s = info.value.status
  if (s === 'active') return '正常运行'
  if (s === 'key_expired') return '密钥已过期'
  if (s === 'insufficient_balance') return '余额不足'
  return info.value.statusMessage || s || '未知'
})

const topModels = computed(() => {
  if (!usage.value?.models) return []
  return Object.entries(usage.value.models)
    .sort((a, b) => b[1].requests - a[1].requests)
    .slice(0, 5)
    .map(([name, stats]) => ({
      model: name,
      requests: stats.requests,
      inputK: (stats.inputTokens / 1000).toFixed(1),
      outputK: (stats.outputTokens / 1000).toFixed(1),
    }))
})

const baseUrl = computed(() => `${location.protocol}//${location.host}`)

// 动态生成"计费标准"表格行：按模型逐行显示（v2 per-model 定价）。
// 后端加新模型（4.8/haiku 等）只需改 SupportedModels() + ModelPrices，前端自动同步。
function lookupModelPrice(modelPrices, model) {
  if (!modelPrices) return null
  const key = String(model).toLowerCase()
  if (modelPrices[key] != null) return modelPrices[key]
  // dash/dot 等价：claude-opus-4.6 ↔ claude-opus-4-6
  const dotForm = key.replace(/-(\d)/g, '.$1')
  if (modelPrices[dotForm] != null) return modelPrices[dotForm]
  const dashForm = key.replace(/\.(\d)/g, '-$1')
  if (modelPrices[dashForm] != null) return modelPrices[dashForm]
  return null
}

const pricingRows = computed(() => {
  const p = pricing.value || {}
  const supported = p.supportedModels || { pro: [], free: [] }
  const modelPrices = p.modelPrices || {}
  const defaults = {
    pro:  p.defaultProPriceUSD  || 0.20,
    free: p.defaultFreePriceUSD || 0.04,
  }
  const rows = []
  for (const pool of ['pro', 'free']) {
    for (const m of supported[pool] || []) {
      const explicit = lookupModelPrice(modelPrices, m)
      const price = explicit != null ? explicit : defaults[pool]
      rows.push({
        model: String(m).replace(/^claude-/, ''),
        pool: pool === 'pro' ? 'PRO' : 'FREE',
        price: '$' + Number(price).toFixed(2),
      })
    }
  }
  return rows
})

function copyUrl() {
  copyToClipboard(baseUrl.value)
  copied.value = true
  setTimeout(() => copied.value = false, 2000)
}
function goRecharge() { router.push('/user/recharge') }
</script>

<template>
  <div class="dashboard" v-if="!loading">
    <!-- 标题 -->
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">用户中心</div>
        <h1 class="page-title">账户概览</h1>
      </div>
      <PlanStatusBadge
        :plan="info.plan"
        :tier="info.tier"
        :balance="info.balance"
        :expires-at="info.expiresAt"
      />
    </header>

    <!-- 未激活提示 -->
    <WorldCard v-if="!info.plan && giftBalanceValue <= 0" padding="md" variant="talisman" class="activate-banner">
      <div class="banner-row">
        <div class="banner-icon"><AlertTriangle :size="22" /></div>
        <div class="banner-text" v-if="isChildKey">
          <h3>账户余额不足</h3>
          <p>此账户由您的服务商管理，如需充值请联系您的服务商</p>
        </div>
        <div class="banner-text" v-else>
          <h3>账号尚未激活</h3>
          <p>请兑换激活码来获取余额或时间，开始使用 API 服务</p>
        </div>
        <WorldButton v-if="!isChildKey" variant="primary" size="md" @click="goRecharge">
          <Gift :size="14" /><span>前往充值</span>
        </WorldButton>
      </div>
    </WorldCard>

    <!-- 4 张状态卡片 -->
    <div class="stat-row">
      <WorldStat
        v-if="isCreditPlan"
        label="账户余额"
        :value="`$${totalBalanceValue.toFixed(2)}`"
        :hint="`充值 $${balanceValue.toFixed(2)} · 赠送 $${giftBalanceValue.toFixed(2)}`"
        :sub-hint="totalBalanceValue < 1 ? (isChildKey ? '请联系服务商充值' : '余额不足') : '账户正常'"
        :variant="balanceVariant"
        :icon="Wallet"
      />

      <WorldStat
        v-if="isTimedPlan"
        label="剩余时间"
        :value="timeRemaining"
        :hint="expiryDate"
        :sub-hint="info.rateLimitPerMin > 0 ? `速率上限 ${info.rateLimitPerMin}/分钟` : ''"
        :variant="timeVariant"
        :icon="Clock"
      />

      <WorldStat
        label="累计请求"
        :value="(info.requests || 0).toLocaleString()"
        :hint="`Token ${((info.tokens || 0) / 1000).toFixed(1)}K`"
        variant="info"
        :icon="Activity"
      />

      <WorldStat
        label="服务状态"
        :value="statusText"
        :variant="statusVariant"
        :icon="ServerCog"
      />
    </div>

    <!-- 模型消耗 + API 接入 -->
    <div class="dash-grid">
      <WorldCard padding="md" class="grid-col-7" v-if="topModels.length > 0">
        <h3 class="section-title">
          <LayoutGrid :size="16" />
          <span>模型消耗排行</span>
        </h3>
        <WorldTable
          :columns="[
            { key: 'model',   label: '模型', mono: true },
            { key: 'requests',label: '请求数', align: 'right' },
            { key: 'inputK',  label: '输入(K)', align: 'right' },
            { key: 'outputK', label: '输出(K)', align: 'right' },
          ]"
          :rows="topModels"
          :compact="true"
          empty-text="暂无调用记录"
        />
      </WorldCard>

      <WorldCard padding="md" class="grid-col-5">
        <h3 class="section-title">
          <ServerCog :size="16" />
          <span>API 接入配置</span>
        </h3>
        <div class="api-cfg">
          <div class="cfg-item">
            <label>接口地址 (Base URL)</label>
            <div class="copy-box">
              <code>{{ baseUrl }}</code>
              <button @click="copyUrl" class="copy-btn" :class="{ copied }" aria-label="复制">
                <Check v-if="copied" :size="14" />
                <Copy v-else :size="14" />
              </button>
            </div>
          </div>
          <div class="cfg-item">
            <label>协议兼容</label>
            <div class="chip-row">
              <WorldChip variant="info" size="sm">OpenAI</WorldChip>
              <WorldChip variant="info" size="sm">Claude</WorldChip>
            </div>
          </div>
        </div>
      </WorldCard>
    </div>

    <!-- 计费标准 -->
    <WorldCard v-if="pricing" padding="md">
      <h3 class="section-title">
        <Wallet :size="16" />
        <span>计费标准</span>
      </h3>
      <WorldTable
        :columns="[
          { key: 'model', label: '模型',  mono: true },
          { key: 'pool',  label: '号池',  align: 'center' },
          { key: 'price', label: '单价 ($/credit)', align: 'right' },
        ]"
        :rows="pricingRows"
        :compact="true"
      />
    </WorldCard>
  </div>

  <div v-else class="loading-wrap">
    <WorldLoader :size="48" label="载入数据中" />
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.75rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}
[data-world="daogui"] .page-title {
  background: linear-gradient(135deg, var(--world-text-primary) 0%, var(--world-paper-aged) 90%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.activate-banner :deep(.world-card),
.activate-banner { padding: 16px 18px; }
.banner-row {
  display: flex;
  align-items: center;
  gap: 14px;
}
.banner-icon {
  width: 44px;
  height: 44px;
  border-radius: var(--world-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(245, 158, 11, 0.12);
  color: var(--world-warning);
  flex-shrink: 0;
}
.banner-text { flex: 1; }
.banner-text h3 {
  margin: 0 0 4px;
  font-size: 0.95rem;
  font-weight: 800;
  color: var(--world-text-primary);
}
.banner-text p {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
}

.stat-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 920px) {
  .stat-row { grid-template-columns: repeat(2, 1fr); }
}
@media (max-width: 480px) {
  .stat-row { grid-template-columns: 1fr; }
  .banner-row { flex-direction: column; align-items: flex-start; }
}

.dash-grid {
  display: flex;
  gap: 14px;
}
.grid-col-7 { flex: 7 1 0; min-width: 0; }
.grid-col-5 { flex: 5 1 0; min-width: 0; }
@media (max-width: 920px) {
  .dash-grid { flex-direction: column; }
  .grid-col-7, .grid-col-5 { flex: 1 1 auto; }
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.95rem;
  font-weight: 800;
  margin: 0 0 14px;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
[data-world="daogui"] .section-title { color: var(--world-paper-aged); }

.api-cfg { display: flex; flex-direction: column; gap: 14px; }
.cfg-item label {
  display: block;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--world-text-mute);
  margin-bottom: 6px;
}
.copy-box {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  font-family: var(--world-font-mono);
}
.copy-box code {
  flex: 1;
  font-size: 0.8125rem;
  color: var(--world-text-primary);
  word-break: break-all;
}
.copy-btn {
  width: 28px; height: 28px;
  border-radius: var(--world-radius-sm);
  background: transparent;
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 200ms ease;
  flex-shrink: 0;
}
.copy-btn:hover { color: var(--world-accent); border-color: var(--world-accent); }
.copy-btn.copied { color: var(--world-success); border-color: var(--world-success); }
.chip-row { display: flex; gap: 6px; }

.loading-wrap {
  min-height: 50vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
