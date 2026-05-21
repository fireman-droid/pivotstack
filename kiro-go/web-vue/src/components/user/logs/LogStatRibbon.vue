<script setup lang="ts">
import { computed } from 'vue'
import EChartsPanel from '../stellar/EChartsPanel.vue'
import CounterNumber from '../stellar/CounterNumber.vue'
import StatusDot from '../stellar/StatusDot.vue'
import { echarts, hexToRgba } from '../../../design/echarts-stellar'

const props = defineProps<{
  todayCount: number
  avgLatencyMs: number
  errorRatePct: number
  // 24h 桶的数据（数组长度 24）
  callsPerHour: number[]
  latencyPerHour: number[]
  errorsPerHour: number[]
}>()

function miniOpt(data: number[], color: string, type: 'bar' | 'line') {
  const safe = data.length ? data : Array(24).fill(0)
  return {
    animation: false,
    grid: { left: 0, right: 0, top: 2, bottom: 2 },
    xAxis: { show: false, type: 'category', data: safe.map(() => '') },
    yAxis: { show: false, type: 'value' },
    tooltip: { show: false },
    series: [{
      type,
      data: safe,
      smooth: type === 'line',
      symbol: 'none',
      barWidth: type === 'bar' ? '70%' : undefined,
      itemStyle: { color, borderRadius: type === 'bar' ? [2, 2, 0, 0] : 0 },
      lineStyle: type === 'line' ? { width: 1, color } : undefined,
      areaStyle: type === 'line' ? {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: hexToRgba(color, 0.25) },
          { offset: 1, color: hexToRgba(color, 0) },
        ]),
      } : undefined,
    }],
  }
}

const callsOpt = computed(() => miniOpt(props.callsPerHour, '#0bd470', 'bar'))
const latencyOpt = computed(() => miniOpt(props.latencyPerHour, '#52a8ff', 'line'))
const errorsOpt = computed(() => miniOpt(props.errorsPerHour, '#ff4d4d', 'bar'))
</script>

<template>
  <section class="stat-ribbon">
    <div class="stat-tile">
      <div class="stat-tile__head">
        <span class="t-label">今日调用</span>
        <StatusDot status="ok" pulse />
      </div>
      <div class="stat-tile__num">
        <CounterNumber :value="todayCount" class="t-hero-lg mono" />
      </div>
      <div class="stat-tile__chart"><EChartsPanel :option="callsOpt" :height="36" no-animation /></div>
    </div>
    <div class="stat-tile">
      <div class="stat-tile__head"><span class="t-label">平均延迟</span></div>
      <div class="stat-tile__num">
        <CounterNumber :value="Math.round(avgLatencyMs)" suffix="ms" class="t-hero-lg mono" />
      </div>
      <div class="stat-tile__chart"><EChartsPanel :option="latencyOpt" :height="36" no-animation /></div>
    </div>
    <div class="stat-tile">
      <div class="stat-tile__head"><span class="t-label">错误率</span></div>
      <div class="stat-tile__num">
        <CounterNumber :value="errorRatePct" :decimals="1" suffix="%" class="t-hero-lg mono" />
      </div>
      <div class="stat-tile__chart"><EChartsPanel :option="errorsOpt" :height="36" no-animation /></div>
    </div>
  </section>
</template>

<style scoped>
.stat-ribbon { display: grid; grid-template-columns: repeat(3, 1fr); gap: 24px; margin-bottom: 24px; }
.stat-tile {
  padding: 20px;
  background: rgba(255,255,255,0.02);
  border-radius: 6px;
  display: flex; flex-direction: column; gap: 8px;
  min-height: 96px;
}
.stat-tile__head { display: flex; align-items: center; justify-content: space-between; }
.stat-tile__num { line-height: 1; }
.stat-tile__chart { height: 36px; margin-top: 8px; }
@media (max-width: 1024px) {
  .stat-ribbon { grid-template-columns: 1fr; }
}
</style>
