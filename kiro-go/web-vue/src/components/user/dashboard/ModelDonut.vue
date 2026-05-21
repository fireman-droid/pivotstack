<script setup lang="ts">
import { computed, ref } from 'vue'
import Tile from '../stellar/Tile.vue'
import EChartsPanel from '../stellar/EChartsPanel.vue'

export interface ModelStat {
  model: string
  requests: number
  costUsd?: number
}

const props = withDefaults(defineProps<{
  models: ModelStat[]
  title?: string
  emptyHint?: string
}>(), {
  title: '渠道分布',
  emptyHint: '本周期还没调用过任何渠道',
})

const palette = ['#0bd470', '#52a8ff', '#f5a623', '#ff4d4d', '#a1a1a1', '#ededed']
const top = computed<ModelStat[]>(() => {
  const arr = [...props.models].sort((a, b) => b.requests - a.requests)
  if (arr.length <= 5) return arr
  // 折叠成 top4 + "其他"
  const head = arr.slice(0, 4)
  const rest = arr.slice(4)
  const restReq = rest.reduce((s, r) => s + r.requests, 0)
  const restCost = rest.reduce((s, r) => s + (r.costUsd || 0), 0)
  head.push({ model: '其他', requests: restReq, costUsd: restCost })
  return head
})
const totalReq = computed(() => top.value.reduce((s, r) => s + r.requests, 0))

const panelRef = ref<any>(null)

const option = computed(() => {
  const data = top.value.map((m, i) => ({
    value: m.requests,
    name: m.model,
    itemStyle: { color: palette[i % palette.length] },
  }))
  return {
    tooltip: {
      formatter: (p: any) => `<div style="font-family:Geist Mono">${p.name}&nbsp;&nbsp;<b>${p.value}</b>&nbsp;&nbsp;${p.percent}%</div>`,
    },
    legend: { show: false },
    graphic: totalReq.value > 0 ? {
      type: 'group',
      left: 'center', top: 'middle',
      children: [
        {
          type: 'text', left: 'center', top: 'center',
          style: { text: totalReq.value.toLocaleString(), fill: '#ededed', font: '600 28px Geist Mono', textAlign: 'center' },
        },
        {
          type: 'text', left: 'center', top: 28,
          style: { text: 'CALLS', fill: '#707070', font: '500 9px Geist', letterSpacing: 1 },
        },
      ],
    } : undefined,
    series: [{
      type: 'pie',
      radius: ['55%', '78%'],
      center: ['50%', '50%'],
      avoidLabelOverlap: false,
      itemStyle: { borderColor: '#000', borderWidth: 2 },
      label: { show: false },
      emphasis: { scale: true, scaleSize: 4, label: { show: false } },
      data,
    }],
  }
})

function highlightLegend(idx: number) {
  const chart = panelRef.value?.getInstance()
  if (chart) chart.dispatchAction({ type: 'highlight', seriesIndex: 0, dataIndex: idx })
}
function downplayLegend(idx: number) {
  const chart = panelRef.value?.getInstance()
  if (chart) chart.dispatchAction({ type: 'downplay', seriesIndex: 0, dataIndex: idx })
}
</script>

<template>
  <Tile>
    <div class="tile__head"><span class="t-display">{{ title }}</span></div>
    <div v-if="top.length" class="donut-row">
      <EChartsPanel ref="panelRef" :option="option" :height="200" class="chart--donut" />
      <div class="donut-legend">
        <div
          v-for="(m, i) in top"
          :key="m.model"
          class="donut-legend__item"
          @mouseenter="highlightLegend(i)"
          @mouseleave="downplayLegend(i)"
        >
          <span class="dot" :style="{ background: palette[i % palette.length] }"></span>
          <div class="donut-legend__label">
            <span class="t-body-strong">{{ m.model }}</span>
            <span class="t-label tertiary">{{ m.requests.toLocaleString() }} calls · {{ totalReq ? ((m.requests / totalReq) * 100).toFixed(0) : 0 }}%</span>
          </div>
          <span class="t-num-strong">${{ (m.costUsd || 0).toFixed(2) }}</span>
        </div>
      </div>
    </div>
    <div v-else class="t-label tertiary" style="padding: 24px 0">{{ emptyHint }}</div>
  </Tile>
</template>

<style scoped>
.donut-row { display: grid; grid-template-columns: 200px 1fr; gap: 24px; align-items: center; }
.chart--donut { height: 200px; }
.donut-legend { display: flex; flex-direction: column; gap: 12px; }
.donut-legend__item {
  display: grid;
  grid-template-columns: 8px 1fr auto;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 150ms ease;
}
.donut-legend__item:hover { background: rgba(255,255,255,0.04); }
.donut-legend__label { display: flex; flex-direction: column; gap: 2px; }
.dot { width: 8px; height: 8px; border-radius: 50%; }
@media (max-width: 768px) {
  .donut-row { grid-template-columns: 1fr; gap: 16px; }
  .chart--donut { height: 220px; }
}
</style>
