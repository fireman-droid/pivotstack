<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Save, Plus, Trash2 } from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const pricing = ref({ defaultInputPrice: 0.3, defaultOutputPrice: 3.5, minRequestCost: 0.001, models: {} })
const loading = ref(true)
const saving = ref(false)
const newModel = ref('')

async function loadPricing() {
  try {
    const res = await api('/pricing')
    if (res.ok) pricing.value = await res.json()
  } catch { toastError('加载定价失败') }
  loading.value = false
}

async function savePricing() {
  saving.value = true
  try {
    await api('/pricing', { method: 'PUT', body: JSON.stringify(pricing.value) })
    success('定价已保存')
  } catch { toastError('保存失败') }
  saving.value = false
}

function addModel() {
  const name = newModel.value.trim()
  if (!name) return
  if (pricing.value.models && pricing.value.models[name]) return toastError('模型已存在')
  if (!pricing.value.models) pricing.value.models = {}
  pricing.value.models[name] = { inputPricePerM: 0.3, outputPricePerM: 3.5, multiplier: 1.0 }
  newModel.value = ''
}

function removeModel(name) {
  delete pricing.value.models[name]
}

onMounted(loadPricing)
</script>

<template>
  <div class="space-y-6 max-w-[1200px] mx-auto pb-20">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">定价配置</h1>
        <p class="text-sm text-[var(--text-secondary)]">设置各模型的计费价格（元/百万Token）</p>
      </div>
      <button @click="savePricing" :disabled="saving"
        class="flex items-center gap-2 px-5 py-2.5 bg-[var(--primary)] text-white rounded-xl text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-95 transition-all">
        <Save class="w-4 h-4" />
        {{ saving ? '保存中...' : '保存配置' }}
      </button>
    </div>

    <!-- Default Pricing -->
    <div class="modern-card p-5 space-y-4">
      <div class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">默认定价</div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">默认输入价 (¥/M tokens)</label>
          <input v-model.number="pricing.defaultInputPrice" type="number" step="0.01"
            class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
        </div>
        <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">默认输出价 (¥/M tokens)</label>
          <input v-model.number="pricing.defaultOutputPrice" type="number" step="0.01"
            class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
        </div>
        <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">最低请求费用 (¥)</label>
          <input v-model.number="pricing.minRequestCost" type="number" step="0.0001"
            class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
        </div>
      </div>
    </div>

    <!-- Model Pricing Table -->
    <div class="modern-card p-5 space-y-4">
      <div class="flex items-center justify-between">
        <div class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">模型定价</div>
        <div class="flex items-center gap-2">
          <input v-model="newModel" placeholder="模型名称" @keyup.enter="addModel"
            class="h-8 px-3 bg-[var(--bg)] border border-[var(--border)] rounded-lg text-xs outline-none focus:border-[var(--primary)] w-48" />
          <button @click="addModel" class="p-1.5 bg-[var(--primary)] rounded-lg hover:scale-105 transition-transform">
            <Plus class="w-4 h-4 text-white" />
          </button>
        </div>
      </div>

      <div v-if="pricing.models && Object.keys(pricing.models).length" class="space-y-2">
        <!-- Header -->
        <div class="grid grid-cols-[2fr_1fr_1fr_1fr_auto] gap-3 px-4 py-2 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">
          <span>模型名称</span>
          <span>输入价 (¥/M)</span>
          <span>输出价 (¥/M)</span>
          <span>倍率</span>
          <span class="w-8"></span>
        </div>
        <!-- Rows -->
        <div v-for="(cfg, model) in pricing.models" :key="model"
          class="grid grid-cols-[2fr_1fr_1fr_1fr_auto] gap-3 px-4 py-2 items-center bg-[var(--bg)]/50 rounded-xl">
          <span class="text-sm font-mono font-bold text-[var(--primary)]">{{ model }}</span>
          <input v-model.number="cfg.inputPricePerM" type="number" step="0.01"
            class="h-8 px-3 bg-[var(--card)] border border-[var(--border)] rounded-lg text-xs outline-none focus:border-[var(--primary)]" />
          <input v-model.number="cfg.outputPricePerM" type="number" step="0.01"
            class="h-8 px-3 bg-[var(--card)] border border-[var(--border)] rounded-lg text-xs outline-none focus:border-[var(--primary)]" />
          <input v-model.number="cfg.multiplier" type="number" step="0.1"
            class="h-8 px-3 bg-[var(--card)] border border-[var(--border)] rounded-lg text-xs outline-none focus:border-[var(--primary)]" />
          <button @click="removeModel(model)" class="p-1.5 rounded-lg hover:bg-rose-500/10">
            <Trash2 class="w-3.5 h-3.5 text-rose-500" />
          </button>
        </div>
      </div>
      <div v-else class="text-center py-8 text-sm text-[var(--text-secondary)]">
        暂无模型定价，将使用默认价格
      </div>
    </div>
  </div>
</template>
