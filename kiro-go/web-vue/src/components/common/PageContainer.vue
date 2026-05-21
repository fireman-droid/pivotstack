<script setup lang="ts">
// PivotStack v6 page wrapper —— 全平台统一外框 + 版心居中。
//
// v6 升级（2026-05）：max-width 1440 → 1600，padding 横向 24 → 32，
// 对齐 admin v6 设计令牌（admin.css 的 .admin-page）。
//
// 现有 view（Settings / Pricing / Codes / Channels 等）通过这里
// 自动升级到 admin v6 视觉，不需要逐个改 template。
</script>

<template>
  <div class="page-container">
    <slot />
  </div>
</template>

<style scoped>
.page-container {
  max-width: 1600px;
  margin: 0 auto;
  padding: 24px 32px 48px;
  min-height: 100vh;
}
@media (max-width: 768px) {
  .page-container { padding: 16px 16px 32px; }
}
</style>
