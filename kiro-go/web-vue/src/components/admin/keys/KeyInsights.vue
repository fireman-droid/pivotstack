<script setup lang="ts">
// KeyDetail 概览 Tab 的"画像"子组件：sparkline + 模型分布 + 时段热力
// 数据全部从父组件传入（logs），不自己调 API
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import type { ApiKeyLogEntry } from '../../../api/admin/keys'

const props = defineProps<{ logs: ApiKeyLogEntry[] }>()

const elSpark = ref<HTMLDivElement | null>(null)
const elDonut = ref<HTMLDivElement | null>(null)
const elHeat = ref<HTMLDivElement | null>(null)
let chSpark: echarts.ECharts | null = null
let chDonut: echarts.ECharts | null = null
let chHeat: echarts.ECharts | null = null

// 7d 消耗
const spark7d = computed(() => {
  const buckets: Record<string, number> = {}
  const now = new Date()
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now); d.setHours(0, 0, 0, 0); d.setDate(d.getDate() - i)
    buckets[d.toISOString().slice(0, 10)] = 0
  }
  for (const l of props.logs) {
    if (!l.timestamp) continue
    const k = new Date(l.timestamp * 1000).toISOString().slice(0, 10)
    if (k in buckets) buckets[k] += (l.paid_credits || 0) + (l.gifted_credits || 0)
  }
  return Object.entries(buckets).map(([date, val]) => ({ date, val }))
})

// 模型分布 top5
const modelDist = computed(() => {
  const m: Record<string, number> = {}
  for (const l of props.logs) {
    const k = l.original_model || 'unknown'
    m[k] = (m[k] || 0) + 1
  }
  return Object.entries(m).sort((a, b) => b[1] - a[1]).slice(0, 5).map(([name, count]) => ({ name, count }))
})
const modelTotal = computed(() => modelDist.value.reduce((s, m) => s + m.count, 0))

// 24h 时段热力（按小时聚合，不分星期）
const hourly = computed(() => {
  const grid = new Array(24).fill(0)
  for (const l of props.logs) {
    if (!l.timestamp) continue
    const h = new Date(l.timestamp * 1000).getHours()
    grid[h] += 1
  }
  return grid
})

const colors = ['#0bd470', '#52a8ff', '#f5a623', '#ff7a7a', '#a1a1a1']

function dispose() {
  ;[chSpark, chDonut, chHeat].forEach(c => c?.dispose())
  chSpark = chDonut = chHeat = null
}

function render() {
  dispose()
  if (elSpark.value) {
    chSpark = echarts.init(elSpark.value, 'stellar')
    chSpark.setOption({
      grid: { left: 4, right: 4, top: 4, bottom: 16, containLabel: true },
      xAxis: { type: 'category', data: spark7d.value.map(p => p.date.slice(5)), axisLabel: { fontSize: 9, color: '#707070' }, axisLine: { show: false }, axisTick: { show: false } },
      yAxis: { type: 'value', show: false },
      tooltip: { trigger: 'axis', formatter: (p: any) => `${p[0].axisValue}: ${p[0].value.toFixed(2)} credits` },
      series: [{
        type: 'line', smooth: true, symbol: 'circle', symbolSize: 3,
        data: spark7d.value.map(p => Number(p.val.toFixed(2))),
        lineStyle: { width: 1.5, color: '#0bd470' },
        itemStyle: { color: '#0bd470' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(11,212,112,0.30)' },
            { offset: 1, color: 'rgba(11,212,112,0)' },
          ]),
        },
      }],
    })
  }
  if (elDonut.value) {
    chDonut = echarts.init(elDonut.value, 'stellar')
    chDonut.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
      series: [{
        type: 'pie',
        radius: ['55%', '80%'],
        center: ['50%', '50%'],
        avoidLabelOverlap: false,
        label: { show: false },
        labelLine: { show: false },
        itemStyle: { borderColor: '#0a0a0a', borderWidth: 2 },
        data: modelDist.value.map((m, i) => ({
          value: m.count, name: m.name,
          itemStyle: { color: colors[i % colors.length] },
        })),
      }],
    })
  }
  if (elHeat.value) {
    chHeat = echarts.init(elHeat.value, 'stellar')
    const max = Math.max(1, ...hourly.value)
    chHeat.setOption({
      tooltip: { formatter: (p: any) => `${String(p.data[0]).padStart(2, '0')}:00 — ${p.data[1]} calls` },
      grid: { left: 16, right: 8, top: 4, bottom: 18, containLabel: true },
      xAxis: {
        type: 'category',
        data: Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0')),
        axisLabel: { fontSize: 9, color: '#707070', interval: 2 },
        axisLine: { show: false }, axisTick: { show: false },
      },
      yAxis: { type: 'category', data: [''], show: false },
      visualMap: {
        show: false,
        min: 0, max,
        inRange: { color: ['rgba(11,212,112,0.06)', 'rgba(11,212,112,0.28)', 'rgba(11,212,112,0.55)', '#0bd470'] },
      },
      series: [{
        type: 'heatmap',
        data: hourly.value.map((v, i) => [i, 0, v]),
        itemStyle: { borderRadius: 2 },
      }],
    })
  }
}

watch([spark7d, modelDist, hourly], () => nextTick(render))
function onResize() { ;[chSpark, chDonut, chHeat].forEach(c => c?.resize()) }
onMounted(() => { nextTick(render); window.addEventListener('resize', onResize) })
onBeforeUnmount(() => { dispose(); window.removeEventListener('resize', onResize) })
</script>

<template>
  <div class="ki">
    <div class="ki__tile">
      <div class="ki__title">
        <span class="t-h-admin">7 天消耗</span>
        <span class="t-label tertiary">CREDITS</span>
      </div>
      <div ref="elSpark" class="ki__chart" />
    </div>

    <div class="ki__tile">
      <div class="ki__title">
        <span class="t-h-admin">模型分布</span>
        <span class="t-label tertiary">TOP 5</span>
      </div>
      <div class="ki__donut-row">
        <div ref="elDonut" class="ki__chart ki__chart--donut" />
        <div class="ki__legend">
          <div v-for="(m, i) in modelDist" :key="m.name" class="ki__legend-item">
            <span class="ki__dot" :style="{ background: colors[i % colors.length] }" />
            <div class="ki__legend-text">
              <span class="ki__model-name">{{ m.name }}</span>
              <span class="t-label tertiary">{{ m.count }} · {{ modelTotal ? ((m.count / modelTotal) * 100).toFixed(0) : 0 }}%</span>
            </div>
          </div>
          <div v-if="!modelDist.length" class="t-label tertiary">暂无数据</div>
        </div>
      </div>
    </div>

    <div class="ki__tile ki__tile--wide">
      <div class="ki__title">
        <span class="t-h-admin">24h 调用时段</span>
        <span class="t-label tertiary">绿色深 = 高峰</span>
      </div>
      <div ref="elHeat" class="ki__chart ki__chart--heat" />
    </div>
  </div>
</template>

<style scoped>
.ki {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-top: 12px;
}
.ki__tile {
  background: var(--st-bg-surface);
  border: 1px solid var(--st-border);
  border-radius: 6px;
  padding: 14px 16px;
}
.ki__tile--wide { grid-column: 1 / -1; }
.ki__title {
  display: flex; align-items: baseline; gap: 8px;
  margin-bottom: 10px;
}
.ki__chart { height: 120px; }
.ki__chart--donut { width: 140px; height: 140px; flex-shrink: 0; }
.ki__chart--heat { height: 80px; }

.ki__donut-row {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 12px;
  align-items: center;
}
.ki__legend { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.ki__legend-item { display: flex; align-items: center; gap: 8px; }
.ki__dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.ki__legend-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.ki__model-name {
  font-size: 12px; color: var(--st-text-pri); font-weight: 500;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}

@media (max-width: 880px) {
  .ki { grid-template-columns: 1fr; }
}
</style>
