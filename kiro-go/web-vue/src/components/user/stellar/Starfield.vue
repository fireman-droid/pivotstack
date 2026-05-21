<script setup lang="ts">
// 全屏星空背景层。fixed z-index:-1，鼠标不响应。
// 由旋转容器 + 烘焙静态星 + DOM twinkle 星 + meteor 生成器 + 中心 vignette 组成。
// 严格还原 design/pivotstack/project/stellar.css + stellar.js 的视觉行为。
import { onMounted, onBeforeUnmount, ref } from 'vue'

const staticLayer = ref<HTMLDivElement | null>(null)
const twinkleLayer = ref<HTMLDivElement | null>(null)
const meteorLayer = ref<HTMLDivElement | null>(null)

let meteorTimer: number | undefined
let meteorLoopRunning = false

function rand(min: number, max: number) { return Math.random() * (max - min) + min }

function bakeStaticStars() {
  const layer = staticLayer.value
  if (!layer) return
  // 旋转容器是 200vmax，我们烘焙一张 3200x3200 的 canvas 平铺
  const w = 3200, h = 3200
  const layers = [
    { count: 600, size: 1, alpha: 0.55 },
    { count: 350, size: 1, alpha: 0.72 },
    { count: 80,  size: 2, alpha: 0.88 },
  ]
  const cvs = document.createElement('canvas')
  cvs.width = w; cvs.height = h
  const ctx = cvs.getContext('2d')
  if (!ctx) return
  for (const { count, size, alpha } of layers) {
    ctx.fillStyle = `rgba(255,255,255,${alpha})`
    for (let i = 0; i < count; i++) {
      const x = Math.random() * w
      const y = Math.random() * h
      ctx.beginPath()
      ctx.arc(x, y, size / 2, 0, Math.PI * 2)
      ctx.fill()
    }
  }
  layer.style.background = `url(${cvs.toDataURL('image/png')}) center / cover no-repeat, #000`
}

function buildTwinkleStars() {
  const layer = twinkleLayer.value
  if (!layer) return
  // rot 容器是 200vmax，twinkle 星均匀撒在整个面积里。
  // viewport 只占大约 1/4，所以总数要 ≥ 120 才能保证视口内有 30+ 颗在呼吸。
  const rotSize = Math.max(window.innerWidth, window.innerHeight) * 2
  const N = 140
  for (let i = 0; i < N; i++) {
    const s = document.createElement('i')
    s.className = 'star'
    if (Math.random() < 0.22) s.classList.add('star--big')
    if (Math.random() < 0.08) { s.classList.add('star--xl'); s.classList.remove('star--big') }
    // 30% 慢呼吸（更明显，4-9s），70% 快闪烁（1.5-4.5s）—— 视觉节奏不单调
    const slow = Math.random() < 0.3
    if (slow) {
      s.classList.add('star--breathe')
      s.style.animationDuration = rand(4, 9) + 's'
    } else {
      s.style.animationDuration = rand(1.5, 4.5) + 's'
    }
    s.style.left = rand(0, rotSize) + 'px'
    s.style.top  = rand(0, rotSize) + 'px'
    s.style.animationDelay = rand(0, 6) + 's'
    layer.appendChild(s)
  }
}

function fireMeteor() {
  const layer = meteorLayer.value
  if (!layer) return
  const m = document.createElement('div')
  m.className = 'meteor'
  const vw = window.innerWidth, vh = window.innerHeight
  const startX = Math.random() * vw * 0.7 + (Math.random() < 0.5 ? 0 : vw * 0.2)
  const startY = Math.random() * vh * 0.55
  const goLeft = startX > vw * 0.55
  const dir = goLeft ? -1 : 1
  const angleDeg = 18 + Math.random() * 22
  const angleRad = (angleDeg * Math.PI) / 180
  const dist = Math.max(vw, vh) * 0.8
  const dx = Math.cos(angleRad) * dist * dir
  const dy = Math.sin(angleRad) * dist
  const rotateDeg = (Math.atan2(dy, dx) * 180) / Math.PI

  m.style.left = startX + 'px'
  m.style.top  = startY + 'px'
  ;(m.style as any).rotate = rotateDeg + 'deg'
  m.style.setProperty('--mx', dx + 'px')
  m.style.setProperty('--my', dy + 'px')
  layer.appendChild(m)
  requestAnimationFrame(() => m.classList.add('is-fire'))
  window.setTimeout(() => m.remove(), 1800)
}

function meteorLoop() {
  if (!meteorLoopRunning) return
  fireMeteor()
  const next = 12000 + Math.random() * 13000  // 12-25s
  meteorTimer = window.setTimeout(meteorLoop, next)
}

onMounted(() => {
  bakeStaticStars()
  buildTwinkleStars()
  meteorLoopRunning = true
  meteorTimer = window.setTimeout(meteorLoop, 4000)
})

onBeforeUnmount(() => {
  meteorLoopRunning = false
  if (meteorTimer) window.clearTimeout(meteorTimer)
})
</script>

<template>
  <div class="starfield" aria-hidden="true">
    <div class="starfield__rot">
      <div ref="staticLayer" class="starfield__static"></div>
      <div ref="twinkleLayer" class="starfield__twinkle"></div>
    </div>
    <div ref="meteorLayer" class="starfield__meteors"></div>
    <div class="starfield__vignette"></div>
  </div>
</template>

<style>
.starfield {
  position: fixed; inset: 0; z-index: 0; pointer-events: none; overflow: hidden;
  background: #000;
}
.starfield__rot {
  position: absolute;
  width: 200vmax; height: 200vmax;
  left: 50%; top: 50%;
  transform-origin: center center;
  animation: stellar-star-rotate 360s linear infinite;
  will-change: transform;
}
@keyframes stellar-star-rotate {
  from { transform: translate(-50%, -50%) rotate(0deg); }
  to   { transform: translate(-50%, -50%) rotate(360deg); }
}
.starfield__static {
  position: absolute; inset: 0;
  background:
    radial-gradient(circle at 80% 10%, rgba(255,255,255,0.025), transparent 40%),
    radial-gradient(circle at 12% 88%, rgba(255,255,255,0.025), transparent 45%),
    #000;
}
.starfield__twinkle { position: absolute; inset: 0; }
.star {
  position: absolute; display: block;
  width: 2px; height: 2px; border-radius: 50%;
  background: rgba(255,255,255,0.7);
  animation: stellar-twinkle 4s ease-in-out infinite;
  will-change: opacity, transform;
}
.star.star--big {
  width: 3px; height: 3px;
  background: rgba(255,255,255,0.9);
  box-shadow: 0 0 8px rgba(255,255,255,0.4);
}
.star.star--xl {
  width: 4px; height: 4px;
  background: #fff;
  box-shadow: 0 0 10px rgba(255,255,255,0.55);
}
@keyframes stellar-twinkle {
  0%, 100% { opacity: 0.25; transform: scale(1); }
  50%      { opacity: 1.0;  transform: scale(1.35); }
}
/* 慢"呼吸"档：更大幅度的亮暗 + scale 变化，让大星显得真在喘气 */
.star.star--breathe { animation-name: stellar-breathe; }
@keyframes stellar-breathe {
  0%, 100% { opacity: 0.12; transform: scale(0.85); }
  45%      { opacity: 0.7;  transform: scale(1.1); }
  55%      { opacity: 1.0;  transform: scale(1.45); }
}

.starfield__meteors { position: absolute; inset: 0; pointer-events: none; overflow: hidden; }
.meteor {
  position: absolute;
  width: 160px; height: 1px;
  background: linear-gradient(90deg,
    rgba(255,255,255,0) 0%,
    rgba(255,255,255,0.08) 35%,
    rgba(255,255,255,0.55) 78%,
    rgba(255,255,255,1)  100%);
  transform-origin: 100% 50%;
  rotate: 0deg;
  translate: 0 0;
  opacity: 0;
  pointer-events: none;
  filter: drop-shadow(0 0 3px rgba(255,255,255,0.45));
}
.meteor::after {
  content: "";
  position: absolute;
  right: -2px; top: -1.5px;
  width: 4px; height: 4px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 0 8px rgba(255,255,255,0.85), 0 0 16px rgba(255,255,255,0.45);
}
.meteor.is-fire { animation: stellar-meteor-fly 1.6s cubic-bezier(0.22, 0.1, 0.55, 1) forwards; }
@keyframes stellar-meteor-fly {
  0%   { opacity: 0; translate: 0 0; }
  8%   { opacity: 1; }
  88%  { opacity: 1; }
  100% { opacity: 0; translate: var(--mx) var(--my); }
}

.starfield__vignette {
  position: absolute; inset: 0;
  background: radial-gradient(ellipse 70% 50% at 50% 45%, rgba(0,0,0,0.70), rgba(0,0,0,0.25) 70%, transparent 100%);
}
</style>
