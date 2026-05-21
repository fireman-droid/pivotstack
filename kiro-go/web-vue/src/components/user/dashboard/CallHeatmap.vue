<script setup lang="ts">
// 24h × 7d 调用热力图
// 接受预聚合数据 [hour, dayIndex(0=今天-6..6=今天), count]
import { computed } from 'vue'
import Tile from '../stellar/Tile.vue'
import EChartsPanel from '../stellar/EChartsPanel.vue'

export interface HeatCell { hour: number; dayOffset: number; count: number }

const props = defineProps<{
  cells: HeatCell[]   // dayOffset: -6..0
}>()

// y 轴：从顶上数起 7 行，分别是 -6 天 ... 0(今天)
const days = ['6 天前', '5 天前', '4 天前', '3 天前', '前天', '昨天', '今天']

const option = computed(() => {
  // 转成 [x, y, v]，y=0 是 6天前（最上），y=6 是今天（最下）
  const data = props.cells.map(c => [c.hour, c.dayOffset + 6, c.count])
  let max = 0
  for (const c of props.cells) if (c.count > max) max = c.count
  if (max < 1) max = 10  // 避免空状态颜色全黑
  return {
    grid: { left: 60, right: 12, top: 8, bottom: 28, containLabel: false },
    tooltip: {
      formatter: (p: any) =>
        `<div style="font-family:Geist Mono">${days[p.value[1]]} ${String(p.value[0]).padStart(2, '0')}:00<br/><b>${p.value[2]}</b> calls</div>`,
    },
    xAxis: {
      type: 'category',
      data: Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0')),
      splitArea: { show: false },
      axisLine: { show: false },
      axisLabel: { color: '#707070', fontSize: 10 },
    },
    yAxis: {
      type: 'category',
      data: days,
      splitArea: { show: false },
      axisLine: { show: false },
      axisLabel: { color: '#707070', fontSize: 10 },
    },
    visualMap: {
      min: 0, max,
      show: false, calculable: false,
      inRange: { color: ['rgba(11,212,112,0.04)', 'rgba(11,212,112,0.35)', '#0bd470'] },
    },
    series: [{
      type: 'heatmap',
      data,
      itemStyle: { borderRadius: 2, borderColor: '#000', borderWidth: 1 },
      emphasis: { itemStyle: { borderColor: '#0bd470', borderWidth: 1 } },
    }],
  }
})
</script>

<template>
  <Tile class="tile tile--wide">
    <div class="tile__head tile__head--split">
      <div>
        <div class="t-display">调用热力图</div>
        <div class="t-label tertiary">WHEN YOU ARE BUSY</div>
      </div>
      <div class="heatmap-legend">
        <span class="t-label tertiary">少</span>
        <span class="heatmap-legend__swatches">
          <i style="background:rgba(11,212,112,0.05)"></i>
          <i style="background:rgba(11,212,112,0.15)"></i>
          <i style="background:rgba(11,212,112,0.3)"></i>
          <i style="background:rgba(11,212,112,0.55)"></i>
          <i style="background:rgba(11,212,112,0.85)"></i>
          <i style="background:#0bd470"></i>
        </span>
        <span class="t-label tertiary">多</span>
      </div>
    </div>
    <EChartsPanel :option="option" :height="180" />
  </Tile>
</template>

<style scoped>
.heatmap-legend { display: flex; align-items: center; gap: 8px; }
.heatmap-legend__swatches { display: inline-flex; gap: 2px; }
.heatmap-legend__swatches i { width: 12px; height: 12px; border-radius: 2px; display: inline-block; }
</style>
