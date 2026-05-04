<script setup>
import { useWorldTheme } from '@/stores/worldTheme'
import { computed } from 'vue'
import { Sun, Moon } from 'lucide-vue-next'

const theme = useWorldTheme()

const isDaogui = computed(() => theme.currentWorld === 'daogui')

/**
 * 用被点击那个段的 rect 作为 portal 动画原点：
 * - 点 "暗色" → 转场从暗色按钮中心发散
 * - 点 "亮色" → 转场从亮色按钮中心收束
 * 比用整个开关容器中心更精致：动画从用户实际点击的位置长出来。
 */
function setWorld(target, event) {
  if (theme.currentWorld === target) return
  if (theme.isTransitioning) return
  const rect = event?.currentTarget?.getBoundingClientRect()
  theme.toggleWorld(rect)
}
</script>

<template>
  <div
    class="world-switch"
    role="radiogroup"
    aria-label="切换主题"
  >
    <button
      type="button"
      role="radio"
      :aria-checked="!isDaogui"
      class="ws-seg"
      :class="{ active: !isDaogui }"
      @click="setWorld('reality', $event)"
      :disabled="theme.isTransitioning"
    >
      <Sun :size="14" stroke-width="2.4" />
      <span>亮色</span>
    </button>

    <button
      type="button"
      role="radio"
      :aria-checked="isDaogui"
      class="ws-seg"
      :class="{ active: isDaogui }"
      @click="setWorld('daogui', $event)"
      :disabled="theme.isTransitioning"
    >
      <Moon :size="14" stroke-width="2.4" />
      <span>暗色</span>
    </button>

    <span class="ws-thumb" :class="{ 'to-daogui': isDaogui }" aria-hidden="true" />
  </div>
</template>

<style scoped>
.world-switch {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 4px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-full);
  gap: 0;
  font-family: var(--world-font-sans);
  user-select: none;
  width: 168px;
  height: 36px;
  flex-shrink: 0;
}

.ws-seg {
  position: relative;
  z-index: 2;
  flex: 1 1 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 0 8px;
  background: transparent;
  border: none;
  border-radius: var(--world-radius-full);
  color: var(--world-text-mute);
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  cursor: pointer;
  transition: color 320ms ease;
  white-space: nowrap;
}
.ws-seg:disabled { cursor: default; }
.ws-seg.active { color: #fff; }
.ws-seg:focus-visible { outline: none; box-shadow: var(--world-focus-ring); }

.ws-thumb {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 4px;
  width: calc(50% - 4px);
  border-radius: var(--world-radius-full);
  transition: transform 480ms cubic-bezier(0.65, 0.05, 0.36, 1),
              background 600ms ease,
              box-shadow 480ms ease;
  z-index: 1;
}
.ws-thumb.to-daogui { transform: translateX(calc(100% + 0px)); }

/* === Reality 形态 === */
[data-world="reality"] .ws-thumb {
  background: linear-gradient(135deg, var(--world-accent), var(--world-accent-soft, #38bdf8));
  box-shadow:
    0 1px 3px rgba(2, 132, 199, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.6) inset;
}
[data-world="reality"] .world-switch {
  background: rgba(15, 23, 42, 0.04);
}

/* === Daogui 形态 === */
[data-world="daogui"] .ws-thumb {
  background: linear-gradient(135deg, #c41e3a 0%, #8b1626 100%);
  box-shadow:
    0 0 14px rgba(196, 30, 58, 0.45),
    0 0 0 1px rgba(184, 134, 11, 0.4) inset;
}
[data-world="daogui"] .world-switch {
  background: rgba(184, 134, 11, 0.06);
  border-color: rgba(184, 134, 11, 0.22);
}
[data-world="daogui"] .ws-seg.active {
  text-shadow: 0 0 6px rgba(255, 255, 255, 0.4);
}
</style>
