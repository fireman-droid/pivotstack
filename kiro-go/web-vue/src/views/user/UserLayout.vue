<script setup>
import { useRouter, useRoute } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'
import { computed, onMounted } from 'vue'
import { LayoutDashboard, Gift, ScrollText, LogOut, Zap } from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()
const auth = useUserAuth()

const navItems = [
  { path: '/user/dashboard', label: '概览', icon: LayoutDashboard },
  { path: '/user/recharge', label: '充值', icon: Gift },
  { path: '/user/logs', label: '日志', icon: ScrollText },
]

const isActivated = computed(() => !!auth.plan)
const balanceValue = computed(() => Number(auth.balance || 0))
const isCreditPlan = computed(() => auth.plan === 'credit' || auth.plan === 'hybrid')
const isTimedPlan = computed(() => auth.plan === 'timed' || auth.plan === 'hybrid')

const balanceDisplay = computed(() => {
  if (!isCreditPlan.value) return null
  return `¥${balanceValue.value.toFixed(2)}`
})

const balanceBadgeClass = computed(() => {
  if (balanceValue.value < 1) return 'low'
  return 'ok'
})

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

const timeBadgeClass = computed(() => {
  if (!isTimedPlan.value) return 'timed'
  const exp = auth.userInfo?.expiresAt || 0
  if (!exp) return 'timed'
  const diff = Math.max(0, exp - Date.now() / 1000)
  if (diff <= 0) return 'expired'
  if (diff < 3 * 86400) return 'urgent'
  if (diff < 7 * 86400) return 'warning'
  return 'timed'
})

function handleLogout() {
  auth.logout()
  router.replace('/login')
}

onMounted(() => {
  auth.refresh()
})
</script>

<template>
  <div class="user-layout">
    <header class="user-header">
      <div class="header-left">
        <div class="brand">
          <div class="brand-icon">
            <Zap :size="16" />
          </div>
          <span class="brand-text">KiroStack</span>
        </div>
        <nav class="header-nav">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            :class="['nav-link', { active: route.path === item.path }]"
          >
            <component :is="item.icon" :size="16" class="nav-icon-svg" />
            <span class="nav-label">{{ item.label }}</span>
          </router-link>
        </nav>
      </div>
      <div class="header-right">
        <div v-if="!isActivated" class="balance-badge inactive">未激活</div>
        <div v-if="balanceDisplay" :class="['balance-badge', balanceBadgeClass]">
          {{ balanceDisplay }}
        </div>
        <div v-if="timeDisplay" :class="['time-badge', timeBadgeClass]">
          {{ timeDisplay }}
        </div>
        <button class="logout-btn" @click="handleLogout" title="退出登录">
          <LogOut :size="16" />
        </button>
      </div>
    </header>

    <main class="user-main" id="main-content">
      <router-view />
    </main>

    <!-- Mobile bottom nav -->
    <nav class="mobile-nav">
      <router-link
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        :class="['mobile-nav-item', { active: route.path === item.path }]"
      >
        <component :is="item.icon" :size="20" />
        <span class="nav-label">{{ item.label }}</span>
      </router-link>
    </nav>
  </div>
</template>

<style scoped>
.user-layout {
  min-height: 100vh;
  background: #0f172a;
  color: #f8fafc;
}

.user-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 1.5rem;
  height: 60px;
  background: rgba(15, 23, 42, 0.8);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 2rem;
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.brand-icon {
  width: 28px;
  height: 28px;
  background: linear-gradient(135deg, #6366f1, #818cf8);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  box-shadow: 0 2px 8px rgba(99, 102, 241, 0.4);
}

.brand-text {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.1rem;
  font-weight: 700;
  background: linear-gradient(135deg, #818cf8, #c084fc);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  white-space: nowrap;
}

.header-nav {
  display: flex;
  gap: 0.25rem;
}

.nav-link {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.75rem;
  border-radius: 8px;
  text-decoration: none;
  color: #64748b;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all 0.2s;
  border-left: 2px solid transparent;
}

.nav-link:hover {
  color: #f8fafc;
  background: rgba(255, 255, 255, 0.06);
}

.nav-link.active {
  color: #818cf8;
  background: rgba(99, 102, 241, 0.1);
  border-left-color: #6366f1;
}

.nav-icon-svg {
  flex-shrink: 0;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.balance-badge {
  padding: 0.25rem 0.75rem;
  border-radius: 20px;
  font-weight: 600;
  font-size: 0.8125rem;
}

.balance-badge.ok {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.balance-badge.low {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.2);
  animation: pulse 2s ease-in-out infinite;
}

.balance-badge.inactive {
  background: rgba(100, 116, 139, 0.12);
  color: #94a3b8;
  border: 1px solid rgba(100, 116, 139, 0.2);
}

.time-badge {
  padding: 0.25rem 0.75rem;
  border-radius: 20px;
  font-weight: 600;
  font-size: 0.8125rem;
}

.time-badge.timed {
  background: rgba(99, 102, 241, 0.12);
  color: #818cf8;
  border: 1px solid rgba(99, 102, 241, 0.2);
}

.time-badge.warning {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.2);
}

.time-badge.urgent {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.2);
  animation: pulse 2s ease-in-out infinite;
}

.time-badge.expired {
  background: rgba(100, 116, 139, 0.12);
  color: #94a3b8;
  border: 1px solid rgba(100, 116, 139, 0.2);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}

.logout-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #64748b;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.logout-btn:hover {
  color: #ef4444;
  border-color: rgba(239, 68, 68, 0.3);
  background: rgba(239, 68, 68, 0.06);
}

.user-main {
  padding: 1.5rem;
  max-width: 1200px;
  margin: 0 auto;
  padding-bottom: 5rem;
}

.mobile-nav {
  display: none;
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  background: rgba(15, 23, 42, 0.95);
  backdrop-filter: blur(12px);
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding: 0.5rem;
  justify-content: space-around;
  z-index: 100;
}

.mobile-nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  text-decoration: none;
  color: #64748b;
  font-size: 0.7rem;
  padding: 0.4rem 1rem;
  border-radius: 8px;
  transition: all 0.2s;
}

.mobile-nav-item.active {
  color: #818cf8;
}

@media (max-width: 768px) {
  .header-nav { display: none; }
  .mobile-nav { display: flex; }
  .user-main { padding: 1rem; padding-bottom: 5rem; }
}
</style>
