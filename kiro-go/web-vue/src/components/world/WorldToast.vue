<script setup>
defineProps({
  toasts: { type: Array, required: true }, // [{ id, type ('success'|'error'|'warning'|'info'), message, duration? }]
})
defineEmits(['dismiss'])
</script>

<template>
  <Teleport to="body">
    <div class="world-toast-container">
      <Transition-group name="world-toast">
        <article
          v-for="t in toasts"
          :key="t.id"
          class="world-toast"
          :class="`v-${t.type || 'info'}`"
          @click="$emit('dismiss', t.id)"
        >
          <span class="toast-icon">
            <svg v-if="t.type === 'success'" width="18" height="18" viewBox="0 0 18 18" fill="none">
              <path d="M3 9l4 4 8-8" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <svg v-else-if="t.type === 'error'" width="18" height="18" viewBox="0 0 18 18" fill="none">
              <path d="M9 4v6M9 13.5v.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
              <circle cx="9" cy="9" r="7.5" stroke="currentColor" stroke-width="1.5" />
            </svg>
            <svg v-else-if="t.type === 'warning'" width="18" height="18" viewBox="0 0 18 18" fill="none">
              <path d="M9 2 L17 16 H1 Z M9 7v4M9 13v.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <svg v-else width="18" height="18" viewBox="0 0 18 18" fill="none">
              <circle cx="9" cy="9" r="7.5" stroke="currentColor" stroke-width="1.5" />
              <path d="M9 12V8M9 5.5v.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </span>
          <span class="toast-msg">{{ t.message }}</span>
        </article>
      </Transition-group>
    </div>
  </Teleport>
</template>

<style scoped>
.world-toast-container {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 10000;
  display: flex;
  flex-direction: column;
  gap: 10px;
  pointer-events: none;
  max-width: calc(100vw - 40px);
}

.world-toast {
  pointer-events: auto;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 16px;
  background: var(--world-bg-card);
  border: 1px solid var(--world-border);
  border-radius: var(--world-radius-lg);
  box-shadow: var(--world-shadow-lg);
  min-width: 240px;
  max-width: 420px;
  cursor: pointer;
  font-family: var(--world-font-sans);
  transition: all 220ms ease;
}
.world-toast:hover { transform: translateX(-3px); }

.toast-icon {
  flex-shrink: 0;
  width: 18px; height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.toast-msg {
  font-size: 0.875rem;
  color: var(--world-text-primary);
  line-height: 1.45;
  flex: 1;
  word-break: break-word;
}

/* === 类型边色 === */
.v-success { border-left: 4px solid var(--world-success); }
.v-success .toast-icon { color: var(--world-success); }
.v-error   { border-left: 4px solid var(--world-error); }
.v-error   .toast-icon { color: var(--world-error); }
.v-warning { border-left: 4px solid var(--world-warning); }
.v-warning .toast-icon { color: var(--world-warning); }
.v-info    { border-left: 4px solid var(--world-info); }
.v-info    .toast-icon { color: var(--world-info); }

/* === Reality 形态: 滑入 === */
[data-world="reality"] .world-toast {
  background: var(--world-glass-bg-strong);
  backdrop-filter: blur(var(--world-glass-blur));
}

/* === Daogui 形态: 朱砂晕染 === */
[data-world="daogui"] .world-toast {
  background: var(--world-glass-bg-strong);
  border-color: var(--world-glass-border);
  backdrop-filter: blur(var(--world-glass-blur));
  box-shadow:
    0 0 18px rgba(196, 30, 58, 0.12),
    var(--world-shadow-md);
}
[data-world="daogui"] .v-success {
  border-left-color: var(--world-success);
  box-shadow: 0 0 18px rgba(82, 121, 111, 0.18), var(--world-shadow-md);
}
[data-world="daogui"] .v-error {
  border-left-color: var(--world-error);
  box-shadow: 0 0 22px rgba(196, 30, 58, 0.28), var(--world-shadow-md);
  animation: talisman-pulse 2s ease-in-out infinite;
}
[data-world="daogui"] .v-warning {
  border-left-color: var(--world-warning);
  box-shadow: 0 0 18px rgba(218, 165, 32, 0.18), var(--world-shadow-md);
}

/* Transitions */
.world-toast-enter-active { transition: all 320ms cubic-bezier(0.34, 1.56, 0.64, 1); }
.world-toast-leave-active { transition: all 220ms ease; }
.world-toast-enter-from   { opacity: 0; transform: translateX(40px); }
.world-toast-leave-to     { opacity: 0; transform: translateX(40px); }
</style>
