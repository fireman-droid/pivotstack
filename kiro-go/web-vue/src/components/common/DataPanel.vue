<script setup lang="ts">
// 详情页 section 容器 — 替代列表/详情卡片，遵循 plan §5.3 卡片纪律。
// 用：标题 + 右侧 actions slot + body slot；用 SectionDivider 分隔多个 panel。

defineProps<{
  title?: string
  desc?: string
}>()
</script>

<template>
  <section class="data-panel">
    <header v-if="title || $slots.actions" class="data-panel__head">
      <div>
        <h3 v-if="title" class="data-panel__title">{{ title }}</h3>
        <p v-if="desc" class="data-panel__desc">{{ desc }}</p>
      </div>
      <div class="data-panel__actions">
        <slot name="actions" />
      </div>
    </header>
    <div class="data-panel__body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.data-panel { margin: 24px 0; }
.data-panel__head {
  display: flex; align-items: flex-start; justify-content: space-between;
  gap: 16px; margin-bottom: 16px;
}
.data-panel__title {
  font-size: 16px; font-weight: 600; margin: 0 0 4px;
  color: var(--color-text-primary, #ededed);
}
.data-panel__desc {
  font-size: 13px; margin: 0;
  color: var(--color-text-secondary, #a1a1a1);
}
.data-panel__actions { display: flex; gap: 8px; align-items: center; }
.data-panel__body { /* 容器透明，由子节点自己定 layout */ }
</style>
