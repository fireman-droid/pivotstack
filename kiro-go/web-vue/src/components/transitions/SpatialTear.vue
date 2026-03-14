<script setup>
/**
 * SpatialTear 空间撕裂 — TO SANITY (理智回归)
 *
 * 效果：黑暗世界从中心被撕开，撕裂缝隙内是白色的现实世界
 * 
 * 结构：
 *   底层 — 白色/亮色（代表"理智回归"后的新世界）
 *   顶层 — 黑色遮罩，从中心向两侧裂开（mask-image: linear-gradient）
 *   边缘 — 撕裂缝隙两侧的蓝白光芒（box-shadow）
 *
 * 时间线：
 *   0-600ms   撕裂遮罩从中心向两侧扩展
 *   100-600ms 白光边缘跟随撕裂缝隙
 */
</script>

<template>
  <div class="tear-container" aria-hidden="true">
    <!-- 底层：被撕裂露出的新世界（白/亮色） -->
    <div class="tear-reveal"></div>

    <!-- 中层：黑色遮罩，从中心裂开 -->
    <div class="tear-panel tear-panel--left"></div>
    <div class="tear-panel tear-panel--right"></div>

    <!-- 顶层：撕裂缝隙边缘的白光 -->
    <div class="tear-edge tear-edge--left"></div>
    <div class="tear-edge tear-edge--right"></div>
  </div>
</template>

<style scoped>
.tear-container {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}

/* ---- 底层：白色新世界 ---- */
.tear-reveal {
  position: absolute;
  inset: 0;
  background: radial-gradient(
    ellipse at center,
    #ffffff 0%,
    #f0f9ff 40%,
    #e0f2fe 70%,
    #bae6fd 100%
  );
  opacity: 0;
  animation: reveal-fade-in 400ms 100ms ease-out forwards;
  will-change: opacity;
}

/* ---- 中层：黑色遮罩面板，向左右滑开 ---- */
.tear-panel {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 50%;
  background: var(--color-void-black, #0a0a0a);
  will-change: transform;
  backface-visibility: hidden;
}

.tear-panel--left {
  left: 0;
  transform-origin: left center;
  animation: panel-slide-left 600ms cubic-bezier(0.75, 0, 0.25, 1) forwards;
}

.tear-panel--right {
  right: 0;
  transform-origin: right center;
  animation: panel-slide-right 600ms cubic-bezier(0.75, 0, 0.25, 1) forwards;
}

/* ---- 顶层：撕裂边缘光芒 ---- */
.tear-edge {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  left: 50%;
  background: #ffffff;
  box-shadow:
    0 0 12px 4px rgba(14, 165, 233, 0.9),
    0 0 30px 10px rgba(255, 255, 255, 0.6),
    0 0 60px 20px rgba(14, 165, 233, 0.25);
  opacity: 0;
  will-change: transform, opacity;
  backface-visibility: hidden;
}

.tear-edge--left {
  animation: edge-left 600ms 80ms cubic-bezier(0.75, 0, 0.25, 1) forwards;
}

.tear-edge--right {
  animation: edge-right 600ms 80ms cubic-bezier(0.75, 0, 0.25, 1) forwards;
}

/* ═══════ Keyframes ═══════ */

/* 底层白色：延迟 100ms 后淡入，给撕裂一个"开始"的感觉 */
@keyframes reveal-fade-in {
  0%   { opacity: 0; }
  100% { opacity: 1; will-change: auto; }
}

/* 左面板向左滑出 */
@keyframes panel-slide-left {
  0%   { transform: translateX(0); }
  100% { transform: translateX(-105%); will-change: auto; }
}

/* 右面板向右滑出 */
@keyframes panel-slide-right {
  0%   { transform: translateX(0); }
  100% { transform: translateX(105%); will-change: auto; }
}

/* 左侧光芒跟随左面板边缘 */
@keyframes edge-left {
  0%   { opacity: 0; transform: translateX(-1px); }
  10%  { opacity: 1; }
  80%  { opacity: 0.6; }
  100% {
    opacity: 0;
    transform: translateX(calc(-50vw - 10px));
    will-change: auto;
  }
}

/* 右侧光芒跟随右面板边缘 */
@keyframes edge-right {
  0%   { opacity: 0; transform: translateX(1px); }
  10%  { opacity: 1; }
  80%  { opacity: 0.6; }
  100% {
    opacity: 0;
    transform: translateX(calc(50vw + 10px));
    will-change: auto;
  }
}

/* 移动端：减小光芒范围 */
@media (max-width: 768px) {
  .tear-edge {
    box-shadow:
      0 0 8px 3px rgba(14, 165, 233, 0.7),
      0 0 20px 6px rgba(255, 255, 255, 0.4);
  }
}
</style>
