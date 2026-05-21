<script setup lang="ts">
defineProps<{
  active?: boolean
  radius?: number
}>()
</script>

<template>
  <div class="gb" :class="{ 'gb--active': active }" :style="{ borderRadius: (radius ?? 8) + 'px' }">
    <span class="gb__halo" :style="{ borderRadius: (radius ?? 8) + 'px' }" />
    <span class="gb__inner" :style="{ borderRadius: ((radius ?? 8) - 1) + 'px' }">
      <slot />
    </span>
  </div>
</template>

<style scoped>
.gb { position: relative; display: inline-block; }
.gb__halo {
  position: absolute; inset: 0;
  background: conic-gradient(from 0deg, #FF0080, #7928CA, #0070F3, #FF0080);
  opacity: 0;
  transition: opacity 200ms cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  z-index: 0;
}
.gb--active .gb__halo,
.gb:hover .gb__halo { opacity: 1; }
.gb__inner {
  position: relative; z-index: 1;
  display: block; margin: 1px;
  background: var(--color-bg-surface, #0a0a0a);
}
@media (prefers-reduced-motion: reduce) {
  .gb__halo { transition: none; }
  .gb--active .gb__halo,
  .gb:hover .gb__halo { opacity: 0.8; }
}
</style>
