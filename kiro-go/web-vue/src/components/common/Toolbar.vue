<script setup lang="ts">
// 列表页顶部 search + filter + actions 横向条。
// 用法：<Toolbar><template #left>...</template><template #right>...</template></Toolbar>
</script>

<template>
  <div class="toolbar">
    <div class="toolbar__left">
      <slot name="left" />
    </div>
    <div class="toolbar__right">
      <slot name="right" />
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; margin-bottom: 16px;
}
.toolbar__left, .toolbar__right {
  display: flex; align-items: center; gap: 8px;
}
@media (max-width: 768px) {
  .toolbar { flex-wrap: wrap; }
}
</style>
