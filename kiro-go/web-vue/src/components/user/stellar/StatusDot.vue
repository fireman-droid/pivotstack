<script setup lang="ts">
defineProps<{
  status?: 'ok' | 'warn' | 'err' | 'gray'
  pulse?: boolean
}>()
</script>

<template>
  <span
    class="dot"
    :class="[
      status === 'warn' ? 'dot--yellow' :
      status === 'err' ? 'dot--red' :
      status === 'gray' ? 'dot--gray' :
      'dot--green',
      pulse && status !== 'gray' ? 'dot--pulse' : '',
    ]"
  />
</template>

<style>
.dot { display: inline-block; width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; background: #707070; }
.dot--green { background: #0bd470; }
.dot--yellow { background: #f5a623; }
.dot--red { background: #ff4d4d; }
.dot--gray { background: #707070; }
.dot--pulse { animation: stellar-dot-pulse 2s ease-in-out infinite; }
@keyframes stellar-dot-pulse {
  0%, 100% { opacity: 0.3; }
  50%      { opacity: 1.0; }
}
</style>
