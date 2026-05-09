<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useToast } from '../composables/useToast'
import {
  Save, Plus, Trash2, ChevronDown, ChevronRight
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldInput from '../components/world/WorldInput.vue'

const auth = useAuthStore()
const { success, error: toastErr } = useToast()
const headers = () => ({ 'X-Admin-Password': auth.password, 'Content-Type': 'application/json' })

const tab = ref('config')
const tabOptions = [
  { value: 'config',    label: '售价配置' },
  { value: 'promotion', label: '活动门槛' },
]
const showCostBlock = ref(false) // 成本端配置折叠区

// Promotion state (v2: per-model 活动价 + 兜底)
const promotion = ref({
  enabled: false,
  name: '',
  modelPrices: {},          // v2: { 'claude-opus-4.7': 0.05, ... }
  defaultProPriceUSD: 0.01, // v2: 活动期 PRO 兜底
  defaultFreePriceUSD: 0.005, // v2: 活动期 FREE 兜底
  minMonthlyRechargeCNY: 0,
  minRecentCalls: 0,
  recentCallsDays: 7,
  whitelist: [],
  startTs: 0,
  endTs: 0,
})
const promotionPreview = ref(null) // 后端 buildPromotionPreview 返回的对照表
const savingPromo = ref(false)
const newWhitelistKeyID = ref('')

// 把日期 string 转 unix ts 方便编辑
const promoStartLocal = computed({
  get: () => promotion.value.startTs ? new Date(promotion.value.startTs * 1000).toISOString().slice(0, 16) : '',
  set: (v) => { promotion.value.startTs = v ? Math.floor(new Date(v).getTime() / 1000) : 0 }
})
const promoEndLocal = computed({
  get: () => promotion.value.endTs ? new Date(promotion.value.endTs * 1000).toISOString().slice(0, 16) : '',
  set: (v) => { promotion.value.endTs = v ? Math.floor(new Date(v).getTime() / 1000) : 0 }
})

// 池价格 hint
const _simplifyModels = (arr) => (arr || []).map(m => String(m).replace(/^claude-/, '')).join(' / ') || '—'
const freePoolHint = computed(() => `${_simplifyModels(pricing.value?.supportedModels?.free)} 使用此默认兜底价（未在下方表格单独配置时）`)
const proPoolHint  = computed(() => `${_simplifyModels(pricing.value?.supportedModels?.pro)} 使用此默认兜底价（未在下方表格单独配置时）`)

// v2 模型售价表（按 preview 渲染，每行可编辑 priceUSD）
const modelPriceRows = computed(() => pricing.value?.preview || [])
function updateModelPrice(model, priceUSD) {
  if (!pricing.value) return
  if (!pricing.value.modelPrices) pricing.value.modelPrices = {}
  const v = Number(priceUSD)
  if (!isFinite(v) || v <= 0) return
  pricing.value.modelPrices[String(model).toLowerCase()] = v
}
function removeModelPrice(model) {
  if (!pricing.value?.modelPrices) return
  delete pricing.value.modelPrices[String(model).toLowerCase()]
  // 同步清掉 preview 里这一行（避免 UI 展示残留），下次 fetchAll 会重建
  if (Array.isArray(pricing.value.preview)) {
    pricing.value.preview = pricing.value.preview.filter(r => r.model !== model)
  }
}
const newModelName = ref('')
const newModelPrice = ref(0.20)
function addCustomModel() {
  const m = (newModelName.value || '').trim().toLowerCase()
  const v = Number(newModelPrice.value)
  if (!m || !v || v <= 0) { toastErr('模型名 + 价格 必填'); return }
  if (!pricing.value.modelPrices) pricing.value.modelPrices = {}
  pricing.value.modelPrices[m] = v
  newModelName.value = ''
  newModelPrice.value = 0.20
}

// v2 模型活动价表（按 promotionPreview.rows 渲染）
const promoModelRows = computed(() => promotionPreview.value?.rows || [])
function updatePromoPrice(model, priceUSD) {
  if (!promotion.value.modelPrices) promotion.value.modelPrices = {}
  const v = Number(priceUSD)
  if (!isFinite(v) || v < 0) return
  if (v === 0) {
    delete promotion.value.modelPrices[String(model).toLowerCase()]
  } else {
    promotion.value.modelPrices[String(model).toLowerCase()] = v
  }
}
function clearPromoPrice(model) {
  if (!promotion.value.modelPrices) return
  delete promotion.value.modelPrices[String(model).toLowerCase()]
}

// keys 用于白名单选择（已在 fetchAll 加载到 keys.value）
const keysForWhitelist = computed(() => {
  return (keys.value || [])
    .map(k => ({
      id: k.id,
      label: `${(k.note || '?')} (${(k.id || '').slice(0, 8)})`,
    }))
    .sort((a, b) => a.label.localeCompare(b.label))
})

const whitelistDetailed = computed(() => {
  const map = {}
  for (const k of (keys.value || [])) map[k.id] = k.note || '?'
  return (promotion.value.whitelist || []).map(id => ({
    id,
    note: map[id] || '(unknown key)',
    short: id.slice(0, 8),
  }))
})

// Data state
const keys = ref([]) // 仅给活动门槛白名单选择器用
const pricing = ref(null)
const loading = ref(true)
const saving = ref(false)
let timer = null

// 采购成本明细表单（折叠区里）
const showProForm = ref(false)
const showFreeForm = ref(false)
const proForm = ref({ count: 1, costCNY: 60, credits: 1500 })
const freeForm = ref({ count: 100, costCNY: 9 })

async function fetchAll() {
  try {
    const [p3, p4, p5] = await Promise.all([
      fetch('/admin/api/apikeys',   { headers: headers() }),
      fetch('/admin/api/pricing',   { headers: headers() }),
      fetch('/admin/api/promotion', { headers: headers() }),
    ])
    if (p5.ok) {
      const pd = await p5.json()
      if (pd && typeof pd === 'object') {
        promotion.value = {
          enabled: !!pd.enabled,
          name: pd.name || '',
          modelPrices: pd.modelPrices || {},
          defaultProPriceUSD: pd.defaultProPriceUSD ?? 0.01,
          defaultFreePriceUSD: pd.defaultFreePriceUSD ?? 0.005,
          minMonthlyRechargeCNY: pd.minMonthlyRechargeCNY ?? 0,
          minRecentCalls: pd.minRecentCalls ?? 0,
          recentCallsDays: pd.recentCallsDays ?? 7,
          whitelist: Array.isArray(pd.whitelist) ? pd.whitelist : [],
          startTs: pd.startTs || 0,
          endTs: pd.endTs || 0,
        }
      }
    }
    if (p3.ok) keys.value = await p3.json()
    if (p4.ok) {
      const data = await p4.json()
      pricing.value = {
        // v2 主字段
        modelPrices:         data.modelPrices         || {},
        defaultProPriceUSD:  data.defaultProPriceUSD  ?? 0.20,
        defaultFreePriceUSD: data.defaultFreePriceUSD ?? 0.04,
        // 后端 preview 数组（直接渲染表格用）
        preview:             data.preview             || [],
        // 成本端
        purchasePriceCNY:    data.purchasePriceCNY    || 0.04,
        proCostEntries:      data.proCostEntries      || [],
        freeCostEntries:     data.freeCostEntries     || [],
        // 元数据
        supportedModels:     data.supportedModels     || {},
        // v1 deprecated 字段（保留 JSON 往返不丢）
        freePoolPriceUSD:    data.freePoolPriceUSD    || 0,
        proPoolPriceUSD:     data.proPoolPriceUSD     || 0,
        modelMultipliers:    data.modelMultipliers    || {},
      }
      promotionPreview.value = data.promotionPreview || null
    }
  } catch (e) { console.error(e) }
  finally { loading.value = false }
}

async function savePromotion() {
  if (promotion.value.enabled && !confirm('确认启用活动门槛？\n\n开启后符合资格（白名单 / 充值 / 活跃度）的 key 将享受活动价。请确认价格和门槛设置无误。')) return
  savingPromo.value = true
  try {
    const res = await fetch('/admin/api/promotion', {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify(promotion.value),
    })
    if (res.ok) { success('活动配置已保存'); fetchAll() }
    else toastErr('保存失败')
  } catch { toastErr('网络错误') }
  savingPromo.value = false
}

async function addWhitelist() {
  const kid = (newWhitelistKeyID.value || '').trim()
  if (!kid) return
  try {
    const res = await fetch('/admin/api/promotion/whitelist', {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ keyID: kid }),
    })
    if (res.ok) { success('已加入白名单'); newWhitelistKeyID.value = ''; fetchAll() }
    else toastErr('添加失败')
  } catch { toastErr('网络错误') }
}

async function removeWhitelist(kid) {
  if (!confirm(`确认从白名单移除 ${kid.slice(0, 8)}？`)) return
  try {
    const res = await fetch(`/admin/api/promotion/whitelist/${kid}`, {
      method: 'DELETE',
      headers: headers(),
    })
    if (res.ok) { success('已移除'); fetchAll() }
    else toastErr('删除失败')
  } catch { toastErr('网络错误') }
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

function fmtDate(ts) {
  if (!ts) return ''
  return new Date(ts * 1000).toLocaleDateString('zh-CN')
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

    <template v-else-if="tab === 'config'">

      <!-- 模型售价表 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>模型售价表（按模型单独定价）</h3>
          <WorldButton variant="primary" size="sm" :loading="saving" @click="savePricing">
            <Save :size="14" /><span>保存</span>
          </WorldButton>
        </header>
        <p class="section-hint">
          每个模型在此显式定 USD/credit 单价。未列出的模型走上方默认兜底价。
          <br>
          ⚠️ 匹配按模型小写名 + 自动兼容 <code>-/.</code> 互换（<code>opus-4.7 ↔ opus-4-7</code>）。
          stealth 替换跨模型时按 originalModel 计费 — 想让 stealth 目标 model 也享同样价，需在此表加同价的一条。
        </p>

        <div class="model-price-table">
          <div class="mpt-head">
            <span style="flex: 2.2; min-width: 180px;">模型</span>
            <span style="flex: 0.6; min-width: 50px;">池</span>
            <span style="flex: 1.2; min-width: 110px;">单价 ($/cr)</span>
            <span style="flex: 1.0; min-width: 80px;">折合 ¥</span>
            <span style="flex: 1.0; min-width: 90px;">成本 ¥</span>
            <span style="flex: 1.0; min-width: 70px;">利润率</span>
            <span style="flex: 0; width: 32px;"></span>
          </div>
          <div v-for="r in modelPriceRows" :key="r.model" class="mpt-row" :class="{ 'is-default': r.isDefault }">
            <span class="model-cell">
              <code>{{ r.model }}</code>
              <span v-if="r.isDefault" class="badge-tip">兜底</span>
            </span>
            <span class="dim" style="flex: 0.6; min-width: 50px; text-transform: uppercase;">{{ r.pool }}</span>
            <span style="flex: 1.2; min-width: 110px;">
              <input
                type="number" step="0.001" min="0"
                class="mpt-input"
                :value="r.priceUSD"
                @change="e => updateModelPrice(r.model, e.target.value)"
              />
            </span>
            <span class="dim" style="flex: 1.0; min-width: 80px;">¥{{ r.priceCNYPerCredit.toFixed(4) }}</span>
            <span class="dim" style="flex: 1.0; min-width: 90px;">
              <span v-if="r.costIsFallback" title="未填进货明细，无法算真实成本">—</span>
              <template v-else>¥{{ r.costCNYPerCredit.toFixed(4) }}</template>
            </span>
            <span style="flex: 1.0; min-width: 70px;"
                  :class="r.costIsFallback ? 'dim' : (r.marginPercent >= 0 ? 'margin-good' : 'margin-bad')">
              <span v-if="r.costIsFallback" title="未填进货明细，无法算利润率">—</span>
              <template v-else>{{ r.marginPercent.toFixed(1) }}%</template>
            </span>
            <button v-if="!r.isDefault" class="del-btn" style="flex: 0; width: 32px;" @click="removeModelPrice(r.model)" aria-label="删除">
              <Trash2 :size="12" />
            </button>
            <span v-else style="flex: 0; width: 32px;"></span>
          </div>
          <div v-if="!modelPriceRows.length" class="entry-empty">无模型数据（请检查 supportedModels 是否注入）</div>
        </div>

        <div class="mult-form" style="margin-top: 14px;">
          <WorldInput v-model="newModelName" placeholder="如：claude-opus-4.7-thinking（自定义/外部）" label="自定义模型名" size="sm" style="flex: 2;" />
          <WorldInput v-model.number="newModelPrice" type="number" step="0.001" label="单价 ($/credit)" size="sm" style="flex: 1;" />
          <WorldButton variant="primary" size="sm" @click="addCustomModel">
            <Plus :size="13" /><span>添加</span>
          </WorldButton>
        </div>
      </WorldCard>

      <!-- Shadow 校验提示（迁移健康检查）-->
      <WorldCard v-if="modelPriceRows.some(r => !r.legacyEqualsActual)" padding="md">
        <header class="section-head">
          <h3 style="color: #f59e0b;">⚠️ 迁移校验：检测到新旧公式不一致</h3>
        </header>
        <p class="section-hint">
          下面这些模型的 v2 价格 跟 v1 公式（pool_price × multiplier）算出的价格不一致 — 可能 admin 手动改过 ModelPrices，或者迁移时遇到了边界 case。
          如果这是 admin 主动调整的结果（在 v1 基础上想给某 model 单独提价/降价），可以忽略此提示。
        </p>
        <div class="entry-list">
          <div v-for="r in modelPriceRows.filter(x => !x.legacyEqualsActual)" :key="r.model" class="entry-row">
            <code style="flex: 2;">{{ r.model }}</code>
            <span class="dim">v2: ${{ r.priceUSD.toFixed(4) }}</span>
            <span class="dim">vs v1: ${{ r.legacyPriceUSD.toFixed(4) }}</span>
            <span style="color: #f59e0b; font-weight: 800;">差 ${{ Math.abs(r.priceUSD - r.legacyPriceUSD).toFixed(4) }}</span>
          </div>
        </div>
      </WorldCard>

      <!-- 成本端配置（折叠区，默认收起）-->
      <WorldCard padding="md">
        <header class="section-head" style="cursor: pointer;" @click="showCostBlock = !showCostBlock">
          <h3>
            <component :is="showCostBlock ? ChevronDown : ChevronRight" :size="16" style="vertical-align: -2px;" />
            成本端配置（采购价 + 进货明细）
          </h3>
          <WorldChip size="sm" variant="default">
            PRO 平均 ¥{{ proSummary.avgCostPerCredit }}/cr · {{ proSummary.totalCount }} 个号
          </WorldChip>
        </header>

        <div v-if="showCostBlock">
          <p class="section-hint">
            这里管<strong>账号采购成本</strong>（admin 买号花了多少钱），跟"卖给用户的价"无关。
            用于在「模型售价表」的"利润率"列计算利润 = 售价 - 进货成本。
          </p>

          <div class="cfg-grid" style="margin-bottom: 16px;">
            <WorldInput
              v-model.number="pricing.purchasePriceCNY"
              type="number" step="0.001"
              label="PRO 进货价 (¥/credit) — 兜底"
              hint="如果下方采购明细为空，利润计算会回落到这个值"
            />
          </div>

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
        </div>
      </WorldCard>
    </template>

    <template v-else-if="tab === 'promotion'">
      <!-- 活动门槛配置 -->
      <WorldCard padding="lg">
        <header class="section-head">
          <h3>活动门槛设置</h3>
          <WorldButton variant="primary" :loading="savingPromo" @click="savePromotion">
            <Save :size="14" /><span>保存</span>
          </WorldButton>
        </header>
        <p class="section-hint">
          开启后，<strong>满足任一条件</strong>的 key 才能享受活动价：
          ① 在白名单 ② 本月充值 ≥ 阈值 ③ 过去 N 天调用 ≥ 阈值。
          阈值设为 0 = 该条件不启用。
        </p>

        <div class="promo-toggle">
          <label class="promo-switch">
            <input type="checkbox" v-model="promotion.enabled" />
            <span>启用活动</span>
          </label>
          <WorldChip v-if="promotion.enabled" variant="success" :dot="true" size="sm">已启用</WorldChip>
          <WorldChip v-else variant="default" size="sm">未启用</WorldChip>
        </div>

        <div class="cfg-grid">
          <WorldInput v-model="promotion.name" label="活动名称" hint="比如：五一骨折特惠" />
          <WorldInput
            v-model.number="promotion.defaultProPriceUSD"
            type="number" step="0.001"
            label="活动期 PRO 默认兜底 ($/credit)"
            hint="未在下方表格单独配置的 PRO 模型用此价"
          />
          <WorldInput
            v-model.number="promotion.defaultFreePriceUSD"
            type="number" step="0.001"
            label="活动期 FREE 默认兜底 ($/credit)"
            hint="未在下方表格单独配置的 FREE 模型用此价"
          />
        </div>
      </WorldCard>

      <!-- 模型活动价表 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>模型活动价表（每个模型可单独定价）</h3>
        </header>
        <p class="section-hint">
          每行单独配置某个模型的活动价；留空（=0）= 走上方默认兜底价。
          表格右侧实时显示<strong>原价 → 活动价 → 真实折扣%</strong>，避免 v1 时代"池一刀切但 multiplier 还在叠加"的混淆。
        </p>

        <div class="model-price-table">
          <div class="mpt-head">
            <span style="flex: 2.2; min-width: 180px;">模型</span>
            <span style="flex: 0.6; min-width: 50px;">池</span>
            <span style="flex: 1.2; min-width: 110px;">原价 ($/cr)</span>
            <span style="flex: 1.4; min-width: 130px;">活动价 ($/cr)</span>
            <span style="flex: 1.0; min-width: 80px;">真实折扣</span>
            <span style="flex: 0; width: 32px;"></span>
          </div>
          <div v-for="r in promoModelRows" :key="r.model" class="mpt-row" :class="{ 'is-default': r.isPromoDefault }">
            <span class="model-cell">
              <code>{{ r.model }}</code>
              <span v-if="r.isPromoDefault" class="badge-tip">兜底</span>
            </span>
            <span class="dim" style="flex: 0.6; min-width: 50px; text-transform: uppercase;">{{ r.pool }}</span>
            <span class="dim" style="flex: 1.2; min-width: 110px;">${{ r.originalUSD.toFixed(4) }}</span>
            <span style="flex: 1.4; min-width: 130px;">
              <input
                type="number" step="0.001" min="0"
                class="mpt-input"
                :value="r.isPromoDefault ? '' : r.promoUSD"
                :placeholder="`兜底 $${r.promoUSD.toFixed(4)}`"
                @change="e => updatePromoPrice(r.model, e.target.value)"
              />
            </span>
            <span :class="r.discountPercent > 0 ? 'margin-good' : 'dim'" style="flex: 1.0; min-width: 80px; font-weight: 800;">
              {{ r.discountPercent.toFixed(1) }}% off
            </span>
            <button v-if="!r.isPromoDefault" class="del-btn" style="flex: 0; width: 32px;" @click="clearPromoPrice(r.model)" aria-label="移除单独定价"><Trash2 :size="12" /></button>
            <span v-else style="flex: 0; width: 32px;"></span>
          </div>
          <div v-if="!promoModelRows.length" class="entry-empty">无模型数据</div>
        </div>
      </WorldCard>

      <!-- 资格条件 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>资格条件（OR 关系）</h3>
        </header>
        <div class="cfg-grid">
          <WorldInput
            v-model.number="promotion.minMonthlyRechargeCNY"
            type="number"
            label="① 本月充值 ≥ ¥"
            hint="0 = 不启用此条件"
          />
          <WorldInput
            v-model.number="promotion.minRecentCalls"
            type="number"
            label="② 调用次数 ≥"
            hint="0 = 不启用此条件"
          />
          <WorldInput
            v-model.number="promotion.recentCallsDays"
            type="number"
            label="② 调用统计窗口 (天)"
            hint="默认 7 天"
          />
        </div>
      </WorldCard>

      <!-- 时间窗 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>活动时间窗</h3>
          <span class="section-hint" style="margin: 0;">留空 = 不限</span>
        </header>
        <div class="cfg-grid">
          <div class="cfg-item">
            <label class="cfg-label">开始时间</label>
            <input type="datetime-local" class="dt-input" v-model="promoStartLocal" />
          </div>
          <div class="cfg-item">
            <label class="cfg-label">结束时间</label>
            <input type="datetime-local" class="dt-input" v-model="promoEndLocal" />
          </div>
        </div>
      </WorldCard>

      <!-- 白名单 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>③ 白名单（{{ whitelistDetailed.length }} 个 key）</h3>
        </header>
        <p class="section-hint">直接通过资格判定，无视充值额/活跃度门槛。适合"熟人 / 内部测试号"。</p>

        <div class="whitelist-form">
          <select v-model="newWhitelistKeyID" class="dt-input">
            <option value="">— 选择 key 加入白名单 —</option>
            <option v-for="k in keysForWhitelist" :key="k.id" :value="k.id">{{ k.label }}</option>
          </select>
          <WorldButton variant="primary" size="sm" :disabled="!newWhitelistKeyID" @click="addWhitelist">
            <Plus :size="13" /><span>加入</span>
          </WorldButton>
        </div>

        <div class="entry-list" style="margin-top: 12px;">
          <div v-for="w in whitelistDetailed" :key="w.id" class="entry-row">
            <span class="dim">{{ w.short }}</span>
            <span>{{ w.note }}</span>
            <button class="del-btn" @click="removeWhitelist(w.id)" aria-label="删除"><Trash2 :size="12" /></button>
          </div>
          <div v-if="!whitelistDetailed.length" class="entry-empty">白名单为空</div>
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

/* ==================== v2 模型售价表 ==================== */
.model-price-table {
  border: 1px solid var(--world-divider);
  border-radius: 6px;
  overflow: hidden;
  background: var(--world-bg-secondary, rgba(0,0,0,0.04));
}
.mpt-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: var(--world-bg-tertiary, rgba(255,255,255,0.04));
  border-bottom: 1px solid var(--world-divider);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.mpt-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--world-divider);
  font-size: 0.85rem;
  transition: background 0.15s;
}
.mpt-row:last-child { border-bottom: none; }
.mpt-row:hover { background: var(--world-bg-tertiary, rgba(255,255,255,0.03)); }
.mpt-row.is-default { opacity: 0.78; }

.model-cell {
  flex: 2.2;
  min-width: 180px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.model-cell code {
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-size: 0.82rem;
  color: var(--world-text-primary);
}
.badge-tip {
  font-size: 0.65rem;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 3px;
  background: rgba(120, 120, 120, 0.18);
  color: var(--world-text-mute);
  letter-spacing: 0.05em;
}

.mpt-input {
  width: 100%;
  padding: 5px 8px;
  border: 1px solid var(--world-divider);
  border-radius: 4px;
  background: var(--world-bg-input, transparent);
  color: var(--world-text-primary);
  font-size: 0.85rem;
  font-family: var(--world-font-mono, ui-monospace, monospace);
}
.mpt-input:focus {
  outline: none;
  border-color: var(--world-accent);
}

.margin-good { color: #16a34a; font-weight: 800; }
.margin-bad  { color: #dc2626; font-weight: 800; }


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

/* Promotion */
.promo-toggle { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; }
.promo-switch { display: inline-flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; }
.promo-switch input { width: 18px; height: 18px; cursor: pointer; }
.promo-switch span { font-size: 0.875rem; font-weight: 700; color: var(--world-text-primary); }

.cfg-item { display: flex; flex-direction: column; gap: 6px; }
.cfg-label { font-size: 0.75rem; font-weight: 700; color: var(--world-text-mute); }
.dt-input {
  width: 100%;
  padding: 8px 10px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-divider);
  border-radius: var(--world-radius-sm);
  color: var(--world-text-primary);
  font-size: 0.875rem;
  font-family: var(--world-font-mono);
}
.dt-input:focus { outline: none; border-color: var(--world-accent); }

.whitelist-form { display: flex; gap: 10px; align-items: center; }
.whitelist-form .dt-input { flex: 1 1 auto; }

/* Model multiplier */
.mult-form {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 12px;
  background: var(--world-overlay-light);
  border-radius: var(--world-radius-md);
  flex-wrap: wrap;
}
</style>
