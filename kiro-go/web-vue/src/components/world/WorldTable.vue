<script setup>
defineProps({
  columns: { type: Array, required: true }, // [{ key, label, align?, width?, mono? }]
  rows: { type: Array, required: true },
  emptyText: { type: String, default: '暂无数据' }, // 字面默认文案
  hover: { type: Boolean, default: true },
  compact: { type: Boolean, default: false },
  maxHeight: { type: String, default: '480px' }, // 限制表内滚动区高度
})
</script>

<template>
  <div
    class="world-table-wrap"
    :class="{ 'is-compact': compact }"
    :style="{ maxHeight }"
  >
    <table class="world-table">
      <thead>
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            :style="{ textAlign: col.align || 'left', width: col.width || 'auto' }"
          >
            {{ col.label }}
          </th>
        </tr>
      </thead>
      <tbody :class="{ 'is-hoverable': hover }">
        <tr v-for="(row, i) in rows" :key="row.id || i">
          <td
            v-for="col in columns"
            :key="col.key"
            :class="{ mono: col.mono }"
            :style="{ textAlign: col.align || 'left' }"
          >
            <slot :name="`cell-${col.key}`" :row="row" :col="col" :value="row[col.key]">
              {{ row[col.key] }}
            </slot>
          </td>
        </tr>
        <tr v-if="!rows.length">
          <td :colspan="columns.length" class="empty">{{ emptyText }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.world-table-wrap {
  width: 100%;
  overflow-x: auto;
  overflow-y: auto;
  border-radius: var(--world-radius-lg);
}
.world-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  font-family: var(--world-font-sans);
}

/* === 表头 sticky === */
.world-table thead th {
  padding: 12px 16px;
  font-size: 0.7rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--world-text-mute);
  border-bottom: 1px solid var(--world-divider);
  background: var(--world-glass-bg-strong);
  position: sticky;
  top: 0;
  z-index: 2;
  /* sticky 需要不透明背景；用 glass-bg-strong 已是 0.95 / 0.92 alpha */
}
.is-compact .world-table thead th { padding: 8px 12px; font-size: 0.65rem; }

/* === 表体 === */
.world-table tbody td {
  padding: 14px 16px;
  font-size: 0.875rem;
  color: var(--world-text-primary);
  border-bottom: 1px solid var(--world-divider);
  transition: background 220ms ease;
  background: transparent;
}
.is-compact .world-table tbody td { padding: 10px 12px; font-size: 0.8125rem; }
.world-table tbody td.mono {
  font-family: var(--world-font-mono);
  font-size: 0.8125rem;
  letter-spacing: 0.02em;
}
.world-table tbody tr:last-child td { border-bottom: none; }

.empty {
  padding: 32px 16px !important;
  text-align: center;
  color: var(--world-text-dim);
  font-size: 0.875rem;
}

/* === Reality 形态 === */
[data-world="reality"] .is-hoverable tr:hover td {
  background: rgba(2, 132, 199, 0.04);
}
[data-world="reality"] .world-table thead th::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 0;
  right: 0;
  height: 2px;
  background: linear-gradient(90deg,
    var(--world-accent) 0%,
    transparent 30%,
    transparent 70%,
    var(--world-accent) 100%);
  opacity: 0.4;
  pointer-events: none;
}

/* === Daogui 形态 === */
[data-world="daogui"] .world-table thead th {
  background: linear-gradient(180deg,
    rgba(20, 16, 14, 0.95) 0%,
    rgba(20, 16, 14, 0.92) 100%);
  color: var(--world-paper-aged);
  border-bottom-color: rgba(184, 134, 11, 0.3);
}
[data-world="daogui"] .world-table thead th::before {
  content: '';
  position: absolute;
  bottom: 2px;
  left: 50%;
  transform: translateX(-50%);
  width: 30%;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--world-accent), transparent);
  opacity: 0.6;
  pointer-events: none;
}
[data-world="daogui"] .is-hoverable tr:hover td {
  background: rgba(196, 30, 58, 0.06);
  box-shadow: inset 0 0 0 1px rgba(196, 30, 58, 0.10);
}

/* 滚动条美化 */
.world-table-wrap::-webkit-scrollbar { width: 6px; height: 6px; }
.world-table-wrap::-webkit-scrollbar-track { background: transparent; }
.world-table-wrap::-webkit-scrollbar-thumb {
  background: var(--world-glass-border);
  border-radius: 3px;
}
.world-table-wrap::-webkit-scrollbar-thumb:hover { background: var(--world-accent); }
</style>
