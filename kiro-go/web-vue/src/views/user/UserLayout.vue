<script setup>
import { useRouter, useRoute } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'
import { computed, onMounted } from 'vue'

const router = useRouter()
const route = useRoute()
const auth = useUserAuth()

const navItems = [
  { path: '/user/dashboard', label: '概览', icon: '📊' },
  { path: '/user/recharge', label: '充值', icon: '💰' },
  { path: '/user/logs', label: '日志', icon: '📋' },
]

const balanceDisplay = computed(() => {
  if (auth.plan === 'timed') return '时间制'
  return `¥${auth.balance.toFixed(2)}`
})

const planLabel = computed(() => {
  const labels = { timed: '时间制', credit: '计量制', hybrid: '混合制' }
  return labels[auth.plan] || auth.plan
})

function handleLogout() {
  auth.logout()
  router.replace('/user/login')
}

onMounted(() => {
  auth.refresh()
})
</script>

<template>
  <div class="user-layout">
    <header class="user-header">
      <div class="header-left">
        <span class="brand">🪙 KiroStack</span>
        <nav class="header-nav">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            :class="{ active: route.path === item.path }"
          >
            <span class="nav-icon">{{ item.icon }}</span>
            <span class="nav-label">{{ item.label }}</span>
          </router-link>
        </nav>
      </div>
      <div class="header-right">
        <div class="balance-badge" :class="{ low: auth.balance < 1 && auth.plan !== 'timed' }">
          {{ balanceDisplay }}
        </div>
        <div class="plan-tag">{{ planLabel }}</div>
        <button class="logout-btn" @click="handleLogout" title="退出登录">
          ↩
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
        :class="{ active: route.path === item.path }"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span class="nav-label">{{ item.label }}</span>
      </router-link>
    </nav>
  </div>
</template>

<style scoped>
.user-layout {
  min-height: 100vh;
  background: linear-gradient(135deg, #0f0c29 0%, #1a1a2e 50%, #16213e 100%);
  color: #e0e0e0;
}

.user-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 1.5rem;
  height: 60px;
  background: rgba(0,0,0,0.3);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(255,255,255,0.06);
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
  font-size: 1.1rem;
  font-weight: 700;
  background: linear-gradient(135deg, #f5af19, #f12711);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  white-space: nowrap;
}

.header-nav {
  display: flex;
  gap: 0.3rem;
}

.header-nav a {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.8rem;
  border-radius: 8px;
  text-decoration: none;
  color: rgba(255,255,255,0.5);
  font-size: 0.85rem;
  transition: all 0.2s;
}

.header-nav a:hover {
  color: #fff;
  background: rgba(255,255,255,0.08);
}

.header-nav a.active {
  color: #f5af19;
  background: rgba(245,175,25,0.1);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 0.8rem;
}

.balance-badge {
  padding: 0.3rem 0.8rem;
  border-radius: 20px;
  background: rgba(34,197,94,0.15);
  color: #22c55e;
  font-weight: 600;
  font-size: 0.85rem;
}

.balance-badge.low {
  background: rgba(255,107,107,0.15);
  color: #ff6b6b;
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%,100% { opacity: 1; }
  50% { opacity: 0.6; }
}

.plan-tag {
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  background: rgba(139,92,246,0.15);
  color: #a78bfa;
  font-size: 0.75rem;
}

.logout-btn {
  background: none;
  border: 1px solid rgba(255,255,255,0.1);
  color: rgba(255,255,255,0.5);
  padding: 0.3rem 0.6rem;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: all 0.2s;
}

.logout-btn:hover {
  color: #ff6b6b;
  border-color: rgba(255,107,107,0.3);
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
  background: rgba(0,0,0,0.8);
  backdrop-filter: blur(12px);
  border-top: 1px solid rgba(255,255,255,0.06);
  padding: 0.5rem;
  justify-content: space-around;
  z-index: 100;
}

.mobile-nav a {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  text-decoration: none;
  color: rgba(255,255,255,0.4);
  font-size: 0.7rem;
  padding: 0.3rem 0.8rem;
  border-radius: 8px;
}

.mobile-nav a.active {
  color: #f5af19;
}

.mobile-nav .nav-icon {
  font-size: 1.2rem;
}

@media (max-width: 768px) {
  .header-nav { display: none; }
  .mobile-nav { display: flex; }
  .user-main { padding: 1rem; padding-bottom: 5rem; }
}
</style>
