<script setup lang="ts">
import { computed } from 'vue'
import { AlertOctagon, X } from 'lucide-vue-next'
import { useNotificationStore } from '../../stores/notifications'

const store = useNotificationStore()
const emit = defineEmits<{ (e: 'open'): void }>()

const banner = computed(() => store.topCritical)

function dismiss(e: Event) {
  e.stopPropagation()
  if (banner.value?.dismissible) {
    store.dismiss(banner.value.id)
  }
}
</script>

<template>
  <Transition name="crit">
    <div v-if="banner" class="crit" @click="emit('open')">
      <span class="crit__lvl">
        <AlertOctagon :size="14" />
        CRITICAL
      </span>
      <span class="crit__msg">
        <strong>{{ banner.title }}</strong>
        <span class="crit__sep">·</span>
        <span class="crit__preview">{{ banner.body.replace(/[*_`#>]/g, '').slice(0, 80) }}</span>
      </span>
      <button class="crit__view" type="button">查看</button>
      <button
        v-if="banner.dismissible"
        class="crit__close"
        type="button"
        @click="dismiss"
        aria-label="关闭"
      >
        <X :size="14" />
      </button>
    </div>
  </Transition>
</template>

<style scoped>
.crit {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: linear-gradient(90deg, rgba(255, 77, 77, 0.12), rgba(255, 77, 77, 0.04));
  border-bottom: 1px solid rgba(255, 77, 77, 0.30);
  color: #ededed;
  font-size: 12px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
}
.crit::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 2px;
  background: #ff4d4d;
  box-shadow: 0 0 8px rgba(255, 77, 77, 0.6);
}

.crit__lvl {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  background: rgba(255, 77, 77, 0.16);
  color: #ff4d4d;
  font-weight: 700;
  font-size: 10px;
  letter-spacing: 0.10em;
  border-radius: 3px;
  flex-shrink: 0;
}

.crit__msg {
  flex: 1;
  min-width: 0;
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
}
.crit__msg strong { color: #ededed; font-weight: 600; }
.crit__sep { color: #707070; }
.crit__preview {
  color: #a1a1a1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.crit__view {
  background: rgba(255, 255, 255, 0.06);
  border: none;
  color: #ededed;
  font-size: 11px;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: 3px;
  cursor: pointer;
  flex-shrink: 0;
  font-family: inherit;
  transition: background 160ms ease;
}
.crit__view:hover { background: rgba(255, 255, 255, 0.12); }

.crit__close {
  width: 22px;
  height: 22px;
  background: transparent;
  border: none;
  color: #707070;
  border-radius: 3px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: background 160ms ease, color 160ms ease;
}
.crit__close:hover { background: rgba(255, 255, 255, 0.06); color: #ededed; }

.crit-enter-active, .crit-leave-active {
  transition: opacity 240ms ease, transform 240ms ease;
}
.crit-enter-from, .crit-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
