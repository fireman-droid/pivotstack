<script setup lang="ts">
defineProps<{ label?: string }>()
</script>

<template>
  <div class="section-divider" :class="{ 'section-divider--label': label }">
    <span v-if="label" class="section-divider__label">{{ label }}</span>
    <span class="section-divider__line" />
  </div>
</template>

<style scoped>
.section-divider {
  display: flex; align-items: center; gap: 12px;
  margin: 24px 0 16px;
}
.section-divider__label {
  font-size: 13px; font-weight: 500;
  color: var(--color-text-secondary, #a1a1a1);
  letter-spacing: 0.02em;
}
.section-divider__line {
  flex: 1; height: 1px;
  background: rgba(255,255,255,0.10);
}
</style>
