<script setup>
import { cn } from '@/lib/utils'

const props = defineProps({
  class: { type: String, default: '' }
})
</script>

<template>
  <div :class="cn('rune-card', props.class)">
    <slot />
  </div>
</template>
