<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { resolveActiveRail, getRailById } from '../../design/rail'

const route = useRoute()
const activeRail = computed(() => getRailById(resolveActiveRail(route.path)))

const activeTo = computed(() => {
  const candidates = activeRail.value.tree
    .map(t => t.to)
    .filter(to => route.path === to || route.path.startsWith(to + '/'))
    .sort((a, b) => b.length - a.length)
  return candidates[0] ?? ''
})

function isActive(to: string) {
  return to === activeTo.value
}
</script>

<template>
  <aside class="tree">
    <div class="tree__brand">
      <span class="tree__brand-name">PivotStack</span>
      <span class="tree__brand-role">ADMIN</span>
    </div>

    <div class="tree__head">
      <span class="tree__head-dot" />
      <span class="tree__title">{{ activeRail.label }}</span>
    </div>

    <nav class="tree__nav" :aria-label="activeRail.label">
      <template v-for="(item, i) in activeRail.tree" :key="item.to">
        <div v-if="item.divider && i > 0" class="tree__divider" aria-hidden="true" />
        <RouterLink :to="item.to" class="tree__item" :class="{ 'is-active': isActive(item.to) }">
          <span class="tree__marker" aria-hidden="true" />
          <span class="tree__body">
            <span class="tree__label">
              {{ item.label }}
              <span v-if="item.isNew" class="tree__new-badge">NEW</span>
            </span>
            <span v-if="item.hint" class="tree__hint">{{ item.hint }}</span>
          </span>
        </RouterLink>
      </template>
    </nav>
  </aside>
</template>

<style scoped>
.tree {
  width: 200px;
  min-width: 200px;
  height: 100vh;
  background: #080808;
  border-right: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  position: relative;
  z-index: 5;
}

/* ─── brand bar ─── */
.tree__brand {
  height: 56px;
  padding: 0 18px;
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.tree__brand-name {
  font-family: "Geist", Inter, sans-serif;
  font-size: 14px;
  font-weight: 600;
  color: #ededed;
  letter-spacing: -0.01em;
}
.tree__brand-role {
  display: inline-flex;
  align-items: center;
  padding: 2px 6px;
  border: 1px solid rgba(11, 212, 112, 0.30);
  border-radius: 3px;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.10em;
  color: #0bd470;
  background: rgba(11, 212, 112, 0.06);
}

/* ─── head: active rail name ─── */
.tree__head {
  height: 36px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.tree__head-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #0bd470;
  box-shadow: 0 0 6px rgba(11, 212, 112, 0.55);
}
.tree__title {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: #a1a1a1;
}

/* ─── nav scroll ─── */
.tree__nav {
  flex: 1;
  overflow-y: auto;
  padding: 6px 8px 16px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.tree__nav::-webkit-scrollbar { width: 4px; }
.tree__nav::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.08);
  border-radius: 2px;
}

.tree__divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.06);
  margin: 8px 0;
}

/* ─── tree item with left-dot marker ─── */
.tree__item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 4px;
  text-decoration: none;
  color: #a1a1a1;
  font-size: 13px;
  transition: background 160ms ease, color 160ms ease;
  position: relative;
}
.tree__marker {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 6px;
  flex-shrink: 0;
  box-sizing: border-box;
  border: 1px solid rgba(255, 255, 255, 0.20);
  background: transparent;
  transition: background 160ms ease, border-color 160ms ease, box-shadow 240ms ease;
}
.tree__item:hover {
  background: rgba(255, 255, 255, 0.04);
  color: #ededed;
}
.tree__item:hover .tree__marker {
  border-color: rgba(255, 255, 255, 0.40);
}
.tree__item.is-active {
  background: rgba(11, 212, 112, 0.08);
  color: #ededed;
}
.tree__item.is-active .tree__marker {
  background: #0bd470;
  border-color: #0bd470;
  box-shadow: 0 0 8px rgba(11, 212, 112, 0.55);
}

.tree__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}
.tree__label {
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.tree__hint {
  font-size: 11px;
  color: #707070;
  letter-spacing: 0.02em;
  line-height: 1.3;
}
.tree__item.is-active .tree__hint { color: #0bd470; }

/* NEW 角标 */
.tree__new-badge {
  display: inline-flex;
  align-items: center;
  height: 14px;
  padding: 0 4px;
  border-radius: 2px;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.08em;
  background: rgba(11, 212, 112, 0.14);
  color: #0bd470;
}
</style>
