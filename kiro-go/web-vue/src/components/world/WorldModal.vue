<script setup>
import { onUnmounted, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, default: '' },
  size: { type: String, default: 'md' }, // sm | md | lg | xl
  closable: { type: Boolean, default: true },
  closeOnBackdrop: { type: Boolean, default: true },
})
const emit = defineEmits(['update:modelValue', 'close'])

function close() {
  emit('update:modelValue', false)
  emit('close')
}
function onBackdrop() {
  if (props.closeOnBackdrop) close()
}

// ESC to close
function onEsc(e) {
  if (e.key === 'Escape' && props.modelValue && props.closable) close()
}
watch(() => props.modelValue, (open) => {
  if (open) {
    document.addEventListener('keydown', onEsc)
    document.body.style.overflow = 'hidden'
  } else {
    document.removeEventListener('keydown', onEsc)
    document.body.style.overflow = ''
  }
})
onUnmounted(() => {
  document.removeEventListener('keydown', onEsc)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <Transition name="world-modal">
      <div v-if="modelValue" class="world-modal-overlay" @click.self="onBackdrop">
        <div class="world-modal-card" :class="`s-${size}`" role="dialog" aria-modal="true">
          <header v-if="title || closable" class="modal-header">
            <h3 v-if="title" class="modal-title">{{ title }}</h3>
            <button v-if="closable" class="modal-close" @click="close" aria-label="关闭">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
                <path d="M4 4l10 10M14 4L4 14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
              </svg>
            </button>
          </header>
          <div class="modal-body">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="modal-footer">
            <slot name="footer" />
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.world-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgba(15, 23, 42, 0.45);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
}
[data-world="daogui"] .world-modal-overlay {
  background: rgba(10, 8, 7, 0.78);
  backdrop-filter: blur(12px) contrast(1.15);
  -webkit-backdrop-filter: blur(12px) contrast(1.15);
}
.world-modal-card {
  position: relative;
  background: var(--world-bg-card);
  border: 1px solid var(--world-border);
  border-radius: var(--world-radius-2xl);
  box-shadow: var(--world-shadow-2xl);
  width: 100%;
  max-width: 480px;
  max-height: calc(100vh - 32px);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.s-sm { max-width: 360px; }
.s-md { max-width: 480px; }
.s-lg { max-width: 720px; }
.s-xl { max-width: 960px; }

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 22px 12px;
  border-bottom: 1px solid var(--world-divider);
}
.modal-title {
  font-size: 1.05rem;
  font-weight: 800;
  color: var(--world-text-primary);
  margin: 0;
  font-family: var(--world-font-display, var(--world-font-sans));
}
.modal-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px; height: 28px;
  border-radius: var(--world-radius-md);
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--world-text-mute);
  transition: all 200ms ease;
}
.modal-close:hover {
  background: var(--world-overlay-light);
  color: var(--world-error);
}
.modal-body {
  padding: 22px;
  overflow-y: auto;
  flex: 1;
}
.modal-footer {
  padding: 14px 22px;
  border-top: 1px solid var(--world-divider);
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  background: var(--world-overlay-light);
}

/* === Daogui 形态: 心蟠中央展开 + 朱砂边 === */
[data-world="daogui"] .world-modal-card {
  background: var(--world-glass-bg-strong);
  border-color: rgba(184, 134, 11, 0.32);
  box-shadow:
    0 0 32px rgba(196, 30, 58, 0.18),
    var(--world-shadow-2xl);
}
[data-world="daogui"] .world-modal-card::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--world-accent), var(--world-paper-aged), var(--world-accent), transparent);
  opacity: 0.5;
}
[data-world="daogui"] .modal-title {
  color: var(--world-paper-aged);
  text-shadow: 0 0 8px rgba(184, 134, 11, 0.3);
}

/* === Transitions === */
.world-modal-enter-active { transition: opacity 220ms ease; }
.world-modal-enter-from   { opacity: 0; }
.world-modal-leave-active { transition: opacity 200ms ease; }
.world-modal-leave-to     { opacity: 0; }

[data-world="reality"] .world-modal-enter-active .world-modal-card {
  animation: rl-modal-in 280ms cubic-bezier(0.34, 1.56, 0.64, 1);
}
@keyframes rl-modal-in {
  from { transform: scale(0.96) translateY(8px); opacity: 0; }
  to   { transform: scale(1) translateY(0);     opacity: 1; }
}

[data-world="daogui"] .world-modal-enter-active .world-modal-card {
  animation: xinpan-bloom 540ms cubic-bezier(0.34, 1.56, 0.64, 1);
}
</style>
