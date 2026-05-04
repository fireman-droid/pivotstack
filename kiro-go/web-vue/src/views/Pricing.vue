<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useToast } from '../composables/useToast'
import {
  DollarSign, TrendingUp, BarChart3, Activity, Clock,
  Save, AlertTriangle, Plus, Trash2, Settings as SettingsIcon
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldStat from '../components/world/WorldStat.vue'
import WorldTable from '../components/world/WorldTable.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldProgress from '../components/world/WorldProgress.vue'

const auth = useAuthStore()
const { success, error: toastErr } = useToast()
const headers = () => ({ 'X-Admin-Password': auth.password, 'Content-Type': 'application/json' })

const tab = ref('analysis')
const tabOptions = [
  { value: 'analysis', label: '成本分析' },
  { value: 'config',   label: '售价配置' },
]

// Data state
const profit = ref(null)
const analysis = ref(null)
const keys = ref([])
const pricing = ref(null)
const loading = ref(true)
const saving = ref(false)
const poolTab = ref('all')
let timer = null

// Cost entry forms
const showProForm = ref(false)
const showFreeForm = ref(false)
const proForm = ref({ count: 1, costCNY: 60, credits: 1500 })
const freeForm = ref({ count: 100, costCNY: 9 })

async function fetchAll() {
  try {
    const [p1, p2, p3, p4] = await Promise.all([
      fetch('/admin/api/profit',           { headers: headers() }),
      fetch('/admin/api/pricing-analysis', { headers: headers() }),
      fetch('/admin/api/apikeys',          { headers: headers() }),
      fetch('/admin/api/pricing',          { headers: headers() }),
    ])
    if (p1.ok) profit.value = await p1.json()
    if (p2.ok) analysis.value = await p2.json()
    if (p3.ok) keys.value = await p3.json()
    if (p4.ok) {
      const data = await p4.json()
      pricing.value = {
        freePoolPriceUSD: data.freePoolPriceUSD || 0.04,
        proPoolPriceUSD:  data.proPoolPriceUSD  || 0.20,
        purchasePriceCNY: data.purchasePriceCNY || 0.04,
        proCostEntries:   data.proCostEntries   || [],
        freeCostEntries:  data.freeCostEntries  || [],
      }
    }
  } catch (e) { console.error(e) }
  finally { loading.value = false }
}

async function savePricing() {
  if (!pricing.value) return
  saving.value = true
  try {
    const res = await fetch('/admin/api/pricing', {
      method: 'PUT', headers: headers(), body: JSON.stringify(pricing.value),
    })
    if (res.ok) { success('定价已保存'); fetchAll() }
    else toastErr('保存失败')
  } catch { toastErr('网络错误') }
  saving.value = false
}

async function addCostEntry(pool) {
  const form = pool === 'pro' ? proForm.value : freeForm.value
  const entry = pool === 'pro'
    ? { count: form.count, costCNY: form.costCNY, credits: form.credits }
    : { count: form.count, costCNY: form.costCNY }
  try {
    const res = await fetch('/admin/api/cost-entry', {
      method: 'POST', headers: headers(),
      body: JSON.stringify({ pool, entry }),
    })
    if (res.ok) {
      success('已添加')
      fetchAll()
      if (pool === 'pro') showProForm.value = false
      else showFreeForm.value = false
    }
  } catch { toastErr('添加失败') }
}

async function removeCostEntry(pool, id) {
  try {
    const res = await fetch('/admin/api/cost-entry', {
      method: 'DELETE', headers: headers(),
      body: JSON.stringify({ pool, id }),
    })
    if (res.ok) { success('已删除'); fetchAll() }
  } catch { toastErr('删除失败') }
}

onMounted(() => { fetchAll(); timer = setInterval(fetchAll, 30000) })
onUnmounted(() => clearInterval(timer))

// Computed
const proSummary = computed(() => {
  const e = pricing.value?.proCostEntries || []
  let totalCost = 0, totalCredits = 0, totalCount = 0
  e.forEach(x => { totalCost += x.costCNY; totalCredits += x.count * (x.credits || 0); totalCount += x.count })
  return {
    totalCost, totalCredits, totalCount,
    avgCostPerCredit: totalCredits > 0 ? (totalCost / totalCredits).toFixed(4) : '—',
  }
})
const freeSummary = computed(() => {
  const e = pricing.value?.freeCostEntries || []
  let totalCost = 0, totalCount = 0
  e.forEach(x => { totalCost += x.costCNY; totalCount += x.count })
  const totalCredits = totalCount * 550
  return {
    totalCost, totalCredits, totalCount,
    avgCostPerCredit: totalCredits > 0 ? (totalCost / totalCredits).toFixed(6) : '—',
  }
})

const models = computed(() => {
  const m = analysis.value?.modelBreakdown || {}
  return Object.entries(m)
    .map(([name, stats]) => ({ name, ...stats }))
    .filter(m => m.name)
    .sort((a, b) => b.totalCredits - a.totalCredits)
})

function isProPool(model) {
  const m = (model || '').toLowerCase()
  return m.includes('opus') || (m.includes('sonnet') && (m.includes('4.6') || m.includes('4-6')))
}

const poolStats = computed(() => {
  const all = { requests: 0, credits: 0, tokens: 0, errors: 0 }
  const free = { requests: 0, credits: 0, tokens: 0, errors: 0 }
  const pro  = { requests: 0, credits: 0, tokens: 0, errors: 0 }
  models.value.forEach(m => {
    const t = isProPool(m.name) ? pro : free
    t.requests += m.requests || 0
    t.credits  += m.totalCredits || 0
    t.tokens   += (m.avgTokens || 0) * (m.requests || 0)
    t.errors   += m.errors || 0
    all.requests += m.requests || 0
    all.credits  += m.totalCredits || 0
    all.tokens   += (m.avgTokens || 0) * (m.requests || 0)
    all.errors   += m.errors || 0
  })
  return { all, free, pro }
})
const currentPoolStats = computed(() => poolStats.value[poolTab.value] || poolStats.value.all)

const keyRanking = computed(() => [...keys.value]
  .filter(k => k.credits > 0 || k.requests > 0)
  .sort((a, b) => b.credits - a.credits)
  .slice(0, 20)
  .map((k, i) => ({
    rank: i + 1,
    id: (k.id || '').substring(0, 8),
    note: k.note || '—',
    plan: k.plan || '—',
    credits: (k.credits || 0).toFixed(4),
    requests: k.requests || 0,
    balance: '$' + (k.balance || 0).toFixed(4),
  })))

const poolStatus = computed(() => analysis.value?.poolStatus || {})
const prediction = computed(() => analysis.value?.prediction || {})

function fmtDate(ts) {
  if (!ts) return ''
  return new Date(ts * 1000).toLocaleDateString('zh-CN')
}
function formatDays(hours) {
  if (!hours || hours <= 0) return '—'
  const d = Math.floor(hours / 24)
  const h = Math.floor(hours % 24)
  return d > 0 ? `${d}天${h}小时` : `${h}小时${Math.floor((hours % 1) * 60)}分`
}
function predictionVariant() {
  const c = prediction.value?.confidence
  if (c === 'high') return 'success'
  if (c === 'medium') return 'warning'
  return 'danger'
}
</script>

<template>
  <div class="pricing-page">
    <!-- Header -->
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">财务管理</div>
        <h1 class="page-title">定价中心</h1>
      </div>
      <WorldSegment v-model="tab" :options="tabOptions" />
    </header>

    <div v-if="loading" class="loading-row">载入中…</div>

    <template v-else-if="tab === 'analysis'">
      <!-- 利润总览 -->
      <div class="stat-row" v-if="profit">
        <WorldStat
          label="总收入" unit="CNY"
          :value="`¥${(profit.revenue_cny || 0).toFixed(2)}`"
          :icon="DollarSign" variant="success"
        />
        <WorldStat
          label="总成本" unit="CNY"
          :value="`¥${(profit.total_cost_cny || 0).toFixed(2)}`"
          :icon="BarChart3" variant="danger"
        />
        <WorldStat
          label="净利润" unit="CNY"
          :value="`¥${(profit.profit_cny || 0).toFixed(2)}`"
          :icon="TrendingUp"
          :variant="(profit.profit_cny || 0) >= 0 ? 'success' : 'danger'"
        />
        <WorldStat
          label="利润率" unit="%"
          :value="(profit.margin_percent || 0).toFixed(1)"
          :icon="Activity" variant="info"
        />
      </div>

      <!-- 采购成本记录 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>账号采购记录</h3>
          <p class="section-hint">记录每批 PRO/FREE 号的采购数量、总价、单号额度</p>
        </header>

        <!-- PRO 采购 -->
        <div class="cost-block">
          <div class="cost-head">
            <div>
              <div class="cost-title">PRO 号采购记录</div>
              <div class="cost-summary">
                平均 ¥{{ proSummary.avgCostPerCredit }}/cr · {{ proSummary.totalCount }} 个号 · ¥{{ proSummary.totalCost.toFixed(0) }}
              </div>
            </div>
            <WorldButton variant="secondary" size="sm" @click="showProForm = !showProForm">
              <Plus :size="13" /><span>{{ showProForm ? '取消' : '添加' }}</span>
            </WorldButton>
          </div>
          <Transition name="fade-slide">
            <div v-if="showProForm" class="cost-form">
              <WorldInput v-model.number="proForm.count" type="number" label="数量" size="sm" />
              <WorldInput v-model.number="proForm.costCNY" type="number" label="花费 (¥)" size="sm" />
              <WorldInput v-model.number="proForm.credits" type="number" label="每号额度 (cr)" size="sm" />
              <WorldButton variant="primary" size="sm" @click="addCostEntry('pro')">确定</WorldButton>
            </div>
          </Transition>
          <div class="entry-list">
            <div v-for="e in (pricing?.proCostEntries || [])" :key="e.id" class="entry-row">
              <span>{{ e.count }} 个号</span>
              <span>¥{{ e.costCNY }}</span>
              <span>{{ e.credits }} cr/号</span>
              <span class="dim">¥{{ (e.costCNY / (e.count * e.credits)).toFixed(4) }}/cr</span>
              <span class="date">{{ fmtDate(e.createdAt) }}</span>
              <button class="del-btn" @click="removeCostEntry('pro', e.id)" aria-label="删除"><Trash2 :size="12" /></button>
            </div>
            <div v-if="!(pricing?.proCostEntries?.length)" class="entry-empty">暂无记录</div>
          </div>
        </div>

        <!-- FREE 采购 -->
        <div class="cost-block">
          <div class="cost-head">
            <div>
              <div class="cost-title">FREE 号采购记录</div>
              <div class="cost-summary">
                平均 ¥{{ freeSummary.avgCostPerCredit }}/cr · {{ freeSummary.totalCount }} 个号 · ¥{{ freeSummary.totalCost.toFixed(0) }} · 固定 550 cr/号
              </div>
            </div>
            <WorldButton variant="secondary" size="sm" @click="showFreeForm = !showFreeForm">
              <Plus :size="13" /><span>{{ showFreeForm ? '取消' : '添加' }}</span>
            </WorldButton>
          </div>
          <Transition name="fade-slide">
            <div v-if="showFreeForm" class="cost-form">
              <WorldInput v-model.number="freeForm.count" type="number" label="数量 (个)" size="sm" />
              <WorldInput v-model.number="freeForm.costCNY" type="number" label="花费 (¥)" size="sm" />
              <WorldInput :modelValue="550" disabled label="每号额度" size="sm" />
              <WorldButton variant="primary" size="sm" @click="addCostEntry('free')">确定</WorldButton>
            </div>
          </Transition>
          <div class="entry-list">
            <div v-for="e in (pricing?.freeCostEntries || [])" :key="e.id" class="entry-row">
              <span>{{ e.count }} 个号</span>
              <span>¥{{ e.costCNY }}</span>
              <span>550 cr/号</span>
              <span class="dim">¥{{ (e.costCNY / (e.count * 550)).toFixed(6) }}/cr</span>
              <span class="date">{{ fmtDate(e.createdAt) }}</span>
              <button class="del-btn" @click="removeCostEntry('free', e.id)" aria-label="删除"><Trash2 :size="12" /></button>
            </div>
            <div v-if="!(pricing?.freeCostEntries?.length)" class="entry-empty">暂无记录</div>
          </div>
        </div>
      </WorldCard>

      <!-- 用量统计 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>用量统计</h3>
          <WorldSegment v-model="poolTab" :options="[
            { value: 'all', label: 'ALL' },
            { value: 'free', label: 'FREE' },
            { value: 'pro',  label: 'PRO' },
          ]" size="sm" />
        </header>
        <div class="stat-row">
          <WorldStat label="请求数" :value="currentPoolStats.requests" :icon="Activity" />
          <WorldStat label="总 Credit" :value="currentPoolStats.credits.toFixed(2)" :icon="DollarSign" variant="warning" />
          <WorldStat label="总 Token" :value="currentPoolStats.tokens.toLocaleString()" variant="success" />
          <WorldStat label="错误数" :value="currentPoolStats.errors" variant="danger" />
        </div>
      </WorldCard>

      <!-- 号池状态 -->
      <div class="pool-grid">
        <WorldCard padding="md">
          <h3 class="section-title">PRO 池</h3>
          <WorldProgress
            :value="poolStatus.pro?.used || 0"
            :max="poolStatus.pro?.total || 1"
            variant="primary"
            :show-label="true"
            :label="`已用 ${(poolStatus.pro?.used || 0).toFixed(0)} / ${(poolStatus.pro?.total || 0).toFixed(0)} cr`"
            :hint="`剩余 ${(poolStatus.pro?.remaining || 0).toFixed(0)} cr`"
          />
        </WorldCard>
        <WorldCard padding="md">
          <h3 class="section-title">FREE 池</h3>
          <WorldProgress
            :value="poolStatus.free?.used || 0"
            :max="poolStatus.free?.total || 1"
            variant="success"
            :show-label="true"
            :label="`已用 ${(poolStatus.free?.used || 0).toFixed(0)} / ${(poolStatus.free?.total || 0).toFixed(0)} cr`"
            :hint="`剩余 ${(poolStatus.free?.remaining || 0).toFixed(0)} cr`"
          />
        </WorldCard>
      </div>

      <!-- Key 消费排行 -->
      <WorldCard padding="md">
        <h3 class="section-title">Key 消费排行</h3>
        <WorldTable
          :columns="[
            { key: 'rank',     label: '#',          align: 'left',  width: '40px' },
            { key: 'id',       label: 'Key ID',     mono: true },
            { key: 'note',     label: '备注' },
            { key: 'plan',     label: '套餐' },
            { key: 'credits',  label: '消耗 Credits', align: 'right', mono: true },
            { key: 'requests', label: '请求次数', align: 'right' },
            { key: 'balance',  label: '余额', align: 'right', mono: true },
          ]"
          :rows="keyRanking"
          empty-text="暂无数据"
        />
      </WorldCard>

      <!-- Credit 预测 -->
      <WorldCard padding="md" class="prediction-card">
        <header class="section-head">
          <h3>Credit 剩余预测</h3>
          <WorldChip
            v-if="prediction.confidence"
            :variant="predictionVariant()"
            :dot="true"
            size="sm"
          >
            {{ prediction.confidence === 'high' ? '高置信' : prediction.confidence === 'medium' ? '中置信' : '低置信' }}
          </WorldChip>
        </header>
        <div v-if="prediction.sufficient" class="pred-row">
          <div class="pred-block">
            <div class="pred-label">按日均消耗</div>
            <div class="pred-value">{{ (prediction.remainingDays || 0).toFixed(1) }} <span class="pred-unit">天</span></div>
          </div>
          <div class="pred-divider" />
          <div class="pred-block">
            <div class="pred-label">活跃使用时</div>
            <div class="pred-value">{{ formatDays(prediction.remainingHours) }}</div>
          </div>
        </div>
        <div v-else class="pred-empty">
          <AlertTriangle :size="16" />
          <span>数据不足（需要 3 次以上请求）</span>
        </div>
        <div class="pred-meta">
          <div><span class="m-label">活跃速率</span><span class="m-val">{{ (prediction.ratePerHour || 0).toFixed(2) }} cr/h</span></div>
          <div><span class="m-label">日均消耗</span><span class="m-val">{{ (prediction.dailyRate || 0).toFixed(3) }} cr/天</span></div>
          <div><span class="m-label">平均 Credit/次</span><span class="m-val">{{ (prediction.avgPerRequest || 0).toFixed(4) }}</span></div>
          <div><span class="m-label">数据量</span><span class="m-val">{{ prediction.totalRecords || 0 }} 条</span></div>
        </div>
      </WorldCard>
    </template>

    <template v-else>
      <!-- 售价配置 tab -->
      <WorldCard padding="lg">
        <header class="section-head">
          <h3>售价配置</h3>
          <WorldButton variant="primary" :loading="saving" @click="savePricing">
            <Save :size="14" /><span>保存配置</span>
          </WorldButton>
        </header>
        <p class="section-hint">设置用户调用接口时的扣费单价（按池）以及进货价（用于利润计算）</p>

        <div class="cfg-grid">
          <WorldInput
            v-model.number="pricing.freePoolPriceUSD"
            type="number"
            label="FREE 池单价 ($/credit)"
            hint="sonnet-4.5 使用此价格"
          />
          <WorldInput
            v-model.number="pricing.proPoolPriceUSD"
            type="number"
            label="PRO 池单价 ($/credit)"
            hint="sonnet-4.6, opus-4.6 使用此价格"
          />
          <WorldInput
            v-model.number="pricing.purchasePriceCNY"
            type="number"
            label="PRO 进货价 (¥/credit)"
            hint="用于利润计算"
          />
        </div>
      </WorldCard>

      <WorldCard padding="md">
        <header class="section-head">
          <h3>快速参考</h3>
          <SettingsIcon :size="14" />
        </header>
        <div class="ref-list">
          <p>· 1 Kiro credit = $2 面值</p>
          <p>· FREE 池默认 $0.04/credit → 用户消耗 1 credit 花费 $0.04</p>
          <p>· PRO 池默认 $0.20/credit → 用户消耗 1 credit 花费 $0.20</p>
          <p>· 进货成本默认 ¥0.04/credit → 利润 = 售价收入 - 进货成本</p>
        </div>
      </WorldCard>
    </template>
  </div>
</template>

<style scoped>
.pricing-page {
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
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}

.loading-row {
  padding: 60px 20px;
  text-align: center;
  color: var(--world-text-mute);
}

.stat-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 920px) { .stat-row { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 480px) { .stat-row { grid-template-columns: 1fr; } }

.section-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.section-head h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
.section-hint {
  font-size: 0.78rem;
  color: var(--world-text-mute);
  margin: 0 0 14px;
}
.section-title {
  font-size: 0.875rem;
  font-weight: 800;
  margin: 0 0 12px;
  color: var(--world-text-primary);
}

.cost-block {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--world-divider);
}
.cost-block:first-of-type { margin-top: 6px; padding-top: 0; border-top: none; }
.cost-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  gap: 12px;
}
.cost-title { font-size: 0.875rem; font-weight: 800; color: var(--world-text-primary); }
.cost-summary {
  font-size: 0.7rem;
  color: var(--world-text-mute);
  margin-top: 2px;
}

.cost-form {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 12px;
  background: var(--world-overlay-light);
  border-radius: var(--world-radius-md);
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.cost-form > * { flex: 1 1 120px; min-width: 0; }
.cost-form > button { flex: 0 0 auto; }

.entry-list { display: flex; flex-direction: column; gap: 4px; }
.entry-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 8px 12px;
  background: var(--world-overlay-light);
  border-radius: var(--world-radius-sm);
  font-size: 0.75rem;
  font-family: var(--world-font-mono);
  color: var(--world-text-primary);
  flex-wrap: wrap;
}
.entry-row .dim { color: var(--world-text-dim); }
.entry-row .date { margin-left: auto; color: var(--world-text-dim); }
.del-btn {
  background: transparent;
  border: none;
  color: var(--world-text-dim);
  cursor: pointer;
  width: 22px; height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--world-radius-sm);
  transition: all 200ms ease;
}
.del-btn:hover { color: var(--world-error); background: rgba(239, 68, 68, 0.1); }
.entry-empty {
  text-align: center;
  padding: 12px;
  font-size: 0.75rem;
  color: var(--world-text-dim);
}

.pool-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
@media (max-width: 768px) { .pool-grid { grid-template-columns: 1fr; } }

.prediction-card .pred-row {
  display: flex;
  align-items: stretch;
  gap: 18px;
}
.pred-block { display: flex; flex-direction: column; gap: 4px; }
.pred-label { font-size: 0.7rem; color: var(--world-text-mute); }
.pred-value { font-size: 2rem; font-weight: 800; color: var(--world-text-primary); font-family: var(--world-font-display); }
.pred-unit { font-size: 1rem; color: var(--world-text-mute); }
.pred-divider { width: 1px; background: var(--world-divider); }
.pred-empty {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 18px;
  font-size: 0.875rem;
  color: var(--world-text-mute);
}
.pred-meta {
  margin-top: 14px;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  padding-top: 14px;
  border-top: 1px solid var(--world-divider);
}
.pred-meta > div { display: flex; flex-direction: column; gap: 2px; font-size: 0.75rem; }
.m-label { color: var(--world-text-mute); }
.m-val { color: var(--world-text-primary); font-weight: 700; font-family: var(--world-font-mono); }
@media (max-width: 768px) { .pred-meta { grid-template-columns: repeat(2, 1fr); } }

.cfg-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 14px;
  margin-top: 14px;
}
@media (max-width: 768px) { .cfg-grid { grid-template-columns: 1fr; } }

.ref-list p {
  margin: 4px 0;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
  line-height: 1.6;
}

.fade-slide-enter-active, .fade-slide-leave-active { transition: all 280ms cubic-bezier(0.16, 1, 0.3, 1); }
.fade-slide-enter-from, .fade-slide-leave-to { opacity: 0; transform: translateY(-6px); }
</style>
