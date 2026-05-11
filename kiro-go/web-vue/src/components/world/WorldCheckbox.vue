<script setup>
import { computed } from 'vue'
import { Check } from 'lucide-vue-next'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  label: { type: String, default: '' },
  hint: { type: String, default: '' },
  size: { type: String, default: 'sm' },   // sm | md
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'change'])

const wrapClass = computed(() => [
  `s-${props.size}`,
  { 'is-checked': props.modelValue, 'is-disabled': props.disabled }
])

function toggle() {
  if (props.disabled) return
  const v = !props.modelValue
  emit('update:modelValue', v)
  emit('change', v)
}
function onKey(e) {
  if (e.key === ' ' || e.key === 'Enter') {
    e.preventDefault()
    toggle()
  }
}
</script>

<template>
  <label
    class="world-checkbox"
    :class="wrapClass"
    :tabindex="disabled ? -1 : 0"
    role="checkbox"
    :aria-checked="modelValue"
    @click="toggle"
    @keydown="onKey"
  >
    <span class="cb-box">
      <Check v-if="modelValue" class="cb-tick" :size="size === 'md' ? 14 : 12" />
    </span>
    <span v-if="label || $slots.default" class="cb-text">
      <span class="cb-label"><slot>{{ label }}</slot></span>
      <span v-if="hint" class="cb-hint">{{ hint }}</span>
    </span>
  </label>
</template>

<style scoped>
.world-checkbox {
  display: inline-flex;
  align-items: flex-start;
  gap: 8px;
  cursor: pointer;
  user-select: none;
  font-family: var(--world-font-sans);
  outline: none;
}
.world-checkbox.is-disabled { opacity: 0.5; cursor: not-allowed; }
.world-checkbox:focus-visible .cb-box {
  box-shadow: var(--world-focus-ring, 0 0 0 3px rgba(2, 132, 199, 0.20));
}

.cb-box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 4px;
  background: var(--world-overlay-light);
  border: 1.5px solid var(--world-glass-border);
  flex-shrink: 0;
  transition: background 200ms, border-color 200ms, transform 160ms;
  margin-top: 1px;
}
.s-md .cb-box { width: 18px; height: 18px; border-radius: 5px; }
.world-checkbox:hover:not(.is-disabled) .cb-box {
  border-color: var(--world-accent);
}
.world-checkbox.is-checked .cb-box {
  background: var(--world-accent);
  border-color: var(--world-accent);
}
.world-checkbox.is-checked:hover:not(.is-disabled) .cb-box {
  transform: scale(1.04);
}
.cb-tick {
  color: #fff;
  stroke-width: 3;
  animation: cb-pop 180ms cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes cb-pop {
  from { opacity: 0; transform: scale(0.5); }
  to   { opacity: 1; transform: scale(1); }
}

.cb-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.cb-label {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--world-text-primary);
  line-height: 1.4;
}
.s-md .cb-label { font-size: 0.9rem; }
.cb-hint {
  font-size: 0.72rem;
  color: var(--world-text-mute);
  line-height: 1.4;
}

/* Daogui 形态 */
[data-world="daogui"] .world-checkbox.is-checked .cb-box {
  box-shadow: 0 0 8px rgba(196, 30, 58, 0.45);
}
</style>
