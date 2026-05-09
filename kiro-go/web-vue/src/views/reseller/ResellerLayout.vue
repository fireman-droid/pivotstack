<script setup>
import { useRoute } from 'vue-router'
import { LayoutDashboard, KeyRound } from 'lucide-vue-next'

const route = useRoute()

const subNav = [
  { path: '/user/reseller/summary', label: '代理总览', icon: LayoutDashboard },
  { path: '/user/reseller/keys',    label: '子 Key 管理', icon: KeyRound },
]
</script>

<template>
  <div class="reseller-shell">
    <header class="reseller-head">
      <div class="title-wrap">
        <div class="eyebrow">代理控制台</div>
        <h1 class="page-title">分销中心</h1>
      </div>
      <nav class="sub-nav" aria-label="代理子导航">
        <router-link
          v-for="item in subNav"
          :key="item.path"
          :to="item.path"
          :class="['sub-pill', { active: route.path === item.path }]"
        >
          <component :is="item.icon" :size="14" stroke-width="2.2" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
    </header>

    <router-view v-slot="{ Component, route: rv }">
      <Transition name="page-fade" mode="out-in">
        <component :is="Component" :key="rv.fullPath" />
      </Transition>
    </router-view>
  </div>
</template>

<style scoped>
.reseller-shell {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.reseller-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.75rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}
[data-world="daogui"] .page-title {
  background: linear-gradient(135deg, var(--world-text-primary) 0%, var(--world-paper-aged) 90%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.sub-nav { display: flex; gap: 4px; flex-wrap: wrap; }
.sub-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: var(--world-radius-lg);
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--world-text-mute);
  text-decoration: none;
  background: var(--world-glass-bg-strong);
  border: 1px solid var(--world-glass-border);
  transition: all 200ms ease;
}
.sub-pill:hover { color: var(--world-text-primary); background: var(--world-overlay-light); }
.sub-pill.active {
  color: white;
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  border-color: transparent;
}

.page-fade-enter-active { transition: all 260ms cubic-bezier(0.16, 1, 0.3, 1); }
.page-fade-leave-active { transition: all 180ms ease; }
.page-fade-enter-from   { opacity: 0; transform: translateY(8px); }
.page-fade-leave-to     { opacity: 0; }
</style>
