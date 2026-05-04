<script setup>
defineProps({
  modelValue: { type: [String, Number], default: '' },
  options: { type: Array, required: true }, // [{ value, label, icon? }]
  size: { type: String, default: 'md' },    // sm | md | lg
  block: { type: Boolean, default: false },
})
defineEmits(['update:modelValue'])
</script>

<template>
  <div class="world-segment" :class="[`s-${size}`, { 'is-block': block }]">
    <button
      v-for="opt in options"
      :key="opt.value"
      type="button"
      class="seg-item"
      :class="{ active: modelValue === opt.value }"
      @click="$emit('update:modelValue', opt.value)"
    >
      <component v-if="opt.icon" :is="opt.icon" class="seg-icon" />
      <span>{{ opt.label }}</span>
    </button>
  </div>
</template>

<style scoped>
.world-segment {
  display: inline-flex;
  align-items: center;
  padding: 4px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-lg);
  gap: 2px;
  position: relative;
}
.is-block { display: flex; width: 100%; }
.seg-item {
  flex: 1 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 6px 14px;
  background: transparent;
  border: none;
  border-radius: calc(var(--world-radius-lg) - 4px);
  color: var(--world-text-mute);
  font-size: 0.8125rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 220ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1));
  white-space: nowrap;
  font-family: var(--world-font-sans);
}
.s-sm .seg-item { padding: 4px 10px; font-size: 0.75rem; }
.s-lg .seg-item { padding: 9px 18px; font-size: 0.9rem; }
.seg-icon { width: 14px; height: 14px; }

.seg-item:hover { color: var(--world-text-primary); }
.seg-item.active { color: #fff; }

/* === Reality 形态 === */
[data-world="reality"] .seg-item.active {
  background: var(--world-accent);
  box-shadow: 0 1px 4px rgba(2, 132, 199, 0.3);
}

/* === Daogui 形态 === */
[data-world="daogui"] .seg-item.active {
  background: linear-gradient(135deg, #c41e3a, #8b1626);
  box-shadow:
    0 0 14px rgba(196, 30, 58, 0.4),
    inset 0 0 0 1px rgba(184, 134, 11, 0.4);
}
[data-world="daogui"] .seg-item:hover:not(.active) {
  background: rgba(184, 134, 11, 0.06);
  color: var(--world-paper-aged);
}
</style>
