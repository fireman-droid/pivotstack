<script setup>
import { ref } from 'vue';

const isSplashing = ref(false);

const triggerSplash = () => {
  isSplashing.value = true;
  setTimeout(() => isSplashing.value = false, 600);
};
</script>

<template>
  <div class="relative inline-block">
    <button
      @click="triggerSplash"
      class="relative z-10 bg-void-black border border-madness-purple hover:bg-vermilion-blood/20 transition-all duration-500 overflow-hidden px-4 py-2 rounded-xl text-cold-white font-bold text-sm"
      v-bind="$attrs"
    >
      <slot />
      <!-- 内部血雾脉冲 -->
      <div v-if="isSplashing" class="absolute inset-0 bg-vermilion-blood animate-ping opacity-25"></div>
    </button>

    <!-- 外部血溅粒子 -->
    <div v-if="isSplashing" class="absolute inset-0 pointer-events-none">
      <div v-for="n in 8" :key="n"
        class="particle"
        :style="{
          '--angle': `${n * 45}deg`,
          '--delay': `${Math.random() * 0.2}s`
        }"
      ></div>
    </div>
  </div>
</template>

<style scoped>
.particle {
  position: absolute;
  top: 50%; left: 50%;
  width: 4px; height: 12px;
  background: var(--color-vermilion-blood, #c41e3a);
  border-radius: 50%;
  transform: translate(-50%, -50%) rotate(var(--angle)) translateY(0);
  animation: splash 0.6s ease-out forwards;
  animation-delay: var(--delay);
}
@keyframes splash {
  to {
    transform: translate(-50%, -50%) rotate(var(--angle)) translateY(-40px);
    opacity: 0;
  }
}
</style>
