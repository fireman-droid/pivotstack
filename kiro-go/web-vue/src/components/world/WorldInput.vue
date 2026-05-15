<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: [String, Number], default: '' },
  label: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  type: { type: String, default: 'text' },     // text | password | number | email
  hint: { type: String, default: '' },
  error: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
  size: { type: String, default: 'md' },       // sm | md | lg
  prefix: { type: String, default: '' },
  suffix: { type: String, default: '' },
  monospace: { type: Boolean, default: false },
  align: { type: String, default: 'left' },    // left | center | right
})
const emit = defineEmits(['update:modelValue', 'blur', 'focus', 'enter'])

const onInput = (e) => emit('update:modelValue', e.target.value)
const onKeydown = (e) => { if (e.key === 'Enter') emit('enter', props.modelValue) }
const wrapClasses = computed(() => [
  `s-${props.size}`,
  { 'has-error': !!props.error, 'is-disabled': props.disabled, 'is-mono': props.monospace }
])
</script>

<template>
  <div class="world-field" :class="wrapClasses">
    <label v-if="label" class="field-label">{{ label }}</label>
    <div class="field-wrap">
      <span v-if="prefix || $slots.prefix" class="affix prefix">
        <slot name="prefix">{{ prefix }}</slot>
      </span>
      <input
        :value="modelValue"
        :placeholder="placeholder"
        :type="type"
        :disabled="disabled"
        class="field-input"
        :style="{ textAlign: align }"
        @input="onInput"
        @blur="$emit('blur', $event)"
        @focus="$emit('focus', $event)"
        @keydown="onKeydown"
      />
      <span v-if="suffix || $slots.append" class="affix suffix">
        <slot name="append">{{ suffix }}</slot>
      </span>
    </div>
    <div v-if="error" class="field-msg error">{{ error }}</div>
    <div v-else-if="hint" class="field-msg hint">{{ hint }}</div>
  </div>
</template>

<style scoped>
.world-field { display: flex; flex-direction: column; gap: 6px; width: 100%; }
.field-label {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--world-text-mute);
  letter-spacing: 0.04em;
}
.field-wrap {
  display: flex;
  align-items: center;
  background: var(--world-bg-card);
  border: 1px solid var(--world-border);
  border-radius: var(--world-radius-md);
  transition: all 220ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1));
  overflow: hidden;
  position: relative;
}
.affix {
  padding: 0 12px;
  font-size: 0.875rem;
  color: var(--world-text-mute);
  flex-shrink: 0;
}
.prefix { border-right: 1px solid var(--world-divider); }
.suffix { border-left: 1px solid var(--world-divider); }
.field-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  padding: 0 14px;
  height: 38px;
  font-family: var(--world-font-sans);
  font-size: 0.9rem;
  color: var(--world-text-primary);
  width: 100%;
  min-width: 0;
}
.is-mono .field-input { font-family: var(--world-font-mono); letter-spacing: 0.04em; }
.s-sm .field-input { height: 30px; font-size: 0.8125rem; }
.s-lg .field-input { height: 46px; font-size: 1rem; }
.field-input::placeholder { color: var(--world-text-dim); }
.field-input:disabled { cursor: not-allowed; }

.is-disabled .field-wrap { opacity: 0.55; cursor: not-allowed; }
.field-msg { font-size: 0.75rem; }
.field-msg.error { color: var(--world-error); }
.field-msg.hint  { color: var(--world-text-dim); }

/* === Reality 形态: 扫描线焦点 === */
[data-world="reality"] .field-wrap:focus-within {
  border-color: var(--world-accent);
  box-shadow: var(--world-focus-ring);
}
[data-world="reality"] .field-wrap:focus-within::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(120deg, transparent 0%, rgba(2, 132, 199, 0.10) 50%, transparent 100%);
  background-size: 200% 100%;
  animation: medical-scan 1.2s linear infinite;
  pointer-events: none;
}
[data-world="reality"] .has-error .field-wrap {
  border-color: var(--world-error);
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.20);
}

/* === Daogui 形态: 墨笔下笔焦点 === */
[data-world="daogui"] .field-wrap {
  background: var(--world-elevated-bg);
  box-shadow: inset 0 0 14px rgba(0, 0, 0, 0.4);
}
[data-world="daogui"] .field-wrap:focus-within {
  border-color: var(--world-paper-aged);
  box-shadow:
    0 0 0 3px rgba(184, 134, 11, 0.18),
    0 0 14px rgba(196, 30, 58, 0.18);
}
[data-world="daogui"] .field-wrap:focus-within::before {
  content: '';
  position: absolute;
  bottom: 0; left: 0; right: 0;
  height: 1.5px;
  background: linear-gradient(90deg, transparent, var(--world-accent), var(--world-paper-aged), transparent);
  animation: ink-bleed 0.6s var(--world-ease, ease-out) forwards;
  background-size: 0% 100%;
  background-repeat: no-repeat;
}
[data-world="daogui"] .has-error .field-wrap {
  border-color: var(--world-error);
  box-shadow: 0 0 18px var(--world-vermilion-glow);
  animation: talisman-pulse 1.4s ease-in-out infinite;
}
</style>
