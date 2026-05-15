<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { api } from '../api/admin'
import { DollarSign, TrendingUp, BarChart3, Activity, Zap, Clock, Users, Save, AlertTriangle, Plus, Trash2 } from 'lucide-vue-next'

// ====== Data ======
const profit = ref(null)
const analysis = ref(null)
const keys = ref([])
const pricing = ref(null)
const loading = ref(true)
const saving = ref(false)
const saveMsg = ref('')
const poolTab = ref('all')
let timer = null

// ====== Cost entry forms ======
const showProForm = ref(false)
const showFreeForm = ref(false)
const proForm = ref({ count: 1, costCNY: 60, credits: 1500 })
const freeForm = ref({ count: 100, costCNY: 9 })

async function fetchAll() {
  try {
    const [profitRes, analysisRes, keysRes, pricingRes] = await Promise.all([
      api('/profit'),
      api('/pricing-analysis'),
      api('/apikeys'),
      api('/pricing'),
    ])
    profit.value = await profitRes.json()
    analysis.value = await analysisRes.json()
    keys.value = await keysRes.json()
    pricing.value = await pricingRes.json()
  } catch (e) { console.error('fetch error:', e) }
  finally { loading.value = false }
}

onMounted(() => { fetchAll(); timer = setInterval(fetchAll, 30000) })
onUnmounted(() => clearInterval(timer))

// ====== Save sell prices ======
async function savePricing() {
  if (!pricing.value) return
  saving.value = true; saveMsg.value = ''
  try {
    await api('/pricing', { method: 'PUT', body: JSON.stringify(pricing.value) })
    saveMsg.value = '✅ 已保存'
    fetchAll()
  } catch { saveMsg.value = '❌ 网络错误' }
  saving.value = false
  setTimeout(() => saveMsg.value = '', 3000)
}

// ====== Cost entry CRUD ======
async function addCostEntry(pool) {
  const form = pool === 'pro' ? proForm.value : freeForm.value
  const entry = pool === 'pro'
    ? { count: form.count, costCNY: form.costCNY, credits: form.credits }
    : { count: form.count, costCNY: form.costCNY }
  try {
    await api('/cost-entry', { method: 'POST', body: JSON.stringify({ pool, entry }) })
    fetchAll()
    if (pool === 'pro') showProForm.value = false
    else showFreeForm.value = false
  } catch (e) { console.error(e) }
}

async function removeCostEntry(pool, id) {
  try {
    await api('/cost-entry', { method: 'DELETE', body: JSON.stringify({ pool, id }) })
    fetchAll()
  } catch (e) { console.error(e) }
}

// ====== Computed: cost summaries ======
const proSummary = computed(() => {
  const entries = pricing.value?.proCostEntries || []
  let totalCost = 0, totalCredits = 0, totalCount = 0
  entries.forEach(e => { totalCost += e.costCNY; totalCredits += e.count * (e.credits || 0); totalCount += e.count })
  return { totalCost, totalCredits, totalCount, avgCostPerCredit: totalCredits > 0 ? (totalCost / totalCredits).toFixed(4) : '—' }
})

const freeSummary = computed(() => {
  const entries = pricing.value?.freeCostEntries || []
  let totalCost = 0, totalCount = 0
  entries.forEach(e => { totalCost += e.costCNY; totalCount += e.count })
  const totalCredits = totalCount * 550
  return { totalCost, totalCredits, totalCount, avgCostPerCredit: totalCredits > 0 ? (totalCost / totalCredits).toFixed(6) : '—' }
})

// ====== Computed: pool stats ======
const models = computed(() => {
  const m = analysis.value?.modelBreakdown || {}
  return Object.entries(m).map(([name, stats]) => ({ name, ...stats })).filter(m => m.name).sort((a, b) => b.totalCredits - a.totalCredits)
})

function isProPool(model) {
  const m = model.toLowerCase()
  return m.includes('opus') || (m.includes('sonnet') && (m.includes('4.6') || m.includes('4-6')))
}

const poolStats = computed(() => {
  const all = { requests: 0, credits: 0, tokens: 0, errors: 0 }
  const free = { requests: 0, credits: 0, tokens: 0, errors: 0 }
  const pro = { requests: 0, credits: 0, tokens: 0, errors: 0 }
  models.value.forEach(m => {
    const target = isProPool(m.name) ? pro : free
    target.requests += m.requests || 0; target.credits += m.totalCredits || 0
    target.tokens += (m.avgTokens || 0) * (m.requests || 0); target.errors += m.errors || 0
    all.requests += m.requests || 0; all.credits += m.totalCredits || 0
    all.tokens += (m.avgTokens || 0) * (m.requests || 0); all.errors += m.errors || 0
  })
  return { all, free, pro }
})

const currentPoolStats = computed(() => poolStats.value[poolTab.value] || poolStats.value.all)
const proModels = computed(() => models.value.filter(m => isProPool(m.name)))
const freeModels = computed(() => models.value.filter(m => !isProPool(m.name)))

// ====== Key ranking ======
const keyRanking = computed(() => [...keys.value].filter(k => k.credits > 0 || k.requests > 0).sort((a, b) => b.credits - a.credits).slice(0, 20))

// ====== Pool status & prediction ======
const poolStatus = computed(() => analysis.value?.poolStatus || {})
const prediction = computed(() => analysis.value?.prediction || {})

function formatDays(hours) {
  if (!hours || hours <= 0) return '—'
  const d = Math.floor(hours / 24), h = Math.floor(hours % 24)
  return d > 0 ? `${d}天${h}小时` : `${h}小时${Math.floor((hours % 1) * 60)}分`
}

function fmtDate(ts) {
  if (!ts) return ''
  return new Date(ts * 1000).toLocaleDateString('zh-CN')
}
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="text-center py-20 text-[var(--text)]/50">加载中...</div>

    <template v-else>
      <!-- ===== 1. 利润总览 ===== -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <div class="bg-gradient-to-br from-green-500/10 to-emerald-500/10 border border-green-500/20 rounded-2xl p-5 text-center">
          <DollarSign class="w-5 h-5 mx-auto mb-2 text-green-400" />
          <div class="text-2xl font-black text-green-400">¥{{ profit?.revenue_cny?.toFixed(2) || '0' }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总收入 (CNY)</div>
        </div>
        <div class="bg-gradient-to-br from-red-500/10 to-orange-500/10 border border-red-500/20 rounded-2xl p-5 text-center">
          <BarChart3 class="w-5 h-5 mx-auto mb-2 text-red-400" />
          <div class="text-2xl font-black text-red-400">¥{{ profit?.total_cost_cny?.toFixed(2) || '0' }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总成本 (CNY)</div>
        </div>
        <div class="bg-gradient-to-br from-emerald-500/10 to-teal-500/10 border border-emerald-500/20 rounded-2xl p-5 text-center">
          <TrendingUp class="w-5 h-5 mx-auto mb-2 text-emerald-400" />
          <div class="text-2xl font-black" :class="(profit?.profit_cny || 0) >= 0 ? 'text-emerald-400' : 'text-red-400'">¥{{ profit?.profit_cny?.toFixed(2) || '0' }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">净利润 (CNY)</div>
        </div>
        <div class="bg-gradient-to-br from-purple-500/10 to-blue-500/10 border border-purple-500/20 rounded-2xl p-5 text-center">
          <Activity class="w-5 h-5 mx-auto mb-2 text-purple-400" />
          <div class="text-2xl font-black text-purple-400">{{ profit?.margin_percent?.toFixed(1) || '0' }}%</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">利润率</div>
        </div>
      </div>

      <!-- ===== 2. 定价与成本 ===== -->
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl p-6">
        <div class="flex items-center justify-between mb-4">
          <div>
            <div class="text-sm font-black text-[var(--text)]">⚙️ 定价与成本</div>
            <div class="text-[10px] text-[var(--text)]/40 mt-1">售价配置 · 采购成本记录</div>
          </div>
          <div class="flex items-center gap-3">
            <span v-if="saveMsg" class="text-xs font-bold" :class="saveMsg.includes('✅') ? 'text-green-400' : 'text-red-400'">{{ saveMsg }}</span>
            <button @click="savePricing" :disabled="saving"
              class="flex items-center gap-2 px-4 py-2 bg-[var(--primary)] text-white rounded-xl text-xs font-bold hover:opacity-90 transition disabled:opacity-50">
              <Save class="w-3.5 h-3.5" /> {{ saving ? '保存中...' : '保存售价' }}
            </button>
          </div>
        </div>

        <!-- 售价 -->
        <div v-if="pricing" class="grid grid-cols-2 gap-4 mb-6">
          <div>
            <label class="block text-[10px] text-[var(--text)]/40 mb-1 font-bold">FREE 池售价 ($/credit)</label>
            <input v-model.number="pricing.freePoolPriceUSD" type="number" step="0.01" min="0"
              class="w-full px-3 py-2 bg-[var(--bg)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:border-[var(--primary)] outline-none" />
          </div>
          <div>
            <label class="block text-[10px] text-[var(--text)]/40 mb-1 font-bold">PRO 池售价 ($/credit)</label>
            <input v-model.number="pricing.proPoolPriceUSD" type="number" step="0.01" min="0"
              class="w-full px-3 py-2 bg-[var(--bg)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:border-[var(--primary)] outline-none" />
          </div>
        </div>

        <!-- PRO 成本记录 -->
        <div class="mb-4">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <span class="text-xs font-black text-purple-400">PRO 号采购记录</span>
              <span class="text-[9px] text-[var(--text)]/30">平均 ¥{{ proSummary.avgCostPerCredit }}/cr · {{ proSummary.totalCount }}个号 · ¥{{ proSummary.totalCost.toFixed(0) }}</span>
            </div>
            <button @click="showProForm = !showProForm" class="flex items-center gap-1 text-[10px] font-bold text-purple-400 hover:text-purple-300">
              <Plus class="w-3.5 h-3.5" /> 添加
            </button>
          </div>
          <!-- add form -->
          <div v-if="showProForm" class="flex items-end gap-2 mb-2 bg-purple-500/5 rounded-lg p-3">
            <div class="flex-1">
              <label class="block text-[9px] text-[var(--text)]/30 mb-0.5">数量</label>
              <input v-model.number="proForm.count" type="number" min="1" class="w-full px-2 py-1.5 bg-[var(--bg)] border border-[var(--border)] rounded text-xs text-[var(--text)] outline-none" />
            </div>
            <div class="flex-1">
              <label class="block text-[9px] text-[var(--text)]/30 mb-0.5">花费 (¥)</label>
              <input v-model.number="proForm.costCNY" type="number" min="0" class="w-full px-2 py-1.5 bg-[var(--bg)] border border-[var(--border)] rounded text-xs text-[var(--text)] outline-none" />
            </div>
            <div class="flex-1">
              <label class="block text-[9px] text-[var(--text)]/30 mb-0.5">每号额度 (cr)</label>
              <input v-model.number="proForm.credits" type="number" min="0" class="w-full px-2 py-1.5 bg-[var(--bg)] border border-[var(--border)] rounded text-xs text-[var(--text)] outline-none" />
            </div>
            <button @click="addCostEntry('pro')" class="px-3 py-1.5 bg-purple-500 text-white rounded text-xs font-bold hover:opacity-90">确定</button>
          </div>
          <!-- list -->
          <div class="space-y-1">
            <div v-for="e in (pricing?.proCostEntries || [])" :key="e.id"
              class="flex items-center justify-between text-[10px] bg-[var(--bg)]/50 rounded-lg px-3 py-2">
              <div class="flex gap-4 text-[var(--text)]/60">
                <span>{{ e.count }}个号</span>
                <span class="text-purple-400">¥{{ e.costCNY }}</span>
                <span>{{ e.credits }}cr/号</span>
                <span class="text-[var(--text)]/30">= ¥{{ (e.costCNY / (e.count * e.credits)).toFixed(4) }}/cr</span>
                <span class="text-[var(--text)]/20">{{ fmtDate(e.createdAt) }}</span>
              </div>
              <button @click="removeCostEntry('pro', e.id)" class="text-red-400/50 hover:text-red-400"><Trash2 class="w-3 h-3" /></button>
            </div>
            <div v-if="!(pricing?.proCostEntries?.length)" class="text-[10px] text-[var(--text)]/20 text-center py-2">暂无记录</div>
          </div>
        </div>

        <!-- FREE 成本记录 -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <span class="text-xs font-black text-green-400">FREE 号采购记录</span>
              <span class="text-[9px] text-[var(--text)]/30">平均 ¥{{ freeSummary.avgCostPerCredit }}/cr · {{ freeSummary.totalCount }}个号 · ¥{{ freeSummary.totalCost.toFixed(0) }} · 固定550cr/号</span>
            </div>
            <button @click="showFreeForm = !showFreeForm" class="flex items-center gap-1 text-[10px] font-bold text-green-400 hover:text-green-300">
              <Plus class="w-3.5 h-3.5" /> 添加
            </button>
          </div>
          <div v-if="showFreeForm" class="flex items-end gap-2 mb-2 bg-green-500/5 rounded-lg p-3">
            <div class="flex-1">
              <label class="block text-[9px] text-[var(--text)]/30 mb-0.5">数量 (个)</label>
              <input v-model.number="freeForm.count" type="number" min="1" class="w-full px-2 py-1.5 bg-[var(--bg)] border border-[var(--border)] rounded text-xs text-[var(--text)] outline-none" />
            </div>
            <div class="flex-1">
              <label class="block text-[9px] text-[var(--text)]/30 mb-0.5">花费 (¥)</label>
              <input v-model.number="freeForm.costCNY" type="number" min="0" class="w-full px-2 py-1.5 bg-[var(--bg)] border border-[var(--border)] rounded text-xs text-[var(--text)] outline-none" />
            </div>
            <div class="flex-1">
              <label class="block text-[9px] text-[var(--text)]/30 mb-0.5">每号额度</label>
              <input value="550" disabled class="w-full px-2 py-1.5 bg-[var(--bg)]/50 border border-[var(--border)] rounded text-xs text-[var(--text)]/30 outline-none" />
            </div>
            <button @click="addCostEntry('free')" class="px-3 py-1.5 bg-green-500 text-white rounded text-xs font-bold hover:opacity-90">确定</button>
          </div>
          <div class="space-y-1">
            <div v-for="e in (pricing?.freeCostEntries || [])" :key="e.id"
              class="flex items-center justify-between text-[10px] bg-[var(--bg)]/50 rounded-lg px-3 py-2">
              <div class="flex gap-4 text-[var(--text)]/60">
                <span>{{ e.count }}个号</span>
                <span class="text-green-400">¥{{ e.costCNY }}</span>
                <span>550cr/号</span>
                <span class="text-[var(--text)]/30">= ¥{{ (e.costCNY / (e.count * 550)).toFixed(6) }}/cr</span>
                <span class="text-[var(--text)]/20">{{ fmtDate(e.createdAt) }}</span>
              </div>
              <button @click="removeCostEntry('free', e.id)" class="text-red-400/50 hover:text-red-400"><Trash2 class="w-3 h-3" /></button>
            </div>
            <div v-if="!(pricing?.freeCostEntries?.length)" class="text-[10px] text-[var(--text)]/20 text-center py-2">暂无记录</div>
          </div>
        </div>
      </div>

      <!-- ===== 3. 用量统计 ===== -->
      <div>
        <div class="flex items-center gap-2 mb-3">
          <button v-for="tab in ['all','free','pro']" :key="tab" @click="poolTab = tab"
            class="px-4 py-1.5 rounded-full text-xs font-bold transition-all"
            :class="poolTab === tab ? 'bg-[var(--primary)] text-white shadow-lg shadow-[var(--primary)]/20' : 'text-[var(--text)]/50 hover:text-[var(--text)] hover:bg-[var(--card)]'">
            {{ tab === 'all' ? 'ALL' : tab.toUpperCase() }}
          </button>
        </div>
        <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
          <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
            <div class="text-2xl font-black text-[var(--text)]">{{ currentPoolStats.requests }}</div>
            <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">请求数</div>
          </div>
          <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
            <div class="text-2xl font-black text-yellow-400">{{ currentPoolStats.credits.toFixed(2) }}</div>
            <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总 Credit</div>
          </div>
          <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
            <div class="text-2xl font-black text-green-400">{{ currentPoolStats.tokens.toLocaleString() }}</div>
            <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总 Token</div>
          </div>
          <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
            <div class="text-2xl font-black text-red-400">{{ currentPoolStats.errors }}</div>
            <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">错误数</div>
          </div>
        </div>
      </div>

      <!-- ===== 4. 号池状态 + 模型数据 ===== -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl p-5">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-black text-purple-400 uppercase tracking-wider">PRO 池</span>
            <span class="text-[10px] text-[var(--text)]/40">{{ poolStatus.pro?.used?.toFixed(0) || 0 }} / {{ poolStatus.pro?.total?.toFixed(0) || 0 }} cr</span>
          </div>
          <div class="w-full h-2 bg-[var(--border)] rounded-full overflow-hidden mb-3">
            <div class="h-full bg-gradient-to-r from-purple-500 to-blue-500 rounded-full transition-all" :style="{ width: poolStatus.pro?.total ? `${(poolStatus.pro.used / poolStatus.pro.total * 100)}%` : '0%' }"></div>
          </div>
          <div class="text-[10px] text-[var(--text)]/30 mb-3">剩余 {{ poolStatus.pro?.remaining?.toFixed(0) || 0 }} cr</div>
          <div v-if="proModels.length" class="space-y-2">
            <div v-for="m in proModels" :key="m.name" class="flex items-center justify-between text-[10px] bg-[var(--bg)]/50 rounded-lg px-3 py-2">
              <span class="font-bold text-[var(--text)]">{{ m.name }}</span>
              <div class="flex gap-4 text-[var(--text)]/50">
                <span>{{ m.requests }}次</span>
                <span class="text-purple-400">{{ m.avgCredits?.toFixed(4) }} cr/次</span>
                <span>{{ m.avgTokens?.toLocaleString() }} tok/次</span>
              </div>
            </div>
          </div>
          <div v-else class="text-[10px] text-[var(--text)]/30 text-center py-2">暂无数据</div>
        </div>
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl p-5">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-black text-green-400 uppercase tracking-wider">FREE 池</span>
            <span class="text-[10px] text-[var(--text)]/40">{{ poolStatus.free?.used?.toFixed(0) || 0 }} / {{ poolStatus.free?.total?.toFixed(0) || 0 }} cr</span>
          </div>
          <div class="w-full h-2 bg-[var(--border)] rounded-full overflow-hidden mb-3">
            <div class="h-full bg-gradient-to-r from-green-500 to-emerald-500 rounded-full transition-all" :style="{ width: poolStatus.free?.total ? `${(poolStatus.free.used / poolStatus.free.total * 100)}%` : '0%' }"></div>
          </div>
          <div class="text-[10px] text-[var(--text)]/30 mb-3">剩余 {{ poolStatus.free?.remaining?.toFixed(0) || 0 }} cr</div>
          <div v-if="freeModels.length" class="space-y-2">
            <div v-for="m in freeModels" :key="m.name" class="flex items-center justify-between text-[10px] bg-[var(--bg)]/50 rounded-lg px-3 py-2">
              <span class="font-bold text-[var(--text)]">{{ m.name }}</span>
              <div class="flex gap-4 text-[var(--text)]/50">
                <span>{{ m.requests }}次</span>
                <span class="text-green-400">{{ m.avgCredits?.toFixed(4) }} cr/次</span>
                <span>{{ m.avgTokens?.toLocaleString() }} tok/次</span>
              </div>
            </div>
          </div>
          <div v-else class="text-[10px] text-[var(--text)]/30 text-center py-2">暂无数据</div>
        </div>
      </div>

      <!-- ===== 5. Key 消费排行 ===== -->
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl overflow-hidden">
        <div class="p-5 border-b border-[var(--border)]">
          <div class="text-sm font-black text-[var(--text)]">👤 Key 消费排行</div>
        </div>
        <div class="overflow-x-auto">
          <table v-if="keyRanking.length" class="w-full text-xs">
            <thead>
              <tr class="text-left text-[var(--text)]/40 border-b border-[var(--border)]">
                <th class="p-3 font-bold">#</th>
                <th class="p-3 font-bold">Key ID</th>
                <th class="p-3 font-bold">备注</th>
                <th class="p-3 font-bold">套餐</th>
                <th class="p-3 font-bold text-right">消耗 Credits</th>
                <th class="p-3 font-bold text-right">请求次数</th>
                <th class="p-3 font-bold text-right">余额</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(k, i) in keyRanking" :key="k.id" class="border-b border-[var(--border)]/30 hover:bg-[var(--primary)]/5">
                <td class="p-3 font-bold text-[var(--text)]/40">{{ i + 1 }}</td>
                <td class="p-3 font-mono text-[var(--text)]/60">{{ k.id?.substring(0, 8) }}</td>
                <td class="p-3 text-[var(--text)]">{{ k.note || '—' }}</td>
                <td class="p-3"><span class="px-2 py-0.5 rounded-full text-[9px] font-bold" :class="{ 'bg-purple-500/20 text-purple-400': k.plan === 'credit', 'bg-blue-500/20 text-blue-400': k.plan === 'hybrid', 'bg-green-500/20 text-green-400': k.plan === 'timed' }">{{ k.plan }}</span></td>
                <td class="p-3 text-right font-bold text-yellow-400">{{ k.credits?.toFixed(4) }}</td>
                <td class="p-3 text-right text-blue-400">{{ k.requests }}</td>
                <td class="p-3 text-right text-green-400">${{ k.balance?.toFixed(4) }}</td>
              </tr>
            </tbody>
          </table>
          <div v-else class="p-8 text-center text-[var(--text)]/30 text-sm">暂无数据</div>
        </div>
      </div>

      <!-- ===== 6. Credit 预测 ===== -->
      <div class="bg-gradient-to-br from-purple-500/10 to-blue-500/10 border border-purple-500/20 rounded-2xl p-6 relative overflow-hidden">
        <div class="absolute top-4 right-4 opacity-10"><Clock class="w-24 h-24 text-purple-400" /></div>
        <div class="flex items-center gap-2 mb-2">
          <div class="text-xs font-bold text-purple-400 uppercase tracking-widest">Credit 剩余预测</div>
          <span v-if="prediction.confidence" class="text-[9px] font-bold px-1.5 py-0.5 rounded-full"
            :class="{ 'bg-red-500/20 text-red-400': prediction.confidence === 'low', 'bg-yellow-500/20 text-yellow-400': prediction.confidence === 'medium', 'bg-green-500/20 text-green-400': prediction.confidence === 'high' }">
            {{ prediction.confidence === 'high' ? '高置信' : prediction.confidence === 'medium' ? '中置信' : '低置信' }}
          </span>
        </div>
        <div v-if="prediction.sufficient" class="flex items-baseline gap-4">
          <div>
            <div class="text-[10px] text-[var(--text)]/40">按日均消耗</div>
            <span class="text-4xl font-black text-[var(--text)]">{{ prediction.remainingDays?.toFixed(1) || '—' }}</span>
            <span class="text-lg text-[var(--text)]/50 ml-1">天</span>
          </div>
          <div class="text-[var(--text)]/20 text-2xl">|</div>
          <div>
            <div class="text-[10px] text-[var(--text)]/40">活跃使用时</div>
            <span class="text-4xl font-black text-purple-400">{{ formatDays(prediction.remainingHours) }}</span>
          </div>
        </div>
        <div v-else class="text-xl font-bold text-[var(--text)]/40">
          <AlertTriangle class="w-5 h-5 inline mr-2 text-yellow-500" /> 数据不足（需要 3 次以上请求）
        </div>
        <div class="mt-4 grid grid-cols-2 lg:grid-cols-4 gap-3 text-xs">
          <div><div class="text-[var(--text)]/40 mb-1">活跃速率</div><div class="font-bold text-purple-300">{{ prediction.ratePerHour?.toFixed(2) || '—' }} cr/h</div></div>
          <div><div class="text-[var(--text)]/40 mb-1">日均消耗</div><div class="font-bold text-amber-300">{{ prediction.dailyRate?.toFixed(3) || '—' }} cr/天</div></div>
          <div><div class="text-[var(--text)]/40 mb-1">平均 Credit/次</div><div class="font-bold text-blue-300">{{ prediction.avgPerRequest?.toFixed(4) || '—' }}</div></div>
          <div><div class="text-[var(--text)]/40 mb-1">数据量</div><div class="font-bold text-green-300">{{ prediction.totalRecords || 0 }} 条</div></div>
        </div>
      </div>
    </template>
  </div>
</template>
