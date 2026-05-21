<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'

const props = withDefaults(defineProps<{
  value: number
  prefix?: string
  suffix?: string
  decimals?: number
  duration?: number
}>(), {
  prefix: '',
  suffix: '',
  decimals: 0,
  duration: 900,
})

const display = ref('')

function animate(from: number, to: number) {
  const start = performance.now()
  const dur = props.duration
  function step(now: number) {
    const t = Math.min(1, (now - start) / dur)
    const eased = 1 - Math.pow(1 - t, 3)
    const v = from + (to - from) * eased
    display.value = props.prefix + v.toFixed(props.decimals) + props.suffix
    if (t < 1) requestAnimationFrame(step)
    else display.value = props.prefix + to.toFixed(props.decimals) + props.suffix
  }
  requestAnimationFrame(step)
}

onMounted(() => animate(0, props.value))
watch(() => props.value, (next, prev) => animate(prev ?? 0, next))
</script>

<template>
  <span>{{ display }}</span>
</template>
