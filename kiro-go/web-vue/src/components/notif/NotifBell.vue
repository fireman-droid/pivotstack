<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Bell, CheckCheck, ExternalLink } from 'lucide-vue-next'
import { useNotificationStore } from '../../stores/notifications'
import type { UserNotification } from '../../api/notifications'

const store = useNotificationStore()

defineProps<{
  /** 触发按钮风格，'compact' 为 user 端顶 nav 用，'rail' 为 admin rail 用 */
  variant?: 'compact' | 'rail'
}>()

const emit = defineEmits<{
  (e: 'open', n: UserNotification): void
  (e: 'see-all'): void
}>()

const open = ref(false)
const anchorRef = ref<HTMLElement | null>(null)

function toggle() { open.value = !open.value }
function close() { open.value = false }

function onItemClick(n: UserNotification) {
  store.read(n.id)
  emit('open', n)
  open.value = false
}

function onClickOutside(e: MouseEvent) {
  if (!open.value) return
  if (anchorRef.value && !anchorRef.value.contains(e.target as Node)) close()
}

const visibleItems = computed(() => store.items.filter(n => !n.dismissed))
const badge = computed(() => {
  if (store.unreadCount === 0) return ''
  if (store.unreadCount > 9) return '9+'
  return String(store.unreadCount)
})
const hasCritical = computed(() =>
  visibleItems.value.some(n => n.level === 'critical' && !n.read),
)

function fmtAgo(ts?: number) {
  if (!ts) return ''
  const diff = Math.floor(Date.now() / 1000 - ts)
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  return `${Math.floor(diff / 86400)} 天前`
}

onMounted(() => window.addEventListener('click', onClickOutside))
onUnmounted(() => window.removeEventListener('click', onClickOutside))
</script>

<template>
  <div ref="anchorRef" class="nbell" :class="['nbell--' + (variant || 'compact')]">
    <button class="nbell__btn" type="button" @click.stop="toggle" :aria-label="`通知（未读 ${store.unreadCount}）`">
      <Bell :size="16" stroke-width="1.75" />
      <span
        v-if="badge"
        class="nbell__badge"
        :class="{ 'nbell__badge--crit': hasCritical }"
      >{{ badge }}</span>
    </button>

    <Transition name="ndrop">
      <div v-if="open" class="ndrop" @click.stop>
        <div class="ndrop__head">
          <span class="ndrop__title">通知</span>
          <button v-if="visibleItems.length > 0" class="ndrop__link" @click="emit('see-all'); close()">
            查看全部
            <ExternalLink :size="12" />
          </button>
        </div>

        <div class="ndrop__body">
          <div v-if="visibleItems.length === 0" class="ndrop__empty">
            <span>暂无通知</span>
          </div>
          <button
            v-for="n in visibleItems.slice(0, 6)"
            :key="n.id"
            class="ndrop__item"
            :class="{ 'is-unread': !n.read }"
            type="button"
            @click="onItemClick(n)"
          >
            <span class="ndrop__dot" :class="'ndrop__dot--' + n.level" />
            <div class="ndrop__main">
              <div class="ndrop__row">
                <span class="ndrop__lvl" :class="'ndrop__lvl--' + n.level">
                  {{ n.level.toUpperCase() }}
                </span>
                <span class="ndrop__t">{{ n.title }}</span>
                <span class="ndrop__time">{{ fmtAgo(n.publishAt) }}</span>
              </div>
              <div class="ndrop__excerpt">{{ n.body.replace(/[*_`#>]/g, '').slice(0, 56) }}</div>
            </div>
          </button>
        </div>

        <div v-if="visibleItems.length > 0" class="ndrop__foot">
          <button class="ndrop__link" @click="store.readAll()">
            <CheckCheck :size="12" /> 全部标已读
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.nbell { position: relative; display: inline-flex; }

.nbell__btn {
  position: relative;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: #a1a1a1;
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease;
}
.nbell__btn:hover { background: rgba(255, 255, 255, 0.06); color: #ededed; }

.nbell--rail .nbell__btn {
  width: 40px;
  height: 40px;
  color: #707070;
}

.nbell__badge {
  position: absolute;
  top: 2px;
  right: 2px;
  min-width: 14px;
  height: 14px;
  padding: 0 4px;
  background: #0bd470;
  color: #000;
  font-size: 9px;
  font-weight: 700;
  line-height: 14px;
  text-align: center;
  border-radius: 7px;
  border: 1px solid #050505;
}
.nbell__badge--crit {
  background: #ff4d4d;
  color: #fff;
  box-shadow: 0 0 6px rgba(255, 77, 77, 0.6);
  animation: nbell-pulse 1.5s ease-in-out infinite;
}
@keyframes nbell-pulse {
  0%, 100% { box-shadow: 0 0 4px rgba(255, 77, 77, 0.4); }
  50%      { box-shadow: 0 0 10px rgba(255, 77, 77, 0.85); }
}

/* ───────── dropdown ───────── */
.ndrop {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  width: 360px;
  max-height: 480px;
  background: #0a0a0a;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.6);
  z-index: 50;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.nbell--rail .ndrop {
  left: calc(100% + 8px);
  right: auto;
  top: 0;
  transform: translateY(-30%);
}

.ndrop__head {
  padding: 12px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.ndrop__title { font-size: 13px; font-weight: 600; color: #ededed; }
.ndrop__link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: transparent;
  border: none;
  color: #a1a1a1;
  font-size: 11px;
  cursor: pointer;
  font-family: inherit;
}
.ndrop__link:hover { color: #0bd470; }

.ndrop__body { flex: 1; overflow-y: auto; padding: 4px; }
.ndrop__empty {
  padding: 32px 14px;
  text-align: center;
  font-size: 12px;
  color: #707070;
}

.ndrop__item {
  display: flex;
  width: 100%;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  text-align: left;
  font-family: inherit;
  color: inherit;
  transition: background 160ms ease;
}
.ndrop__item:hover { background: rgba(255, 255, 255, 0.04); }
.ndrop__item.is-unread .ndrop__t { font-weight: 600; color: #ededed; }
.ndrop__item:not(.is-unread) .ndrop__t { color: #a1a1a1; }

.ndrop__dot {
  width: 6px;
  height: 6px;
  margin-top: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.ndrop__dot--info { background: #52a8ff; }
.ndrop__dot--warn { background: #f5a623; }
.ndrop__dot--critical { background: #ff4d4d; box-shadow: 0 0 6px rgba(255, 77, 77, 0.6); }

.ndrop__main { flex: 1; min-width: 0; }
.ndrop__row { display: flex; align-items: center; gap: 6px; }
.ndrop__lvl {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.08em;
  padding: 1px 4px;
  border-radius: 2px;
}
.ndrop__lvl--info { background: rgba(82, 168, 255, 0.12); color: #52a8ff; }
.ndrop__lvl--warn { background: rgba(245, 166, 35, 0.12); color: #f5a623; }
.ndrop__lvl--critical { background: rgba(255, 77, 77, 0.14); color: #ff4d4d; }
.ndrop__t {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ndrop__time {
  font-size: 10px;
  font-family: "Geist Mono", ui-monospace, monospace;
  color: #707070;
  flex-shrink: 0;
}
.ndrop__excerpt {
  font-size: 11px;
  color: #707070;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 2px;
}

.ndrop__foot {
  padding: 8px 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

/* Transition */
.ndrop-enter-active, .ndrop-leave-active {
  transition: opacity 160ms ease, transform 160ms ease;
}
.ndrop-enter-from, .ndrop-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
.nbell--rail .ndrop-enter-from, .nbell--rail .ndrop-leave-to {
  transform: translateY(-30%) translateX(-4px);
}
</style>
