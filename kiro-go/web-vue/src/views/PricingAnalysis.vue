<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import { Clock, Zap, TrendingUp, AlertTriangle, DollarSign, BarChart3, Activity } from 'lucide-vue-next'

const auth = useAuthStore()
const data = ref(null)
const loading = ref(true)
const error = ref('')
let timer = null

async function fetchData() {
  try {
    const res = await fetch('/admin/api/pricing-analysis', {
      headers: { 'X-Admin-Password': auth.password }
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    data.value = await res.json()
    error.value = ''
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
  timer = setInterval(fetchData, 30000) // 每30秒刷新
})
onUnmounted(() => clearInterval(timer))

// ====== 常量：你的成本结构 ======
const COST_PER_CREDIT = 0.04        // 1 Credit = ¥0.04
const CNY_PER_PANEL_USD = 0.20      // 面板 $1 = 你真收 ¥0.20
const CREDITS_PER_ACCOUNT = 1500    // PRO 号 1500 Credits
const ACCOUNT_COST_CNY = 60         // PRO 号 ¥60/月
const TARGET_PROFIT_MULT = 3        // 目标利润倍数（3倍=赚2倍）

const prediction = computed(() => data.value?.prediction || {})
const summary = computed(() => data.value?.summary || {})
const models = computed(() => {
  const m = data.value?.modelBreakdown || {}
  return Object.entries(m).map(([name, stats]) => ({ name, ...stats }))
    .filter(m => m.name)
    .sort((a, b) => b.totalCredits - a.totalCredits)
})
const pool = computed(() => data.value?.poolStatus || {})

// 数据是否足够做定价
const hasEnoughData = computed(() => (summary.value.successRequests || 0) >= 10)

// ====== 数据驱动的定价计算 ======
const pricingPerModel = computed(() => {
  return models.value
    .filter(m => m.avgCredits > 0 && m.requests - m.errors > 0)
    .map(m => {
      const realCostCNY = m.avgCredits * COST_PER_CREDIT           // 单次真实成本(CNY)
      const breakEvenUSD = realCostCNY / CNY_PER_PANEL_USD          // 面板保本价(USD)
      const recommendUSD = breakEvenUSD * TARGET_PROFIT_MULT        // 面板推荐价(USD)
      // 从 creditPerKTok 反推倍率：倍率 = recommendUSD / 标准费率
      // OpenAI 标准费率约 $0.003/1K tok (sonnet 级别)
      const stdRate = 0.003
      const ratio = m.creditPerKTok > 0
        ? Math.ceil((m.creditPerKTok * COST_PER_CREDIT * TARGET_PROFIT_MULT) / (stdRate * CNY_PER_PANEL_USD))
        : 10
      return {
        name: m.name,
        avgCredits: m.avgCredits,
        avgTokens: m.avgTokens,
        creditPerKTok: m.creditPerKTok,
        realCostCNY,
        breakEvenUSD,
        recommendUSD,
        ratio,
        requests: m.requests - m.errors,
      }
    })
})

// 全局推荐起步价：从平均单次成本反推
const recommendedStartPrice = computed(() => {
  const avgCr = summary.value.avgCreditsPerReq || 0
  if (avgCr <= 0) return { usd: 0.25, formula: '默认值（数据不足）' }
  const realCost = avgCr * COST_PER_CREDIT
  const breakEven = realCost / CNY_PER_PANEL_USD
  const recommend = breakEven * TARGET_PROFIT_MULT
  // 向上取到 $0.05 的整数倍
  const rounded = Math.ceil(recommend / 0.05) * 0.05
  return {
    usd: Math.max(rounded, 0.05),
    realCostCNY: realCost,
    breakEvenUSD: breakEven,
    rawUSD: recommend,
    formula: `${avgCr.toFixed(4)} cr × ¥${COST_PER_CREDIT} ÷ ¥${CNY_PER_PANEL_USD}/$ × ${TARGET_PROFIT_MULT}x`
  }
})

// 盈亏模拟
const profitSim = computed(() => {
  const proRemaining = pool.value.pro?.remaining || 0
  const proTotal = pool.value.pro?.total || CREDITS_PER_ACCOUNT
  const usedCredits = (pool.value.pro?.used || 0)
  const successReqs = summary.value.successRequests || 0
  const avgCr = summary.value.avgCreditsPerReq || 0
  if (avgCr <= 0 || successReqs <= 0) return null

  const estimatedTotalReqs = Math.floor(proTotal / avgCr)           // 整个号能撑多少次
  const startP = recommendedStartPrice.value.usd
  const estimatedRevenue = estimatedTotalReqs * startP              // 预计面板总收入(USD)
  const realRevenueCNY = estimatedRevenue * CNY_PER_PANEL_USD       // 真实收入(CNY)
  const profitCNY = realRevenueCNY - ACCOUNT_COST_CNY               // 利润(CNY)
  const profitRate = ACCOUNT_COST_CNY > 0 ? (profitCNY / ACCOUNT_COST_CNY * 100) : 0

  return {
    estimatedTotalReqs,
    estimatedRevenue: estimatedRevenue.toFixed(2),
    realRevenueCNY: realRevenueCNY.toFixed(2),
    profitCNY: profitCNY.toFixed(2),
    profitRate: profitRate.toFixed(0),
    accountCost: ACCOUNT_COST_CNY,
  }
})

function formatDays(hours) {
  if (!hours || hours <= 0) return '—'
  const d = Math.floor(hours / 24)
  const h = Math.floor(hours % 24)
  if (d > 0) return `${d}天${h}小时`
  return `${h}小时${Math.floor((hours % 1) * 60)}分`
}
</script>

<template>
  <div class="space-y-6">
    <!-- 加载/错误 -->
    <div v-if="loading" class="text-center py-20 text-[var(--text)]/50">加载中...</div>
    <div v-else-if="error" class="text-center py-20 text-red-400">{{ error }}</div>

    <template v-else>
      <!-- ===== 顶部：剩余天数大卡片 ===== -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <!-- 预测卡片 -->
        <div class="lg:col-span-2 bg-gradient-to-br from-purple-500/10 to-blue-500/10 border border-purple-500/20 rounded-2xl p-6 relative overflow-hidden">
          <div class="absolute top-4 right-4 opacity-10">
            <Clock class="w-24 h-24 text-purple-400" />
          </div>
          <div class="flex items-center gap-2 mb-2">
            <div class="text-xs font-bold text-purple-400 uppercase tracking-widest">Credit 剩余预测</div>
            <span v-if="prediction.confidence" class="text-[9px] font-bold px-1.5 py-0.5 rounded-full"
                  :class="{
                    'bg-red-500/20 text-red-400': prediction.confidence === 'low',
                    'bg-yellow-500/20 text-yellow-400': prediction.confidence === 'medium',
                    'bg-green-500/20 text-green-400': prediction.confidence === 'high'
                  }">
              {{ prediction.confidence === 'high' ? '高置信' : prediction.confidence === 'medium' ? '中置信' : '低置信' }}
            </span>
          </div>
          <div v-if="prediction.sufficient" class="space-y-2">
            <div class="flex items-baseline gap-4">
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
          </div>
          <div v-else class="text-2xl font-bold text-[var(--text)]/40">
            <AlertTriangle class="w-6 h-6 inline mr-2 text-yellow-500" />
            数据不足（需要 3 次以上请求）
          </div>
          <div class="mt-4 grid grid-cols-2 lg:grid-cols-5 gap-3 text-xs">
            <div>
              <div class="text-[var(--text)]/40 mb-1">活跃速率</div>
              <div class="font-bold text-purple-300">{{ prediction.ratePerHour?.toFixed(2) || '—' }} cr/h</div>
            </div>
            <div>
              <div class="text-[var(--text)]/40 mb-1">日均消耗</div>
              <div class="font-bold text-amber-300">{{ prediction.dailyRate?.toFixed(3) || '—' }} cr/天</div>
            </div>
            <div>
              <div class="text-[var(--text)]/40 mb-1">平均 Credit/次</div>
              <div class="font-bold text-blue-300">{{ prediction.avgPerRequest?.toFixed(4) || '—' }}</div>
            </div>
            <div>
              <div class="text-[var(--text)]/40 mb-1">会话数</div>
              <div class="font-bold text-green-300">{{ prediction.activeSessions || 0 }} 次 ({{ prediction.avgSessionLength?.toFixed(0) || 0 }}m/次)</div>
            </div>
            <div>
              <div class="text-[var(--text)]/40 mb-1">数据量</div>
              <div class="font-bold text-green-300">{{ prediction.totalRecords || 0 }} 条</div>
            </div>
          </div>
        </div>

        <!-- 号池状态 -->
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl p-6">
          <div class="text-xs font-bold text-[var(--text)]/40 uppercase tracking-widest mb-4">号池状态</div>
          <div class="space-y-4">
            <div>
              <div class="flex justify-between text-xs mb-1">
                <span class="font-bold text-purple-400">PRO</span>
                <span class="text-[var(--text)]/60">{{ pool.pro?.used?.toFixed(0) || 0 }} / {{ pool.pro?.total?.toFixed(0) || 0 }}</span>
              </div>
              <div class="w-full h-2 bg-[var(--border)] rounded-full overflow-hidden">
                <div class="h-full bg-gradient-to-r from-purple-500 to-blue-500 rounded-full transition-all"
                     :style="{ width: pool.pro?.total ? `${(pool.pro.used / pool.pro.total * 100)}%` : '0%' }"></div>
              </div>
              <div class="text-[10px] text-[var(--text)]/40 mt-1">剩余 {{ pool.pro?.remaining?.toFixed(0) || 0 }} credits</div>
            </div>
            <div v-if="pool.free?.total > 0">
              <div class="flex justify-between text-xs mb-1">
                <span class="font-bold text-green-400">FREE</span>
                <span class="text-[var(--text)]/60">{{ pool.free?.used?.toFixed(0) || 0 }} / {{ pool.free?.total?.toFixed(0) || 0 }}</span>
              </div>
              <div class="w-full h-2 bg-[var(--border)] rounded-full overflow-hidden">
                <div class="h-full bg-gradient-to-r from-green-500 to-emerald-500 rounded-full"
                     :style="{ width: pool.free?.total ? `${(pool.free.used / pool.free.total * 100)}%` : '0%' }"></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ===== 数据采集统计 ===== -->
      <div class="grid grid-cols-2 lg:grid-cols-5 gap-3">
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
          <Activity class="w-4 h-4 mx-auto mb-2 text-blue-400" />
          <div class="text-2xl font-black text-[var(--text)]">{{ summary.totalRequests || 0 }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总请求</div>
        </div>
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
          <Zap class="w-4 h-4 mx-auto mb-2 text-yellow-400" />
          <div class="text-2xl font-black text-[var(--text)]">{{ summary.totalCredits?.toFixed(2) || '0' }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总 Credit</div>
        </div>
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
          <BarChart3 class="w-4 h-4 mx-auto mb-2 text-green-400" />
          <div class="text-2xl font-black text-[var(--text)]">{{ (summary.totalTokens || 0).toLocaleString() }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总 Token</div>
        </div>
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
          <DollarSign class="w-4 h-4 mx-auto mb-2 text-red-400" />
          <div class="text-2xl font-black text-[var(--text)]">¥{{ summary.totalCostCNY?.toFixed(2) || '0' }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">总成本</div>
        </div>
        <div class="bg-[var(--card)] border border-[var(--border)] rounded-xl p-4 text-center">
          <TrendingUp class="w-4 h-4 mx-auto mb-2 text-purple-400" />
          <div class="text-2xl font-black text-[var(--text)]">{{ summary.avgTokensPerReq?.toLocaleString() || '0' }}</div>
          <div class="text-[10px] text-[var(--text)]/40 font-bold mt-1">平均 Token/次</div>
        </div>
      </div>

      <!-- ===== 模型数据分析 ===== -->
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl overflow-hidden">
        <div class="p-5 border-b border-[var(--border)]">
          <div class="text-sm font-black text-[var(--text)]">模型级数据分析</div>
          <div class="text-[10px] text-[var(--text)]/40 mt-1">揭开 Kiro Credit 计费黑盒 — 每个模型每 1K Token 的真实 Credit 成本</div>
        </div>
        <div class="overflow-x-auto">
          <table v-if="models.length" class="w-full text-xs">
            <thead>
              <tr class="text-left text-[var(--text)]/40 border-b border-[var(--border)]">
                <th class="p-3 font-bold">模型</th>
                <th class="p-3 font-bold text-right">请求数</th>
                <th class="p-3 font-bold text-right">总 Credit</th>
                <th class="p-3 font-bold text-right">平均 Credit/次</th>
                <th class="p-3 font-bold text-right">平均 Token/次</th>
                <th class="p-3 font-bold text-right">Credit/1K Token</th>
                <th class="p-3 font-bold text-right">错误</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in models" :key="m.name" class="border-b border-[var(--border)]/30 hover:bg-[var(--primary)]/5">
                <td class="p-3 font-bold text-[var(--text)]">{{ m.name }}</td>
                <td class="p-3 text-right text-blue-400">{{ m.requests }}</td>
                <td class="p-3 text-right text-yellow-400">{{ m.totalCredits?.toFixed(4) }}</td>
                <td class="p-3 text-right font-bold text-purple-400">{{ m.avgCredits?.toFixed(4) }}</td>
                <td class="p-3 text-right text-green-400">{{ m.avgTokens?.toLocaleString() }}</td>
                <td class="p-3 text-right font-bold text-red-400">{{ m.creditPerKTok?.toFixed(4) }}</td>
                <td class="p-3 text-right text-red-400/60">{{ m.errors }}</td>
              </tr>
            </tbody>
          </table>
          <div v-else class="p-8 text-center text-[var(--text)]/30 text-sm">暂无模型数据</div>
        </div>
      </div>

      <!-- ===== 数据驱动定价推导（逐模型） ===== -->
      <div class="bg-[var(--card)] border border-[var(--border)] rounded-2xl overflow-hidden">
        <div class="p-5 border-b border-[var(--border)]">
          <div class="text-sm font-black text-[var(--text)]">💰 数据驱动定价推导</div>
          <div class="text-[10px] text-[var(--text)]/40 mt-1">
            公式：真实成本 = avgCredit × ¥0.04 → 保本价 = 成本 ÷ ¥0.20/$ → 推荐 = 保本 × {{ TARGET_PROFIT_MULT }}倍
          </div>
        </div>
        <div v-if="!hasEnoughData" class="p-8 text-center text-[var(--text)]/30 text-sm">
          <AlertTriangle class="w-5 h-5 inline mr-2 text-yellow-500" />
          需要至少 10 次成功请求后才能生成准确的定价方案（当前 {{ summary.successRequests || 0 }} 次）
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full text-xs">
            <thead>
              <tr class="text-left text-[var(--text)]/40 border-b border-[var(--border)]">
                <th class="p-3 font-bold">模型</th>
                <th class="p-3 font-bold text-right">样本数</th>
                <th class="p-3 font-bold text-right">平均 Credit</th>
                <th class="p-3 font-bold text-right">真实成本/次</th>
                <th class="p-3 font-bold text-right">面板保本价</th>
                <th class="p-3 font-bold text-right">推荐售价</th>
                <th class="p-3 font-bold text-right">推荐倍率</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in pricingPerModel" :key="m.name" class="border-b border-[var(--border)]/30">
                <td class="p-3 font-bold text-[var(--text)]">{{ m.name }}</td>
                <td class="p-3 text-right text-[var(--text)]/60">{{ m.requests }}</td>
                <td class="p-3 text-right text-purple-400">{{ m.avgCredits.toFixed(4) }}</td>
                <td class="p-3 text-right text-red-400">¥{{ m.realCostCNY.toFixed(4) }}</td>
                <td class="p-3 text-right text-yellow-400">${{ m.breakEvenUSD.toFixed(4) }}</td>
                <td class="p-3 text-right font-bold text-green-400">${{ m.recommendUSD.toFixed(3) }}</td>
                <td class="p-3 text-right">
                  <span class="font-black text-amber-400 text-base">{{ m.ratio }}×</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- ===== 底部：起步价 + 盈亏模拟 ===== -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <!-- 推荐起步价 -->
        <div class="bg-gradient-to-br from-amber-500/10 to-orange-500/10 border border-amber-500/20 rounded-2xl p-6">
          <div class="text-sm font-black text-[var(--text)] mb-4">📐 推荐起步价（数据驱动）</div>
          <div class="text-center mb-4">
            <div class="text-4xl font-black text-amber-400">${{ recommendedStartPrice.usd.toFixed(2) }}</div>
            <div class="text-[10px] text-[var(--text)]/40 mt-2">每次请求最低收费</div>
          </div>
          <div class="bg-[var(--card)]/40 rounded-xl p-3 space-y-2 text-[10px]">
            <div class="flex justify-between">
              <span class="text-[var(--text)]/50">计算公式</span>
              <span class="text-amber-300 font-mono">{{ recommendedStartPrice.formula }}</span>
            </div>
            <div v-if="recommendedStartPrice.realCostCNY" class="flex justify-between">
              <span class="text-[var(--text)]/50">单次真实成本</span>
              <span class="text-red-400">¥{{ recommendedStartPrice.realCostCNY.toFixed(4) }}</span>
            </div>
            <div v-if="recommendedStartPrice.breakEvenUSD" class="flex justify-between">
              <span class="text-[var(--text)]/50">面板保本价</span>
              <span class="text-yellow-400">${{ recommendedStartPrice.breakEvenUSD.toFixed(4) }}</span>
            </div>
          </div>
          <div class="mt-3 text-[10px] text-[var(--text)]/30 text-center">
            用户充值 1元 = $5面板额度 | 面板$1 = 你收 ¥0.20 | 1 Credit = ¥0.04
          </div>
        </div>

        <!-- 盈亏模拟 -->
        <div class="bg-gradient-to-br from-emerald-500/10 to-teal-500/10 border border-emerald-500/20 rounded-2xl p-6">
          <div class="text-sm font-black text-[var(--text)] mb-4">📊 单号生命周期盈亏模拟</div>
          <div v-if="profitSim" class="space-y-3">
            <div class="flex justify-between items-center bg-[var(--card)]/40 rounded-xl p-3">
              <span class="text-xs text-[var(--text)]/60">预计可服务请求数</span>
              <span class="text-lg font-black text-blue-400">{{ profitSim.estimatedTotalReqs.toLocaleString() }} 次</span>
            </div>
            <div class="flex justify-between items-center bg-[var(--card)]/40 rounded-xl p-3">
              <span class="text-xs text-[var(--text)]/60">预计面板总收入</span>
              <span class="text-lg font-black text-green-400">${{ profitSim.estimatedRevenue }}</span>
            </div>
            <div class="flex justify-between items-center bg-[var(--card)]/40 rounded-xl p-3">
              <span class="text-xs text-[var(--text)]/60">真实收入（CNY）</span>
              <span class="text-lg font-black text-emerald-400">¥{{ profitSim.realRevenueCNY }}</span>
            </div>
            <div class="flex justify-between items-center rounded-xl p-3" :class="parseFloat(profitSim.profitCNY) >= 0 ? 'bg-green-500/20' : 'bg-red-500/20'">
              <span class="text-xs font-bold" :class="parseFloat(profitSim.profitCNY) >= 0 ? 'text-green-400' : 'text-red-400'">
                {{ parseFloat(profitSim.profitCNY) >= 0 ? '✅ 预计利润' : '❌ 预计亏损' }}
              </span>
              <div class="text-right">
                <span class="text-xl font-black" :class="parseFloat(profitSim.profitCNY) >= 0 ? 'text-green-400' : 'text-red-400'">
                  ¥{{ profitSim.profitCNY }}
                </span>
                <span class="text-xs ml-1" :class="parseFloat(profitSim.profitCNY) >= 0 ? 'text-green-400/60' : 'text-red-400/60'">
                  ({{ profitSim.profitRate }}%)
                </span>
              </div>
            </div>
            <div class="text-[10px] text-[var(--text)]/30 text-center">
              基于：号成本 ¥{{ profitSim.accountCost }} / {{ CREDITS_PER_ACCOUNT }} Credits | 起步价 ${{ recommendedStartPrice.usd.toFixed(2) }}
            </div>
          </div>
          <div v-else class="text-center text-[var(--text)]/30 text-sm py-8">
            <AlertTriangle class="w-5 h-5 inline mr-2 text-yellow-500" />
            需要更多成功请求数据
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
