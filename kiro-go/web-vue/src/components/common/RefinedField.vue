<script setup lang="ts">
// RefinedDialog 配套的"精致字段"组件：label + control + hint。
// 用法：
//   <RefinedField label="渠道名" required hint="将作为 token name 同时用于 PivotStack channel">
//     <n-input v-model:value="..." />
//   </RefinedField>
defineProps<{
  label: string
  hint?: string
  required?: boolean
  // 错误提示，传非空字符串时显示红色 + border 红色
  error?: string
}>()
</script>

<template>
  <div class="rf" :class="{ 'rf--error': !!error }">
    <label class="rf__label">
      <span class="rf__label-text">{{ label }}</span>
      <span v-if="required" class="rf__req">*</span>
    </label>
    <div class="rf__ctrl">
      <slot />
      <p v-if="error" class="rf__error">{{ error }}</p>
      <p v-else-if="hint" class="rf__hint">{{ hint }}</p>
    </div>
  </div>
</template>

<style scoped>
.rf {
  display: grid;
  grid-template-columns: 110px 1fr;
  gap: 12px 16px;
  align-items: start;
  margin-bottom: 16px;
}
.rf:last-child { margin-bottom: 0; }
.rf__label {
  color: #a3a3a3;
  font-size: 12px;
  font-weight: 500;
  padding-top: 7px;
  display: flex;
  align-items: center;
  gap: 4px;
  letter-spacing: 0.01em;
}
.rf__req { color: #ff7a7a; font-weight: 600; }
.rf__ctrl { min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.rf__hint { margin: 0; color: #707070; font-size: 11px; line-height: 1.4; }
.rf__error { margin: 0; color: #ff7a7a; font-size: 11px; line-height: 1.4; }
.rf--error :deep(.n-input .n-input__border),
.rf--error :deep(.n-input .n-input__state-border) { border-color: rgba(255, 122, 122, 0.5) !important; }
@media (max-width: 640px) {
  .rf { grid-template-columns: 1fr; gap: 6px; }
  .rf__label { padding-top: 0; }
}
</style>
