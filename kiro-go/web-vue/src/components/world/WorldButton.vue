<script setup>
import { ref } from 'vue'

const props = defineProps({
  variant: { type: String, default: 'primary' },   // primary | secondary | ghost | danger
  size:    { type: String, default: 'md' },        // sm | md | lg
  type:    { type: String, default: 'button' },
  disabled:{ type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  block:   { type: Boolean, default: false },
})
const emit = defineEmits(['click'])

const isSplashing = ref(false)
function onClick(e) {
  if (props.disabled || props.loading) return
  isSplashing.value = true
  setTimeout(() => { isSplashing.value = false }, 600)
  emit('click', e)
}
</script>

<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    class="world-btn"
    :class="[`v-${variant}`, `s-${size}`, { 'is-block': block, 'is-loading': loading }]"
    @click="onClick"
  >
    <span class="btn-content">
      <span v-if="loading" class="btn-spinner" />
      <slot />
    </span>
    <!-- Daogui: 血溅粒子 -->
    <span v-if="isSplashing" class="splash-layer" aria-hidden="true">
      <span v-for="n in 8" :key="n" class="particle" :style="{ '--angle': `${n * 45}deg`, '--delay': `${Math.random() * 0.15}s` }" />
    </span>
    <!-- Reality: 扫描线 -->
    <span v-if="isSplashing" class="scan-layer" aria-hidden="true" />
  </button>
</template>

<style scoped>
.world-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0 18px;
  height: 38px;
  border-radius: var(--world-radius-lg);
  font-family: var(--world-font-sans);
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  border: 1px solid transparent;
  cursor: pointer;
  transition: transform 180ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1)),
              box-shadow 220ms ease,
              filter 220ms ease,
              background 220ms ease,
              border-color 220ms ease;
  overflow: hidden;
  user-select: none;
  white-space: nowrap;
}
.world-btn:focus-visible { outline: none; box-shadow: var(--world-focus-ring); }
.world-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.is-block { display: flex; width: 100%; }

.s-sm { height: 30px; padding: 0 12px; font-size: 0.75rem; }
.s-md { height: 38px; padding: 0 18px; font-size: 0.875rem; }
.s-lg { height: 46px; padding: 0 24px; font-size: 0.95rem; }

.btn-content { position: relative; z-index: 1; display: inline-flex; align-items: center; gap: 8px; }
.btn-spinner {
  width: 14px; height: 14px; border-radius: 50%;
  border: 2px solid currentColor;
  border-right-color: transparent;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* === Variant: primary === */
.v-primary {
  background: var(--world-accent);
  color: #fff;
  border-color: var(--world-accent);
}
.v-primary:hover:not(:disabled) { filter: brightness(1.08); transform: translateY(-1px); }
.v-primary:active:not(:disabled) { transform: scale(0.98); }

/* === Variant: secondary === */
.v-secondary {
  background: transparent;
  color: var(--world-text-primary);
  border-color: var(--world-border);
}
.v-secondary:hover:not(:disabled) {
  background: var(--world-overlay-light);
  border-color: var(--world-accent);
  color: var(--world-accent);
}

/* === Variant: ghost === */
.v-ghost {
  background: transparent;
  color: var(--world-text-secondary);
  border-color: transparent;
}
.v-ghost:hover:not(:disabled) {
  background: var(--world-overlay-light);
  color: var(--world-text-primary);
}

/* === Variant: danger === */
.v-danger {
  background: var(--world-error);
  color: #fff;
  border-color: var(--world-error);
}
.v-danger:hover:not(:disabled) { filter: brightness(1.08); }

/* === Reality 形态: 医疗扫描线 === */
[data-world="reality"] .v-primary {
  box-shadow: 0 1px 2px rgba(2, 132, 199, 0.3);
}
[data-world="reality"] .v-primary:hover:not(:disabled) {
  box-shadow: 0 4px 12px rgba(2, 132, 199, 0.35);
}
[data-world="reality"] .scan-layer {
  position: absolute; inset: 0;
  background: linear-gradient(120deg, transparent 0%, rgba(255, 255, 255, 0.4) 50%, transparent 100%);
  background-size: 200% 100%;
  animation: medical-scan 0.6s ease-out;
  pointer-events: none;
}
[data-world="reality"] .splash-layer { display: none; }

/* === Daogui 形态: 朱砂血溅 === */
[data-world="daogui"] .v-primary {
  background: linear-gradient(135deg, #c41e3a 0%, #8b1626 100%);
  border-color: rgba(196, 30, 58, 0.6);
  box-shadow: 0 0 0 0 rgba(196, 30, 58, 0);
}
[data-world="daogui"] .v-primary:hover:not(:disabled) {
  filter: drop-shadow(0 0 8px rgba(196, 30, 58, 0.5));
  box-shadow: 0 0 18px rgba(196, 30, 58, 0.4);
}
[data-world="daogui"] .v-primary:active:not(:disabled) {
  transform: scale(0.96);
  filter: drop-shadow(0 0 14px rgba(196, 30, 58, 0.7));
}
[data-world="daogui"] .v-secondary {
  border-color: rgba(184, 134, 11, 0.30);
  color: var(--world-paper, #d4a574);
}
[data-world="daogui"] .v-secondary:hover:not(:disabled) {
  border-color: var(--world-paper-aged, #b8860b);
  background: rgba(184, 134, 11, 0.08);
  color: var(--world-paper-aged, #b8860b);
}
[data-world="daogui"] .splash-layer {
  position: absolute; inset: 0;
  pointer-events: none;
}
[data-world="daogui"] .splash-layer .particle {
  position: absolute;
  top: 50%; left: 50%;
  width: 4px; height: 12px;
  background: var(--color-vermilion-blood, #c41e3a);
  border-radius: 50%;
  transform: translate(-50%, -50%) rotate(var(--angle)) translateY(0);
  animation: btn-splash 0.6s ease-out forwards;
  animation-delay: var(--delay);
  filter: drop-shadow(0 0 4px rgba(196, 30, 58, 0.8));
}
@keyframes btn-splash {
  to {
    transform: translate(-50%, -50%) rotate(var(--angle)) translateY(-32px);
    opacity: 0;
  }
}
[data-world="daogui"] .scan-layer { display: none; }
</style>
