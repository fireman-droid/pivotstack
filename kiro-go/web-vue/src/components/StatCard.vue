<script setup>
defineProps({ label: String, value: [String, Number] })
</script>

<template>
  <div class="bg-[var(--card)] rounded-xl p-3 text-center shadow-sm">
    <div class="text-xl font-bold text-[var(--primary)]">{{ value }}</div>
    <div class="text-[11px] text-[var(--text-secondary)] mt-0.5">{{ label }}</div>
  </div>
</template>
