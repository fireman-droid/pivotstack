<script setup>
defineProps({
  variant: { type: String, default: 'neutral' }, // neutral | success | warning | danger | info | accent
  size: { type: String, default: 'md' },         // sm | md
  dot: { type: Boolean, default: false },
  pulse: { type: Boolean, default: false },
})
</script>

<template>
  <span class="world-chip" :class="[`v-${variant}`, `s-${size}`, { 'is-pulse': pulse }]">
    <span v-if="dot" class="chip-dot" />
    <slot />
  </span>
</template>

<style scoped>
.world-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: var(--world-radius-full);
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  border: 1px solid transparent;
  white-space: nowrap;
  transition: all 200ms ease;
}
.s-sm { padding: 2px 8px; font-size: 0.7rem; }
.chip-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 6px currentColor;
  flex-shrink: 0;
}

.is-pulse { animation: chip-pulse 2.4s ease-in-out infinite; }
@keyframes chip-pulse {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.7; }
}

/* === Reality variants === */
[data-world="reality"] .v-neutral {
  background: rgba(15, 23, 42, 0.06);
  color: var(--world-text-secondary);
  border-color: rgba(15, 23, 42, 0.10);
}
[data-world="reality"] .v-success {
  background: rgba(16, 185, 129, 0.10);
  color: #047857;
  border-color: rgba(16, 185, 129, 0.25);
}
[data-world="reality"] .v-warning {
  background: rgba(245, 158, 11, 0.10);
  color: #b45309;
  border-color: rgba(245, 158, 11, 0.25);
}
[data-world="reality"] .v-danger {
  background: rgba(239, 68, 68, 0.10);
  color: #b91c1c;
  border-color: rgba(239, 68, 68, 0.25);
}
[data-world="reality"] .v-info {
  background: rgba(2, 132, 199, 0.10);
  color: #0369a1;
  border-color: rgba(2, 132, 199, 0.25);
}
[data-world="reality"] .v-accent {
  background: var(--world-accent);
  color: #fff;
  border-color: var(--world-accent);
}

/* === Daogui variants === */
[data-world="daogui"] .v-neutral {
  background: rgba(184, 134, 11, 0.06);
  color: var(--world-text-mute);
  border-color: rgba(184, 134, 11, 0.18);
}
[data-world="daogui"] .v-success {
  background: rgba(82, 121, 111, 0.12);
  color: #95b5a8;             /* 铜绿亮一档 */
  border-color: rgba(82, 121, 111, 0.32);
}
[data-world="daogui"] .v-warning {
  background: rgba(218, 165, 32, 0.12);
  color: #f3c66e;             /* 赤金亮一档 */
  border-color: rgba(218, 165, 32, 0.32);
}
[data-world="daogui"] .v-danger {
  background: rgba(196, 30, 58, 0.14);
  color: #f5707f;             /* 朱砂亮一档以保 AA */
  border-color: rgba(196, 30, 58, 0.36);
}
[data-world="daogui"] .v-info {
  background: rgba(124, 58, 237, 0.12);
  color: #b39be8;
  border-color: rgba(124, 58, 237, 0.32);
}
[data-world="daogui"] .v-accent {
  background: linear-gradient(135deg, #c41e3a, #8b1626);
  color: #fff;
  border-color: rgba(196, 30, 58, 0.62);
  box-shadow: 0 0 12px rgba(196, 30, 58, 0.3);
}
</style>
