<script setup lang="ts">
import { computed } from 'vue'

type Status = 'enabled' | 'disabled' | 'error' | 'pending' | 'success' | 'warning'

const props = defineProps<{
  status: Status
  label?: string
}>()

const palette: Record<Status, { color: string; text: string }> = {
  enabled: { color: 'var(--color-success, #0bd470)', text: '启用' },
  disabled: { color: 'var(--color-text-tertiary, #707070)', text: '禁用' },
  error: { color: 'var(--color-error, #ff4d4d)', text: '错误' },
  pending: { color: 'var(--color-warning, #f5a623)', text: '处理中' },
  success: { color: 'var(--color-success, #0bd470)', text: '成功' },
  warning: { color: 'var(--color-warning, #f5a623)', text: '警告' },
}

const tone = computed(() => palette[props.status])
const display = computed(() => props.label ?? tone.value.text)
</script>

<template>
  <span class="status-badge" :style="{ color: tone.color }">
    <span class="status-badge__dot" :style="{ background: tone.color }" />
    {{ display }}
  </span>
</template>

<style scoped>
.status-badge {
  display: inline-flex; align-items: center; gap: 6px;
  font-size: 12px; line-height: 16px;
  white-space: nowrap;
}
.status-badge__dot {
  width: 8px; height: 8px; border-radius: 50%; display: inline-block;
}
</style>
