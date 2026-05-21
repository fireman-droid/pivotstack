<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import {
  Save, Plus, Trash2, AlertTriangle
} from 'lucide-vue-next'
import { listChannels, getSellPrices, updateSellPrices, updateChannel } from '../api/admin'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldSelect from '../components/world/WorldSelect.vue'
import WorldDatePicker from '../components/world/WorldDatePicker.vue'
import WorldCheckbox from '../components/world/WorldCheckbox.vue'

const { success, error: toastErr } = useToast()

const tab = ref('config')
const tabOptions = [
  { value: 'config',    label: '售价配置' },
  { value: 'promotion', label: '活动门槛' },
]

// === v3 渠道定价 (售价 + 成本) ===
const channels = ref([])
const channelLocalChanges = ref({}) // { "channelId|model": { inputPerM?, outputPerM?, costInputPerM?, costOutputPerM? } }
const savingChannelId = ref(null)
// 「+ 添加定价」inline 表单状态
const addingForChannel = ref(null) // 当前展开的 channel id
const addingModel = ref('')
const addingInputPerM = ref(0)
const addingOutputPerM = ref(0)
const addingCostInput = ref(0)
const addingCostOutput = ref(0)

// 拉渠道列表
async function fetchSellAndChannels() {
  try {
    channels.value = (await listChannels()) || []
  } catch (e) {
    console.error('fetch channels:', e)
  }
}

// 工具：(channelId, model) → 唯一 key
function lcKey(channelId, model) {
  return `${channelId}|${String(model).toLowerCase()}`
}

// 渠道下已定价模型（按 channel.modelPrices key 排序）
function channelPricedRows(ch) {
  const prices = ch.modelPrices || {}
  return Object.keys(prices).map(model => ({ model, price: prices[model] }))
}

// 渠道下还没定价的模型（用于「+ 添加定价」下拉候选）
function channelUnpricedModels(ch) {
  const prices = ch.modelPrices || {}
  const pricedKeys = new Set(Object.keys(prices).map(k => k.toLowerCase()))
  return (ch.models || []).filter(m => !pricedKeys.has(String(m).toLowerCase()))
}

// 取某 (channel, model) 某字段的当前值（local change 优先）
function getPriceField(channelId, model, field) {
  const key = lcKey(channelId, model)
  const lc = channelLocalChanges.value[key]
  if (lc && lc[field] != null) return lc[field]
  const ch = channels.value.find(c => c.id === channelId)
  const original = ch?.modelPrices?.[model] || ch?.modelPrices?.[String(model).toLowerCase()]
  return original ? (original[field] || 0) : 0
}

// 写某 (channel, model) 某字段
function setPriceField(channelId, model, field, val) {
  const v = Number(val)
  if (!isFinite(v) || v < 0) return
  const key = lcKey(channelId, model)
  const existing = channelLocalChanges.value[key] || {}
  channelLocalChanges.value = {
    ...channelLocalChanges.value,
    [key]: { ...existing, [field]: v },
  }
}

// 该 channel 下有多少 local changes
function channelChangeCount(channelId) {
  const prefix = `${channelId}|`
  return Object.keys(channelLocalChanges.value).filter(k => k.startsWith(prefix)).length
}
function hasChannelChanges(channelId) {
  return channelChangeCount(channelId) > 0
}

// 利润率展示
function profitRate(channelId, model) {
  const inP = Number(getPriceField(channelId, model, 'inputPerM')) || 0
  const outP = Number(getPriceField(channelId, model, 'outputPerM')) || 0
  const inC = Number(getPriceField(channelId, model, 'costInputPerM')) || 0
  const outC = Number(getPriceField(channelId, model, 'costOutputPerM')) || 0
  if (inC + outC === 0) return '—'  // 没填成本
  if (inP + outP === 0) return '—'
  // 简化：假设 1:1 input/output 比例（admin 看大概；精确利润看日志）
  const totalP = inP + outP
  const totalC = inC + outC
  if (totalP <= 0) return '—'
  const rate = ((totalP - totalC) / totalP) * 100
  return rate.toFixed(1) + '%'
}
function profitClass(channelId, model) {
  const txt = profitRate(channelId, model)
  if (txt === '—') return 'profit-na'
  const rate = parseFloat(txt)
  if (rate >= 50) return 'profit-good'
  if (rate >= 0) return 'profit-ok'
  return 'profit-bad'
}

// 保存该渠道的所有 local changes
async function saveChannelPrices(ch) {
  const prefix = `${ch.id}|`
  const changes = {}
  for (const [k, v] of Object.entries(channelLocalChanges.value)) {
    if (!k.startsWith(prefix)) continue
    const model = k.slice(prefix.length)
    changes[model] = v
  }
  if (Object.keys(changes).length === 0) return
  savingChannelId.value = ch.id
  try {
    const newPrices = { ...(ch.modelPrices || {}) }
    for (const [model, fields] of Object.entries(changes)) {
      const cur = newPrices[model] || newPrices[model.toLowerCase()] || {}
      newPrices[model.toLowerCase()] = {
        inputPerM: Number(fields.inputPerM ?? cur.inputPerM) || 0,
        outputPerM: Number(fields.outputPerM ?? cur.outputPerM) || 0,
        costInputPerM: Number(fields.costInputPerM ?? cur.costInputPerM) || 0,
        costOutputPerM: Number(fields.costOutputPerM ?? cur.costOutputPerM) || 0,
      }
    }
    await updateChannel(ch.id, {
      id: ch.id,
      type: ch.type,
      baseUrl: ch.baseUrl,
      apiKey: '',
      models: ch.models,
      modelPrices: newPrices,
      enabled: ch.enabled,
    })
    success(`${ch.id} 已保存 ${Object.keys(changes).length} 处改动`)
    // 清掉该 channel 的 local changes
    const next = { ...channelLocalChanges.value }
    for (const k of Object.keys(next)) if (k.startsWith(prefix)) delete next[k]
    channelLocalChanges.value = next
    await fetchSellAndChannels()
  } catch (e) {
    toastErr(e.message || '保存失败')
  } finally {
    savingChannelId.value = null
  }
}

// 「+ 添加定价」点击 → 展开 inline 表单
function openAddPrice(ch) {
  addingForChannel.value = ch.id
  addingModel.value = ''
  addingInputPerM.value = 0
  addingOutputPerM.value = 0
  addingCostInput.value = 0
  addingCostOutput.value = 0
}
function cancelAddPrice() {
  addingForChannel.value = null
}
async function confirmAddPrice(ch) {
  const m = (addingModel.value || '').trim()
  if (!m) { toastErr('请选择模型'); return }
  const inP = Number(addingInputPerM.value) || 0
  const outP = Number(addingOutputPerM.value) || 0
  if (inP === 0 && outP === 0) { toastErr('输入/输出售价至少一个 > 0'); return }
  savingChannelId.value = ch.id
  try {
    const newPrices = { ...(ch.modelPrices || {}) }
    newPrices[m.toLowerCase()] = {
      inputPerM: inP,
      outputPerM: outP,
      costInputPerM: Number(addingCostInput.value) || 0,
      costOutputPerM: Number(addingCostOutput.value) || 0,
    }
    await updateChannel(ch.id, {
      id: ch.id,
      type: ch.type,
      baseUrl: ch.baseUrl,
      apiKey: '',
      models: ch.models,
      modelPrices: newPrices,
      enabled: ch.enabled,
    })
    success(`已添加 ${m} 定价`)
    addingForChannel.value = null
    await fetchSellAndChannels()
  } catch (e) {
    toastErr(e.message || '添加失败')
  } finally {
    savingChannelId.value = null
  }
}

async function deleteChannelPrice(ch, model) {
  if (!confirm(`删除渠道 "${ch.id}" 的 "${model}" 定价？\n删除后该模型调用会返回 sell_price_missing 错误（fail closed）`)) return
  savingChannelId.value = ch.id
  try {
    const newPrices = { ...(ch.modelPrices || {}) }
    delete newPrices[model]
    delete newPrices[model.toLowerCase()]
    await updateChannel(ch.id, {
      id: ch.id,
      type: ch.type,
      baseUrl: ch.baseUrl,
      apiKey: '',
      models: ch.models,
      modelPrices: newPrices,
      enabled: ch.enabled,
    })
    success(`已删除 ${model} 定价`)
    await fetchSellAndChannels()
  } catch (e) {
    toastErr(e.message || '删除失败')
  } finally {
    savingChannelId.value = null
  }
}

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
// WorldSelect options 形态
const whitelistSelectOptions = computed(() =>
  keysForWhitelist.value.map(k => ({ value: k.id, label: k.label }))
)

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

async function fetchAll() {
  try {
    const [p3, p4, p5] = await Promise.all([
      api('/apikeys'),
      api('/pricing'),
      api('/promotion'),
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
    await api('/promotion', { method: 'PUT', body: JSON.stringify(promotion.value) })
    success('活动配置已保存')
    fetchAll()
  } catch { toastErr('网络错误') }
  savingPromo.value = false
}

async function addWhitelist() {
  const kid = (newWhitelistKeyID.value || '').trim()
  if (!kid) return
  try {
    await api('/promotion/whitelist', { method: 'POST', body: JSON.stringify({ keyID: kid }) })
    success('已加入白名单')
    newWhitelistKeyID.value = ''
    fetchAll()
  } catch { toastErr('网络错误') }
}

async function removeWhitelist(kid) {
  if (!confirm(`确认从白名单移除 ${kid.slice(0, 8)}？`)) return
  try {
    await api(`/promotion/whitelist/${kid}`, { method: 'DELETE' })
    success('已移除')
    fetchAll()
  } catch { toastErr('网络错误') }
}

async function savePricing() {
  if (!pricing.value) return
  saving.value = true
  try {
    await api('/pricing', { method: 'PUT', body: JSON.stringify(pricing.value) })
    success('定价已保存')
    fetchAll()
  } catch { toastErr('网络错误') }
  saving.value = false
}

onMounted(() => {
  fetchAll()
  fetchSellAndChannels()
  timer = setInterval(() => {
    fetchAll()
    fetchSellAndChannels()
  }, 30000)
})
onUnmounted(() => clearInterval(timer))
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
      <p class="config-hint">
        💡 每个渠道独立定价：售价是用户扣费、成本是 admin 追踪进货。利润率 = (售价 - 成本) / 售价。
        单位 <code>虚拟$/1M token</code>，1 USD = 20 虚拟$ = 1 ¥。
      </p>

      <div v-if="!channels.length" class="empty-state">
        <p>还没配置任何渠道。请先到「渠道」页新建。</p>
      </div>

      <WorldCard v-for="ch in channels" :key="ch.id" padding="md" class="channel-pricing-card">
        <header class="section-head">
          <div class="channel-title">
            <WorldChip :variant="ch.type === 'kiro' ? 'info' : 'success'" size="sm">
              {{ ch.type === 'kiro' ? '🔵' : '🟢' }} {{ ch.type }}
            </WorldChip>
            <h3>{{ ch.id }}</h3>
            <span v-if="!ch.enabled" class="dim">（已禁用）</span>
          </div>
          <div class="channel-actions">
            <WorldButton
              v-if="channelUnpricedModels(ch).length > 0"
              variant="ghost" size="sm" @click="openAddPrice(ch)"
            >
              <Plus :size="13" /><span>添加定价</span>
            </WorldButton>
            <WorldButton
              variant="primary" size="sm"
              :loading="savingChannelId === ch.id"
              :disabled="!hasChannelChanges(ch.id)"
              @click="saveChannelPrices(ch)"
            >
              <Save :size="14" />
              <span>{{ channelChangeCount(ch.id) ? `保存 (${channelChangeCount(ch.id)})` : '无改动' }}</span>
            </WorldButton>
          </div>
        </header>

        <div class="cp-rows">
          <div class="cp-head">
            <span class="model-col">模型</span>
            <span class="num-col">售价 in $/M</span>
            <span class="num-col">售价 out $/M</span>
            <span class="num-col">成本 in $/M</span>
            <span class="num-col">成本 out $/M</span>
            <span class="profit-col">利润率</span>
            <span class="action-col"></span>
          </div>
          <div v-for="row in channelPricedRows(ch)" :key="row.model" class="cp-row">
            <code class="model-col">{{ row.model }}</code>
            <input type="number" step="0.1" min="0" class="cp-input"
              :value="getPriceField(ch.id, row.model, 'inputPerM')"
              @input="e => setPriceField(ch.id, row.model, 'inputPerM', e.target.value)" />
            <input type="number" step="0.1" min="0" class="cp-input"
              :value="getPriceField(ch.id, row.model, 'outputPerM')"
              @input="e => setPriceField(ch.id, row.model, 'outputPerM', e.target.value)" />
            <input type="number" step="0.1" min="0" class="cp-input cost-input"
              :value="getPriceField(ch.id, row.model, 'costInputPerM')"
              @input="e => setPriceField(ch.id, row.model, 'costInputPerM', e.target.value)" />
            <input type="number" step="0.1" min="0" class="cp-input cost-input"
              :value="getPriceField(ch.id, row.model, 'costOutputPerM')"
              @input="e => setPriceField(ch.id, row.model, 'costOutputPerM', e.target.value)" />
            <span class="profit-col" :class="profitClass(ch.id, row.model)">
              {{ profitRate(ch.id, row.model) }}
            </span>
            <button class="del-btn action-col"
              @click="deleteChannelPrice(ch, row.model)" aria-label="删除定价">
              <Trash2 :size="12" />
            </button>
          </div>
          <div v-if="!channelPricedRows(ch).length" class="entry-empty">
            该渠道下还没有任何模型定价。点击右上「+ 添加定价」配置。
          </div>
        </div>

        <!-- Inline "添加定价" 下拉，点添加按钮后展开 -->
        <div v-if="addingForChannel === ch.id" class="add-price-row">
          <WorldSelect
            v-model="addingModel"
            :options="channelUnpricedModels(ch).map(m => ({ value: m, label: m }))"
            placeholder="选择要定价的模型"
            searchable
            size="sm"
          />
          <input type="number" step="0.1" min="0" class="cp-input"
            v-model.number="addingInputPerM" placeholder="售价 in" />
          <input type="number" step="0.1" min="0" class="cp-input"
            v-model.number="addingOutputPerM" placeholder="售价 out" />
          <input type="number" step="0.1" min="0" class="cp-input cost-input"
            v-model.number="addingCostInput" placeholder="成本 in（选填）" />
          <input type="number" step="0.1" min="0" class="cp-input cost-input"
            v-model.number="addingCostOutput" placeholder="成本 out（选填）" />
          <WorldButton variant="primary" size="sm" @click="confirmAddPrice(ch)">确认</WorldButton>
          <WorldButton variant="ghost" size="sm" @click="cancelAddPrice">取消</WorldButton>
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
          <WorldCheckbox v-model="promotion.enabled" label="启用活动" />
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
            <WorldDatePicker v-model="promoStartLocal" mode="datetime" size="md" placeholder="不限" />
          </div>
          <div class="cfg-item">
            <label class="cfg-label">结束时间</label>
            <WorldDatePicker v-model="promoEndLocal" mode="datetime" size="md" placeholder="不限" />
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
          <WorldSelect
            v-model="newWhitelistKeyID"
            :options="whitelistSelectOptions"
            size="md"
            :searchable="true"
            placeholder="选择 key 加入白名单"
            class="whitelist-select"
          />
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

/* ==================== v3 按渠道分组的定价卡片 ==================== */
.config-hint {
  background: rgba(59, 130, 246, 0.08);
  border-left: 3px solid #3b82f6;
  padding: 10px 14px;
  border-radius: 6px;
  font-size: 0.85rem;
  color: var(--world-text-mute);
  margin: 0;
}
.config-hint code { font-family: var(--world-font-mono, monospace); }
.empty-state {
  padding: 40px 20px;
  text-align: center;
  color: var(--world-text-mute);
}
.channel-pricing-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.channel-title {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.channel-title h3 {
  margin: 0;
  font-family: var(--world-font-mono, monospace);
  font-size: 1rem;
}
.channel-title .dim { color: var(--world-text-mute); font-size: 0.85rem; }
.channel-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.cp-rows {
  display: flex;
  flex-direction: column;
  border-radius: 6px;
  overflow-x: auto;
  border: 1px solid var(--world-divider, rgba(255,255,255,0.06));
}
.cp-head, .cp-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  font-size: 0.85rem;
}
.cp-head {
  background: var(--world-bg-tertiary, rgba(255,255,255,0.03));
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--world-text-mute);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--world-divider, rgba(255,255,255,0.06));
}
.cp-row {
  border-bottom: 1px solid var(--world-divider, rgba(255,255,255,0.04));
}
.cp-row:last-child { border-bottom: none; }
.cp-row:hover { background: var(--world-bg-tertiary, rgba(255,255,255,0.03)); }

.cp-row .model-col,
.cp-head .model-col {
  flex: 1.6;
  min-width: 140px;
  font-family: var(--world-font-mono, monospace);
}
.num-col { flex: 1; min-width: 84px; text-align: center; }
.profit-col { flex: 0.8; min-width: 70px; text-align: center; font-weight: 700; }
.action-col { flex: 0; width: 32px; }

.cp-input {
  flex: 1;
  min-width: 70px;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid var(--world-divider, rgba(0,0,0,0.1));
  background: var(--world-bg-secondary, transparent);
  color: inherit;
  font-family: var(--world-font-mono, monospace);
  font-size: 0.85rem;
  text-align: right;
}
.cp-input:focus { outline: 2px solid var(--world-accent, #06f); outline-offset: -1px; }
.cp-input.cost-input {
  border-style: dashed;
  opacity: 0.85;
}

.profit-good { color: #22c55e; }
.profit-ok { color: #f59e0b; }
.profit-bad { color: #ef4444; }
.profit-na { color: var(--world-text-mute); font-weight: normal; }

.add-price-row {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 10px;
  margin-top: 8px;
  background: var(--world-bg-tertiary, rgba(59, 130, 246, 0.04));
  border: 1px dashed var(--world-accent, #3b82f6);
  border-radius: 6px;
  flex-wrap: wrap;
}
.add-price-row > :first-child { min-width: 200px; flex: 1.5; }

.del-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--world-text-mute);
  padding: 4px;
  border-radius: 4px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.del-btn:hover { color: #ef4444; background: rgba(239, 68, 68, 0.1); }

.entry-empty {
  padding: 20px;
  text-align: center;
  color: var(--world-text-mute);
  font-size: 0.85rem;
}

/* ==================== v3 Token 售价表（已删除，保留 anchor 给 CSS 顺序） ==================== */
.sell-price-table {
  display: flex;
  flex-direction: column;
  border-radius: 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  border: 1px solid var(--world-divider, rgba(255,255,255,0.06));
}
.sp-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--world-bg-tertiary, rgba(255,255,255,0.03));
  font-size: 0.75rem;
  color: var(--world-text-mute);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 700;
  border-bottom: 1px solid var(--world-divider, rgba(255,255,255,0.06));
}
.sp-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--world-divider, rgba(255,255,255,0.04));
  font-size: 0.85rem;
}
.sp-row:last-child { border-bottom: none; }
.sp-row:hover { background: var(--world-bg-tertiary, rgba(255,255,255,0.03)); }
.sp-row.is-missing { background: rgba(245, 158, 11, 0.06); }
.sp-row .model-cell { flex: 2; min-width: 160px; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.sp-row .channel-cell { flex: 1; min-width: 100px; }
.sp-row .dim { color: var(--world-text-mute); font-family: var(--world-font-mono, monospace); }

.legacy-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  margin: 4px 0;
}
.legacy-divider .line {
  flex: 1;
  height: 1px;
  background: var(--world-divider, rgba(255,255,255,0.08));
}
.legacy-divider .text {
  font-size: 0.75rem;
  color: var(--world-text-mute);
  letter-spacing: 0.04em;
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

.cfg-item { display: flex; flex-direction: column; gap: 6px; }
.cfg-label { font-size: 0.75rem; font-weight: 700; color: var(--world-text-mute); }

.whitelist-form { display: flex; gap: 10px; align-items: center; }
.whitelist-select { flex: 1 1 auto; min-width: 240px; }
.whitelist-select :deep(.ws-trigger) { width: 100%; }

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
