<script setup>
defineProps({
  padding: { type: String, default: 'md' }, // sm | md | lg | none
  hover: { type: Boolean, default: true },
  elevated: { type: Boolean, default: false },
  variant: { type: String, default: 'default' }, // default | talisman | medical
  as: { type: String, default: 'div' },
})
</script>

<template>
  <component
    :is="as"
    class="world-card"
    :class="[
      `pad-${padding}`,
      { 'is-hoverable': hover, 'is-elevated': elevated, [`v-${variant}`]: variant !== 'default' }
    ]"
  >
    <slot />
  </component>
</template>

<style scoped>
.world-card {
  position: relative;
  background: var(--world-glass-bg);
  border: 1px solid var(--world-glass-border);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  border-radius: var(--world-radius-xl);
  transition: transform 220ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1)),
              box-shadow 220ms ease,
              border-color 220ms ease;
  box-shadow: var(--world-shadow-sm);
}
.pad-none { padding: 0; }
.pad-sm   { padding: 12px; }
.pad-md   { padding: 18px; }
.pad-lg   { padding: 24px; }

.is-elevated { box-shadow: var(--world-shadow-md); }

.is-hoverable:hover {
  box-shadow: var(--world-shadow-lg);
}

/* === Reality 形态：玻璃纸面 + 微抬升 + 扫描线 === */
[data-world="reality"] .world-card {
  background: var(--world-glass-bg);
  border-color: var(--world-glass-border);
}
[data-world="reality"] .world-card.is-hoverable:hover {
  transform: translateY(-2px);
  border-color: var(--world-accent);
}
[data-world="reality"] .world-card.v-medical::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(120deg, transparent 0%, rgba(2, 132, 199, 0.06) 50%, transparent 100%);
  background-size: 200% 100%;
  background-position: -100% 0;
  pointer-events: none;
  opacity: 0;
  transition: opacity 320ms ease;
}
[data-world="reality"] .world-card.v-medical:hover::before {
  opacity: 1;
  animation: medical-scan 1.6s ease-in-out;
}

/* === Daogui 形态：朱砂边晕（圆角，无斜边） === */
[data-world="daogui"] .world-card {
  background: var(--world-glass-bg);
  border-color: var(--world-glass-border);
}
[data-world="daogui"] .world-card.is-hoverable:hover {
  border-color: rgba(196, 30, 58, 0.42);
  box-shadow:
    0 0 22px rgba(196, 30, 58, 0.14),
    inset 0 0 28px rgba(74, 26, 74, 0.06),
    var(--world-shadow-md);
}
[data-world="daogui"] .world-card::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 1px;
  background: linear-gradient(90deg,
    transparent 0%,
    rgba(184, 134, 11, 0.32) 50%,
    transparent 100%);
  opacity: 0;
  transition: opacity 320ms ease;
  pointer-events: none;
}
[data-world="daogui"] .world-card.is-hoverable:hover::before {
  opacity: 1;
}
[data-world="daogui"] .world-card.v-talisman {
  border-color: rgba(196, 30, 58, 0.32);
}
[data-world="daogui"] .world-card.v-talisman:hover {
  animation: talisman-pulse 2.4s ease-in-out infinite;
}
</style>
