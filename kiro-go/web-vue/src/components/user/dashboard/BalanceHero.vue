<script setup lang="ts">
import { computed } from 'vue'
import { Plus } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import Tile from '../stellar/Tile.vue'
import LiveIndicator from '../stellar/LiveIndicator.vue'
import CounterNumber from '../stellar/CounterNumber.vue'
import { useSystemUnit } from '../../../composables/useSystemUnit'

const props = defineProps<{
  balance: number
  giftBalance: number
  dailyAvgUsd?: number
}>()

const router = useRouter()
const { toCny } = useSystemUnit()

const total = computed(() => props.balance + props.giftBalance)
const cny = computed(() => toCny(total.value).toFixed(2))
const daysLeft = computed(() => {
  if (!props.dailyAvgUsd || props.dailyAvgUsd <= 0) return '—'
  return Math.max(0, Math.floor(total.value / props.dailyAvgUsd))
})
</script>

<template>
  <Tile>
    <div class="tile__head">
      <span class="t-label">余额 BALANCE</span>
      <LiveIndicator mini :show-ago="false" />
    </div>
    <div class="hero-num">
      <CounterNumber
        :value="total"
        prefix="$"
        :decimals="2"
        class="t-hero-xl mono"
      />
    </div>
    <div class="t-body sub">≈ ¥{{ cny }} <template v-if="daysLeft !== '—'">·&nbsp;够 ~{{ daysLeft }} 天</template></div>
    <div class="hairline"></div>
    <div class="balance-split">
      <div class="balance-split__item">
        <span class="t-label tertiary">已充值</span>
        <span class="mono">${{ balance.toFixed(2) }}</span>
      </div>
      <div class="balance-split__divider"></div>
      <div class="balance-split__item">
        <span class="t-label tertiary">赠送</span>
        <span class="mono" style="color: var(--st-success)">${{ giftBalance.toFixed(2) }}</span>
      </div>
    </div>
    <button class="btn btn--primary btn--block" @click="router.push('/user/recharge')">
      <Plus :size="14" />立即充值
    </button>
  </Tile>
</template>

<style scoped>
.hero-num { display: flex; align-items: baseline; flex-wrap: wrap; margin: 8px 0; }
.balance-split {
  display: flex; align-items: center; gap: 20px;
  margin-bottom: 24px;
  padding: 4px 4px 0;
}
.balance-split__item { display: flex; flex-direction: column; gap: 6px; }
.balance-split__item .t-label { line-height: 1; }
.balance-split__item .mono { font-size: 15px; font-weight: 500; line-height: 1; letter-spacing: -0.01em; }
.balance-split__divider { width: 1px; height: 28px; background: var(--st-border); }
</style>
