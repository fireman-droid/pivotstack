<script setup lang="ts">
import { computed } from 'vue'
import type { TrendPoint } from '../../../api/admin/businessBoard'

const props = defineProps<{
  label: string
  value: number
  unit?: string
  formatter?: (v: number) => string
  trend?: TrendPoint[]
  /** 'profit' = revenue_cny - cost_cny（派生，保持 sparkline 与卡片指标一致） */
  trendKey?: 'revenue_cny' | 'cost_cny' | 'profit'
  tone?: 'neutral' | 'positive' | 'negative' | 'warning'
}>()

const tone = computed(() => props.tone || 'neutral')
const formatted = computed(() => {
  if (props.formatter) return props.formatter(props.value)
  return props.value.toFixed(2)
})

function trendValue(p: TrendPoint, key: NonNullable<typeof props.trendKey>): number {
  if (key === 'profit') return (p.revenue_cny ?? 0) - (p.cost_cny ?? 0)
  return p[key] ?? 0
}

// Build sparkline path from trend data
const sparkPath = computed<string>(() => {
  const data = props.trend
  if (!data || data.length < 2 || !props.trendKey) return ''
  const key = props.trendKey
  const xs = data.map((_, i) => i)
  const ys = data.map(p => trendValue(p, key))
  const max = Math.max(...ys, 0.0001)
  const min = Math.min(...ys, 0)
  const range = max - min || 1
  const w = 100
  const h = 28
  const stepX = w / Math.max(xs.length - 1, 1)
  return data
    .map((p, i) => {
      const x = i * stepX
      const v = trendValue(p, key)
      const y = h - ((v - min) / range) * h
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
})

const sparkClass = computed(() => `bb-spark bb-spark--${tone.value}`)
</script>

<template>
  <div :class="['bb-kpi', `bb-kpi--${tone}`]">
    <div class="bb-kpi__label">{{ label }}</div>
    <div class="bb-kpi__value">
      <span class="bb-kpi__num">{{ formatted }}</span>
      <span v-if="unit" class="bb-kpi__unit">{{ unit }}</span>
    </div>
    <svg v-if="sparkPath" class="bb-kpi__spark" viewBox="0 0 100 28" preserveAspectRatio="none">
      <path :d="sparkPath" :class="sparkClass" fill="none" stroke-width="1.5" />
    </svg>
    <div v-else class="bb-kpi__spark bb-kpi__spark--empty">暂无趋势</div>
  </div>
</template>

<style scoped>
.bb-kpi {
  flex: 1 1 0;
  min-width: 0;
  padding: 14px 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--st-border);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.bb-kpi__label {
  font-size: 11px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--st-text-ter);
}
.bb-kpi__value {
  display: flex;
  align-items: baseline;
  gap: 4px;
  font-variant-numeric: tabular-nums;
  font-family: var(--st-font-mono, ui-monospace);
}
.bb-kpi__num {
  font-size: 24px;
  font-weight: 500;
  color: var(--st-text-pri);
}
.bb-kpi--positive .bb-kpi__num { color: var(--st-success); }
.bb-kpi--negative .bb-kpi__num { color: var(--st-error); }
.bb-kpi--warning .bb-kpi__num { color: var(--st-warning); }
.bb-kpi__unit { font-size: 12px; color: var(--st-text-ter); }
.bb-kpi__spark { width: 100%; height: 28px; margin-top: 2px; }
.bb-kpi__spark--empty { color: var(--st-text-ter); font-size: 11px; display: flex; align-items: center; }
.bb-spark--neutral { stroke: rgba(160, 160, 160, 0.5); }
.bb-spark--positive { stroke: var(--st-success); }
.bb-spark--negative { stroke: var(--st-error); }
.bb-spark--warning { stroke: var(--st-warning); }
</style>
