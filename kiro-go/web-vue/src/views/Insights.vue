<script setup>
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/admin'
import { useAuthStore } from '../stores/auth'
import { useToast } from '../composables/useToast'
import {
  BarChart3, RefreshCw, TrendingUp, Users, Calendar, Crown,
  ShieldAlert, Eye, DollarSign, Activity, Plus, Moon
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldStat from '../components/world/WorldStat.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldTable from '../components/world/WorldTable.vue'
import WorldSelect from '../components/world/WorldSelect.vue'
import WorldDatePicker from '../components/world/WorldDatePicker.vue'
import WorldCheckbox from '../components/world/WorldCheckbox.vue'
import WorldModal from '../components/world/WorldModal.vue'

const auth = useAuthStore()
const { success } = useToast()
const route = useRoute()
const router = useRouter()
const headers = () => ({ 'X-Admin-Password': auth.password })
const adminHeaders = () => ({ 'X-Admin-Password': auth.password, 'Content-Type': 'application/json' })

// === Tab：3 个新 tab + 旧 hash 兼容 ===
const tabOptions = [
  { value: 'revenue', label: '营收' },
  { value: 'users',   label: '用户' },
  { value: 'abuse',   label: '风控' },
]
const legacyTabMap = {
  overview:    'revenue',
  whales:      'users',
  freeloaders: 'abuse',
  daily:       'revenue',
}
function resolveInitialTab() {
  const q = String(route.query.tab || '').toLowerCase()
  if (legacyTabMap[q]) return legacyTabMap[q]
  if (['revenue', 'users', 'abuse'].includes(q)) return q
  const h = (typeof window !== 'undefined' ? (window.location.hash || '') : '').replace('#', '')
  if (legacyTabMap[h]) return legacyTabMap[h]
  if (['revenue', 'users', 'abuse'].includes(h)) return h
  return 'revenue'
}
const tab = ref(resolveInitialTab())
watch(tab, (v) => {
  router.replace({ query: { ...route.query, tab: v } }).catch(() => {})
})

// === 利润总览（搬自 Dashboard） ===
const profit = ref(null)
const profitPeriod = ref('this_month')
const profitIncludeGift = ref(false)
const profitPeriodOptions = [
  { value: 'this_month', label: '本月' },
  { value: 'last_month', label: '上月' },
  { value: '7d',         label: '近 7 天' },
  { value: '30d',        label: '近 30 天' },
  { value: 'all',        label: '全部' },
]
const showPurchaseModal = ref(false)
const purchasePoolOptions = [
  { value: 'pro',  label: 'PRO' },
  { value: 'free', label: 'FREE' },
]
const purchaseForm = ref({ pool: 'pro', count: 1, costCNY: 0, credits: 1500, note: '' })
const savingPurchase = ref(false)

async function fetchProfit() {
  try {
    const qs = new URLSearchParams({
      period: profitPeriod.value,
      include_gift: String(profitIncludeGift.value),
    })
    const res = await fetch('/admin/api/profit?' + qs.toString(), { headers: adminHeaders() })
    if (res.ok) profit.value = await res.json()
  } catch (e) { console.error('fetchProfit failed:', e) }
}
async function loadProfitIncludeGiftPref() {
  try {
    const res = await fetch('/admin/api/settings', { headers: adminHeaders() })
    if (res.ok) {
      const d = await res.json()
      if (typeof d.profitIncludeGift === 'boolean') profitIncludeGift.value = d.profitIncludeGift
    }
  } catch {}
}
async function toggleIncludeGift(v) {
  profitIncludeGift.value = v
  try {
    await fetch('/admin/api/profit-include-gift', {
      method: 'POST', headers: adminHeaders(), body: JSON.stringify({ value: v }),
    })
  } catch {}
  fetchProfit()
}
async function submitPurchase() {
  if (!purchaseForm.value.costCNY || purchaseForm.value.costCNY <= 0) return success('请填花费 (¥)')
  if (!purchaseForm.value.count || purchaseForm.value.count < 1) return
  savingPurchase.value = true
  try {
    const entry = { count: purchaseForm.value.count, costCNY: purchaseForm.value.costCNY, note: purchaseForm.value.note }
    if (purchaseForm.value.pool === 'pro') entry.credits = purchaseForm.value.credits
    const res = await fetch('/admin/api/cost-entry', {
      method: 'POST', headers: adminHeaders(),
      body: JSON.stringify({ pool: purchaseForm.value.pool, entry }),
    })
    if (res.ok) {
      success('采购已入账')
      showPurchaseModal.value = false
      purchaseForm.value = { pool: 'pro', count: 1, costCNY: 0, credits: 1500, note: '' }
      fetchProfit()
    }
  } catch (e) { console.error(e) }
  savingPurchase.value = false
}

// === 按日明细（吞掉旧每日报表） ===
const dailyDate = ref(new Date().toISOString().slice(0, 10))
const dailyData = ref(null)
const dailyExpanded = ref(false)
async function fetchDaily() {
  try {
    const res = await fetch(`/admin/api/insights/daily?date=${dailyDate.value}`, { headers: headers() })
    if (res.ok) dailyData.value = await res.json()
  } catch (e) { console.error(e) }
}

// === 用户 tab ===
const funnel = ref(null)
const whales = ref([])
const whaleMetric = ref('credits')
const whaleDays = ref(30)
const whaleMetricOptions = [
  { value: 'credits',  label: 'Credits 消耗' },
  { value: 'recharge', label: '充值额 ¥' },
  { value: 'requests', label: '调用次数' },
]
async function fetchFunnel() {
  try {
    const res = await fetch('/admin/api/insights/funnel', { headers: headers() })
    if (res.ok) funnel.value = await res.json()
  } catch (e) { console.error(e) }
}
async function fetchWhales() {
  try {
    const res = await fetch(`/admin/api/insights/whales?metric=${whaleMetric.value}&limit=20&days=${whaleDays.value}`, { headers: headers() })
    if (res.ok) {
      const data = await res.json()
      whales.value = data.rows || []
    }
  } catch (e) { console.error(e) }
}

// === 沉睡用户（搬自 Dashboard） ===
const inactiveDays = ref(30)
const inactiveDayOptions = [
  { value: 30, label: '30 天+' },
  { value: 60, label: '60 天+' },
  { value: 90, label: '90 天+' },
]
const inactiveData = ref({ count: 0, keys: [] })
const inactiveLoading = ref(false)
async function loadInactive() {
  inactiveLoading.value = true
  try {
    const res = await api(`/inactive-keys?days=${inactiveDays.value}`)
    if (res.ok) inactiveData.value = await res.json()
  } catch {}
  inactiveLoading.value = false
}
watch(inactiveDays, loadInactive)

const inactiveColumns = [
  { key: 'note',     label: '备注' },
  { key: 'daysIdle', label: '闲置天数', align: 'right' },
  { key: 'balance',  label: '余额',     align: 'right', mono: true },
  { key: 'giftBalance', label: '赠金',  align: 'right', mono: true },
  { key: 'requests', label: '总请求',   align: 'right', mono: true },
  { key: 'lastUsed', label: '末次使用', align: 'left' },
]
function formatLastUsed(ts) {
  if (!ts) return '从未使用'
  return new Date(ts * 1000).toLocaleDateString('zh-CN')
}

// === 风控（白嫖党） ===
const freeloaders = ref([])
const freeloaderSince = ref('')
const freeloaderMinCalls = ref(5)
async function fetchFreeloaders() {
  try {
    let url = `/admin/api/insights/freeloaders?min_calls=${freeloaderMinCalls.value}`
    if (freeloaderSince.value) {
      const ts = Math.floor(new Date(freeloaderSince.value).getTime() / 1000)
      if (ts > 0) url += `&since=${ts}`
    }
    const res = await fetch(url, { headers: headers() })
    if (res.ok) {
      const data = await res.json()
      freeloaders.value = data.rows || []
      if (data.since && !freeloaderSince.value) {
        freeloaderSince.value = new Date(data.since * 1000).toISOString().slice(0, 16)
      }
    }
  } catch (e) { console.error(e) }
}

// === 公共刷新 ===
let refreshTimer = null
function refreshActiveTab() {
  if (tab.value === 'revenue') {
    fetchProfit(); fetchDaily()
  } else if (tab.value === 'users') {
    fetchFunnel(); fetchWhales(); loadInactive()
  } else if (tab.value === 'abuse') {
    fetchFreeloaders()
  }
}

onMounted(async () => {
  // 处理 ?purchase=1 跳转自动开 modal（来自 Dashboard 的 ghost 入口）
  if (route.query.purchase === '1' || route.query.purchase === 'true') {
    showPurchaseModal.value = true
    const next = { ...route.query }
    delete next.purchase
    router.replace({ query: next }).catch(() => {})
  }
  await loadProfitIncludeGiftPref()
  fetchProfit()
  fetchDaily()
  fetchFunnel()
  fetchWhales()
  loadInactive()
  fetchFreeloaders()
  refreshTimer = setInterval(fetchProfit, 30000)
})
onUnmounted(() => clearInterval(refreshTimer))

// === Computed ===
const funnelSteps = computed(() => {
  const f = funnel.value || {}
  const raw = [
    { label: '注册',         val: f.totalKeys || 0 },
    { label: '已启用',       val: f.enabled || 0 },
    { label: '有余额',       val: f.withBalance || 0 },
    { label: '7 天活跃',     val: f.weekActive || 0 },
    { label: '24h 活跃',     val: f.dayActive || 0 },
    { label: '小时活跃',     val: f.hourActive || 0 },
    { label: '5 分钟在线',   val: f.online5m || 0, live: true },
  ]
  const max = raw[0].val || 1
  return raw.map((s, i, arr) => {
    const widthPct = max > 0 ? Math.max(2, (s.val / max) * 100) : 2
    const totalPct = max > 0 ? Math.round((s.val / max) * 100) : 0
    const dropFromPrev = i === 0 ? null : (arr[i-1].val > 0 ? Math.round(((arr[i-1].val - s.val) / arr[i-1].val) * 100) : 0)
    return { ...s, widthPct, totalPct, dropFromPrev }
  })
})

const whaleRows = computed(() => (whales.value || []).map((r, i) => ({
  rank: i + 1,
  short: (r.keyId || '').slice(0, 8),
  note: r.note || '—',
  calls: (r.calls || 0).toLocaleString(),
  credits: (r.credits || 0).toFixed(2),
  recharge: '¥' + (r.rechargeCNY || 0).toFixed(2),
})))

const freeloaderRows = computed(() => (freeloaders.value || []).map(r => {
  let tag = '核心', variant = 'success'
  if (r.score >= 8)      { tag = '极纯白嫖'; variant = 'danger' }
  else if (r.score >= 6) { tag = '白嫖嫌疑'; variant = 'warning' }
  else if (r.score >= 4) { tag = '突击';     variant = 'warning' }
  else if (r.score >= 2) { tag = '轻度';     variant = 'neutral' }
  const surge = r.normal === 0 ? '∞' : (r.surge || 0).toFixed(1) + 'x'
  return {
    score: r.score, tag, variant,
    short: (r.keyId || '').slice(0, 8),
    note: r.note || '—',
    rch: '¥' + (r.rechargeCNY || 0).toFixed(0),
    normal: r.normal || 0,
    active: r.active || 0,
    surge,
    paid: '¥' + (r.activePaidCNY || 0).toFixed(2),
    saved: '¥' + (r.savedCNY || 0).toFixed(2),
  }
}))

const fLeaderSummary = computed(() => {
  const rows = freeloaders.value || []
  return {
    r1: rows.filter(r => r.score >= 8).length,
    r2: rows.filter(r => r.score >= 6 && r.score < 8).length,
    r3: rows.filter(r => r.score >= 4 && r.score < 6).length,
    total: rows.length,
    saved: rows.reduce((s, r) => s + (r.savedCNY || 0), 0),
    suspectsSaved: rows.filter(r => r.score >= 6).reduce((s, r) => s + (r.savedCNY || 0), 0),
  }
})

const dailyCoreFields = computed(() => {
  const d = dailyData.value || {}
  return [
    { label: '总调用',     val: (d.calls || 0).toLocaleString() },
    { label: '独立 keys',  val: d.uniqueKeys || 0 },
    { label: '收入 ¥',     val: '¥' + (d.costCNY || 0).toFixed(2),     accent: true },
    { label: '当日充值',   val: '¥' + (d.rechargeCNY || 0).toFixed(2), warm: true },
    { label: '错误数',     val: d.errors || 0,                          alert: (d.errors || 0) > 50 },
    { label: '下游 credits', val: (d.credits || 0).toFixed(2) },
  ]
})
const dailyDetailFields = computed(() => {
  const d = dailyData.value || {}
  return [
    { label: '上游 credits',  val: (d.upstreamCredits || 0).toFixed(2) },
    { label: '收入 USD face', val: '$' + (d.costUSD || 0).toFixed(2) },
    { label: 'paid_credits',  val: (d.paidCredits || 0).toFixed(2) },
    { label: 'gifted_credits', val: (d.giftedCredits || 0).toFixed(2) },
    { label: '充值人数',      val: d.rechargersCount || 0 },
    { label: '充值 USD face', val: '$' + (d.rechargeUSD || 0).toFixed(2) },
  ]
})
</script>

<template>
  <div class="insights-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">运营数据</div>
        <h1 class="page-title">数据洞察</h1>
      </div>
      <div class="status-row">
        <WorldChip variant="success" :dot="true" :pulse="true">实时</WorldChip>
        <WorldButton variant="ghost" size="sm" @click="refreshActiveTab">
          <RefreshCw :size="13" />
          <span>刷新</span>
        </WorldButton>
      </div>
    </header>

    <WorldSegment v-model="tab" :options="tabOptions" />

    <!-- ============ 营收 tab ============ -->
    <template v-if="tab === 'revenue'">
      <WorldCard padding="md" v-if="profit">
        <header class="profit-head">
          <div class="profit-title-row">
            <div class="section-title-wrap">
              <DollarSign :size="16" />
              <h3 class="section-title">利润总览</h3>
            </div>
            <WorldSegment v-model="profitPeriod" :options="profitPeriodOptions" size="sm" @update:modelValue="fetchProfit" />
          </div>
          <div class="profit-controls">
            <WorldCheckbox :modelValue="profitIncludeGift" @update:modelValue="toggleIncludeGift" label="计入赠送余额" />
            <WorldButton variant="primary" size="sm" @click="showPurchaseModal = true">
              <Plus :size="13" /><span>采购入账</span>
            </WorldButton>
          </div>
        </header>
        <div class="stats-row">
          <WorldStat label="收入" unit="CNY"
            :value="`¥${(profit.revenue_cny || 0).toFixed(2)}`"
            :hint="`余额卡 ¥${((profit.revenue_breakdown && profit.revenue_breakdown.balance_cards) || 0).toFixed(2)} · 天卡 ¥${((profit.revenue_breakdown && profit.revenue_breakdown.time_cards) || 0).toFixed(2)}${profitIncludeGift ? ` · 赠送 ¥${((profit.revenue_breakdown && profit.revenue_breakdown.gift) || 0).toFixed(2)}` : ''}`"
            :icon="DollarSign" variant="success" />
          <WorldStat label="成本" unit="CNY"
            :value="`¥${(profit.cost_cny || 0).toFixed(2)}`"
            :hint="`PRO ¥${((profit.cost_breakdown && profit.cost_breakdown.pro) || 0).toFixed(2)} · FREE ¥${((profit.cost_breakdown && profit.cost_breakdown.free) || 0).toFixed(2)}`"
            :icon="BarChart3" variant="danger" />
          <WorldStat label="净利润" unit="CNY"
            :value="`¥${(profit.profit_cny || 0).toFixed(2)}`"
            :hint="(profit.profit_cny || 0) >= 0 ? '盈利中' : '亏损中'"
            :icon="TrendingUp"
            :variant="(profit.profit_cny || 0) >= 0 ? 'success' : 'danger'" />
          <WorldStat label="利润率" unit="%"
            :value="(profit.revenue_cny || 0) > 0 ? (profit.margin_percent || 0).toFixed(1) : '—'"
            :hint="(profit.revenue_cny || 0) > 0 ? ((profit.margin_percent || 0) >= 30 ? '健康' : ((profit.margin_percent || 0) >= 0 ? '偏低' : '亏损')) : '本期暂无收入'"
            :icon="Activity"
            :variant="(profit.revenue_cny || 0) > 0 ? ((profit.margin_percent || 0) >= 30 ? 'info' : ((profit.margin_percent || 0) >= 0 ? 'warning' : 'danger')) : 'default'" />
        </div>
      </WorldCard>

      <WorldCard padding="md">
        <header class="section-head">
          <div class="section-title-wrap">
            <Calendar :size="16" />
            <h3 class="section-title">按日明细</h3>
          </div>
          <WorldDatePicker v-model="dailyDate" mode="date" size="sm" :clearable="false" @change="fetchDaily" />
        </header>
        <div class="detail-grid">
          <div v-for="f in dailyCoreFields" :key="f.label" class="dl-item">
            <span class="dl-label">{{ f.label }}</span>
            <span class="dl-val" :class="{ accent: f.accent, warm: f.warm, alert: f.alert }">{{ f.val }}</span>
          </div>
        </div>
        <div v-if="dailyExpanded" class="detail-grid extra-grid">
          <div v-for="f in dailyDetailFields" :key="f.label" class="dl-item">
            <span class="dl-label">{{ f.label }}</span>
            <span class="dl-val">{{ f.val }}</span>
          </div>
        </div>
        <button class="expand-btn" @click="dailyExpanded = !dailyExpanded" type="button">
          {{ dailyExpanded ? '收起完整字段' : `展开完整字段（${dailyDetailFields.length} 项）` }}
        </button>
      </WorldCard>
    </template>

    <!-- ============ 用户 tab ============ -->
    <template v-if="tab === 'users'">
      <WorldCard padding="md">
        <header class="section-head">
          <div class="section-title-wrap">
            <Users :size="16" />
            <h3 class="section-title">活跃度漏斗</h3>
          </div>
          <span class="section-hint">条形宽度反映流量占比，逐级递减</span>
        </header>
        <div class="funnel">
          <div v-for="(s, i) in funnelSteps" :key="i" class="f-row">
            <div class="f-label">
              <span class="f-step">{{ String(i+1).padStart(2,'0') }}</span>
              <span class="f-cn">{{ s.label }}</span>
              <span v-if="s.live && s.val > 0" class="live-dot"></span>
            </div>
            <div class="f-bar-wrap">
              <div class="f-bar" :style="{ width: s.widthPct + '%' }">
                <span class="f-bar-num">{{ s.val }}</span>
              </div>
            </div>
            <div class="f-meta">
              <span class="f-pct">{{ s.totalPct }}%</span>
              <span v-if="s.dropFromPrev !== null" class="f-drop" :class="{ heavy: s.dropFromPrev > 50 }">
                {{ s.dropFromPrev > 0 ? '↓' + s.dropFromPrev + '%' : '—' }}
              </span>
              <span v-else class="f-drop">—</span>
            </div>
          </div>
        </div>
      </WorldCard>

      <WorldCard padding="md">
        <header class="section-head">
          <div class="section-title-wrap">
            <Crown :size="16" />
            <h3 class="section-title">大客户榜（最近 {{ whaleDays }} 天）</h3>
          </div>
          <div class="filter-row">
            <WorldSegment v-model="whaleMetric" :options="whaleMetricOptions" size="sm" @update:modelValue="fetchWhales" />
            <WorldInput v-model.number="whaleDays" type="number" size="sm" label="天" style="width: 80px" @blur="fetchWhales" />
          </div>
        </header>
        <WorldTable
          :columns="[
            { key: 'rank',     label: '#',         align: 'left',  width: '40px' },
            { key: 'short',    label: 'Key',       mono: true },
            { key: 'note',     label: '备注' },
            { key: 'calls',    label: '调用次数',  align: 'right', mono: true },
            { key: 'credits',  label: 'Credits',   align: 'right', mono: true },
            { key: 'recharge', label: '期间充值',  align: 'right', mono: true },
          ]"
          :rows="whaleRows"
          empty-text="暂无数据"
        />
      </WorldCard>

      <WorldCard padding="md">
        <header class="section-head">
          <div class="section-title-wrap">
            <Moon :size="16" />
            <h3 class="section-title">沉睡用户</h3>
            <WorldChip v-if="inactiveData.count > 0" size="sm" variant="warning">{{ inactiveData.count }}</WorldChip>
          </div>
          <div class="filter-row">
            <WorldSegment v-model="inactiveDays" :options="inactiveDayOptions" size="sm" />
            <WorldButton variant="ghost" size="sm" @click="loadInactive">
              <RefreshCw :size="13" :class="{ spin: inactiveLoading }" />
            </WorldButton>
          </div>
        </header>
        <p class="section-hint">长期未使用的 API Key（含从未发起请求的）。可用于追踪潜在僵尸账户。</p>
        <WorldTable
          :columns="inactiveColumns"
          :rows="inactiveData.keys"
          empty-text="暂无沉睡用户"
          max-height="420px"
        >
          <template #cell-note="{ row }">
            <span class="note-cell">
              {{ row.note || '—' }}
              <WorldChip v-if="row.neverUsed" size="sm" variant="neutral">从未使用</WorldChip>
              <WorldChip v-else-if="!row.enabled" size="sm" variant="danger">已禁用</WorldChip>
            </span>
          </template>
          <template #cell-daysIdle="{ row }">
            <span :class="['days-cell', row.daysIdle >= 90 && 'is-grave', row.daysIdle >= 60 && 'is-warn']">
              {{ row.daysIdle }} 天
            </span>
          </template>
          <template #cell-balance="{ row }">
            <span class="mono">${{ Number(row.balance || 0).toFixed(2) }}</span>
          </template>
          <template #cell-giftBalance="{ row }">
            <span class="mono">${{ Number(row.giftBalance || 0).toFixed(2) }}</span>
          </template>
          <template #cell-requests="{ row }">
            <span class="mono">{{ row.requests || 0 }}</span>
          </template>
          <template #cell-lastUsed="{ row }">
            <span class="mono small">{{ formatLastUsed(row.lastUsed) }}</span>
          </template>
        </WorldTable>
      </WorldCard>
    </template>

    <!-- ============ 风控 tab ============ -->
    <template v-if="tab === 'abuse'">
      <WorldCard padding="md">
        <header class="section-head">
          <div class="section-title-wrap">
            <Eye :size="16" />
            <h3 class="section-title">白嫖党扫描</h3>
          </div>
          <span class="section-hint">扫描活动期内调用激增、平时不活跃的 key</span>
        </header>
        <div class="scanner">
          <div class="scan-field">
            <label>活动起点</label>
            <WorldDatePicker v-model="freeloaderSince" mode="datetime" size="sm" placeholder="选起始时刻" />
          </div>
          <div class="scan-field">
            <label>最少活动期调用</label>
            <WorldInput v-model.number="freeloaderMinCalls" type="number" size="sm" />
          </div>
          <WorldButton variant="primary" size="md" @click="fetchFreeloaders">
            <Eye :size="14" /><span>开始扫描</span>
          </WorldButton>
        </div>
      </WorldCard>

      <WorldCard padding="md" v-if="freeloaders.length > 0">
        <header class="section-head">
          <div class="section-title-wrap">
            <ShieldAlert :size="16" />
            <h3 class="section-title">嫌疑分布</h3>
          </div>
        </header>
        <div class="abuse-stats-row">
          <WorldStat label="🔴 极纯白嫖" :value="fLeaderSummary.r1" variant="danger" />
          <WorldStat label="🟡 白嫖嫌疑" :value="fLeaderSummary.r2" variant="warning" />
          <WorldStat label="🟠 突击"   :value="fLeaderSummary.r3" variant="info" />
          <WorldStat label="参与总数"  :value="fLeaderSummary.total" />
          <WorldStat label="嫌疑薅走"  :value="'¥' + fLeaderSummary.suspectsSaved.toFixed(2)" variant="danger" />
          <WorldStat label="总让利"    :value="'¥' + fLeaderSummary.saved.toFixed(2)" variant="warning" />
        </div>
      </WorldCard>

      <WorldCard padding="md">
        <WorldTable
          :columns="[
            { key: 'score',  label: '分',   align: 'left',  width: '50px', mono: true },
            { key: 'tag',    label: '判定', width: '100px' },
            { key: 'short',  label: 'Key',  mono: true, width: '90px' },
            { key: 'note',   label: '备注' },
            { key: 'rch',    label: '累充', align: 'right', mono: true },
            { key: 'normal', label: '平时', align: 'right', mono: true },
            { key: 'active', label: '活动', align: 'right', mono: true },
            { key: 'surge',  label: '倍数', align: 'right', mono: true },
            { key: 'paid',   label: '实付', align: 'right', mono: true },
            { key: 'saved',  label: '省了', align: 'right', mono: true },
          ]"
          :rows="freeloaderRows"
          empty-text="未扫描或暂无嫌疑"
        >
          <template #cell-tag="{ row }">
            <WorldChip :variant="row.variant" size="sm">{{ row.tag }}</WorldChip>
          </template>
        </WorldTable>
      </WorldCard>
    </template>

    <!-- 采购入账 modal（搬自 Dashboard） -->
    <WorldModal v-model="showPurchaseModal" title="采购入账">
      <div class="purchase-form">
        <p class="hint-line">
          这里登记你<strong>买号花了多少钱</strong>。会在选定时间区间内被计入"成本"，参与利润计算。
        </p>
        <div class="dual">
          <div class="lab">
            <label class="lab-text">账号类型</label>
            <WorldSelect v-model="purchaseForm.pool" :options="purchasePoolOptions" size="md" />
          </div>
          <WorldInput v-model.number="purchaseForm.count" type="number" label="数量（个）" />
        </div>
        <div class="dual">
          <WorldInput v-model.number="purchaseForm.costCNY" type="number" step="0.01" label="花费 (¥)" placeholder="比如 60" />
          <WorldInput v-if="purchaseForm.pool === 'pro'" v-model.number="purchaseForm.credits" type="number" label="每号额度 (cr)" />
          <WorldInput v-else :modelValue="550" disabled label="每号额度" />
        </div>
        <WorldInput v-model="purchaseForm.note" label="备注（可选）" placeholder="批次说明" />
      </div>
      <template #footer>
        <WorldButton variant="ghost" @click="showPurchaseModal = false">取消</WorldButton>
        <WorldButton variant="primary" :loading="savingPurchase" @click="submitPurchase">入账</WorldButton>
      </template>
    </WorldModal>
  </div>
</template>

<style scoped>
.insights-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

/* === Header（与 Dashboard 同款）==================== */
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
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}
.status-row { display: inline-flex; align-items: center; gap: 8px; }

.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* === Section head ================================== */
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.section-title-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--world-text-primary);
}
.section-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
.section-hint { font-size: 0.78rem; color: var(--world-text-mute); }
.filter-row { display: inline-flex; align-items: center; gap: 10px; flex-wrap: wrap; }

/* === 利润总览 head ================================== */
.profit-head {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 14px;
}
.profit-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.profit-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

/* === Stats row =================================== */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 920px) { .stats-row { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 480px) { .stats-row { grid-template-columns: 1fr; } }

.abuse-stats-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
}

/* === Funnel ====================================== */
.funnel { display: flex; flex-direction: column; margin: -4px 0; }
.f-row {
  display: grid;
  grid-template-columns: 130px 1fr 100px;
  gap: 14px;
  padding: 10px 0;
  align-items: center;
  border-bottom: 1px dashed var(--world-divider);
}
.f-row:last-child { border-bottom: none; }
.f-label { display: inline-flex; align-items: center; gap: 8px; }
.f-step {
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--world-text-dim);
}
.f-cn { font-size: 0.9rem; font-weight: 700; color: var(--world-text-primary); }
.live-dot {
  width: 8px; height: 8px;
  background: var(--world-accent);
  border-radius: 50%;
  box-shadow: 0 0 8px var(--world-accent);
  animation: live 1.6s ease-in-out infinite;
}
@keyframes live {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.3); opacity: 0.5; }
}
.f-bar-wrap {
  position: relative;
  height: 30px;
  background: var(--world-overlay-light);
  border-radius: 4px;
  overflow: hidden;
}
.f-bar {
  height: 100%;
  background: linear-gradient(90deg, var(--world-accent), var(--world-accent-soft, var(--world-accent)));
  display: flex;
  align-items: center;
  padding-left: 12px;
  transition: width 600ms cubic-bezier(0.16, 1, 0.3, 1);
  min-width: 36px;
  border-radius: 4px;
}
.f-bar-num {
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-size: 0.95rem;
  font-weight: 800;
  color: white;
  font-variant-numeric: tabular-nums;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
}
.f-meta {
  display: flex;
  align-items: baseline;
  justify-content: flex-end;
  gap: 12px;
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-variant-numeric: tabular-nums;
}
.f-pct { font-size: 0.9rem; font-weight: 800; color: var(--world-text-primary); }
.f-drop { font-size: 0.75rem; font-weight: 600; color: var(--world-text-dim); }
.f-drop.heavy { color: var(--world-error); font-weight: 800; }

/* === Scanner ====================================== */
.scanner {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 14px;
  align-items: end;
}
@media (max-width: 768px) { .scanner { grid-template-columns: 1fr; } }
.scan-field { display: flex; flex-direction: column; gap: 6px; }
.scan-field > label {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--world-text-mute);
}

/* === Detail grid =================================== */
.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
}
.extra-grid { margin-top: 10px; }
.dl-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  background: var(--world-overlay-light);
  border-radius: var(--world-radius-sm);
}
.dl-label { font-size: 0.72rem; color: var(--world-text-mute); }
.dl-val {
  font-size: 1rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-variant-numeric: tabular-nums;
}
.dl-val.accent { color: var(--world-accent); }
.dl-val.warm   { color: var(--world-warning); }
.dl-val.alert  { color: var(--world-error); }

.expand-btn {
  margin-top: 12px;
  padding: 6px 10px;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--world-text-mute);
  background: transparent;
  border: 1px dashed var(--world-divider);
  border-radius: var(--world-radius-sm);
  cursor: pointer;
  transition: all 200ms;
}
.expand-btn:hover {
  color: var(--world-accent);
  border-color: var(--world-accent);
}

/* === 沉睡用户表格细节 ============================== */
.note-cell { display: inline-flex; align-items: center; gap: 8px; font-weight: 700; }
.days-cell {
  font-family: var(--world-font-mono);
  font-weight: 800;
  color: var(--world-text-primary);
}
.days-cell.is-warn  { color: var(--world-warning); }
.days-cell.is-grave { color: var(--world-error); }
.mono {
  font-family: var(--world-font-mono);
  font-weight: 700;
  color: var(--world-text-primary);
}
.mono.small { font-size: 0.78rem; color: var(--world-text-mute); }

/* === 采购 modal 表单 =============================== */
.purchase-form { display: flex; flex-direction: column; gap: 12px; padding: 8px 4px; }
.purchase-form .hint-line {
  color: var(--world-text-mute);
  font-size: 0.85rem;
  line-height: 1.5;
  margin: 0;
}
.purchase-form .dual {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  align-items: end;
}
.purchase-form .lab {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.purchase-form .lab-text {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--world-text-mute);
  letter-spacing: 0.04em;
}

/* 响应式 */
@media (max-width: 768px) {
  .f-row { grid-template-columns: 100px 1fr 70px; gap: 10px; }
  .f-cn { font-size: 0.82rem; }
}
</style>
