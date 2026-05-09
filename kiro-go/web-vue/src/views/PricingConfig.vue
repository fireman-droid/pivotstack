<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Save } from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const pricing = ref({ freePoolPriceUSD: 0.04, proPoolPriceUSD: 0.20, purchasePriceCNY: 0.04 })
const loading = ref(true)
const saving = ref(false)

async function loadPricing() {
  try {
    const res = await api('/pricing')
    if (res.ok) {
      const data = await res.json()
      pricing.value = {
        freePoolPriceUSD: data.freePoolPriceUSD || 0.04,
        proPoolPriceUSD: data.proPoolPriceUSD || 0.20,
        purchasePriceCNY: data.purchasePriceCNY || 0.04,
      }
    }
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

onMounted(loadPricing)
</script>

<template>
  <div class="space-y-6 max-w-[800px] mx-auto pb-20">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">定价配置</h1>
        <p class="text-sm text-[var(--text-secondary)]">Credit 计费单价（按池设置）</p>
      </div>
      <button @click="savePricing" :disabled="saving"
        class="flex items-center gap-2 px-5 py-2.5 bg-[var(--primary)] text-white rounded-xl text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-95 transition-all">
        <Save class="w-4 h-4" />
        {{ saving ? '保存中...' : '保存配置' }}
      </button>
    </div>

    <div class="modern-card p-5 space-y-5">
      <div class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">售价设置（用户扣费单价）</div>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
        <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">FREE 池单价 ($/credit)</label>
          <div class="text-[10px] text-[var(--text-secondary)] opacity-60">sonnet-4.5 使用此价格</div>
          <input v-model.number="pricing.freePoolPriceUSD" type="number" step="0.01"
            class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
        </div>
        <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">PRO 池单价 ($/credit)</label>
          <div class="text-[10px] text-[var(--text-secondary)] opacity-60">sonnet-4.6 / opus-4.6 / opus-4.7 使用此价格</div>
          <input v-model.number="pricing.proPoolPriceUSD" type="number" step="0.01"
            class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
        </div>
      </div>
    </div>

    <div class="modern-card p-5 space-y-5">
      <div class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">成本设置（进货价）</div>
      <div class="space-y-1" style="max-width: 300px">
        <label class="text-xs text-[var(--text-secondary)]">PRO 账号进货价 (¥/credit)</label>
        <div class="text-[10px] text-[var(--text-secondary)] opacity-60">用于利润计算，PRO号实际Credit成本</div>
        <input v-model.number="pricing.purchasePriceCNY" type="number" step="0.001"
          class="w-full h-10 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-sm outline-none focus:border-[var(--primary)]" />
      </div>
    </div>

    <!-- Quick Reference -->
    <div class="modern-card p-5">
      <div class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)] mb-3">快速参考</div>
      <div class="text-xs text-[var(--text-secondary)] space-y-1">
        <p>· 1 Kiro credit = $2 面值</p>
        <p>· FREE 池默认 $0.04/credit → 用户消耗1个credit花费 $0.04</p>
        <p>· PRO 池默认 $0.20/credit → 用户消耗1个credit花费 $0.20</p>
        <p>· 进货成本默认 ¥0.04/credit → 利润 = 售价收入 - 进货成本</p>
      </div>
    </div>
  </div>
</template>
