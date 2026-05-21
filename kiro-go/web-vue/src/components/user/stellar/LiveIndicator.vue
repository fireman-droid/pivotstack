<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import StatusDot from './StatusDot.vue'

const props = defineProps<{
  showAgo?: boolean
  mini?: boolean
}>()

const seconds = ref(12)
let timer: number | undefined

onMounted(() => {
  timer = window.setInterval(() => {
    seconds.value = (seconds.value + 1) % 60
  }, 1000)
})

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<template>
  <div class="live" :class="{ 'live--mini': mini }">
    <StatusDot status="ok" pulse />
    <span :class="mini ? 't-micro' : 't-label'">Live</span>
    <span v-if="showAgo !== false" class="t-label live__ago">· {{ seconds }}s ago</span>
  </div>
</template>

<style>
.live { display: inline-flex; align-items: center; gap: 6px; }
.live--mini { gap: 4px; }
.live__ago { color: #707070; }
</style>
