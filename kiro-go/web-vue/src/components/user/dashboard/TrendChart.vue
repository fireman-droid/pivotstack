<script setup lang="ts">
import { computed } from 'vue'
import Tile from '../stellar/Tile.vue'
import EChartsPanel from '../stellar/EChartsPanel.vue'
import { echarts } from '../../../design/echarts-stellar'

const props = defineProps<{
  days: { date: string; calls: number }[]
}>()

const total = computed(() => props.days.reduce((s, d) => s + (d.calls || 0), 0))
const labels = computed(() => props.days.map(d => {
  const m = /(\d{2})-(\d{2})$/.exec(d.date)
  return m ? `${m[1]}/${m[2]}` : d.date
}))
const values = computed(() => props.days.map(d => d.calls || 0))

const option = computed(() => {
  const data = values.value
  return {
    grid: { left: 8, right: 24, top: 24, bottom: 24, containLabel: true },
    xAxis: {
      type: 'category',
      data: labels.value,
      boundaryGap: false,
    },
    yAxis: { type: 'value', splitNumber: 3 },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line', lineStyle: { color: 'rgba(255,255,255,0.15)', type: 'dashed' } },
      formatter: (p: any) => {
        const v = p[0]
        const prev = data[v.dataIndex - 1]
        const diff = prev ? Math.round(((v.value - prev) / prev) * 100) : 0
        const arrow = diff >= 0 ? '↗' : '↘'
        const cls = diff >= 0 ? '#0bd470' : '#ff4d4d'
        return `<div style="font-family:Geist Mono">${v.name}&nbsp;&nbsp;<b>${v.value}</b> calls${prev ? `&nbsp;&nbsp;<span style="color:${cls}">${arrow} ${diff >= 0 ? '+' : ''}${diff}%</span>` : ''}</div>`
      },
    },
    series: [{
      type: 'line',
      smooth: true,
      data,
      lineStyle: { width: 1.5, color: '#0bd470' },
      itemStyle: { color: '#0bd470' },
      showSymbol: false,
      emphasis: { showSymbol: true, itemStyle: { color: '#0bd470', borderColor: '#000', borderWidth: 2 } },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(11,212,112,0.22)' },
          { offset: 1, color: 'rgba(11,212,112,0)' },
        ]),
      },
      markPoint: data.length ? {
        data: [{ type: 'max', name: '峰值' }],
        symbol: 'circle', symbolSize: 6,
        itemStyle: { color: '#0bd470', borderColor: '#000', borderWidth: 2 },
        label: {
          show: true, position: 'top',
          color: '#ededed', fontSize: 11, fontFamily: 'Geist Mono',
          formatter: '{c}',
        },
      } : undefined,
    }],
  }
})
</script>

<template>
  <Tile>
    <div class="tile__head tile__head--split">
      <div>
        <div class="t-display">调用趋势</div>
        <div class="t-label tertiary">LAST {{ days.length }} DAYS</div>
      </div>
      <div class="tile__head-right">
        <div class="t-num-strong">{{ total.toLocaleString() }} <span class="t-body sub">calls</span></div>
        <div class="t-label tertiary">TOTAL</div>
      </div>
    </div>
    <EChartsPanel :option="option" :height="200" />
  </Tile>
</template>
