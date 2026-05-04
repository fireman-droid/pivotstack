<script setup>
const props = defineProps({
  modelValue: { type: Boolean, default: false },
  disabled:   { type: Boolean, default: false },
  size:       { type: String,  default: 'md' }, // sm | md | lg
})
const emit = defineEmits(['update:modelValue'])
function onToggle(e) {
  if (props.disabled) return
  emit('update:modelValue', e.target.checked)
}
</script>

<template>
  <label class="ws-switch" :class="[`s-${size}`, { 'is-disabled': disabled }]">
    <input
      type="checkbox"
      :checked="modelValue"
      :disabled="disabled"
      @change="onToggle"
    />
    <span class="toggle-track">
      <span class="toggle-knob" />
    </span>
  </label>
</template>

<style scoped>
.ws-switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
  cursor: pointer;
  flex-shrink: 0;
}
.s-sm { width: 32px; height: 18px; }
.s-lg { width: 52px; height: 28px; }
.is-disabled { opacity: 0.5; cursor: not-allowed; }

.ws-switch input {
  position: absolute;
  opacity: 0;
  width: 100%;
  height: 100%;
  cursor: inherit;
  margin: 0;
}

.toggle-track {
  display: block;
  width: 100%;
  height: 100%;
  background: var(--world-border);
  border-radius: var(--world-radius-full);
  transition: background 220ms cubic-bezier(0.4, 0, 0.2, 1);
}

.toggle-knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  background: #ffffff;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.20);
  transition: transform 220ms cubic-bezier(0.4, 0, 0.2, 1);
}
.s-sm .toggle-knob { width: 14px; height: 14px; }
.s-lg .toggle-knob { width: 24px; height: 24px; }

/* === On state: sibling selectors propagate properly === */
.ws-switch input:checked ~ .toggle-track {
  background: var(--world-accent);
}
.ws-switch input:checked ~ .toggle-track .toggle-knob {
  transform: translateX(20px);
}
.s-sm input:checked ~ .toggle-track .toggle-knob { transform: translateX(14px); }
.s-lg input:checked ~ .toggle-track .toggle-knob { transform: translateX(24px); }

.ws-switch input:focus-visible ~ .toggle-track {
  box-shadow: var(--world-focus-ring);
}

/* === Daogui state polish === */
[data-world="daogui"] .ws-switch input:checked ~ .toggle-track {
  background: linear-gradient(135deg, var(--world-accent), var(--world-accent-deep, #8b1626));
  box-shadow: 0 0 12px rgba(196, 30, 58, 0.45);
}
[data-world="daogui"] .ws-switch input:checked ~ .toggle-track .toggle-knob {
  background: var(--world-paper-aged);
  box-shadow: 0 0 8px rgba(184, 134, 11, 0.6);
}
</style>
