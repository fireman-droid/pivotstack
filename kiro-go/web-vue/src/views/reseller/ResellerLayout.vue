<script setup lang="ts">
import { useRoute } from 'vue-router'
import { LayoutDashboard, KeyRound } from 'lucide-vue-next'

const route = useRoute()
const subNav = [
  { path: '/user/reseller/summary', label: '代理总览', icon: LayoutDashboard },
  { path: '/user/reseller/keys', label: '子 Key 管理', icon: KeyRound },
]
</script>

<template>
  <div class="reseller-shell">
    <nav class="sub-nav" aria-label="代理子导航">
      <router-link
        v-for="item in subNav"
        :key="item.path"
        :to="item.path"
        :class="['sub-pill', { active: route.path === item.path }]"
      >
        <component :is="item.icon" :size="14" stroke-width="2" />
        <span>{{ item.label }}</span>
      </router-link>
    </nav>

    <router-view v-slot="{ Component, route: rv }">
      <component :is="Component" :key="rv.fullPath" />
    </router-view>
  </div>
</template>

<style scoped>
.reseller-shell { display: flex; flex-direction: column; gap: 12px; }
.sub-nav { display: flex; gap: 4px; padding: 8px 0 0; }
.sub-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 13px;
  color: #a1a1a1;
  text-decoration: none;
  transition: color 150ms, background 150ms;
}
.sub-pill:hover { color: #ededed; background: rgba(255,255,255,0.04); }
.sub-pill.active { color: #ededed; background: rgba(255,255,255,0.06); }
</style>
