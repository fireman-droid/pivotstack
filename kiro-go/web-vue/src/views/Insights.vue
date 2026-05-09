<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useToast } from '../composables/useToast'
import {
  BarChart3, RefreshCw, Activity, TrendingUp, Users,
  AlertTriangle, ShieldAlert, Eye, Clock, Calendar, Crown,
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldStat from '../components/world/WorldStat.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldTable from '../components/world/WorldTable.vue'

const auth = useAuthStore()
const { error: toastErr } = useToast()
const headers = () => ({ 'X-Admin-Password': auth.password })

const tab = ref('overview')
const tabOptions = [
  { value: 'overview',    label: '总览' },
  { value: 'whales',      label: '大客户榜' },
  { value: 'freeloaders', label: '白嫖党检测' },
  { value: 'daily',       label: '每日报表' },
]

// State
const funnel = ref(null)
const whales = ref([])
const whaleMetric = ref('credits')
const whaleDays = ref(30)
const whaleMetricOptions = [
  { value: 'credits',  label: 'Credits 消耗' },
  { value: 'recharge', label: '充值额 ¥' },
  { value: 'requests', label: '调用次数' },
]
const freeloaders = ref([])
const freeloaderSince = ref('')
const freeloaderMinCalls = ref(5)
const dailyData = ref(null)
const dailyDate = ref(new Date().toISOString().slice(0, 10))

const loading = ref(false)
let refreshTimer = null

async function fetchFunnel() {
  try {
    const res = await fetch('/admin/api/insights/funnel', { headers: headers() })
    if (res.ok) funnel.value = await res.json()
  } catch (e) { console.error(e) }
}
async function fetchWhales() {
  loading.value = true
  try {
    const res = await fetch(`/admin/api/insights/whales?metric=${whaleMetric.value}&limit=20&days=${whaleDays.value}`, { headers: headers() })
    if (res.ok) {
      const data = await res.json()
      whales.value = data.rows || []
    }
  } catch (e) { console.error(e) }
  loading.value = false
}
async function fetchFreeloaders() {
  loading.value = true
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
  loading.value = false
}
async function fetchDaily() {
  loading.value = true
  try {
    const res = await fetch(`/admin/api/insights/daily?date=${dailyDate.value}`, { headers: headers() })
    if (res.ok) dailyData.value = await res.json()
  } catch (e) { console.error(e) }
  loading.value = false
}

function refreshActiveTab() {
  fetchFunnel()
  if (tab.value === 'whales')      fetchWhales()
  if (tab.value === 'freeloaders') fetchFreeloaders()
  if (tab.value === 'daily')       fetchDaily()
}

onMounted(() => {
  fetchFunnel()
  fetchDaily()
  fetchWhales()
  fetchFreeloaders()
  refreshTimer = setInterval(fetchFunnel, 30000)
})
onUnmounted(() => clearInterval(refreshTimer))

const cnyPerUSD = 0.05

// 漏斗：横向条形，宽度 ∝ 当前值/注册总数
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

const dailyHero = computed(() => {
  const d = dailyData.value || {}
  const costCNY = (d.costUSD || 0) * cnyPerUSD
  const upstreamCNY = (d.upstreamCredits || 0) * 1.4 * cnyPerUSD
  const profit = costCNY - upstreamCNY
  return {
    revenue: costCNY, profit, upstream: upstreamCNY,
    calls: d.calls || 0, errors: d.errors || 0, uniqueKeys: d.uniqueKeys || 0,
    credits: d.credits || 0, recharge: d.rechargeCNY || 0,
  }
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

function fmtNum(n) { return Number(n || 0).toLocaleString() }
function fmtCny(n) { return '¥' + Number(n || 0).toFixed(2) }
</script>

<template>
  <div class="insights-page">
    <!-- ============ Header（与 Dashboard / Pricing 同款）============ -->
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">运营数据</div>
        <h1 class="page-title">数据洞察</h1>
      </div>
      <div class="status-row">
        <WorldChip variant="success" :dot="true" :pulse="true">实时</WorldChip>
        <WorldButton variant="ghost" size="sm" @click="refreshActiveTab" :disabled="loading">
          <RefreshCw :size="13" :class="{ spin: loading }" />
          <span>刷新</span>
        </WorldButton>
      </div>
    </header>

    <!-- Tab -->
    <WorldSegment v-model="tab" :options="tabOptions" />

    <!-- ============ 总览 ============ -->
    <template v-if="tab === 'overview'">
      <!-- 今日核心：左大数 + 右辅数（hero pattern）-->
      <WorldCard padding="lg" class="hero-card">
        <div class="hero-grid">
          <div class="hero-main">
            <div class="hero-label">
              <TrendingUp :size="14" />
              <span>今日营收</span>
            </div>
            <div class="hero-num">{{ fmtCny(dailyHero.revenue) }}</div>
            <div class="hero-foot">
              <span>总调用 <strong>{{ fmtNum(dailyHero.calls) }}</strong></span>
              <span class="dot">·</span>
              <span>独立用户 <strong>{{ dailyHero.uniqueKeys }}</strong></span>
              <span class="dot">·</span>
              <span :class="{ alert: dailyHero.errors > 50 }">错误 <strong>{{ dailyHero.errors }}</strong></span>
            </div>
          </div>
          <div class="hero-side">
            <div class="side-row">
              <span class="side-label">净利润</span>
              <span class="side-val" :class="dailyHero.profit >= 0 ? 'positive' : 'negative'">
                {{ fmtCny(dailyHero.profit) }}
              </span>
            </div>
            <div class="side-row">
              <span class="side-label">上游成本</span>
              <span class="side-val negative">{{ fmtCny(dailyHero.upstream) }}</span>
            </div>
            <div class="side-row">
              <span class="side-label">总 credits</span>
              <span class="side-val">{{ dailyHero.credits.toFixed(2) }}</span>
            </div>
            <div class="side-row">
              <span class="side-label">当日充值</span>
              <span class="side-val warm">{{ fmtCny(dailyHero.recharge) }}</span>
            </div>
          </div>
        </div>
      </WorldCard>

      <!-- 活跃度漏斗：横向条形递减 -->
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
    </template>

    <!-- ============ 大客户榜 ============ -->
    <template v-if="tab === 'whales'">
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
    </template>

    <!-- ============ 白嫖党检测 ============ -->
    <template v-if="tab === 'freeloaders'">
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
            <input type="datetime-local" v-model="freeloaderSince" />
          </div>
          <div class="scan-field">
            <label>最少活动期调用</label>
            <input type="number" v-model.number="freeloaderMinCalls" min="1"/>
          </div>
          <WorldButton variant="primary" size="md" :loading="loading" @click="fetchFreeloaders">
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
        <div class="stats-row">
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

    <!-- ============ 每日报表 ============ -->
    <template v-if="tab === 'daily'">
      <WorldCard padding="md">
        <header class="section-head">
          <div class="section-title-wrap">
            <Calendar :size="16" />
            <h3 class="section-title">每日报表</h3>
          </div>
          <input type="date" v-model="dailyDate" @change="fetchDaily" class="date-picker" />
        </header>
        <div class="hero-grid">
          <div class="hero-main">
            <div class="hero-label">
              <TrendingUp :size="14" />
              <span>当日营收 · {{ dailyDate }}</span>
            </div>
            <div class="hero-num">{{ fmtCny(dailyHero.revenue) }}</div>
            <div class="hero-foot">
              <span>总调用 <strong>{{ fmtNum(dailyHero.calls) }}</strong></span>
              <span class="dot">·</span>
              <span>独立用户 <strong>{{ dailyHero.uniqueKeys }}</strong></span>
            </div>
          </div>
          <div class="hero-side">
            <div class="side-row">
              <span class="side-label">净利润</span>
              <span class="side-val" :class="dailyHero.profit >= 0 ? 'positive' : 'negative'">
                {{ fmtCny(dailyHero.profit) }}
              </span>
            </div>
            <div class="side-row">
              <span class="side-label">上游成本</span>
              <span class="side-val negative">{{ fmtCny(dailyHero.upstream) }}</span>
            </div>
            <div class="side-row">
              <span class="side-label">充值</span>
              <span class="side-val warm">{{ fmtCny(dailyHero.recharge) }}</span>
            </div>
          </div>
        </div>
      </WorldCard>

      <WorldCard padding="md" v-if="dailyData">
        <header class="section-head">
          <div class="section-title-wrap">
            <BarChart3 :size="16" />
            <h3 class="section-title">详细数据</h3>
          </div>
        </header>
        <div class="detail-grid">
          <div class="dl-item"><span class="dl-label">总调用</span><span class="dl-val">{{ dailyData.calls }}</span></div>
          <div class="dl-item"><span class="dl-label">错误数</span><span class="dl-val">{{ dailyData.errors }}</span></div>
          <div class="dl-item"><span class="dl-label">独立 keys</span><span class="dl-val">{{ dailyData.uniqueKeys }}</span></div>
          <div class="dl-item"><span class="dl-label">下游 credits</span><span class="dl-val">{{ (dailyData.credits || 0).toFixed(2) }}</span></div>
          <div class="dl-item"><span class="dl-label">上游 credits</span><span class="dl-val">{{ (dailyData.upstreamCredits || 0).toFixed(2) }}</span></div>
          <div class="dl-item"><span class="dl-label">收入 USD face</span><span class="dl-val">${{ (dailyData.costUSD || 0).toFixed(2) }}</span></div>
          <div class="dl-item"><span class="dl-label">收入 ¥</span><span class="dl-val accent">¥{{ (dailyData.costCNY || 0).toFixed(2) }}</span></div>
          <div class="dl-item"><span class="dl-label">paid_credits</span><span class="dl-val">{{ (dailyData.paidCredits || 0).toFixed(2) }}</span></div>
          <div class="dl-item"><span class="dl-label">gifted_credits</span><span class="dl-val">{{ (dailyData.giftedCredits || 0).toFixed(2) }}</span></div>
          <div class="dl-item"><span class="dl-label">充值人数</span><span class="dl-val">{{ dailyData.rechargersCount }}</span></div>
          <div class="dl-item"><span class="dl-label">充值额 ¥</span><span class="dl-val warm">¥{{ (dailyData.rechargeCNY || 0).toFixed(2) }}</span></div>
          <div class="dl-item"><span class="dl-label">充值 USD face</span><span class="dl-val">${{ (dailyData.rechargeUSD || 0).toFixed(2) }}</span></div>
        </div>
      </WorldCard>
    </template>
  </div>
</template>

<style scoped>
.insights-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

/* === Header（跟 Dashboard 同款）==================== */
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
.section-title-wrap { display: inline-flex; align-items: center; gap: 8px; color: var(--world-text-primary); }
.section-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
.section-hint { font-size: 0.78rem; color: var(--world-text-mute); }
.filter-row { display: inline-flex; align-items: center; gap: 10px; flex-wrap: wrap; }

/* === Hero (今日核心) =============================== */
.hero-card { position: relative; overflow: hidden; }
.hero-card::before {
  content: '';
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: linear-gradient(180deg, var(--world-accent), var(--world-accent-soft, var(--world-accent)));
}
.hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) minmax(0, 1fr);
  gap: 0;
}
@media (max-width: 880px) { .hero-grid { grid-template-columns: 1fr; } }
.hero-main {
  padding-right: 28px;
  border-right: 1px dashed var(--world-divider);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
@media (max-width: 880px) {
  .hero-main { border-right: none; border-bottom: 1px dashed var(--world-divider); padding-right: 0; padding-bottom: 18px; }
}
.hero-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--world-text-mute);
  text-transform: uppercase;
  letter-spacing: 0.12em;
}
.hero-num {
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-size: 3.2rem;
  font-weight: 800;
  line-height: 1;
  letter-spacing: -0.03em;
  color: var(--world-text-primary);
  font-variant-numeric: tabular-nums;
}
.hero-foot {
  display: flex;
  align-items: baseline;
  gap: 10px;
  font-size: 0.85rem;
  color: var(--world-text-mute);
  flex-wrap: wrap;
}
.hero-foot strong {
  font-weight: 800;
  color: var(--world-text-primary);
  margin-left: 2px;
  font-variant-numeric: tabular-nums;
}
.hero-foot .alert strong { color: var(--world-error); }
.hero-foot .dot { color: var(--world-text-dim); }

.hero-side {
  padding-left: 28px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 12px;
}
@media (max-width: 880px) { .hero-side { padding-left: 0; padding-top: 18px; } }
.side-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  padding-bottom: 10px;
  border-bottom: 1px dashed var(--world-divider);
  gap: 12px;
}
.side-row:last-child { border-bottom: none; padding-bottom: 0; }
.side-label {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--world-text-mute);
}
.side-val {
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-size: 1.05rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-variant-numeric: tabular-nums;
}
.side-val.positive { color: var(--world-success); }
.side-val.negative { color: var(--world-error); }
.side-val.warm     { color: var(--world-warning); }

/* === Funnel (横向条形漏斗) ========================= */
.funnel {
  display: flex;
  flex-direction: column;
  margin: -4px 0;
}
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
.f-cn {
  font-size: 0.9rem;
  font-weight: 700;
  color: var(--world-text-primary);
}
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

/* === Stats row（白嫖党嫌疑分布）==================== */
.stats-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
}

/* === Scanner ====================================== */
.scanner {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 14px;
  align-items: end;
}
@media (max-width: 768px) { .scanner { grid-template-columns: 1fr; } }
.scan-field { display: flex; flex-direction: column; gap: 6px; }
.scan-field label {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--world-text-mute);
}
.scan-field input,
.date-picker {
  background: var(--world-overlay-light);
  border: 1px solid var(--world-divider);
  border-radius: var(--world-radius-sm);
  color: var(--world-text-primary);
  padding: 8px 12px;
  font-size: 0.85rem;
  font-family: var(--world-font-mono, ui-monospace, monospace);
}
.scan-field input:focus,
.date-picker:focus { outline: none; border-color: var(--world-accent); }

/* === Detail grid =================================== */
.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
}
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

/* 响应式 */
@media (max-width: 768px) {
  .hero-num { font-size: 2.4rem; }
  .f-row { grid-template-columns: 100px 1fr 70px; gap: 10px; }
  .f-cn { font-size: 0.82rem; }
}
</style>
