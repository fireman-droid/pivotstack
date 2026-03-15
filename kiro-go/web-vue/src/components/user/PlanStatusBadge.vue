<script setup>
import { computed } from 'vue'

const props = defineProps({
  plan: String,
  balance: Number,
  expiresAt: Number,
})

const status = computed(() => {
  const now = Date.now() / 1000
  if (props.expiresAt && now > props.expiresAt) return 'expired'
  if (props.expiresAt && props.expiresAt - now < 86400 * 3) return 'expiring'
  if (props.plan !== 'timed' && props.balance < 5) return 'low'
  return 'active'
})

const configs = {
  active:   { label: '正常',     bg: 'rgba(34,197,94,0.12)',  color: '#22c55e' },
  expiring: { label: '即将到期', bg: 'rgba(245,158,11,0.12)', color: '#f59e0b' },
  low:      { label: '余额不足', bg: 'rgba(245,158,11,0.12)', color: '#f59e0b' },
  expired:  { label: '已过期',   bg: 'rgba(239,68,68,0.12)',  color: '#ef4444' },
}
</script>

<template>
  <span class="plan-badge" :style="{ background: configs[status].bg, color: configs[status].color }">
    ● {{ configs[status].label }}
  </span>
</template>

<style scoped>
.plan-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 700;
}
</style>
