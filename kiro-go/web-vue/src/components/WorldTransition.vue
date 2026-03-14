<script setup>
/**
 * WorldTransition.vue — Sanity ↔ Madness 过场动画
 *
 * 严格按照 sanity-madness-transition-refactor.md 计划实现
 *
 * ═══ TO MADNESS (坠入深渊) ═══
 * 0-800ms     符文崩解 → 12 个道符从中心向外裂开
 * 800-1000ms  血色侵蚀 → radial-gradient mask 从中心扩散吞噬画面
 * 1000ms      DOM 切换点
 * 1000-1200ms 红光闪烁 → 简单 opacity 动画
 * 1200-2000ms 噪点淡出 → 预渲染纹理 opacity 淡出
 *
 * ═══ TO SANITY (理智回归) ═══
 * 0-600ms     空间撕裂 → 黑暗遮罩从中心向两侧裂开，里面是白色新世界
 * 600-1000ms  白光切割 → 撕裂边缘的 box-shadow 蓝白光
 * 1000ms      DOM 切换点
 * 1000-1400ms 纯白闪光 → 覆盖 DOM 切换
 * 1400-2000ms 淡出
 *
 * 性能约束：
 * - 零 SVG filter / backdrop-filter / mix-blend-mode
 * - 仅 transform / opacity / box-shadow
 * - 同时最多 2 层动画
 * - will-change 动画结束后清理
 */
import { ref, watch } from 'vue'
import { useWorldTheme } from '@/stores/worldTheme'
import RuneDisintegration from './transitions/RuneDisintegration.vue'
import SpatialTear from './transitions/SpatialTear.vue'

const theme = useWorldTheme()
const transitionDir = ref('')

watch(() => theme.isTransitioning, (isTrans) => {
  if (isTrans) {
    transitionDir.value = theme.currentWorld === 'reality' ? 'to-daogui' : 'to-reality'
  }
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="theme.isTransitioning"
      class="world-transition-portal"
      role="status"
      aria-live="polite"
      aria-label="世界切换中"
    >
      <!-- ════════════════════════════════════
           TO DAOGUI (坠入深渊 / MADNESS)
           ════════════════════════════════════ -->
      <div v-if="transitionDir === 'to-daogui'" class="transition-layer madness-layer">
        <!-- Phase 1: 符文崩解 (0-800ms) -->
        <RuneDisintegration />

        <!-- Phase 2: 血色侵蚀 (800-1000ms) — radial-gradient 从中心扩散 -->
        <div class="blood-spread"></div>

        <!-- Phase 3: 红光闪烁 (1000-1200ms) — 掩盖 DOM 切换 -->
        <div class="red-flash"></div>

        <!-- Phase 4: 噪点淡出 (1200-2000ms) -->
        <div class="noise-fade"></div>
      </div>

      <!-- ════════════════════════════════════
           TO REALITY (理智回归 / SANITY)
           ════════════════════════════════════ -->
      <div v-if="transitionDir === 'to-reality'" class="transition-layer sanity-layer">
        <!-- Phase 1+2: 空间撕裂 + 白光切割 (0-1000ms) -->
        <!-- 撕裂打开时，里面已经是白色新世界 -->
        <SpatialTear />

        <!-- Phase 3: 纯白闪光 (1000-1400ms) — 覆盖 DOM 切换 -->
        <div class="white-flash"></div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* ═══════════════════════════════
   Portal 容器
   ═══════════════════════════════ */
.world-transition-portal {
  position: fixed;
  inset: 0;
  z-index: 99999;
  pointer-events: none;
  overflow: hidden;
}

.transition-layer {
  position: absolute;
  inset: 0;
  animation: fade-out 2000ms ease-in-out forwards;
}

/* ═══════════════════════════════
   TO DAOGUI — 坠入深渊
   ═══════════════════════════════ */

/*
 * 血色侵蚀 (Phase 2: 800-1000ms)
 * radial-gradient 从中心圆点扩散为全屏，吞噬画面
 * 替代原 SVG feTurbulence + feDisplacementMap
 */
.blood-spread {
  position: absolute;
  inset: 0;
  background: radial-gradient(
    circle at center,
    rgba(196, 30, 58, 0) 0%,
    rgba(196, 30, 58, 0.3) 40%,
    rgba(7, 0, 0, 0.9) 80%,
    rgba(7, 0, 0, 1) 100%
  );
  opacity: 0;
  transform: scale(0);
  animation: blood-expand 200ms 800ms ease-out forwards;
  will-change: transform, opacity;
  backface-visibility: hidden;
}

/*
 * 红光闪烁 (Phase 3: 1000-1200ms)
 * 简单 opacity 脉冲，掩盖 1000ms 的 DOM 切换
 * 无 mix-blend-mode，纯 opacity
 */
.red-flash {
  position: absolute;
  inset: 0;
  background: rgba(80, 0, 0, 0.6);
  opacity: 0;
  animation: flash-pulse 200ms 1000ms ease-in-out forwards;
  will-change: opacity;
  backface-visibility: hidden;
}

/*
 * 噪点淡出 (Phase 4: 1200-2000ms)
 * 使用 CSS repeating-conic-gradient 模拟胶片噪点
 * 替代原内联 SVG feTurbulence
 */
.noise-fade {
  position: absolute;
  inset: 0;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)' opacity='1'/%3E%3C/svg%3E");
  background-size: 200px 200px;
  opacity: 0;
  animation: noise-appear 800ms 1200ms ease-out forwards;
  will-change: opacity;
  backface-visibility: hidden;
}

/* ═══════════════════════════════
   TO REALITY — 理智回归
   ═══════════════════════════════ */

/*
 * 纯白闪光 (Phase 3: 1000-1400ms)
 * 覆盖 DOM 切换的强白光
 * 简化为单层 opacity 动画
 */
.white-flash {
  position: absolute;
  inset: 0;
  background: white;
  opacity: 0;
  animation: white-pulse 400ms 1000ms ease-out forwards;
  will-change: opacity;
  backface-visibility: hidden;
}

/* ═══════════════════════════════
   关键帧
   ═══════════════════════════════ */

/* 血色侵蚀：scale(0) → scale(1.5)，200ms 急速扩张 */
@keyframes blood-expand {
  0% {
    opacity: 0;
    transform: scale(0);
  }
  100% {
    opacity: 1;
    transform: scale(1.5);
    will-change: auto;
  }
}

/* 红光脉冲：闪烁后消退 */
@keyframes flash-pulse {
  0%, 100% { opacity: 0; }
  50% { opacity: 1; }
}

/* 噪点：出现 → 峰值 → 消退 */
@keyframes noise-appear {
  0%   { opacity: 0; }
  50%  { opacity: 0.35; }
  100% { opacity: 0; will-change: auto; }
}

/* 白光脉冲：亮起后柔和消退 */
@keyframes white-pulse {
  0%   { opacity: 0; }
  40%  { opacity: 1; }
  100% { opacity: 0; will-change: auto; }
}

/* 全局淡出 */
@keyframes fade-out {
  0%   { opacity: 1; }
  90%  { opacity: 1; }
  100% { opacity: 0; }
}

/* ═══════════════════════════════
   无障碍降级
   ═══════════════════════════════ */
@media (prefers-reduced-motion: reduce) {
  .transition-layer {
    animation: instant-fade 300ms ease-out forwards;
  }

  .blood-spread,
  .red-flash,
  .noise-fade,
  .white-flash {
    display: none;
  }

  @keyframes instant-fade {
    0%   { background: var(--from-color, black); }
    100% { background: var(--to-color, white); }
  }
}

/* ═══════════════════════════════
   移动端降级
   ═══════════════════════════════ */
@media (max-width: 768px) {
  .noise-fade {
    background-image: none;
    background: rgba(26, 26, 26, 0.3);
  }
}
</style>
