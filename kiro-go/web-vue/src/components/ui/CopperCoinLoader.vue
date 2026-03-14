<template>
  <div class="flex items-center justify-center p-8 relative">
    <svg class="w-16 h-16 animate-coin-spin" viewBox="0 0 100 100">
      <!-- 青铜钱币 + 方孔 -->
      <circle cx="50" cy="50" r="45" fill="none" stroke="currentColor" stroke-width="8" class="text-bronze-rune" />
      <rect x="35" y="35" width="30" height="30" fill="none" stroke="currentColor" stroke-width="6" class="text-bronze-rune" />
      <!-- 刻蚀符文（风格化） -->
      <path d="M50 15 L50 25 M85 50 L75 50 M50 85 L50 75 M15 50 L25 50" stroke="currentColor" stroke-width="4" class="text-bronze-rune/50" />
    </svg>
    <span class="absolute mt-24 text-xs tracking-widest text-cold-white/40 uppercase">加载中...</span>
  </div>
</template>

<style scoped>
svg {
  perspective: 1000px;
  filter: drop-shadow(0 0 8px rgba(184, 134, 11, 0.4));
}
</style>
