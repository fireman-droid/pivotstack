<script setup lang="ts">
import { computed } from 'vue'
import { X } from 'lucide-vue-next'
import type { UserNotification } from '../../api/notifications'
import { useNotificationStore } from '../../stores/notifications'

const props = defineProps<{ notif: UserNotification | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const store = useNotificationStore()

const open = computed(() => !!props.notif)

function close() {
  emit('close')
}

function fmtTime(ts?: number) {
  if (!ts) return ''
  return new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false })
}

function dismiss() {
  if (props.notif) {
    store.dismiss(props.notif.id)
  }
  close()
}

// 极简 markdown 渲染：换行 + **加粗** + *斜体* + `code`
// 复杂的留给后续 markdown lib（避免 XSS：仅替换白名单字符）
function renderBody(text: string): string {
  const escaped = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return escaped
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\n\n/g, '</p><p>')
    .replace(/\n/g, '<br>')
}
</script>

<template>
  <Transition name="nm">
    <div v-if="open && notif" class="nm__backdrop" @click.self="close">
      <div class="nm" role="dialog" aria-modal="true">
        <button class="nm__close" type="button" @click="close" aria-label="关闭">
          <X :size="16" />
        </button>

        <div class="nm__head">
          <span class="nm__lvl" :class="'nm__lvl--' + notif.level">
            <span class="nm__dot" />
            {{ notif.level.toUpperCase() }}
          </span>
          <h2 class="nm__title">{{ notif.title }}</h2>
          <div class="nm__meta">
            <span>发布 · {{ fmtTime(notif.publishAt) }}</span>
            <span v-if="notif.expireAt"> · 到期 · {{ fmtTime(notif.expireAt) }}</span>
          </div>
        </div>

        <div class="nm__body">
          <p v-html="'<p>' + renderBody(notif.body) + '</p>'" />
        </div>

        <div class="nm__foot">
          <button v-if="notif.dismissible" class="nm__btn nm__btn--ghost" @click="dismiss">不再提醒</button>
          <button class="nm__btn nm__btn--primary" @click="close">知道了</button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.nm__backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.72);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 24px;
}

.nm {
  position: relative;
  width: 100%;
  max-width: 520px;
  max-height: 80vh;
  background: #0a0a0a;
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 8px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.7);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.nm__close {
  position: absolute;
  top: 12px;
  right: 12px;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: #707070;
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease;
  z-index: 2;
}
.nm__close:hover { background: rgba(255, 255, 255, 0.06); color: #ededed; }

.nm__head {
  padding: 24px 28px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.nm__lvl {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.10em;
  margin-bottom: 8px;
}
.nm__lvl--info { background: rgba(82, 168, 255, 0.10); color: #52a8ff; }
.nm__lvl--warn { background: rgba(245, 166, 35, 0.10); color: #f5a623; }
.nm__lvl--critical { background: rgba(255, 77, 77, 0.12); color: #ff4d4d; }
.nm__dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }

.nm__title {
  font-size: 18px;
  font-weight: 600;
  color: #ededed;
  letter-spacing: -0.01em;
  margin: 0;
  line-height: 1.3;
}
.nm__meta {
  margin-top: 6px;
  font-size: 11px;
  color: #707070;
  letter-spacing: 0.03em;
  font-family: "Geist Mono", ui-monospace, monospace;
}

.nm__body {
  flex: 1;
  overflow-y: auto;
  padding: 18px 28px;
  font-size: 13px;
  color: #ededed;
  line-height: 1.6;
}
.nm__body :deep(p) { margin: 0 0 12px; }
.nm__body :deep(p:last-child) { margin-bottom: 0; }
.nm__body :deep(strong) { font-weight: 600; color: #ededed; }
.nm__body :deep(em) { color: #a1a1a1; font-style: italic; }
.nm__body :deep(code) {
  font-family: "Geist Mono", ui-monospace, monospace;
  font-size: 12px;
  padding: 1px 5px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 3px;
  color: #0bd470;
}

.nm__foot {
  padding: 16px 28px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.nm__btn {
  height: 32px;
  padding: 0 14px;
  border-radius: 4px;
  border: none;
  font-size: 12px;
  font-weight: 500;
  font-family: inherit;
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease, opacity 160ms ease;
}
.nm__btn--ghost {
  background: transparent;
  color: #a1a1a1;
}
.nm__btn--ghost:hover { background: rgba(255, 255, 255, 0.06); color: #ededed; }
.nm__btn--primary {
  background: #ededed;
  color: #000;
}
.nm__btn--primary:hover { background: #fff; }

/* Transition */
.nm-enter-active, .nm-leave-active { transition: opacity 200ms ease; }
.nm-enter-active .nm, .nm-leave-active .nm {
  transition: opacity 200ms ease, transform 200ms cubic-bezier(0.16, 1, 0.3, 1);
}
.nm-enter-from, .nm-leave-to { opacity: 0; }
.nm-enter-from .nm, .nm-leave-to .nm {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}
</style>
