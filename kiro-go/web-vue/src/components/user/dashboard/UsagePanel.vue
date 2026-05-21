<script setup lang="ts">
import { TrendingUp } from 'lucide-vue-next'
import Tile from '../stellar/Tile.vue'
import CounterNumber from '../stellar/CounterNumber.vue'

defineProps<{
  today: number
  todayDelta?: number       // 百分比，>0 表示涨
  thisMonth: number
  monthCost?: number        // USD
  total: number
  sinceLabel?: string       // e.g. "since 2026-01"
}>()
</script>

<template>
  <Tile>
    <div class="tile__head"><span class="t-label">USAGE</span></div>

    <div class="usage-row">
      <span class="usage-row__label t-body">今日</span>
      <CounterNumber :value="today" class="usage-row__num t-num-strong" />
      <span v-if="todayDelta !== undefined" class="chip" :class="(todayDelta ?? 0) >= 0 ? 'chip--up' : 'chip--err'">
        <TrendingUp :size="11" /> {{ (todayDelta ?? 0) >= 0 ? '+' : '' }}{{ (todayDelta ?? 0).toFixed(0) }}%
      </span>
      <span v-else class="chip chip--mono">today</span>
    </div>
    <div class="hairline"></div>

    <div class="usage-row">
      <span class="usage-row__label t-body">本月</span>
      <CounterNumber :value="thisMonth" class="usage-row__num t-num-strong" />
      <span v-if="monthCost !== undefined" class="chip chip--mono">${{ (monthCost ?? 0).toFixed(2) }}</span>
      <span v-else class="chip chip--mono">month</span>
    </div>
    <div class="hairline"></div>

    <div class="usage-row">
      <span class="usage-row__label t-body">累计</span>
      <CounterNumber :value="total" class="usage-row__num t-num-strong" />
      <span class="chip chip--mono">{{ sinceLabel || 'all time' }}</span>
    </div>
  </Tile>
</template>

<style scoped>
.usage-row {
  display: grid;
  grid-template-columns: 56px 1fr auto;
  align-items: center;
  gap: 12px;
  height: 36px;
}
.usage-row__label { color: var(--st-text-sec); }
.usage-row__num { text-align: right; padding-right: 12px; color: var(--st-text-pri); }
</style>
