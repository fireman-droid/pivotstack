<script setup>
defineProps({
  items: { type: Array, required: true }, // [{ id, title, time, status?, meta?, ...slot data }]
  emptyText: { type: String, default: '暂无记录' }, // 字面默认
  variant: { type: String, default: 'compact' }, // compact | spacious
})
</script>

<template>
  <div class="world-timeline" :class="`v-${variant}`">
    <div v-if="!items.length" class="empty">{{ emptyText }}</div>
    <article
      v-for="item in items"
      :key="item.id"
      class="tl-item"
      :class="item.status ? `is-${item.status}` : ''"
    >
      <div class="tl-rail">
        <span class="tl-dot" />
      </div>
      <div class="tl-content">
        <header class="tl-head">
          <slot name="title" :item="item">
            <span class="tl-title">{{ item.title }}</span>
          </slot>
          <span class="tl-time">{{ item.time }}</span>
        </header>
        <div v-if="$slots.body || item.meta" class="tl-body">
          <slot name="body" :item="item">
            <span v-if="item.meta" class="tl-meta">{{ item.meta }}</span>
          </slot>
        </div>
      </div>
    </article>
  </div>
</template>

<style scoped>
.world-timeline {
  display: flex;
  flex-direction: column;
  gap: 0;
}
.empty {
  padding: 32px 16px;
  text-align: center;
  color: var(--world-text-dim);
  font-size: 0.875rem;
}

.tl-item {
  display: flex;
  gap: 14px;
  position: relative;
  padding: 14px 4px;
}
.v-spacious .tl-item { padding: 22px 4px; }

.tl-rail {
  position: relative;
  width: 14px;
  flex-shrink: 0;
  display: flex;
  justify-content: center;
}
.tl-rail::before {
  content: '';
  position: absolute;
  top: 0; bottom: 0;
  left: 50%;
  width: 1px;
  background: var(--world-divider);
  transform: translateX(-50%);
}
.tl-item:first-child .tl-rail::before { top: 12px; }
.tl-item:last-child  .tl-rail::before { bottom: calc(100% - 12px); }

.tl-dot {
  position: relative;
  width: 9px; height: 9px;
  border-radius: 50%;
  background: var(--world-text-mute);
  margin-top: 4px;
  flex-shrink: 0;
  transition: all 220ms ease;
  z-index: 1;
}
.is-success .tl-dot { background: var(--world-success); box-shadow: 0 0 8px var(--world-success); }
.is-warning .tl-dot { background: var(--world-warning); box-shadow: 0 0 8px var(--world-warning); }
.is-danger  .tl-dot { background: var(--world-error);   box-shadow: 0 0 8px var(--world-error);   }
.is-info    .tl-dot { background: var(--world-info);    box-shadow: 0 0 8px var(--world-info);    }

.tl-content {
  flex: 1;
  min-width: 0;
}
.tl-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 4px;
}
.tl-title {
  font-size: 0.875rem;
  font-weight: 700;
  color: var(--world-text-primary);
}
.tl-time {
  font-size: 0.7rem;
  color: var(--world-text-dim);
  font-family: var(--world-font-mono);
  flex-shrink: 0;
}
.tl-body {
  font-size: 0.8125rem;
  color: var(--world-text-mute);
  line-height: 1.5;
}
.tl-meta {
  display: inline-block;
  margin-right: 12px;
}

/* === Reality 形态: 病历单 === */
[data-world="reality"] .tl-item:hover {
  background: rgba(2, 132, 199, 0.04);
  border-radius: var(--world-radius-md);
}

/* === Daogui 形态: 司命卷轴 === */
[data-world="daogui"] .tl-rail::before {
  background: linear-gradient(180deg, transparent, rgba(184, 134, 11, 0.3), transparent);
  width: 1.5px;
}
[data-world="daogui"] .tl-item:hover {
  background: rgba(196, 30, 58, 0.04);
  border-radius: var(--world-radius-md);
}
[data-world="daogui"] .tl-item:hover .tl-dot {
  box-shadow: 0 0 12px var(--world-vermilion-glow);
}
[data-world="daogui"] .tl-title {
  color: var(--world-text-primary);
}
</style>
