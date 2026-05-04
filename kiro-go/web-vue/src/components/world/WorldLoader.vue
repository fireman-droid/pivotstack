<script setup>
defineProps({
  size: { type: [String, Number], default: 56 },
  label: { type: String, default: '加载中...' }, // 字面文案
  showLabel: { type: Boolean, default: true },
  inline: { type: Boolean, default: false },
})
</script>

<template>
  <div class="world-loader" :class="{ 'is-inline': inline }">
    <!-- Reality: 心电图扫描波 -->
    <svg
      class="loader-reality"
      :width="size" :height="size"
      viewBox="0 0 64 64"
      aria-hidden="true"
    >
      <path
        d="M2 32 L14 32 L18 18 L24 46 L30 22 L36 42 L42 28 L48 32 L62 32"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>

    <!-- Daogui: 青铜钱币 -->
    <svg
      class="loader-daogui"
      :width="size" :height="size"
      viewBox="0 0 100 100"
      aria-hidden="true"
    >
      <circle cx="50" cy="50" r="44" fill="none" stroke="currentColor" stroke-width="6" />
      <rect x="36" y="36" width="28" height="28" fill="none" stroke="currentColor" stroke-width="5" />
      <path
        d="M50 14 L50 24 M86 50 L76 50 M50 86 L50 76 M14 50 L24 50"
        stroke="currentColor"
        stroke-width="3"
        opacity="0.6"
      />
    </svg>

    <span v-if="showLabel" class="loader-label">{{ label }}</span>
  </div>
</template>

<style scoped>
.world-loader {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  padding: 20px;
  color: var(--world-accent);
}
.is-inline { padding: 0; flex-direction: row; gap: 10px; }

.loader-reality, .loader-daogui { display: none; }

[data-world="reality"] .loader-reality {
  display: block;
  filter: drop-shadow(0 0 6px rgba(2, 132, 199, 0.45));
  animation: rl-scan 1.4s var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1)) infinite;
  stroke-dasharray: 240;
}
@keyframes rl-scan {
  0%   { stroke-dashoffset: 240; opacity: 0.4; }
  50%  { opacity: 1; }
  100% { stroke-dashoffset: 0;   opacity: 0.6; }
}

[data-world="daogui"] .loader-daogui {
  display: block;
  color: var(--world-paper-aged);
  filter: drop-shadow(0 0 8px rgba(184, 134, 11, 0.5));
  animation: dg-coin 2.4s linear infinite;
}
@keyframes dg-coin {
  0%   { transform: rotateY(0deg); }
  100% { transform: rotateY(360deg); }
}

.loader-label {
  font-size: 0.75rem;
  letter-spacing: 0.18em;
  color: var(--world-text-dim);
  text-transform: uppercase;
  font-family: var(--world-font-mono);
}
[data-world="daogui"] .loader-label {
  color: var(--world-text-mute);
  letter-spacing: 0.24em;
}
</style>
