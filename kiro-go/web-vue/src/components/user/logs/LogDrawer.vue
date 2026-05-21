<script setup lang="ts">
// 调用日志详情抽屉
import { computed } from 'vue'
import { X } from 'lucide-vue-next'
import EChartsPanel from '../stellar/EChartsPanel.vue'
import { fmtCost } from '../../../utils/format'

export interface LogDetail {
  request_id?: string
  time?: string
  timestamp?: number
  original_model?: string
  actual_model?: string
  channel_id?: string
  channel_alias?: string
  input_tokens?: number
  output_tokens?: number
  duration_ms?: number
  cost_usd?: number
  status?: string
  error?: string
}

const props = defineProps<{
  log: LogDetail | null
  // 同模型最近 N 次延迟，用作直方图基线
  baseline: number[]
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

function fmtTime(l: LogDetail) {
  const t = l.timestamp ? new Date(l.timestamp * 1000) : (l.time ? new Date(l.time) : null)
  if (!t) return '-'
  return t.toLocaleString('zh-CN', { hour12: false })
}

const total = computed(() => (props.log?.input_tokens || 0) + (props.log?.output_tokens || 0))

// 把 baseline 分桶（10 个 bucket: 0-50, 50-100, ... 500+）
const histogram = computed(() => {
  const buckets = new Array(10).fill(0)
  const cur = props.log?.duration_ms || 0
  let currentBucket = -1
  for (const v of props.baseline) {
    const i = Math.min(9, Math.floor(v / 50))
    buckets[i] += 1
  }
  currentBucket = Math.min(9, Math.floor(cur / 50))
  return { buckets, currentBucket }
})

const histOpt = computed(() => {
  const { buckets, currentBucket } = histogram.value
  return {
    animation: true,
    grid: { left: 0, right: 0, top: 18, bottom: 18, containLabel: false },
    xAxis: {
      type: 'category',
      data: ['0', '50', '100', '150', '200', '250', '300', '400', '500', '1s+'],
      axisLabel: { color: '#707070', fontSize: 9 },
      axisLine: { show: false },
    },
    yAxis: { show: false, type: 'value' },
    tooltip: { show: false },
    series: [{
      type: 'bar',
      barWidth: '65%',
      data: buckets.map((v: number, i: number) => ({
        value: v,
        itemStyle: { color: i === currentBucket ? '#0bd470' : 'rgba(255,255,255,0.18)' },
        label: i === currentBucket ? {
          show: true, position: 'top',
          color: '#0bd470', fontSize: 10, fontFamily: 'Geist Mono',
          formatter: '本次',
        } : { show: false },
      })),
    }],
  }
})
</script>

<template>
  <aside v-if="log" class="drawer">
    <div class="drawer__head">
      <div>
        <div class="t-label tertiary">REQUEST</div>
        <div class="mono drawer__rid">{{ log.request_id || '-' }}</div>
      </div>
      <button class="btn btn--ghost btn--icon" @click="emit('close')"><X :size="14" /></button>
    </div>

    <div class="drawer__sect">
      <div class="kv-row"><span class="t-label">时间</span><span class="mono t-body">{{ fmtTime(log) }}</span></div>
      <div class="kv-row"><span class="t-label">模型</span><span class="t-body">{{ log.original_model || log.actual_model || '-' }}</span></div>
      <div class="kv-row"><span class="t-label">路由上游</span><span class="chip chip--mono">{{ log.channel_alias || log.channel_id || '-' }}</span></div>
      <div v-if="log.error" class="kv-row kv-row--block"><span class="t-label">错误</span><span class="t-body t-body--err">{{ log.error }}</span></div>
    </div>

    <div class="drawer__sect">
      <div class="t-label">TOKENS</div>
      <div class="tok-row"><span>input</span><span class="mono">{{ (log.input_tokens || 0).toLocaleString() }}</span></div>
      <div class="tok-row"><span>output</span><span class="mono">{{ (log.output_tokens || 0).toLocaleString() }}</span></div>
      <div class="hairline"></div>
      <div class="tok-row tok-row--total"><span class="t-body-strong">total</span><span class="mono t-num-strong">{{ total.toLocaleString() }}</span></div>
    </div>

    <div class="drawer__sect">
      <div class="t-label">COST</div>
      <div class="tok-row"><span>总花费</span><span class="mono" style="color: var(--st-success)">{{ fmtCost(log.cost_usd) }}</span></div>
    </div>

    <div class="drawer__sect">
      <div class="t-label">延迟 ({{ log.duration_ms || 0 }}ms) vs 同模型 {{ baseline.length }} 次</div>
      <EChartsPanel :option="histOpt" :height="100" />
    </div>
  </aside>
</template>

<style scoped>
.drawer {
  background: var(--st-bg-surface);
  border-radius: 8px;
  padding: 20px;
  position: sticky; top: 80px;
  max-height: calc(100vh - 100px);
  overflow-y: auto;
}
.drawer__head {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 16px;
}
.drawer__rid { font-size: 13px; color: var(--st-text-pri); margin-top: 2px; word-break: break-all; }
.drawer__sect { padding: 12px 0; border-top: 1px solid var(--st-border); }
.drawer__sect:first-of-type { border-top: none; padding-top: 0; }
.drawer__sect .t-label { display: block; margin-bottom: 8px; }
.kv-row { display: flex; align-items: center; justify-content: space-between; min-height: 28px; gap: 12px; }
.kv-row .t-label { flex-shrink: 0; }
.kv-row .t-body { min-width: 0; text-align: right; word-break: break-all; line-height: 1.5; }
/* 长 error / 长文本时改成 label 在上、内容换行在下，不要挤同一行 */
.kv-row--block { flex-direction: column; align-items: stretch; gap: 4px; }
.kv-row--block .t-body { text-align: left; }
.t-body--err {
  color: var(--st-error);
  font-family: var(--st-font-mono, "Geist Mono", monospace);
  font-size: 11px;
  padding: 6px 8px;
  background: rgba(255, 87, 87, 0.06);
  border: 1px solid rgba(255, 87, 87, 0.20);
  border-radius: 3px;
  white-space: pre-wrap;
}
.tok-row { display: flex; align-items: center; justify-content: space-between; height: 26px; font-size: 12px; color: var(--st-text-sec); }
.tok-row .mono { font-size: 12px; color: var(--st-text-pri); }
.tok-row--total { height: 32px; margin-top: 4px; }
.tok-row--total .mono { font-size: 14px; }
@media (max-width: 1024px) {
  .drawer { position: relative; top: 0; max-height: none; }
}
</style>
