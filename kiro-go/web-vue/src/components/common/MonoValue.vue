<script setup lang="ts">
import CopyableText from './CopyableText.vue'

defineProps<{
  value: string
  copyable?: boolean
}>()
</script>

<template>
  <CopyableText v-if="copyable" :text="value" mono />
  <span v-else class="mono-value">{{ value }}</span>
</template>

<style scoped>
.mono-value {
  font-family: 'Geist Mono', 'JetBrains Mono', ui-monospace, monospace;
  font-size: 13px;
  color: var(--color-text-secondary, #a1a1a1);
  font-feature-settings: 'tnum';
}
</style>
