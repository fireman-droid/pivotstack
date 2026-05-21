<script setup lang="ts">
// PivotStack v6 统一 Page Header。
//
// v6 升级（2026-05）：视觉对齐 .page-head（admin.css），
// kicker 改成「<b>域名</b> / 子项」面包屑风格。
//
// 用法兼容旧 view（kicker + title + desc + actions slot）。

defineProps<{
  kicker?: string
  /** kicker 前可选的圆点颜色（已废弃，保留 prop 兼容旧调用） */
  kickerDot?: string
  title: string
  desc?: string
}>()
</script>

<template>
  <header class="page-head">
    <div>
      <div v-if="kicker" class="page-head__crumb">{{ kicker }}</div>
      <div class="page-head__title">
        <div class="t-display-admin">{{ title }}</div>
        <div v-if="desc" class="page-head__sub">{{ desc }}</div>
      </div>
    </div>
    <div v-if="$slots.tabs || $slots.actions" class="page-head__right">
      <slot name="tabs" />
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
/* 这里不重复定义 .page-head 等 — 全部走 admin.css 的全局样式 */
/* 仅响应式补丁 */
@media (max-width: 768px) {
  .page-head {
    flex-direction: column !important;
    align-items: flex-start !important;
  }
  .page-head__right {
    width: 100%;
    margin-top: 12px;
  }
}
</style>
