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

const balanceDisplay = computed(() => {
  if (!isActivated.value) return '未激活'
  if (auth.plan === 'timed') {
    // 显示剩余时间
    const exp = auth.userInfo?.expiresAt || 0
    if (!exp) return '永久'
    const diff = Math.max(0, exp - Date.now() / 1000)
    if (diff <= 0) return '已过期'
    if (diff >= 86400) return Math.floor(diff / 86400) + '天'
    if (diff >= 3600) return Math.floor(diff / 3600) + '小时'
    return Math.max(1, Math.ceil(diff / 60)) + '分钟'
  }
  return `¥${balanceValue.value.toFixed(2)}`
})

const balanceBadgeClass = computed(() => {
  if (!isActivated.value) return 'inactive'
  if (auth.plan === 'timed') return 'timed'
  if (balanceValue.value < 1) return 'low'
  return 'ok'
})

const planLabel = computed(() => {
  if (!isActivated.value) return '未激活'
  const labels = { timed: '时间制', credit: '计量制', hybrid: '混合制' }
  return labels[auth.plan] || auth.plan
})

const planTagClass = computed(() => {
  const map = { timed: 'blue', credit: 'green', hybrid: 'purple' }
  return map[auth.plan] || 'gray'
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
        <div :class="['balance-badge', balanceBadgeClass]">
          {{ balanceDisplay }}
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

.balance-badge.timed {
  background: rgba(99, 102, 241, 0.12);
  color: #818cf8;
  border: 1px solid rgba(99, 102, 241, 0.2);
}

.balance-badge.inactive {
  background: rgba(100, 116, 139, 0.12);
  color: #94a3b8;
  border: 1px solid rgba(100, 116, 139, 0.2);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}

.plan-tag {
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  font-size: 0.75rem;
  font-weight: 600;
}

.plan-tag.green {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.plan-tag.blue {
  background: rgba(99, 102, 241, 0.1);
  color: #818cf8;
}

.plan-tag.purple {
  background: rgba(168, 85, 247, 0.1);
  color: #c084fc;
}

.plan-tag.gray {
  background: rgba(100, 116, 139, 0.1);
  color: #94a3b8;
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
