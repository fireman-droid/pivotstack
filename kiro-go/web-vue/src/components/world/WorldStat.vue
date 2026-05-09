<script setup>
defineProps({
  label: { type: String, required: true },     // 字面文案：余额 / 剩余天数 等
  value: { type: [String, Number], required: true },
  unit: { type: String, default: '' },          // $/￥/credit/天 字面单位
  hint: { type: String, default: '' },
  subHint: { type: String, default: '' },       // 第二行小字（状态短语等）
  variant: { type: String, default: 'primary' },// primary | success | warning | danger | info
  icon: { type: [Object, Function], default: null },
  trend: { type: String, default: '' },         // 'up' | 'down' | ''
  trendValue: { type: String, default: '' },
})
</script>

<template>
  <div class="world-stat" :class="`v-${variant}`">
    <div class="stat-head">
      <span class="stat-label">{{ label }}</span>
      <component v-if="icon" :is="icon" class="stat-icon" />
    </div>
    <div class="stat-value-row">
      <span class="stat-value">{{ value }}</span>
      <span v-if="unit" class="stat-unit">{{ unit }}</span>
    </div>
    <div v-if="hint || trend" class="stat-foot">
      <span v-if="trend" :class="['trend', trend]">
        <span class="trend-arrow">{{ trend === 'up' ? '↑' : '↓' }}</span>
        {{ trendValue }}
      </span>
      <span v-if="hint" class="stat-hint">{{ hint }}</span>
    </div>
    <div v-if="subHint" class="stat-subhint">{{ subHint }}</div>
  </div>
</template>

<style scoped>
.world-stat {
  position: relative;
  background: var(--world-glass-bg);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-xl);
  padding: 18px 20px;
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: all 240ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1));
  overflow: hidden;
}
.stat-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.stat-label {
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: var(--world-text-mute);
  text-transform: uppercase;
}
.stat-icon {
  width: 18px;
  height: 18px;
  color: var(--world-text-mute);
  opacity: 0.6;
}
.stat-value-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-family: var(--world-font-display, var(--world-font-sans));
}
.stat-value {
  font-size: 1.875rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--world-text-primary);
  line-height: 1;
}
.stat-unit {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--world-text-mute);
}
.stat-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.75rem;
  color: var(--world-text-mute);
}
.trend {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 8px;
  border-radius: var(--world-radius-full);
  font-weight: 700;
}
.trend.up {
  background: rgba(16, 185, 129, 0.10);
  color: var(--world-success);
}
.trend.down {
  background: rgba(239, 68, 68, 0.10);
  color: var(--world-error);
}
.trend-arrow { font-size: 0.7rem; }

.stat-subhint {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--world-text-mute);
  opacity: 0.75;
  letter-spacing: 0.02em;
  margin-top: 2px;
}
.v-success .stat-subhint { color: var(--world-success); opacity: 0.8; }
.v-warning .stat-subhint { color: var(--world-warning); opacity: 0.85; }
.v-danger  .stat-subhint { color: var(--world-error);   opacity: 0.85; }

/* === Variant 强调（左侧装饰条 + 数字色） === */
.world-stat::before {
  content: '';
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: var(--world-accent);
  opacity: 0.7;
}
.v-success::before { background: var(--world-success); }
.v-warning::before { background: var(--world-warning); }
.v-danger::before  { background: var(--world-error); }
.v-info::before    { background: var(--world-info); }

/* === Reality 形态: 数显/医疗 === */
[data-world="reality"] .world-stat:hover {
  transform: translateY(-2px);
  box-shadow: var(--world-shadow-lg);
  border-color: var(--world-accent);
}

/* === Daogui 形态: 印章风（数字嵌红章，圆角无斜边） === */
[data-world="daogui"] .world-stat::after {
  content: '';
  position: absolute;
  top: -50px;
  right: -50px;
  width: 140px;
  height: 140px;
  background: radial-gradient(circle, rgba(196, 30, 58, 0.12), transparent 70%);
  pointer-events: none;
  opacity: 0.6;
}
[data-world="daogui"] .stat-value {
  color: var(--world-paper-aged);
  text-shadow:
    0 0 12px rgba(184, 134, 11, 0.3),
    0 0 4px rgba(196, 30, 58, 0.2);
}
[data-world="daogui"] .v-success .stat-value { color: #95b5a8; text-shadow: 0 0 10px rgba(82, 121, 111, 0.4); }
[data-world="daogui"] .v-warning .stat-value { color: #f3c66e; text-shadow: 0 0 10px rgba(218, 165, 32, 0.4); }
[data-world="daogui"] .v-danger  .stat-value { color: #f5707f; text-shadow: 0 0 12px rgba(196, 30, 58, 0.5); }
[data-world="daogui"] .v-info    .stat-value { color: #b39be8; text-shadow: 0 0 10px rgba(124, 58, 237, 0.4); }

[data-world="daogui"] .world-stat:hover {
  transform: translateY(-3px);
  border-color: rgba(196, 30, 58, 0.42);
  box-shadow:
    0 0 24px rgba(196, 30, 58, 0.18),
    inset 0 0 28px rgba(74, 26, 74, 0.08);
}
</style>
