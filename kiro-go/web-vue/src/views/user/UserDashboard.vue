<script setup>
import { ref, onMounted, computed } from 'vue'
import { useUserAuth } from '../../stores/userAuth'
import { userApi } from '../../api/user'

const auth = useUserAuth()
const usage = ref(null)
const pricing = ref(null)
const loading = ref(true)

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

const daysRemaining = computed(() => {
  if (!info.value.expiresAt || info.value.expiresAt === 0) return '∞'
  const days = Math.max(0, Math.floor((info.value.expiresAt - Date.now()/1000) / 86400))
  return days
})

const statusColor = computed(() => {
  const s = info.value.status
  if (s === 'active') return '#22c55e'
  if (s === 'key_expired') return '#ef4444'
  if (s === 'insufficient_balance') return '#f59e0b'
  return '#6b7280'
})

const topModels = computed(() => {
  if (!usage.value?.models) return []
  return Object.entries(usage.value.models)
    .sort((a, b) => b[1].requests - a[1].requests)
    .slice(0, 5)
})

const baseUrl = computed(() => `${location.protocol}//${location.host}`)
const copied = ref(false)

function copyUrl() {
  navigator.clipboard.writeText(baseUrl.value)
  copied.value = true
  setTimeout(() => copied.value = false, 2000)
}
</script>

<template>
  <div class="dashboard" v-if="!loading">
    <!-- Stats Cards -->
    <div class="stat-cards">
      <div class="stat-card">
        <div class="stat-label">当前余额</div>
        <div class="stat-value balance">
          <template v-if="info.plan === 'timed'">时间制</template>
          <template v-else>¥{{ (info.balance || 0).toFixed(2) }}</template>
        </div>
        <div class="stat-sub" v-if="info.plan !== 'timed'" :style="{ color: info.balance < 1 ? '#ff6b6b' : '#22c55e' }">
          {{ info.balance < 1 ? '⚠️ 余额不足' : '✅ 正常' }}
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-label">剩余天数</div>
        <div class="stat-value">{{ daysRemaining }}</div>
        <div class="stat-sub" v-if="info.expiresAt > 0">
          到期: {{ new Date(info.expiresAt * 1000).toLocaleDateString() }}
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-label">累计请求</div>
        <div class="stat-value">{{ (info.requests || 0).toLocaleString() }}</div>
        <div class="stat-sub">
          Token: {{ ((info.tokens || 0) / 1000).toFixed(1) }}K
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-label">状态</div>
        <div class="stat-value status-dot" :style="{ color: statusColor }">
          ● {{ info.status === 'active' ? '正常' : info.statusMessage || info.status }}
        </div>
      </div>
    </div>

    <!-- Model Usage -->
    <div class="section" v-if="topModels.length > 0">
      <h3>📈 模型使用排行</h3>
      <div class="model-table">
        <div class="model-row header">
          <span>模型</span>
          <span>请求数</span>
          <span>输入Token</span>
          <span>输出Token</span>
        </div>
        <div class="model-row" v-for="[model, stats] in topModels" :key="model">
          <span class="model-name">{{ model }}</span>
          <span>{{ stats.requests }}</span>
          <span>{{ (stats.inputTokens / 1000).toFixed(1) }}K</span>
          <span>{{ (stats.outputTokens / 1000).toFixed(1) }}K</span>
        </div>
      </div>
    </div>

    <!-- API Access Info -->
    <div class="section">
      <h3>🔗 API 接入信息</h3>
      <div class="api-info">
        <div class="info-row">
          <label>Base URL</label>
          <div class="url-copy">
            <code>{{ baseUrl }}</code>
            <button @click="copyUrl" class="copy-btn">
              {{ copied ? '✅' : '📋' }}
            </button>
          </div>
        </div>
        <div class="info-row">
          <label>兼容格式</label>
          <div class="tags">
            <span class="tag">Claude API</span>
            <span class="tag">OpenAI API</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Pricing -->
    <div class="section" v-if="pricing && pricing.models">
      <h3>💲 当前定价</h3>
      <div class="model-table">
        <div class="model-row header">
          <span>模型</span>
          <span>输入 (¥/M)</span>
          <span>输出 (¥/M)</span>
          <span>倍率</span>
        </div>
        <div class="model-row" v-for="(p, model) in pricing.models" :key="model">
          <span class="model-name">{{ model }}</span>
          <span>¥{{ p.inputPricePerM }}</span>
          <span>¥{{ p.outputPricePerM }}</span>
          <span>×{{ p.multiplier }}</span>
        </div>
      </div>
    </div>
  </div>

  <div v-else class="loading">加载中...</div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.stat-card {
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 14px;
  padding: 1.2rem;
  transition: transform 0.2s;
}

.stat-card:hover { transform: translateY(-2px); }

.stat-label {
  font-size: 0.8rem;
  color: rgba(255,255,255,0.45);
  margin-bottom: 0.5rem;
}

.stat-value {
  font-size: 1.6rem;
  font-weight: 700;
  color: #fff;
}

.stat-value.balance {
  background: linear-gradient(135deg, #22c55e, #10b981);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.stat-sub {
  font-size: 0.75rem;
  color: rgba(255,255,255,0.4);
  margin-top: 0.3rem;
}

.section {
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 14px;
  padding: 1.2rem;
}

.section h3 {
  font-size: 1rem;
  margin: 0 0 1rem 0;
  color: rgba(255,255,255,0.8);
}

.model-table {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.model-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  padding: 0.5rem 0.8rem;
  border-radius: 8px;
  font-size: 0.85rem;
}

.model-row.header {
  color: rgba(255,255,255,0.4);
  font-size: 0.75rem;
}

.model-row:not(.header) {
  background: rgba(255,255,255,0.03);
}

.model-row:not(.header):hover {
  background: rgba(255,255,255,0.06);
}

.model-name {
  font-family: monospace;
  color: #a78bfa;
}

.api-info {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.info-row label {
  min-width: 80px;
  color: rgba(255,255,255,0.5);
  font-size: 0.85rem;
}

.url-copy {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: rgba(0,0,0,0.3);
  padding: 0.4rem 0.8rem;
  border-radius: 8px;
  flex: 1;
}

.url-copy code {
  font-size: 0.85rem;
  color: #22c55e;
  flex: 1;
}

.copy-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1rem;
  padding: 0.2rem;
}

.tags {
  display: flex;
  gap: 0.5rem;
}

.tag {
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  background: rgba(139,92,246,0.15);
  color: #a78bfa;
  font-size: 0.75rem;
}

.loading {
  text-align: center;
  padding: 3rem;
  color: rgba(255,255,255,0.4);
}
</style>
