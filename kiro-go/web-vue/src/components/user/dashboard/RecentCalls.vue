<script setup lang="ts">
import { useRouter } from 'vue-router'
import StatusDot from '../stellar/StatusDot.vue'
import Tile from '../stellar/Tile.vue'

export interface LogRow {
  time: string
  requestId: string
  model: string
  channel: string
  inputTokens: number
  outputTokens: number
  total: number
  costUsd: number
  durationMs: number
  status: 'ok' | 'warn' | 'err'
}

defineProps<{
  rows: LogRow[]
}>()

const router = useRouter()

function fmtNum(n: number) { return n.toLocaleString('en-US') }
function dollar(n: number) { return '$' + (n < 0.01 ? n.toFixed(3) : n.toFixed(2)) }
function latencyClass(ms: number) {
  if (ms < 150) return 'num--green'
  if (ms < 280) return ''
  return 'num--warn'
}
function shortRid(r: string) {
  return r.length > 10 ? r.slice(0, 8) : r
}
</script>

<template>
  <Tile class="tile--wide">
    <div class="tile__head tile__head--split">
      <div>
        <div class="t-display">最近调用</div>
        <div class="t-label tertiary">LATEST {{ rows.length }}</div>
      </div>
      <button class="btn btn--ghost btn--sm" @click="router.push('/user/logs')">全部日志 →</button>
    </div>
    <div class="table">
      <div class="table__head">
        <div style="width:80px">时间</div>
        <div style="width:100px">request_id</div>
        <div style="flex:1">模型</div>
        <div style="width:100px">上游</div>
        <div style="width:64px;text-align:right">in</div>
        <div style="width:64px;text-align:right">out</div>
        <div style="width:72px;text-align:right">total</div>
        <div style="width:72px;text-align:right">花费</div>
        <div style="width:80px;text-align:right">延迟</div>
        <div style="width:40px;text-align:center">状态</div>
      </div>
      <div class="table__body">
        <div v-for="r in rows" :key="r.requestId" class="table__row">
          <div class="time" style="width:80px">{{ r.time }}</div>
          <div style="width:100px" class="mono">{{ shortRid(r.requestId) }}</div>
          <div style="flex:1">{{ r.model }}</div>
          <div style="width:100px"><span class="chip chip--mono">{{ r.channel }}</span></div>
          <div class="num" style="width:64px;text-align:right">{{ fmtNum(r.inputTokens) }}</div>
          <div class="num" style="width:64px;text-align:right">{{ fmtNum(r.outputTokens) }}</div>
          <div class="num num--strong" style="width:72px;text-align:right">{{ fmtNum(r.total) }}</div>
          <div class="num num--green" style="width:72px;text-align:right">{{ dollar(r.costUsd) }}</div>
          <div class="num" :class="latencyClass(r.durationMs)" style="width:80px;text-align:right">{{ r.durationMs }}ms</div>
          <div style="width:40px;text-align:center"><StatusDot :status="r.status" /></div>
        </div>
        <div v-if="!rows.length" class="t-label tertiary" style="padding: 16px 12px">还没有调用记录，开始接入 API 后会显示</div>
      </div>
    </div>
  </Tile>
</template>
