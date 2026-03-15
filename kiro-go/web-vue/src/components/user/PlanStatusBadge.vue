<script setup>
import { computed } from 'vue'

const props = defineProps({
  plan: String,
  tier: String,
  balance: Number,
  expiresAt: Number,
})

const status = computed(() => {
  if (!props.plan) return 'not_activated'
  const now = Date.now() / 1000
  if (props.expiresAt && now > props.expiresAt) return 'expired'
  if (props.expiresAt && props.expiresAt - now < 86400 * 3) return 'expiring'
  if (props.plan !== 'timed' && props.balance < 5) return 'low'
  return 'active'
})

const configs = {
  not_activated: { label: '未激活', bg: 'rgba(107,114,128,0.12)', color: '#9ca3af' },
  active:        { label: '正常',   bg: 'rgba(34,197,94,0.12)',   color: '#22c55e' },
  expiring:      { label: '即将到期', bg: 'rgba(245,158,11,0.12)', color: '#f59e0b' },
  low:           { label: '余额不足', bg: 'rgba(245,158,11,0.12)', color: '#f59e0b' },
  expired:       { label: '已过期',   bg: 'rgba(239,68,68,0.12)',  color: '#ef4444' },
}

const tierLabel = computed(() => {
  if (props.tier === 'pro') return 'Pro'
  if (props.tier === 'free') return 'Free'
  return ''
})

const tierStyle = computed(() => {
  if (props.tier === 'pro') return { background: 'rgba(245,158,11,0.12)', color: '#f59e0b' }
  return { background: 'rgba(56,189,248,0.12)', color: '#38bdf8' }
})
</script>

<template>
  <div class="badges">
    <span class="plan-badge" :style="{ background: configs[status].bg, color: configs[status].color }">
      ● {{ configs[status].label }}
    </span>
    <span v-if="tierLabel" class="plan-badge" :style="tierStyle">
      {{ tier === 'pro' ? '👑' : '🔒' }} {{ tierLabel }}
    </span>
  </div>
</template>

<style scoped>
.badges {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
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
