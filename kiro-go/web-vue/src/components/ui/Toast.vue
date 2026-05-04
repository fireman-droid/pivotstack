<script setup>
import { useToast } from '../../composables/useToast'
import { CheckCircle2, XCircle, AlertTriangle, Info } from 'lucide-vue-next'
const { toasts } = useToast()
</script>

<template>
  <Teleport to="body">
    <div class="toast-container">
      <TransitionGroup name="toast">
        <article
          v-for="t in toasts"
          :key="t.id"
          class="toast"
          :class="`v-${t.type || 'info'}`"
        >
          <span class="toast-icon">
            <CheckCircle2 v-if="t.type === 'success'" :size="16" />
            <XCircle v-else-if="t.type === 'error'" :size="16" />
            <AlertTriangle v-else-if="t.type === 'warning'" :size="16" />
            <Info v-else :size="16" />
          </span>
          <span class="toast-msg">{{ t.message }}</span>
        </article>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-container {
  position: fixed;
  top: 22px;
  right: 22px;
  z-index: 10000;
  display: flex;
  flex-direction: column;
  gap: 10px;
  pointer-events: none;
}

.toast {
  pointer-events: auto;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 16px;
  min-width: 240px;
  max-width: 420px;
  background: var(--world-glass-bg-strong);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  border: 1px solid var(--world-border);
  border-radius: var(--world-radius-lg);
  box-shadow: var(--world-shadow-lg);
  font-family: var(--world-font-sans);
}

.toast-icon { flex-shrink: 0; }
.toast-msg {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--world-text-primary);
  line-height: 1.45;
  flex: 1;
  word-break: break-word;
}

.v-success { border-left: 4px solid var(--world-success); }
.v-success .toast-icon { color: var(--world-success); }
.v-error   { border-left: 4px solid var(--world-error); }
.v-error   .toast-icon { color: var(--world-error); }
.v-warning { border-left: 4px solid var(--world-warning); }
.v-warning .toast-icon { color: var(--world-warning); }
.v-info    { border-left: 4px solid var(--world-info); }
.v-info    .toast-icon { color: var(--world-info); }

[data-world="daogui"] .v-error {
  box-shadow: 0 0 22px rgba(196, 30, 58, 0.28), var(--world-shadow-md);
  animation: talisman-pulse 2s ease-in-out infinite;
}
[data-world="daogui"] .v-success { box-shadow: 0 0 18px rgba(82, 121, 111, 0.18), var(--world-shadow-md); }

.toast-enter-active { transition: all 320ms cubic-bezier(0.34, 1.56, 0.64, 1); }
.toast-leave-active { transition: all 220ms ease; }
.toast-enter-from   { opacity: 0; transform: translateX(40px); }
.toast-leave-to     { opacity: 0; transform: translateX(40px); }
</style>
