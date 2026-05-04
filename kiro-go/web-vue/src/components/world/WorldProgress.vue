<script setup>
import { computed } from 'vue'

const props = defineProps({
  value: { type: Number, default: 0 },
  max: { type: Number, default: 100 },
  variant: { type: String, default: 'primary' }, // primary | success | warning | danger
  size: { type: String, default: 'md' },          // sm | md | lg
  showLabel: { type: Boolean, default: false },
  label: { type: String, default: '' },           // 字面文案，若 showLabel
  hint: { type: String, default: '' },            // 字面副文案
  indeterminate: { type: Boolean, default: false },
})
const pct = computed(() => {
  if (props.indeterminate) return 100
  return Math.min(100, Math.max(0, (props.value / props.max) * 100))
})
</script>

<template>
  <div class="world-progress" :class="`s-${size}`">
    <div v-if="showLabel || hint" class="progress-head">
      <span v-if="showLabel" class="progress-label">{{ label }}</span>
      <span v-if="hint" class="progress-hint">{{ hint }}</span>
    </div>
    <div class="progress-track" :class="{ 'is-indeterminate': indeterminate }">
      <div
        class="progress-fill"
        :class="`v-${variant}`"
        :style="{ width: `${pct}%` }"
      />
    </div>
  </div>
</template>

<style scoped>
.world-progress { display: flex; flex-direction: column; gap: 6px; width: 100%; }
.progress-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.75rem;
}
.progress-label { color: var(--world-text-primary); font-weight: 700; }
.progress-hint  { color: var(--world-text-mute); }

.progress-track {
  position: relative;
  width: 100%;
  height: 8px;
  background: var(--world-overlay-medium);
  border-radius: var(--world-radius-full);
  overflow: hidden;
}
.s-sm .progress-track { height: 5px; }
.s-lg .progress-track { height: 12px; }

.progress-fill {
  height: 100%;
  border-radius: inherit;
  transition: width 540ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1));
}
.v-primary { background: var(--world-accent); }
.v-success { background: var(--world-success); }
.v-warning { background: var(--world-warning); }
.v-danger  { background: var(--world-error); }

.is-indeterminate .progress-fill {
  position: absolute;
  width: 30% !important;
  animation: indet-slide 1.4s ease-in-out infinite;
}
@keyframes indet-slide {
  0%   { left: -30%; }
  100% { left: 100%; }
}

/* === Reality: 流光填充 === */
[data-world="reality"] .progress-fill {
  background-image: linear-gradient(90deg,
    var(--world-accent) 0%,
    var(--world-accent-soft, #38bdf8) 50%,
    var(--world-accent) 100%);
}

/* === Daogui: 朱印浮起 + 流光 === */
[data-world="daogui"] .progress-track {
  background: rgba(184, 134, 11, 0.10);
  box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.5);
}
[data-world="daogui"] .v-primary {
  background: linear-gradient(90deg, #8b1626 0%, #c41e3a 50%, #b8860b 100%);
  background-size: 200% 100%;
  animation: dg-progress-flow 2.4s linear infinite;
  box-shadow: 0 0 8px rgba(196, 30, 58, 0.4);
}
[data-world="daogui"] .v-success { background: linear-gradient(90deg, #3d6157, #52796f, #95b5a8); }
[data-world="daogui"] .v-warning { background: linear-gradient(90deg, #b8860b, #daa520, #f3c66e); }
[data-world="daogui"] .v-danger  { background: linear-gradient(90deg, #8b1626, #c41e3a, #f5707f); }

@keyframes dg-progress-flow {
  0%   { background-position: 0% 50%; }
  100% { background-position: 200% 50%; }
}
</style>
