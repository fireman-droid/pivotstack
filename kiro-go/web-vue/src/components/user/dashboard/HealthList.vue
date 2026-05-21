<script setup lang="ts">
// 上游健康列表 —— 每行一个渠道 + 状态点 + 延迟 + mini sparkline
// 注意：后端目前没暴露逐 channel 的实时延迟/错误率 metrics endpoint。
// 这里展示从 user logs 聚合出的"我使用过的渠道"健康度（基于最近调用的 status & duration_ms）。
import { computed } from 'vue'
import Tile from '../stellar/Tile.vue'
import StatusDot from '../stellar/StatusDot.vue'
import EChartsPanel from '../stellar/EChartsPanel.vue'
import { echarts, hexToRgba } from '../../../design/echarts-stellar'

export interface ChannelHealth {
  name: string
  alias?: string
  avgLatencyMs: number
  errorRate: number    // 0-1
  recentDurations: number[]   // 最近 N 次延迟，用作 sparkline
}

const props = defineProps<{
  channels: ChannelHealth[]
}>()

function statusOf(c: ChannelHealth): 'ok' | 'warn' | 'err' {
  if (c.errorRate >= 0.1 || c.avgLatencyMs >= 800) return 'err'
  if (c.errorRate >= 0.02 || c.avgLatencyMs >= 300) return 'warn'
  return 'ok'
}
function colorOf(s: 'ok' | 'warn' | 'err') {
  return s === 'ok' ? '#0bd470' : s === 'warn' ? '#f5a623' : '#ff4d4d'
}

function sparkOption(durations: number[], color: string) {
  // 确保至少 2 个点不然 echarts 区域不画
  const data = durations.length >= 2 ? durations : [...durations, ...durations]
  return {
    animation: false,
    grid: { left: 0, right: 0, top: 2, bottom: 2 },
    xAxis: { show: false, type: 'category', data: data.map(() => '') },
    yAxis: { show: false, type: 'value' },
    tooltip: { show: false },
    series: [{
      type: 'line', smooth: true, symbol: 'none',
      data,
      lineStyle: { width: 1, color },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: hexToRgba(color, 0.25) },
          { offset: 1, color: hexToRgba(color, 0) },
        ]),
      },
    }],
  }
}

const rows = computed(() => props.channels.map(c => {
  const s = statusOf(c)
  return { ...c, status: s, color: colorOf(s) }
}))

function chipCls(errorRate: number): string {
  if (errorRate >= 0.10) return 'chip--err'
  if (errorRate >= 0.02) return 'chip--warning'
  return 'chip--up'
}
</script>

<template>
  <Tile>
    <div class="tile__head tile__head--split">
      <div>
        <div class="t-display">上游健康</div>
        <div class="t-label tertiary">REAL-TIME · 基于你最近调用统计</div>
      </div>
    </div>
    <!-- 列头：明确每列含义 -->
    <div class="health-head">
      <span></span>
      <span>渠道</span>
      <span class="health-head__num" title="最近调用的平均响应延迟">平均延迟</span>
      <span class="health-head__num" title="最近调用里失败次数 / 总调用">错误率</span>
      <span class="health-head__num">延迟趋势</span>
    </div>
    <div class="health-list">
      <div v-for="r in rows" :key="r.name" class="health-row" :title="`${r.alias || r.name} · ${r.status === 'ok' ? '正常' : r.status === 'warn' ? '降级' : '异常'}`">
        <StatusDot :status="r.status" :pulse="r.status === 'ok'" />
        <span class="health-row__name">{{ r.alias || r.name }}</span>
        <span class="health-row__lat">{{ Math.round(r.avgLatencyMs) }}ms</span>
        <span class="chip" :class="chipCls(r.errorRate)" :title="`错误率 ${(r.errorRate * 100).toFixed(1)}%（${Math.round(r.errorRate * 100)} 错 / 100 调用）`">
          {{ (r.errorRate * 100).toFixed(1) }}%
        </span>
        <div class="health-row__spark">
          <EChartsPanel :option="sparkOption(r.recentDurations, r.color)" :height="24" />
        </div>
      </div>
      <div v-if="!rows.length" class="t-label tertiary" style="padding: 12px 4px">还没有可统计的上游</div>
    </div>
  </Tile>
</template>

<style scoped>
.health-head, .health-list { display: flex; flex-direction: column; }
.health-head {
  display: grid;
  grid-template-columns: 12px 1fr 60px 56px 56px;
  align-items: center;
  gap: 12px;
  padding: 4px 4px 6px;
  color: #707070;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.health-head__num { text-align: right; }
.health-row {
  display: grid;
  grid-template-columns: 12px 1fr 60px 56px 56px;
  align-items: center;
  gap: 12px;
  height: 48px;
  padding: 0 4px;
  border-radius: 4px;
  transition: background 150ms ease;
}
.health-row:hover { background: rgba(255,255,255,0.02); }
.health-row__name {
  color: var(--st-text-pri); font-weight: 500; font-size: 13px;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.health-row__lat {
  font-family: var(--st-font-mono); font-size: 12px; color: var(--st-text-sec);
  text-align: right; font-variant-numeric: tabular-nums;
}
.health-row__spark { width: 56px; height: 24px; justify-self: end; }
.health-row + .health-row { border-top: 1px solid rgba(255,255,255,0.04); }
</style>
