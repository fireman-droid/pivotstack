<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  listChannels, createChannel, updateChannel, deleteChannel, testChannel
} from '../api/admin'
import { useToast } from '../composables/useToast'
import {
  Globe, Plus, Pencil, Trash, Eye, EyeOff, RefreshCw, Power
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldModal from '../components/world/WorldModal.vue'
import WorldInput from '../components/world/WorldInput.vue'

const router = useRouter()
const toast = useToast()

const channels = ref([])
const loading = ref(true)
const showModal = ref(false)
const editingId = ref(null)
const form = ref({
  id: '',
  type: 'openai',
  baseUrl: 'https://api.openai.com/v1',
  apiKey: '',
  models: [],
  modelPrices: {},  // { "model-name": { inputPerM, outputPerM } }
  enabled: true,
})
const modelInput = ref('')

// 表单里某个 model 的当前定价（用 computed/setter 实现双向绑定）
function priceFor(model, field) {
  const key = String(model).toLowerCase()
  return form.value.modelPrices?.[key]?.[field] ?? 0
}
function setPriceFor(model, field, val) {
  const key = String(model).toLowerCase()
  const v = Number(val) || 0
  if (!form.value.modelPrices) form.value.modelPrices = {}
  const existing = form.value.modelPrices[key] || { inputPerM: 0, outputPerM: 0 }
  form.value.modelPrices = {
    ...form.value.modelPrices,
    [key]: { ...existing, [field]: v },
  }
}
const testing = ref({})
const testResults = ref({})
const revealKey = ref({})

async function fetchChannels() {
  loading.value = true
  try {
    channels.value = await listChannels()
  } catch (e) {
    toast.error?.(e.message) || console.error(e)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = {
    id: '',
    type: 'openai',
    baseUrl: 'https://api.openai.com/v1',
    apiKey: '',
    models: [],
    modelPrices: {},
    enabled: true,
  }
  modelInput.value = ''
  showModal.value = true
}

function openEdit(ch) {
  editingId.value = ch.id
  form.value = {
    id: ch.id,
    type: ch.type,
    baseUrl: ch.baseUrl || '',
    apiKey: '', // 留空 = 不修改
    models: Array.isArray(ch.models) ? [...ch.models] : [],
    modelPrices: ch.modelPrices ? { ...ch.modelPrices } : {},
    enabled: ch.enabled,
  }
  modelInput.value = ''
  showModal.value = true
}

async function save() {
  if (!form.value.id) {
    toast.error?.('请填写渠道 ID')
    return
  }
  if (!editingId.value) {
    // 新建时检查 ID 重复
    if (channels.value.some(c => c.id === form.value.id)) {
      toast.error?.(`渠道 ID "${form.value.id}" 已存在`)
      return
    }
  }
  // baseUrl 必填 + http(s):// 格式
  const url = (form.value.baseUrl || '').trim()
  if (!url) {
    toast.error?.('请填写 Base URL')
    return
  }
  if (!/^https?:\/\/\S+/.test(url)) {
    toast.error?.('Base URL 必须以 http:// 或 https:// 开头')
    return
  }
  if (!form.value.models.length) {
    toast.error?.('请至少添加一个模型')
    return
  }
  try {
    if (editingId.value) {
      await updateChannel(editingId.value, form.value)
      toast.success?.('渠道已更新')
    } else {
      await createChannel(form.value)
      toast.success?.('渠道已创建')
    }
    showModal.value = false
    await fetchChannels()
  } catch (e) {
    toast.error?.(e.message)
  }
}

async function remove(id) {
  if (!confirm(`确认删除渠道 "${id}"？此操作不可恢复`)) return
  try {
    await deleteChannel(id)
    toast.success?.('已删除')
    await fetchChannels()
  } catch (e) {
    toast.error?.(e.message)
  }
}

async function toggleEnabled(ch) {
  try {
    await updateChannel(ch.id, { ...ch, enabled: !ch.enabled })
    toast.success?.(ch.enabled ? '已禁用' : '已启用')
    await fetchChannels()
  } catch (e) {
    toast.error?.(e.message)
  }
}

async function runTest(id) {
  testing.value = { ...testing.value, [id]: true }
  testResults.value = { ...testResults.value, [id]: null }
  try {
    const res = await testChannel(id)
    testResults.value = {
      ...testResults.value,
      [id]: {
        ok: res.success !== false,
        latencyMs: res.latencyMs,
        models: res.models || [],
        type: res.type,
      },
    }
  } catch (e) {
    testResults.value = {
      ...testResults.value,
      [id]: { ok: false, error: e.message },
    }
  } finally {
    testing.value = { ...testing.value, [id]: false }
  }
}

function addModel() {
  const m = modelInput.value.trim()
  if (!m) return
  if (!form.value.models.includes(m)) {
    form.value.models.push(m)
  }
  modelInput.value = ''
}

function removeModelChip(idx) {
  form.value.models.splice(idx, 1)
}

const hasChannels = computed(() => channels.value.length > 0)

onMounted(fetchChannels)
</script>

<template>
  <div class="channels-page">
    <header class="page-head">
      <div>
        <div class="eyebrow">路由管理</div>
        <h1 class="page-title">上游渠道</h1>
        <p class="page-subtitle">配置上游 API 提供者；每个模型只能由一个启用渠道服务。</p>
      </div>
      <div class="head-actions">
        <WorldButton variant="ghost" size="sm" @click="fetchChannels">
          <RefreshCw :size="14" /> 刷新
        </WorldButton>
        <WorldButton variant="primary" size="sm" @click="openCreate">
          <Plus :size="14" /> 新建外部渠道
        </WorldButton>
      </div>
    </header>

    <div v-if="loading && !hasChannels" class="loading-state">
      正在加载渠道信息...
    </div>

    <div v-else-if="!hasChannels" class="empty-state">
      <WorldCard class="empty-card">
        <Globe :size="48" class="empty-icon" />
        <h3>还没有配置渠道</h3>
        <p>添加外部渠道（OpenAI 兼容）让平台支持更多模型。</p>
        <WorldButton variant="primary" @click="openCreate">
          <Plus :size="14" /> 添加第一个渠道
        </WorldButton>
      </WorldCard>
    </div>

    <div v-else class="channel-grid">
      <WorldCard v-for="ch in channels" :key="ch.id" class="channel-card">
        <div class="card-head">
          <span class="type-icon">{{ ch.type === 'kiro' ? '🔵' : '🟢' }}</span>
          <span class="channel-id">{{ ch.id }}</span>
          <WorldChip :variant="ch.type === 'kiro' ? 'info' : 'success'" size="sm">
            {{ ch.type }}
          </WorldChip>
          <span class="spacer" />
          <WorldChip v-if="!ch.enabled" variant="warning" size="sm">已禁用</WorldChip>
        </div>

        <div class="card-body">
          <template v-if="ch.type === 'kiro'">
            <div class="info-row">
              <span class="label">账号池：</span>
              <span class="value">使用 Kiro 内置账号池</span>
              <button class="link-btn" @click="router.push('/accounts')">管理账号 →</button>
            </div>
          </template>
          <template v-else>
            <div class="info-row">
              <span class="label">Base URL：</span>
              <span class="value">{{ ch.baseUrl }}</span>
            </div>
            <div class="info-row">
              <span class="label">API Key：</span>
              <code class="apikey">
                {{ revealKey[ch.id] ? (ch.apiKey || '(未设置)') : '••••••••••••' + (ch.apiKey ? ch.apiKey.slice(-4) : '') }}
              </code>
              <button
                class="icon-btn"
                @click="revealKey[ch.id] = !revealKey[ch.id]"
                :title="revealKey[ch.id] ? '隐藏' : '显示'"
                :aria-label="revealKey[ch.id] ? '隐藏 API Key' : '显示 API Key'"
              >
                <component :is="revealKey[ch.id] ? EyeOff : Eye" :size="13" />
              </button>
              <button class="icon-btn" @click="openEdit(ch)" title="修改" aria-label="编辑渠道">
                <Pencil :size="13" />
              </button>
            </div>
          </template>

          <div class="models-section">
            <div class="models-label">服务模型 ({{ ch.models?.length || 0 }})</div>
            <div class="models-list">
              <WorldChip v-for="m in ch.models" :key="m" variant="neutral" size="sm">{{ m }}</WorldChip>
            </div>
          </div>

          <div v-if="testResults[ch.id]" class="test-result" :class="testResults[ch.id].ok ? 'ok' : 'fail'">
            <template v-if="testResults[ch.id].ok">
              ✓ 测试通过{{ testResults[ch.id].latencyMs != null ? ` · ${testResults[ch.id].latencyMs}ms` : '' }}
              <span v-if="testResults[ch.id].models?.length"> · 上游支持 {{ testResults[ch.id].models.length }} 个模型</span>
            </template>
            <template v-else>
              ✗ 失败: {{ testResults[ch.id].error || '未知错误' }}
            </template>
          </div>
        </div>

        <div class="card-actions">
          <WorldButton variant="secondary" size="sm" :loading="testing[ch.id]" @click="runTest(ch.id)">
            测试连接
          </WorldButton>
          <span class="spacer" />
          <template v-if="ch.type !== 'kiro'">
            <WorldButton variant="ghost" size="sm" @click="toggleEnabled(ch)">
              <Power :size="13" /> {{ ch.enabled ? '禁用' : '启用' }}
            </WorldButton>
            <WorldButton variant="danger" size="sm" @click="remove(ch.id)" aria-label="删除渠道">
              <Trash :size="13" />
            </WorldButton>
          </template>
        </div>
      </WorldCard>
    </div>

    <WorldModal v-model="showModal" :title="editingId ? '编辑渠道' : '新建外部渠道'" size="md">
      <div class="form-grid">
        <WorldInput
          v-model="form.id"
          label="渠道 ID"
          placeholder="如 tcdmx-openai"
          :disabled="!!editingId"
        />
        <WorldInput
          v-model="form.baseUrl"
          label="Base URL"
          placeholder="https://api.openai.com/v1"
        />
        <WorldInput
          v-model="form.apiKey"
          label="API Key"
          type="password"
          :placeholder="editingId ? '留空表示不修改' : 'sk-xxxxxxxx'"
        />

        <div class="models-input">
          <label class="input-label">支持模型</label>
          <div class="chips-area">
            <WorldChip v-for="(m, i) in form.models" :key="m + i" variant="info" size="sm">
              {{ m }}
              <button class="chip-x" @click="removeModelChip(i)" type="button">×</button>
            </WorldChip>
            <span v-if="!form.models.length" class="muted-hint">尚未添加模型</span>
          </div>
          <div class="add-row">
            <WorldInput
              v-model="modelInput"
              placeholder="输入模型名后按回车"
              @keyup.enter.prevent="addModel"
            />
            <WorldButton variant="secondary" size="sm" @click="addModel">添加</WorldButton>
          </div>
        </div>

        <div v-if="form.models.length" class="model-prices-input">
          <label class="input-label">模型定价（虚拟$/1M token）</label>
          <p class="input-hint">⚠️ 缺定价的模型调用会返回 sell_price_missing 错误。1 USD = 20 虚拟$ = 1¥</p>
          <div class="price-table">
            <div class="price-head">
              <span class="model-col">模型</span>
              <span class="num-col">输入 $/M</span>
              <span class="num-col">输出 $/M</span>
              <span class="num-col">输入 ¥/M</span>
              <span class="num-col">输出 ¥/M</span>
            </div>
            <div v-for="m in form.models" :key="`price-${m}`" class="price-row">
              <code class="model-col">{{ m }}</code>
              <input
                type="number" step="0.1" min="0"
                class="price-input"
                :value="priceFor(m, 'inputPerM')"
                @input="e => setPriceFor(m, 'inputPerM', e.target.value)"
              />
              <input
                type="number" step="0.1" min="0"
                class="price-input"
                :value="priceFor(m, 'outputPerM')"
                @input="e => setPriceFor(m, 'outputPerM', e.target.value)"
              />
              <span class="dim num-col">¥{{ (priceFor(m, 'inputPerM') * 0.05).toFixed(4) }}</span>
              <span class="dim num-col">¥{{ (priceFor(m, 'outputPerM') * 0.05).toFixed(4) }}</span>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <WorldButton variant="ghost" @click="showModal = false">取消</WorldButton>
        <WorldButton variant="primary" @click="save">保存</WorldButton>
      </template>
    </WorldModal>
  </div>
</template>

<style scoped>
.channels-page { display: flex; flex-direction: column; gap: 20px; }
.page-head { display: flex; justify-content: space-between; align-items: flex-end; gap: 16px; flex-wrap: wrap; }
.eyebrow { color: var(--world-text-mute); font-size: 0.7rem; letter-spacing: 0.1em; text-transform: uppercase; margin-bottom: 4px; }
.page-title { font-size: 1.5rem; font-weight: 700; margin: 0; }
.page-subtitle { color: var(--world-text-mute); font-size: 0.85rem; margin: 4px 0 0; }
.head-actions { display: flex; gap: 8px; }

.loading-state, .empty-state { padding: 60px 0; display: flex; justify-content: center; color: var(--world-text-mute); }
.empty-card { max-width: 420px; text-align: center; display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 32px 24px; }
.empty-icon { color: var(--world-text-dim); }

.channel-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(360px, 100%), 1fr)); gap: 16px; }
.channel-card { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.card-head { display: flex; align-items: center; gap: 8px; }
.type-icon { font-size: 1.1rem; }
.channel-id { font-weight: 700; font-family: var(--world-font-mono, monospace); }
.spacer { flex: 1; }

.card-body { display: flex; flex-direction: column; gap: 10px; }
.info-row { display: flex; align-items: center; gap: 6px; font-size: 0.85rem; flex-wrap: wrap; }
.info-row .label { color: var(--world-text-mute); }
.info-row .value { font-family: var(--world-font-mono, monospace); color: var(--world-text-primary); word-break: break-all; }
.apikey { font-family: var(--world-font-mono, monospace); background: var(--world-overlay-light, rgba(0,0,0,0.05)); padding: 2px 6px; border-radius: 4px; font-size: 0.8rem; }
.link-btn { background: none; border: none; cursor: pointer; color: var(--world-accent, #06f); font-size: 0.85rem; font-weight: 600; padding: 0; }
.icon-btn { background: none; border: none; cursor: pointer; color: var(--world-text-dim); padding: 4px; border-radius: 4px; display: inline-flex; align-items: center; }
.icon-btn:hover { background: var(--world-overlay-light, rgba(0,0,0,0.06)); color: var(--world-text-primary); }

.models-section { display: flex; flex-direction: column; gap: 6px; }
.models-label { font-size: 0.7rem; color: var(--world-text-mute); letter-spacing: 0.04em; }
.models-list { display: flex; flex-wrap: wrap; gap: 4px; }

.test-result { font-size: 0.75rem; padding: 6px 10px; border-radius: 6px; }
.test-result.ok { background: rgba(34, 197, 94, 0.12); color: #15803d; }
.test-result.fail { background: rgba(239, 68, 68, 0.12); color: #b91c1c; word-break: break-word; }

.card-actions { display: flex; align-items: center; gap: 8px; padding-top: 8px; border-top: 1px solid var(--world-divider, rgba(0,0,0,0.06)); }

.form-grid { display: flex; flex-direction: column; gap: 14px; }
.input-label { display: block; font-size: 0.8rem; color: var(--world-text-mute); margin-bottom: 6px; font-weight: 600; }
.models-input { display: flex; flex-direction: column; gap: 8px; }
.chips-area { display: flex; flex-wrap: wrap; gap: 4px; min-height: 32px; padding: 8px; border: 1px dashed var(--world-divider, rgba(0,0,0,0.1)); border-radius: 6px; align-items: center; }
.chip-x { background: none; border: none; cursor: pointer; color: currentColor; margin-left: 4px; font-size: 14px; line-height: 1; }
.muted-hint { color: var(--world-text-mute); font-size: 0.8rem; }
.add-row { display: flex; gap: 8px; align-items: flex-end; }
.add-row > :first-child { flex: 1; }

.model-prices-input { display: flex; flex-direction: column; gap: 8px; }
.input-hint { color: var(--world-text-mute); font-size: 0.75rem; margin: -2px 0 4px; }
.price-table { display: flex; flex-direction: column; border: 1px solid var(--world-divider, rgba(0,0,0,0.08)); border-radius: 6px; overflow-x: auto; }
.price-head, .price-row { display: flex; align-items: center; gap: 8px; padding: 6px 10px; font-size: 0.75rem; }
.price-head { background: var(--world-bg-tertiary, rgba(0,0,0,0.02)); color: var(--world-text-mute); text-transform: uppercase; letter-spacing: 0.04em; font-weight: 700; }
.price-row { border-top: 1px solid var(--world-divider, rgba(0,0,0,0.04)); font-size: 0.85rem; }
.price-row:hover { background: var(--world-bg-tertiary, rgba(0,0,0,0.02)); }
.price-row .model-col { flex: 2; min-width: 140px; font-family: var(--world-font-mono, monospace); }
.price-head .model-col { flex: 2; min-width: 140px; }
.num-col { flex: 1; min-width: 60px; text-align: right; }
.price-input {
  flex: 1; min-width: 60px;
  padding: 4px 8px; border-radius: 4px;
  border: 1px solid var(--world-divider, rgba(0,0,0,0.1));
  background: var(--world-bg-secondary, transparent);
  color: inherit;
  font-family: var(--world-font-mono, monospace);
  text-align: right;
  font-size: 0.85rem;
}
.price-input:focus { outline: 2px solid var(--world-accent, #06f); outline-offset: -1px; }
.dim { color: var(--world-text-mute); font-family: var(--world-font-mono, monospace); }
</style>
