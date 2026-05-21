<script setup lang="ts">
defineProps<{
  icon?: string         // emoji 或单字符；正式版后续换 lucide
  title: string
  desc?: string
}>()
</script>

<template>
  <div class="empty">
    <div class="empty__icon" v-if="icon">{{ icon }}</div>
    <div class="empty__title">{{ title }}</div>
    <div v-if="desc" class="empty__desc">{{ desc }}</div>
    <div class="empty__cta">
      <slot name="cta" />
    </div>
  </div>
</template>

<style scoped>
.empty {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 8px; padding: 48px 24px;
  border: 1px dashed rgba(255,255,255,0.08);
  border-radius: 8px;
  text-align: center;
  color: var(--color-text-tertiary, #707070);
}
.empty__icon {
  font-size: 32px; line-height: 1; margin-bottom: 4px;
  opacity: 0.6;
}
.empty__title {
  font-size: 14px; font-weight: 500;
  color: var(--color-text-secondary, #a1a1a1);
}
.empty__desc { font-size: 13px; color: var(--color-text-tertiary, #707070); }
.empty__cta { margin-top: 12px; }
</style>
