<script setup lang="ts">
import { computed } from 'vue'
import { NPopover, NTag } from 'naive-ui'
import { formatPrice, formatRange, type PriceSummary } from '../../../composables/useNewAPIPricing'

const props = defineProps<{
  summary: PriceSummary | null
  // 卖价倍率 = markup × yuanPerUpstreamDollar × dollarsPerYuan
  // 默认显示入价；传 sellMultiplier 时显示卖价（虚拟 $）
  sellMultiplier?: number
}>()

const isSell = computed(() => typeof props.sellMultiplier === 'number' && props.sellMultiplier > 0)
const mult = computed(() => props.sellMultiplier || 1)

const inputRange = computed(() => {
  if (!props.summary) return ''
  return formatRange(props.summary.inputMin * mult.value, props.summary.inputMax * mult.value)
})
const outputRange = computed(() => {
  if (!props.summary) return ''
  return formatRange(props.summary.outputMin * mult.value, props.summary.outputMax * mult.value)
})
</script>

<template>
  <template v-if="!summary">
    <span class="dim warn">⚠ 上游分组数据不一致</span>
  </template>
  <template v-else-if="summary.modelsInGroup === 0">
    <span class="dim">该分组无按量模型（按次计费）</span>
  </template>
  <n-popover v-else trigger="hover" :show-arrow="false" placement="left-start">
    <template #trigger>
      <div class="cell">
        <span class="cell__range">{{ inputRange }}<span class="cell__unit"> /in</span></span>
        <span class="cell__range cell__range--sub">{{ outputRange }}<span class="cell__unit"> /out</span></span>
      </div>
    </template>
    <div class="pop">
      <div class="pop__head">
        <span class="pop__label">分组倍率</span>
        <n-tag size="small" :bordered="false">{{ summary.groupRatio }}×</n-tag>
        <span class="pop__sub">{{ summary.modelsInGroup }} 个模型</span>
      </div>
      <div class="pop__row pop__row--head">
        <span class="pop__col1">模型</span>
        <span class="pop__col2">{{ isSell ? '卖价' : '入价' }} 输入</span>
        <span class="pop__col3">{{ isSell ? '卖价' : '入价' }} 输出</span>
      </div>
      <div v-for="m in summary.topModels" :key="m.name" class="pop__row">
        <span class="pop__col1 mono">{{ m.name }}</span>
        <span class="pop__col2 mono">{{ formatPrice(m.inputPrice * mult) }}</span>
        <span class="pop__col3 mono">{{ formatPrice(m.outputPrice * mult) }}</span>
      </div>
      <div v-if="isSell" class="pop__hint">已乘 {{ mult.toFixed(2) }}× （markup × ¥→$）</div>
    </div>
  </n-popover>
</template>

<style scoped>
.dim { color: #707070; font-size: 11px; }
.warn { color: #d4a73a; }
.cell { display: flex; flex-direction: column; gap: 1px; line-height: 1.3; cursor: help; }
.cell__range { font-family: "Geist Mono", ui-monospace, monospace; font-size: 12px; color: #ededed; font-variant-numeric: tabular-nums; }
.cell__range--sub { color: #a3a3a3; font-size: 11px; }
.cell__unit { color: #707070; font-size: 10px; margin-left: 2px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; font-variant-numeric: tabular-nums; }
.pop { display: flex; flex-direction: column; gap: 8px; min-width: 320px; }
.pop__head { display: flex; align-items: center; gap: 8px; padding-bottom: 6px; border-bottom: 1px solid rgba(255,255,255,0.06); }
.pop__label { color: #707070; font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; }
.pop__sub { color: #707070; font-size: 12px; margin-left: auto; }
.pop__row { display: grid; grid-template-columns: 1fr 80px 80px; gap: 8px; font-size: 12px; color: #ededed; }
.pop__row--head { color: #707070; font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; padding-bottom: 4px; border-bottom: 1px dashed rgba(255,255,255,0.04); }
.pop__col1 { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pop__col2, .pop__col3 { text-align: right; }
.pop__hint { color: #707070; font-size: 10.5px; padding-top: 4px; border-top: 1px dashed rgba(255,255,255,0.04); }
</style>
