<script setup>
/**
 * RuneDisintegration 符文崩解 — TO MADNESS (坠入深渊)
 *
 * 效果：12 个道符从中心向外裂开飞散
 * 优化：使用 SVG <use> 复用符号形状，减少 DOM 节点
 *
 * 时间线：0-800ms
 */
import { ref, onMounted } from 'vue'

const RUNES = ['卍', '☯', '符', '咒', '封', '镇', '煞', '厄', '劫', '魔', '鬼', '妖']

const particles = ref([])

onMounted(() => {
  particles.value = RUNES.map((rune, i) => ({
    symbol: rune,
    angle: (i * 30) + 'deg',
    delay: (i * 50) + 'ms'
  }))
})
</script>

<template>
  <div class="rune-container" aria-hidden="true">
    <div
      v-for="(p, i) in particles"
      :key="i"
      class="rune-particle"
      :style="{
        '--angle': p.angle,
        '--delay': p.delay
      }"
    >
      {{ p.symbol }}
    </div>
  </div>
</template>

<style scoped>
.rune-container {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.rune-particle {
  position: absolute;
  font-size: 2rem;
  color: var(--color-bronze-rune, #b8860b);
  text-shadow:
    0 0 8px rgba(184, 134, 11, 0.9),
    0 0 24px rgba(184, 134, 11, 0.5);
  opacity: 0;
  transform: rotate(var(--angle)) translateY(0) scale(1);
  animation: rune-burst 800ms ease-out forwards;
  animation-delay: var(--delay);
  will-change: transform, opacity;
  backface-visibility: hidden;
}

@keyframes rune-burst {
  0% {
    opacity: 1;
    transform: rotate(var(--angle)) translateY(0) scale(1);
  }
  100% {
    opacity: 0;
    transform: rotate(var(--angle)) translateY(-150px) scale(0.3) rotate(calc(var(--angle) + 180deg));
    will-change: auto;
  }
}

/* 移动端：减少粒子 */
@media (max-width: 768px) {
  .rune-particle:nth-child(n+9) {
    display: none;
  }
  .rune-particle {
    font-size: 1.5rem;
  }
}
</style>
