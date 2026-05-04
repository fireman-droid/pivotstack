<script setup>
import { useRouter, useRoute } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'
import { computed, onMounted } from 'vue'
import { LayoutDashboard, Gift, ScrollText, LogOut, Zap } from 'lucide-vue-next'
import WorldSwitcher from '../../components/WorldSwitcher.vue'
import WorldChip from '../../components/world/WorldChip.vue'

const router = useRouter()
const route = useRoute()
const auth = useUserAuth()

const navItems = [
  { path: '/user/dashboard', label: '概览', icon: LayoutDashboard },
  { path: '/user/recharge',  label: '充值', icon: Gift },
  { path: '/user/logs',      label: '日志', icon: ScrollText },
]

const isActivated = computed(() => !!auth.plan)
const balanceValue = computed(() => Number(auth.balance || 0))
const isCreditPlan = computed(() => auth.plan === 'credit' || auth.plan === 'hybrid')
const isTimedPlan  = computed(() => auth.plan === 'timed'  || auth.plan === 'hybrid')

const balanceDisplay = computed(() => {
  if (!isCreditPlan.value) return null
  return `$${balanceValue.value.toFixed(2)}`
})
const balanceVariant = computed(() => balanceValue.value < 1 ? 'danger' : 'success')

const timeDisplay = computed(() => {
  if (!isTimedPlan.value) return null
  const exp = auth.userInfo?.expiresAt || 0
  if (!exp) return '永久'
  const diff = Math.max(0, exp - Date.now() / 1000)
  if (diff <= 0) return '已过期'
  const d = Math.floor(diff / 86400)
  const h = Math.floor((diff % 86400) / 3600)
  const m = Math.max(1, Math.ceil((diff % 3600) / 60))
  let t = ''
  if (d > 0) t += d + '天'
  if (h > 0) t += h + '时'
  if (d === 0 && m > 0) t += m + '分'
  return t || '1分'
})
const timeVariant = computed(() => {
  if (!isTimedPlan.value) return 'info'
  const exp = auth.userInfo?.expiresAt || 0
  if (!exp) return 'info'
  const diff = Math.max(0, exp - Date.now() / 1000)
  if (diff <= 0) return 'neutral'
  if (diff < 3 * 86400) return 'danger'
  if (diff < 7 * 86400) return 'warning'
  return 'info'
})

function handleLogout() {
  auth.logout()
  router.replace('/login')
}

onMounted(() => { auth.refresh() })
</script>

<template>
  <div class="user-shell">
    <header class="topbar">
      <div class="topbar-inner">
        <div class="topbar-left">
          <router-link to="/user/dashboard" class="brand">
            <div class="brand-mark">
              <Zap :size="14" stroke-width="2.6" />
            </div>
            <span class="brand-name">KiroStack</span>
          </router-link>
          <nav class="nav-row" aria-label="用户导航">
            <router-link
              v-for="item in navItems"
              :key="item.path"
              :to="item.path"
              :class="['nav-pill', { active: route.path === item.path }]"
            >
              <component :is="item.icon" :size="14" stroke-width="2.2" />
              <span>{{ item.label }}</span>
            </router-link>
          </nav>
        </div>

        <div class="topbar-right">
          <WorldChip v-if="!isActivated" variant="neutral">未激活</WorldChip>
          <WorldChip v-if="balanceDisplay" :variant="balanceVariant" :dot="true">
            {{ balanceDisplay }}
          </WorldChip>
          <WorldChip v-if="timeDisplay" :variant="timeVariant" :pulse="timeVariant === 'danger'">
            {{ timeDisplay }}
          </WorldChip>
          <WorldSwitcher class="hide-on-mobile" />
          <button class="icon-btn" @click="handleLogout" title="退出登录" aria-label="退出">
            <LogOut :size="15" />
          </button>
        </div>
      </div>
    </header>

    <main class="user-main" id="main-content">
      <router-view v-slot="{ Component }">
        <Transition name="page-fade" mode="out-in">
          <component :is="Component" />
        </Transition>
      </router-view>
    </main>

    <nav class="mobile-tabbar" aria-label="移动端导航">
      <router-link
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        :class="['tabbar-item', { active: route.path === item.path }]"
      >
        <span class="tabbar-icon">
          <component :is="item.icon" :size="18" stroke-width="2.2" />
        </span>
        <span class="tabbar-label">{{ item.label }}</span>
      </router-link>
    </nav>
  </div>
</template>

<style scoped>
.user-shell {
  height: 100vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  color: var(--world-text-primary);
  background: var(--world-bg-main);
  font-family: var(--world-font-sans);
  position: relative;
}
.user-shell::before {
  content: '';
  position: fixed;
  inset: 0;
  z-index: -1;
  pointer-events: none;
  background-image:
    linear-gradient(rgba(148, 163, 184, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.05) 1px, transparent 1px);
  background-size: 40px 40px;
  opacity: 0.5;
}
[data-world="daogui"] .user-shell::before { opacity: 0.06; }

.topbar {
  flex-shrink: 0;
  z-index: 100;
  padding: 0 16px;
  margin-top: 16px;
}
.topbar-inner {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 16px;
  border-radius: var(--world-radius-2xl);
  background: var(--world-glass-bg-strong);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  border: 1px solid var(--world-glass-border);
  box-shadow: var(--world-shadow-md);
}
[data-world="daogui"] .topbar-inner { border-color: rgba(184, 134, 11, 0.22); }

.topbar-left { display: flex; align-items: center; gap: 28px; min-width: 0; }
.brand { display: flex; align-items: center; gap: 9px; flex-shrink: 0; text-decoration: none; }
.brand-mark {
  width: 28px; height: 28px;
  display: flex; align-items: center; justify-content: center;
  border-radius: var(--world-radius-md);
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  color: white;
}
[data-world="daogui"] .brand-mark { box-shadow: 0 0 14px rgba(196, 30, 58, 0.4); }
.brand-name {
  font-size: 1.05rem;
  font-weight: 800;
  letter-spacing: -0.01em;
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  white-space: nowrap;
  font-family: var(--world-font-display);
}

.nav-row { display: flex; gap: 4px; }
.nav-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: var(--world-radius-lg);
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--world-text-mute);
  text-decoration: none;
  transition: all 200ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1));
}
.nav-pill:hover { color: var(--world-text-primary); background: var(--world-overlay-light); }
.nav-pill.active {
  color: white;
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
}
[data-world="daogui"] .nav-pill.active { box-shadow: 0 4px 14px -4px rgba(196, 30, 58, 0.5); }

.topbar-right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }

.icon-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 32px; height: 32px;
  border-radius: var(--world-radius-md);
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  cursor: pointer;
  transition: all 200ms ease;
}
.icon-btn:hover {
  color: var(--world-error);
  background: rgba(239, 68, 68, 0.10);
  border-color: rgba(239, 68, 68, 0.35);
}

.user-main {
  flex: 1;
  overflow-y: auto;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 28px 24px 96px;
}

.mobile-tabbar {
  display: none;
  position: fixed;
  bottom: 12px; left: 12px; right: 12px;
  z-index: 100;
  background: var(--world-glass-bg-strong);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-2xl);
  padding: 8px;
  justify-content: space-around;
  box-shadow: var(--world-shadow-md);
}
[data-world="daogui"] .mobile-tabbar { border-color: rgba(184, 134, 11, 0.22); }

.tabbar-item {
  flex: 1;
  display: flex; flex-direction: column; align-items: center;
  gap: 2px; padding: 8px 4px;
  border-radius: var(--world-radius-lg);
  color: var(--world-text-mute);
  text-decoration: none;
  font-size: 0.7rem;
  font-weight: 700;
  transition: all 200ms ease;
}
.tabbar-icon {
  display: flex; align-items: center; justify-content: center;
  width: 28px; height: 28px;
  border-radius: var(--world-radius-md);
  transition: all 220ms ease;
}
.tabbar-item.active { color: var(--world-accent); }
.tabbar-item.active .tabbar-icon {
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  color: white;
}
[data-world="daogui"] .tabbar-item.active .tabbar-icon { box-shadow: 0 0 12px rgba(196, 30, 58, 0.5); }

.page-fade-enter-active { transition: all 260ms cubic-bezier(0.16, 1, 0.3, 1); }
.page-fade-leave-active { transition: all 180ms ease; }
.page-fade-enter-from   { opacity: 0; transform: translateY(8px); }
.page-fade-leave-to     { opacity: 0; }

@media (max-width: 768px) {
  .topbar { top: 8px; padding: 0 8px; margin-top: 8px; }
  .topbar-inner { padding: 0 12px; height: 52px; }
  .nav-row { display: none; }
  .brand-name { display: none; }
  .hide-on-mobile { display: none; }
  .mobile-tabbar { display: flex; }
  .user-main { padding: 20px 16px 96px; }
  .topbar-right { gap: 6px; }
  .icon-btn { width: 28px; height: 28px; }
}
</style>
