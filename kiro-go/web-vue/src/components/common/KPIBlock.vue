<script setup lang="ts">
import { computed } from 'vue'
import { NSkeleton } from 'naive-ui'

interface Props {
  label: string
  value: string | number
  trend?: number | null  // 百分比；正 = 涨；负 = 跌；null = 无趋势
  loading?: boolean
  hint?: string
}
const props = defineProps<Props>()

const trendColor = computed(() => {
  if (props.trend == null) return 'var(--color-text-tertiary, #707070)'
  if (props.trend > 0) return 'var(--color-success, #0bd470)'
  if (props.trend < 0) return 'var(--color-error, #ff4d4d)'
  return 'var(--color-text-tertiary, #707070)'
})

const trendLabel = computed(() => {
  if (props.trend == null) return ''
  const sign = props.trend > 0 ? '+' : ''
  return `${sign}${props.trend.toFixed(1)}%`
})
</script>

<template>
  <div class="kpi">
    <div class="kpi__label">
      {{ label }}
      <span v-if="hint" class="kpi__hint" :title="hint">?</span>
    </div>
    <n-skeleton v-if="loading" :width="120" :height="32" class="kpi__skeleton" />
    <div v-else class="kpi__value">{{ value }}</div>
    <div v-if="!loading && trend != null" class="kpi__trend">
      <span class="kpi__dot" :style="{ background: trendColor }" />
      <span :style="{ color: trendColor }">{{ trendLabel }}</span>
    </div>
  </div>
</template>

<style scoped>
.kpi { display: flex; flex-direction: column; gap: 8px; }
.kpi__label {
  font-size: 13px;
  color: var(--color-text-secondary, #a1a1a1);
  display: flex; align-items: center; gap: 6px;
}
.kpi__hint {
  display: inline-flex; align-items: center; justify-content: center;
  width: 14px; height: 14px; border-radius: 50%;
  border: 1px solid rgba(255,255,255,0.16);
  font-size: 10px; color: var(--color-text-tertiary, #707070);
  cursor: help;
}
.kpi__value {
  font-size: 32px; line-height: 40px; font-weight: 600;
  color: var(--color-text-primary, #ededed);
  font-feature-settings: 'tnum';
}
.kpi__trend { display: flex; align-items: center; gap: 4px; font-size: 12px; }
.kpi__dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.kpi__skeleton { border-radius: 4px; }
</style>
