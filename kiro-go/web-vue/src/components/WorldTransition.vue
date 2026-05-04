<script setup>
/**
 * WorldTransition.vue v6 — 道诡异仙双世界过场（血色癫狂 vs 光晶重构）
 *
 * 核心原则（v5 卡顿+粗糙的反思）：
 *   1. GPU-only：仅动 transform / opacity；删除所有 filter:url() 实时滤镜
 *   2. 烘焙纹理：纸面噪点用静态 SVG data URI 作 background-image，浏览器一次性栅格化
 *   3. 3 母题 × 3 层视差：远景虚化、中景主体、近景高光，纵深感来自层级而非堆元素
 *   4. 物理曲线：7-keyframe anticipation → overshoot → rebound → settle → linger → release
 *   5. 戏剧节奏：80% 时间给 1-2 个主事件，20% 收尾，留呼吸空间
 *
 * 总时长 1100ms（DOM swap @ 550ms 印章触纸瞬间）。
 *
 * ─────────────────────────────────────────────
 * TO-DAOGUI（亮 → 暗）— 血色癫狂
 *   ACT I  朱砂墨涌      0–520ms   远晕/中池/近高光 3 层从原点炸开
 *   ACT II 印章触纸     420–760ms  3 层（halo / body / burst）+ 7-keyframe 物理
 *   ACT III 镜面碎裂    640–1100ms 12 块三角碎片向外飞散，露出墨黑底
 *
 * TO-REALITY（暗 → 亮）— 光晶重构
 *   ACT I  白光黎明      0–520ms   远晕/中核/近锐 3 层
 *   ACT II 瞳孔聚焦     420–760ms  3 层同心圆（aperture / iris / pupil burst）
 *   ACT III 光晶重构    640–1100ms 12 块碎片由外向中心聚拢，露出净白底
 *
 * 动态原点：var(--portal-origin-x/y) 由 worldTheme.toggleWorld(rect) 注入。
 */
import { ref, watch } from 'vue'
import { useWorldTheme } from '@/stores/worldTheme'

const theme = useWorldTheme()
const direction = ref('')
const shards = ref([])

// 12 块三角碎片：每块有出射方向、旋转、延迟、形状
function generateShards(count = 12) {
  const out = []
  for (let i = 0; i < count; i++) {
    const baseAngle = (360 / count) * i + (Math.random() - 0.5) * 24
    const distance = 55 + Math.random() * 35  // 飞散距离 vmax
    const rad = (baseAngle * Math.PI) / 180
    const dx = Math.cos(rad) * distance
    const dy = Math.sin(rad) * distance
    const rot = (Math.random() - 0.5) * 540    // -270 ~ +270 deg 自旋
    const delay = Math.random() * 90
    // 三角形：以原点为 tip，远端两个尖角（不规则）
    const size = 14 + Math.random() * 18
    const taperA = baseAngle + 6 + Math.random() * 8
    const taperB = baseAngle - 6 - Math.random() * 8
    const r1 = size * (0.55 + Math.random() * 0.45)
    const r2 = size * (0.55 + Math.random() * 0.45)
    const x1 = Math.cos((taperA * Math.PI) / 180) * r1
    const y1 = Math.sin((taperA * Math.PI) / 180) * r1
    const x2 = Math.cos((taperB * Math.PI) / 180) * r2
    const y2 = Math.sin((taperB * Math.PI) / 180) * r2
    out.push({
      points: `0,0 ${x1.toFixed(1)},${y1.toFixed(1)} ${x2.toFixed(1)},${y2.toFixed(1)}`,
      dx: dx.toFixed(1),
      dy: dy.toFixed(1),
      rot: rot.toFixed(1),
      delay: delay.toFixed(0),
    })
  }
  return out
}

watch(
  () => theme.isTransitioning,
  (now) => {
    if (now) {
      direction.value = theme.currentWorld === 'reality' ? 'to-daogui' : 'to-reality'
      shards.value = generateShards(window.innerWidth < 768 ? 8 : 12)
    }
  }
)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="theme.isTransitioning"
      class="portal"
      :class="direction"
      role="status"
      aria-live="polite"
      aria-label="主题切换中"
    >
      <!-- 静态纸面 grain（一次性 SVG 栅格化，无运行时） -->
      <div class="grain-static" />

      <!-- ═══════════ TO-DAOGUI · 血色癫狂 ═══════════ -->
      <template v-if="direction === 'to-daogui'">
        <!-- ACT I — 朱砂墨涌 3 层视差 -->
        <div class="ink-far"  />
        <div class="ink-mid"  />
        <div class="ink-near" />

        <!-- ACT II — 印章 3 层（halo / body / burst） -->
        <div class="seal-halo-d" />
        <div class="seal-burst-d" />
        <div class="seal-d">
          <svg viewBox="0 0 60 60" aria-hidden="true">
            <g>
              <rect x="3" y="3" width="54" height="54" fill="none" stroke-width="3" stroke="#5a0a14" />
              <rect x="9" y="9" width="42" height="42" fill="none" stroke-width="0.7" stroke="#c41e3a" stroke-dasharray="1.5 1.5" opacity="0.55" />
              <line x1="20" y1="18" x2="40" y2="18" stroke-width="2.6" stroke-linecap="round" stroke="#5a0a14" />
              <line x1="20" y1="30" x2="40" y2="30" stroke-width="2.6" stroke-linecap="round" stroke="#5a0a14" />
              <line x1="20" y1="42" x2="40" y2="42" stroke-width="2.6" stroke-linecap="round" stroke="#5a0a14" />
              <line x1="30" y1="14" x2="30" y2="46" stroke-width="2.6" stroke-linecap="round" stroke="#5a0a14" />
              <circle cx="30" cy="30" r="2.4" fill="#ff4458" />
            </g>
          </svg>
        </div>

        <!-- ACT III — 镜面碎裂：12 块三角碎片飞散 + 黑底显出 -->
        <div class="void-bg" />
        <svg class="shatter-d" viewBox="-50 -50 100 100" preserveAspectRatio="none" aria-hidden="true">
          <polygon
            v-for="(s, i) in shards"
            :key="'sd-' + i"
            class="shard-d"
            :style="{
              '--dx': s.dx + 'vmax',
              '--dy': s.dy + 'vmax',
              '--rot': s.rot + 'deg',
              '--delay': s.delay + 'ms',
            }"
            :points="s.points"
          />
        </svg>
      </template>

      <!-- ═══════════ TO-REALITY · 光晶重构 ═══════════ -->
      <template v-else-if="direction === 'to-reality'">
        <!-- ACT I — 白光黎明 3 层视差 -->
        <div class="light-far"  />
        <div class="light-mid"  />
        <div class="light-near" />

        <!-- ACT II — 瞳孔聚焦 3 层（aperture / iris / pupil burst） -->
        <div class="iris-aperture" />
        <div class="iris-burst" />
        <div class="iris-body">
          <svg viewBox="0 0 60 60" aria-hidden="true">
            <g>
              <circle cx="30" cy="30" r="26" fill="none" stroke-width="0.6" stroke-dasharray="2 2" stroke="rgba(186,230,253,0.6)" />
              <circle cx="30" cy="30" r="20" fill="none" stroke-width="1.2" stroke="rgba(255,255,255,0.95)" />
              <circle cx="30" cy="30" r="13" fill="none" stroke-width="0.8" stroke="rgba(255,255,255,0.7)" />
              <circle cx="30" cy="30" r="4" fill="rgba(255,255,255,0.95)" />
              <circle cx="28" cy="28" r="1.2" fill="rgba(186,230,253,0.95)" />
            </g>
          </svg>
        </div>

        <!-- ACT III — 光晶重构：碎片由外向中心聚拢 + 净白底 -->
        <div class="dawn-bg" />
        <svg class="shatter-r" viewBox="-50 -50 100 100" preserveAspectRatio="none" aria-hidden="true">
          <polygon
            v-for="(s, i) in shards"
            :key="'sr-' + i"
            class="shard-r"
            :style="{
              '--dx': s.dx + 'vmax',
              '--dy': s.dy + 'vmax',
              '--rot': s.rot + 'deg',
              '--delay': s.delay + 'ms',
            }"
            :points="s.points"
          />
        </svg>
      </template>
    </div>
  </Teleport>
</template>

<style scoped>
/* ═══════════════════════════════════════════════════════
   Portal 根 + 调色板
   ═══════════════════════════════════════════════════════ */
.portal {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
  overflow: hidden;
  contain: strict;
  /* 强制单独合成层，所有动画走 GPU */
  transform: translateZ(0);

  /* 朱砂 + 紫煞 + 铜符 5 色阶 */
  --ink-shadow: #5a0a14;
  --ink-body:   #c41e3a;
  --ink-bright: #ff4458;
  --ink-warm:   #ffe6c8;
  --ink-void:   #1a0608;
  --ink-purple: #4a1a4a;
  --ink-bronze: #b8860b;

  /* 现实冷色 */
  --real-ice:   #bae6fd;
  --real-cyan:  #38bdf8;
  --real-deep:  #0284c7;
  --real-veil:  #f0f9ff;
}

/* 静态纸面 grain — 浏览器栅格化一次，零运行时 */
.grain-static {
  position: absolute;
  inset: 0;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='240' height='240'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.92' numOctaves='2' seed='5'/><feColorMatrix values='0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 0.85 0'/></filter><rect width='100%25' height='100%25' filter='url(%23n)'/></svg>");
  background-size: 240px 240px;
  mix-blend-mode: multiply;
  opacity: 0;
  will-change: opacity;
  animation: grain-fade 1100ms ease-out forwards;
}
@keyframes grain-fade {
  0%   { opacity: 0; }
  20%  { opacity: 0.16; }
  85%  { opacity: 0.18; }
  100% { opacity: 0; }
}

/* ═══════════════════════════════════════════════════════
   TO-DAOGUI · ACT I — 朱砂墨涌（3 层视差）
   远晕 (60vmax, blur 38px, multiply) — 营造大空间晕染
   中池 (36vmax, blur 8px)             — 主墨池
   近高光 (22vmax, sharp)              — 朱砂火点睛
   ═══════════════════════════════════════════════════════ */
.ink-far,
.ink-mid,
.ink-near {
  position: absolute;
  left: var(--portal-origin-x, 50%);
  top: var(--portal-origin-y, 50%);
  border-radius: 50%;
  pointer-events: none;
  will-change: transform, opacity;
  backface-visibility: hidden;
}

.ink-far {
  width: 60vmax;
  height: 60vmax;
  background: radial-gradient(circle,
    rgba(196, 30, 58, 0.55) 0%,
    rgba(74, 26, 74, 0.5) 30%,
    rgba(26, 6, 8, 0.32) 62%,
    rgba(26, 6, 8, 0) 92%);
  filter: blur(38px);
  mix-blend-mode: multiply;
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  animation: ink-far-spread 720ms cubic-bezier(0.22, 0.61, 0.36, 1) forwards;
}

.ink-mid {
  width: 36vmax;
  height: 36vmax;
  background:
    radial-gradient(circle at 48% 52%,
      rgba(255, 230, 200, 0.4) 0%,
      rgba(255, 68, 88, 0.7) 14%,
      rgba(196, 30, 58, 0.92) 32%,
      rgba(90, 10, 20, 0.78) 62%,
      rgba(26, 6, 8, 0) 96%),
    radial-gradient(circle at 62% 38%,
      rgba(255, 68, 88, 0.45) 0%,
      rgba(255, 68, 88, 0) 38%);
  filter: blur(7px);
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  animation: ink-mid-spread 660ms cubic-bezier(0.16, 1, 0.3, 1) 30ms forwards;
}

.ink-near {
  width: 22vmax;
  height: 22vmax;
  background: radial-gradient(circle,
    rgba(255, 240, 220, 0.95) 0%,
    rgba(255, 68, 88, 0.78) 26%,
    rgba(196, 30, 58, 0.4) 56%,
    rgba(196, 30, 58, 0) 78%);
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  filter: blur(2px);
  animation: ink-near-spread 540ms cubic-bezier(0.34, 1.56, 0.64, 1) 60ms forwards;
}

/* 远景：慢推、放大到 1.5x、长尾保留 */
@keyframes ink-far-spread {
  0%   { transform: translate3d(-50%, -50%, 0) scale(0);    opacity: 0; }
  18%  { opacity: 1; }
  72%  { transform: translate3d(-50%, -50%, 0) scale(1.35); opacity: 0.92; }
  100% { transform: translate3d(-50%, -50%, 0) scale(1.55); opacity: 0; }
}

/* 中景：弹性扩张 + 二次脉冲 */
@keyframes ink-mid-spread {
  0%   { transform: translate3d(-50%, -50%, 0) scale(0);    opacity: 0; }
  16%  { opacity: 1; }
  46%  { transform: translate3d(-50%, -50%, 0) scale(0.88); opacity: 1; }
  60%  { transform: translate3d(-50%, -50%, 0) scale(1.05); opacity: 0.95; }
  100% { transform: translate3d(-50%, -50%, 0) scale(1.3);  opacity: 0; }
}

/* 近景：anticipation→overshoot→settle */
@keyframes ink-near-spread {
  0%   { transform: translate3d(-50%, -50%, 0) scale(0);    opacity: 0; }
  18%  { transform: translate3d(-50%, -50%, 0) scale(0.6);  opacity: 0.7; }
  38%  { transform: translate3d(-50%, -50%, 0) scale(1.18); opacity: 1; }
  56%  { transform: translate3d(-50%, -50%, 0) scale(0.92); opacity: 0.95; }
  100% { transform: translate3d(-50%, -50%, 0) scale(1.15); opacity: 0; }
}

/* ═══════════════════════════════════════════════════════
   TO-DAOGUI · ACT II — 印章触纸（halo / body / burst）
   位置：屏幕几何中心 (50%, 55%)
   时机：420–760ms（ACT I 收尾时印章已开始 anticipation）
   ═══════════════════════════════════════════════════════ */
.seal-halo-d,
.seal-burst-d,
.seal-d {
  position: absolute;
  left: 50%;
  top: 55%;
  pointer-events: none;
  will-change: transform, opacity;
  backface-visibility: hidden;
}

/* halo: 朱砂→紫煞外晕（大、虚） */
.seal-halo-d {
  width: 320px;
  height: 320px;
  border-radius: 50%;
  background: radial-gradient(circle,
    rgba(255, 68, 88, 0.5) 0%,
    rgba(196, 30, 58, 0.4) 24%,
    rgba(74, 26, 74, 0.32) 52%,
    rgba(26, 6, 8, 0) 80%);
  filter: blur(28px);
  mix-blend-mode: multiply;
  transform: translate3d(-50%, -50%, 0) scale(0.4);
  opacity: 0;
  animation: seal-halo-d 480ms cubic-bezier(0.16, 1, 0.3, 1) 420ms forwards;
}
@keyframes seal-halo-d {
  0%   { transform: translate3d(-50%, -50%, 0) scale(0.4);  opacity: 0; }
  35%  { transform: translate3d(-50%, -50%, 0) scale(1.05); opacity: 0.95; }
  72%  { transform: translate3d(-50%, -50%, 0) scale(1.3);  opacity: 0.65; }
  100% { transform: translate3d(-50%, -50%, 0) scale(1.5);  opacity: 0; }
}

/* burst: 触纸瞬间白热核闪 */
.seal-burst-d {
  width: 140px;
  height: 140px;
  border-radius: 50%;
  background: radial-gradient(circle,
    rgba(255, 255, 240, 1) 0%,
    rgba(255, 230, 200, 0.85) 25%,
    rgba(255, 68, 88, 0.5) 55%,
    rgba(255, 68, 88, 0) 80%);
  filter: blur(2px);
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  animation: seal-burst-d 160ms cubic-bezier(0.4, 0, 0.6, 1) 480ms forwards;
}
@keyframes seal-burst-d {
  0%   { transform: translate3d(-50%, -50%, 0) scale(0);   opacity: 0; }
  36%  { transform: translate3d(-50%, -50%, 0) scale(1.3); opacity: 1; }
  100% { transform: translate3d(-50%, -50%, 0) scale(1.6); opacity: 0; }
}

/* body: 印章主体 7-keyframe 物理 */
.seal-d {
  width: 130px;
  height: 130px;
  transform: translate3d(-50%, -50%, 0) scale(0) rotate(-14deg);
  opacity: 0;
  filter: drop-shadow(0 0 16px rgba(196, 30, 58, 0.85));
  animation: seal-d 700ms 480ms forwards;
}
.seal-d svg {
  width: 100%;
  height: 100%;
}
@keyframes seal-d {
  0%   { transform: translate3d(-50%, -50%, 0) scale(0)    rotate(-14deg); opacity: 0; }
  18%  { transform: translate3d(-50%, -50%, 0) scale(1.4)  rotate(-8deg);  opacity: 1; }
  26%  { transform: translate3d(-50%, -50%, 0) scale(0.84) rotate(-3deg);  opacity: 1; }
  36%  { transform: translate3d(-50%, -50%, 0) scale(1.08) rotate(0deg);   opacity: 1; }
  46%  { transform: translate3d(-50%, -50%, 0) scale(1)    rotate(0deg);   opacity: 1; }
  72%  { transform: translate3d(-50%, -50%, 0) scale(1)    rotate(0deg);   opacity: 0.95; }
  100% { transform: translate3d(-50%, -50%, 0) scale(0.88) rotate(2deg);   opacity: 0; }
}

/* ═══════════════════════════════════════════════════════
   TO-DAOGUI · ACT III — 镜面碎裂
   12 块三角碎片向外飞散 + 墨黑底（露出新世界）
   纯 transform + opacity，每片独立 GPU 层
   ═══════════════════════════════════════════════════════ */
.void-bg {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 50% 55%,
      rgba(26, 6, 8, 0.96) 0%,
      rgba(10, 10, 10, 0.98) 60%,
      rgba(10, 10, 10, 1) 100%);
  opacity: 0;
  will-change: opacity;
  animation: void-fade 460ms cubic-bezier(0.4, 0, 0.6, 1) 640ms forwards;
}
@keyframes void-fade {
  0%   { opacity: 0; }
  35%  { opacity: 0.85; }
  100% { opacity: 0; }
}

.shatter-d {
  position: absolute;
  left: 50%;
  top: 55%;
  width: 200vmax;
  height: 200vmax;
  transform: translate3d(-50%, -50%, 0);
  overflow: visible;
}
.shard-d {
  fill: var(--ink-body);
  stroke: var(--ink-bright);
  stroke-width: 0.18;
  /* 起始：聚集在中心、不可见。
     结束：飞散到 (--dx, --dy)、自旋 (--rot)、淡出。
     transform/opacity-only，每片独立合成层。 */
  transform: translate3d(0, 0, 0) rotate(0deg) scale(0.4);
  opacity: 0;
  filter: drop-shadow(0 0 1px rgba(255, 68, 88, 0.6));
  will-change: transform, opacity;
  animation: shard-fly-out 460ms cubic-bezier(0.34, 0.05, 0.6, 1) forwards;
  animation-delay: calc(640ms + var(--delay, 0ms));
}
@keyframes shard-fly-out {
  0%   {
    transform: translate3d(0, 0, 0) rotate(0deg) scale(0.4);
    opacity: 0;
  }
  18%  {
    transform: translate3d(calc(var(--dx) * 0.12), calc(var(--dy) * 0.12), 0)
               rotate(calc(var(--rot) * 0.14)) scale(1.05);
    opacity: 1;
  }
  100% {
    transform: translate3d(var(--dx), var(--dy), 0)
               rotate(var(--rot)) scale(0.6);
    opacity: 0;
  }
}

/* ═══════════════════════════════════════════════════════
   TO-REALITY · ACT I — 白光黎明（3 层视差）
   ═══════════════════════════════════════════════════════ */
.light-far,
.light-mid,
.light-near {
  position: absolute;
  left: var(--portal-origin-x, 50%);
  top: var(--portal-origin-y, 50%);
  border-radius: 50%;
  pointer-events: none;
  will-change: transform, opacity;
  backface-visibility: hidden;
  mix-blend-mode: screen;
}

.light-far {
  width: 60vmax;
  height: 60vmax;
  background: radial-gradient(circle,
    rgba(186, 230, 253, 0.55) 0%,
    rgba(56, 189, 248, 0.35) 30%,
    rgba(2, 132, 199, 0.18) 62%,
    rgba(2, 132, 199, 0) 92%);
  filter: blur(38px);
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  animation: ink-far-spread 720ms cubic-bezier(0.22, 0.61, 0.36, 1) forwards;
}

.light-mid {
  width: 36vmax;
  height: 36vmax;
  background:
    radial-gradient(circle at 48% 52%,
      rgba(255, 255, 255, 0.95) 0%,
      rgba(240, 249, 255, 0.85) 18%,
      rgba(186, 230, 253, 0.65) 42%,
      rgba(56, 189, 248, 0.3) 70%,
      rgba(255, 255, 255, 0) 96%);
  filter: blur(7px);
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  animation: ink-mid-spread 660ms cubic-bezier(0.16, 1, 0.3, 1) 30ms forwards;
}

.light-near {
  width: 22vmax;
  height: 22vmax;
  background: radial-gradient(circle,
    rgba(255, 255, 255, 1) 0%,
    rgba(240, 249, 255, 0.9) 28%,
    rgba(186, 230, 253, 0.5) 60%,
    rgba(186, 230, 253, 0) 80%);
  filter: blur(2px);
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  animation: ink-near-spread 540ms cubic-bezier(0.34, 1.56, 0.64, 1) 60ms forwards;
}

/* ═══════════════════════════════════════════════════════
   TO-REALITY · ACT II — 瞳孔聚焦（aperture / iris / pupil burst）
   位置：屏幕中心 (50%, 50%)
   ═══════════════════════════════════════════════════════ */
.iris-aperture,
.iris-burst,
.iris-body {
  position: absolute;
  left: 50%;
  top: 50%;
  pointer-events: none;
  will-change: transform, opacity;
  backface-visibility: hidden;
}

/* aperture: 视野光圈外晕 */
.iris-aperture {
  width: 320px;
  height: 320px;
  border-radius: 50%;
  background: radial-gradient(circle,
    rgba(255, 255, 255, 0.5) 0%,
    rgba(186, 230, 253, 0.4) 30%,
    rgba(56, 189, 248, 0.25) 60%,
    rgba(2, 132, 199, 0) 88%);
  filter: blur(26px);
  mix-blend-mode: screen;
  transform: translate3d(-50%, -50%, 0) scale(0.4);
  opacity: 0;
  animation: seal-halo-d 480ms cubic-bezier(0.16, 1, 0.3, 1) 420ms forwards;
}

/* pupil burst: 瞳孔白热闪 */
.iris-burst {
  width: 140px;
  height: 140px;
  border-radius: 50%;
  background: radial-gradient(circle,
    rgba(255, 255, 255, 1) 0%,
    rgba(240, 249, 255, 0.9) 30%,
    rgba(186, 230, 253, 0.5) 55%,
    rgba(186, 230, 253, 0) 80%);
  filter: blur(2px);
  transform: translate3d(-50%, -50%, 0) scale(0);
  opacity: 0;
  animation: seal-burst-d 160ms cubic-bezier(0.4, 0, 0.6, 1) 480ms forwards;
}

/* iris body: 同心圆瞳孔本体 7-keyframe 物理 */
.iris-body {
  width: 130px;
  height: 130px;
  transform: translate3d(-50%, -50%, 0) scale(0) rotate(0deg);
  opacity: 0;
  filter: drop-shadow(0 0 18px rgba(186, 230, 253, 0.85));
  animation: iris-body 700ms 480ms forwards;
}
.iris-body svg {
  width: 100%;
  height: 100%;
}
@keyframes iris-body {
  0%   { transform: translate3d(-50%, -50%, 0) scale(0)    rotate(0deg);  opacity: 0; }
  18%  { transform: translate3d(-50%, -50%, 0) scale(1.4)  rotate(-4deg); opacity: 1; }
  26%  { transform: translate3d(-50%, -50%, 0) scale(0.84) rotate(0deg);  opacity: 1; }
  36%  { transform: translate3d(-50%, -50%, 0) scale(1.08) rotate(2deg);  opacity: 1; }
  46%  { transform: translate3d(-50%, -50%, 0) scale(1)    rotate(0deg);  opacity: 1; }
  72%  { transform: translate3d(-50%, -50%, 0) scale(1)    rotate(0deg);  opacity: 0.95; }
  100% { transform: translate3d(-50%, -50%, 0) scale(0.88) rotate(0deg);  opacity: 0; }
}

/* ═══════════════════════════════════════════════════════
   TO-REALITY · ACT III — 光晶重构
   12 块碎片由外向中心聚拢，露出净白底
   ═══════════════════════════════════════════════════════ */
.dawn-bg {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 50% 50%,
      rgba(255, 255, 255, 0.96) 0%,
      rgba(240, 249, 255, 0.92) 50%,
      rgba(224, 242, 254, 0.88) 100%);
  opacity: 0;
  will-change: opacity;
  animation: dawn-fade 460ms cubic-bezier(0.4, 0, 0.6, 1) 640ms forwards;
}
@keyframes dawn-fade {
  0%   { opacity: 0; }
  35%  { opacity: 0.85; }
  100% { opacity: 0; }
}

.shatter-r {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 200vmax;
  height: 200vmax;
  transform: translate3d(-50%, -50%, 0);
  overflow: visible;
}
.shard-r {
  fill: rgba(186, 230, 253, 0.85);
  stroke: rgba(255, 255, 255, 0.95);
  stroke-width: 0.18;
  /* 起始：散落在外（dx, dy 方向），不可见。
     结束：聚拢到中心、淡出。 */
  transform: translate3d(var(--dx), var(--dy), 0)
             rotate(var(--rot)) scale(0.6);
  opacity: 0;
  filter: drop-shadow(0 0 1.5px rgba(186, 230, 253, 0.85));
  will-change: transform, opacity;
  animation: shard-fly-in 460ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
  animation-delay: calc(640ms + var(--delay, 0ms));
}
@keyframes shard-fly-in {
  0%   {
    transform: translate3d(var(--dx), var(--dy), 0)
               rotate(var(--rot)) scale(0.6);
    opacity: 0;
  }
  20%  {
    opacity: 1;
  }
  82%  {
    transform: translate3d(calc(var(--dx) * 0.1), calc(var(--dy) * 0.1), 0)
               rotate(calc(var(--rot) * 0.12)) scale(1.0);
    opacity: 1;
  }
  100% {
    transform: translate3d(0, 0, 0) rotate(0deg) scale(0.4);
    opacity: 0;
  }
}

/* ═══════════════════════════════════════════════════════
   prefers-reduced-motion + 移动端适配
   ═══════════════════════════════════════════════════════ */
@media (prefers-reduced-motion: reduce) {
  .portal { display: none; }
}

@media (max-width: 768px) {
  .ink-far, .light-far { width: 90vmax; height: 90vmax; }
  .ink-mid, .light-mid { width: 56vmax; height: 56vmax; }
  .ink-near, .light-near { width: 36vmax; height: 36vmax; }
  .seal-halo-d, .iris-aperture { width: 240px; height: 240px; }
  .seal-burst-d, .iris-burst { width: 110px; height: 110px; }
  .seal-d, .iris-body { width: 110px; height: 110px; }
}
</style>
