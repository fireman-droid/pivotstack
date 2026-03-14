<script setup>
import { useWorldTheme } from '@/stores/worldTheme';
const theme = useWorldTheme();
</script>

<template>
  <button
    @click="theme.toggleWorld"
    :aria-label="theme.currentWorld === 'reality' ? '切换到道诡世界' : '切换到现实世界'"
    :aria-pressed="theme.currentWorld === 'daogui'"
    role="switch"
    class="relative w-12 h-12 flex items-center justify-center transition-all duration-500 hover:rotate-90"
    :class="theme.currentWorld === 'daogui' ? 'text-red-600' : 'text-blue-500'"
  >
    <!-- 八卦铜镜 SVG -->
    <svg viewBox="0 0 100 100" class="w-10 h-10 drop-shadow-lg" aria-hidden="true">
      <circle cx="50" cy="50" r="45" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="10 5" />
      <circle cx="50" cy="50" r="30" :fill="theme.currentWorld === 'daogui' ? '#c41e3a' : '#e2e8f0'" class="transition-colors duration-700" />
      <!-- 镜面裂纹 - 仅道诡模式可见 -->
      <path v-if="theme.currentWorld === 'daogui'" d="M35 35 L65 65 M35 65 L65 35" stroke="black" stroke-width="1" opacity="0.5" />
    </svg>

    <!-- 屏幕阅读器文本 -->
    <span class="sr-only">
      当前主题：{{ theme.currentWorld === 'reality' ? '现实世界' : '道诡世界' }}
    </span>

    <!-- 视觉标签 -->
    <span class="absolute -bottom-1 text-[10px] font-bold uppercase tracking-tighter" aria-hidden="true">
      {{ theme.currentWorld === 'reality' ? 'Sanity' : 'Madness' }}
    </span>
  </button>
</template>

<style scoped>
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}
</style>
