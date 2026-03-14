<script setup>
import { computed } from 'vue';

const props = defineProps({
  type: { type: String, default: 'info' }, // 'success' | 'error' | 'info'
  message: { type: String, required: true },
})

const runeSymbol = computed(() => {
  switch (props.type) {
    case 'success': return '☯'; // 金铜符文
    case 'error': return '☠'; // 血滴符文
    default: return '卍'; // 中性符文
  }
});

const colorClass = computed(() => {
  switch (props.type) {
    case 'success': return 'border-bronze-rune text-bronze-rune';
    case 'error': return 'border-vermilion-blood text-vermilion-blood';
    default: return 'border-madness-purple text-cold-white';
  }
});
</script>

<template>
  <div
    class="rune-card p-4 flex items-center gap-3 animate-in slide-in-from-top-5 duration-300"
    :class="colorClass"
  >
    <span class="text-2xl animate-pulse">{{ runeSymbol }}</span>
    <p class="text-sm">{{ message }}</p>
  </div>
</template>
