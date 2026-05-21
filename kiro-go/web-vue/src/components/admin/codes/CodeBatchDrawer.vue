<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { NInput, NInputNumber, NRadioGroup, NRadio, NButton, useMessage } from 'naive-ui'
import { Ticket } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import { createCodes, type CreateCodesRequest, type CreateCodesResponse } from '../../../api/admin/codes'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'generated', resp: CreateCodesResponse, meta: { type: string; amount: number; tier?: string; salePriceCNY?: number }): void
}>()

const message = useMessage()
const submitting = ref(false)

const codeType = ref<'balance' | 'days' | 'time'>('balance')
const amount = ref(50)
const tier = ref<'free' | 'pro'>('pro')
const count = ref(10)
const note = ref('')
const salePriceCNY = ref(0)

watch(() => props.show, v => {
  if (!v) return
  codeType.value = 'balance'
  amount.value = 50
  tier.value = 'pro'
  count.value = 10
  note.value = ''
  salePriceCNY.value = 0
})

const amountUnit = computed(() => {
  if (codeType.value === 'balance') return '¥（按 PivotStack 1:20 兑换为虚拟 $）'
  if (codeType.value === 'days') return '天'
  return '秒（高级用法）'
})
const amountStep = computed(() => (codeType.value === 'balance' ? 10 : 1))
const isTimed = computed(() => codeType.value === 'days' || codeType.value === 'time')

const summary = computed(() => {
  if (codeType.value === 'balance') {
    const total = amount.value * count.value
    return `${count.value} 张 × ¥${amount.value.toFixed(2)} = 总面值 ¥${total.toFixed(2)}`
  }
  if (codeType.value === 'days') {
    const sellTotal = salePriceCNY.value * count.value
    return `${count.value} 张 × ${amount.value} 天 (${tier.value.toUpperCase()})` + (salePriceCNY.value > 0 ? ` · 售价 ¥${sellTotal.toFixed(2)}` : '')
  }
  return `${count.value} 张 × ${amount.value} 秒 (${tier.value.toUpperCase()})`
})

function close() {
  if (submitting.value) return
  emit('update:show', false)
}

async function submit() {
  if (amount.value <= 0) {
    message.warning('面额/天数必须大于 0')
    return
  }
  if (count.value <= 0 || count.value > 100) {
    message.warning('单次生成 1 - 100 张')
    return
  }
  if (isTimed.value && salePriceCNY.value < 0) {
    message.warning('售价不能为负数')
    return
  }
  submitting.value = true
  try {
    const req: CreateCodesRequest = {
      type: codeType.value,
      amount: amount.value,
      count: count.value,
      note: note.value.trim() || undefined,
    }
    if (isTimed.value) {
      req.tier = tier.value
      if (salePriceCNY.value > 0) req.salePriceCNY = salePriceCNY.value
    }
    const resp = await createCodes(req)
    message.success(`已生成 ${resp.count} 张激活码`)
    emit('generated', resp, {
      type: codeType.value,
      amount: amount.value,
      tier: isTimed.value ? tier.value : undefined,
      salePriceCNY: isTimed.value ? salePriceCNY.value : undefined,
    })
    emit('update:show', false)
  } catch (e: any) {
    message.error(e?.message || '生成失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <RefinedDrawer
    :show="show"
    title="批量生成激活码"
    subtitle="单次生成 1-100 张；天卡/时长卡支持设售价用于利润统计"
    :icon="Ticket"
    :loading="submitting"
    :width="520"
    @update:show="(v) => emit('update:show', v)"
  >
    <RefinedField label="激活码类型">
      <n-radio-group v-model:value="codeType">
        <n-radio value="balance">余额型</n-radio>
        <n-radio value="days">天卡（按天）</n-radio>
        <n-radio value="time">时长（按秒）</n-radio>
      </n-radio-group>
    </RefinedField>

    <RefinedField :label="codeType === 'balance' ? '单张面额' : '单张时长'">
      <n-input-number
        v-model:value="amount"
        :min="0.01"
        :precision="codeType === 'balance' ? 2 : 0"
        :step="amountStep"
        style="width: 100%"
      >
        <template #suffix>{{ amountUnit }}</template>
      </n-input-number>
    </RefinedField>

    <RefinedField v-if="isTimed" label="账户级别">
      <n-radio-group v-model:value="tier">
        <n-radio value="free">FREE</n-radio>
        <n-radio value="pro">PRO</n-radio>
      </n-radio-group>
    </RefinedField>

    <RefinedField v-if="isTimed" label="单张售价 ¥" hint="用于利润统计；可留 0">
      <n-input-number v-model:value="salePriceCNY" :min="0" :precision="2" :step="5" style="width: 100%">
        <template #prefix>¥</template>
      </n-input-number>
    </RefinedField>

    <RefinedField label="生成数量" hint="单次最多 100 张">
      <n-input-number v-model:value="count" :min="1" :max="100" :step="1" style="width: 100%" />
    </RefinedField>

    <RefinedField label="批次备注" hint="可选；用于推广/活动追踪">
      <n-input v-model:value="note" placeholder="例如：618 大促 / 推广渠道 A" maxlength="60" show-count />
    </RefinedField>

    <div class="summary">{{ summary }}</div>

    <template #footer>
      <n-button :disabled="submitting" quaternary @click="close">取消</n-button>
      <n-button type="primary" :loading="submitting" @click="submit">生成</n-button>
    </template>
  </RefinedDrawer>
</template>

<style scoped>
.summary {
  margin-top: 12px;
  padding: 10px 12px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 4px;
  font-family: var(--st-font-mono, "Geist Mono", ui-monospace, monospace);
  font-variant-numeric: tabular-nums;
  font-size: 12px;
  color: #a1a1a1;
}
</style>
