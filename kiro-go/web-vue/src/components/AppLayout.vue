<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import WorldSwitcher from './WorldSwitcher.vue'
import {
  LayoutDashboard, KeyRound, FileText, Users, Settings,
  LogOut, Menu, X, ChevronRight, Shield, Gift,
  Key, DollarSign, Beaker, Trophy, BarChart3
} from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const isSidebarOpen = ref(false)

watch(() => route.path, () => { isSidebarOpen.value = false })

// 主菜单
const navMain = [
  { name: '数据面板',     path: '/',         icon: LayoutDashboard },
  { name: 'Key 管理',     path: '/apikeys',  icon: KeyRound },
  { name: 'API 接入说明', path: '/api',      icon: Key },
  { name: '使用日志',     path: '/logs',     icon: FileText },
  { name: '定价中心',     path: '/pricing',  icon: DollarSign },
  { name: '运营洞察',     path: '/insights', icon: BarChart3 },
  { name: '激活码',       path: '/codes',    icon: Gift },
  { name: '账号管理',     path: '/accounts', icon: Users },
  { name: '排行榜',       path: '/leaderboard', icon: Trophy },
  { name: '系统设置',     path: '/settings', icon: Settings },
]

// 实验性（管理员自用，不放主菜单视觉重点位）
const navExperimental = [
  { name: '请求分流',     path: '/stealth',  icon: Beaker },
]

const allNav = computed(() => [...navMain, ...navExperimental])

const pageTitle = computed(() =>
  allNav.value.find(item => item.path === route.path)?.name || '控制台'
)

function handleLogout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <div class="admin-shell">
    <!-- Mobile overlay backdrop -->
    <div v-if="isSidebarOpen" class="mobile-backdrop" @click="isSidebarOpen = false"></div>

    <!-- Sidebar -->
    <aside class="sidebar" :class="{ 'is-open': isSidebarOpen }">
      <!-- Brand -->
      <div class="sidebar-brand">
        <div class="brand-mark">
          <Shield :size="14" stroke-width="2.6" />
        </div>
        <div class="brand-text-wrap">
          <span class="brand-name">Pivot<span class="brand-accent">Stack</span></span>
          <div class="brand-eyebrow">ADMIN PANEL</div>
        </div>
      </div>

      <!-- World switcher -->
      <div class="sidebar-switcher">
        <WorldSwitcher />
      </div>

      <!-- Nav -->
      <nav class="sidebar-nav" aria-label="后台导航">
        <div class="nav-eyebrow">主菜单</div>
        <router-link
          v-for="item in navMain"
          :key="item.path"
          :to="item.path"
          class="nav-item"
        >
          <component :is="item.icon" :size="14" stroke-width="2.2" />
          <span>{{ item.name }}</span>
          <ChevronRight :size="13" class="nav-chevron" />
        </router-link>

        <div class="nav-eyebrow nav-eyebrow-mt">实验性</div>
        <router-link
          v-for="item in navExperimental"
          :key="item.path"
          :to="item.path"
          class="nav-item"
        >
          <component :is="item.icon" :size="14" stroke-width="2.2" />
          <span>{{ item.name }}</span>
          <ChevronRight :size="13" class="nav-chevron" />
        </router-link>
      </nav>

      <!-- User card -->
      <div class="sidebar-user">
        <div class="user-card">
          <div class="user-avatar">AD</div>
          <div class="user-info">
            <div class="user-name">系统管理员</div>
            <div class="user-status">
              <span class="status-dot" />
              在线
            </div>
          </div>
          <button @click="handleLogout" class="logout-btn" title="退出登录" aria-label="退出">
            <LogOut :size="14" />
          </button>
        </div>
      </div>
    </aside>

    <!-- Main -->
    <main class="admin-main">
      <!-- Mobile header -->
      <header class="mobile-header">
        <button @click="isSidebarOpen = !isSidebarOpen" class="hamburger" :aria-label="isSidebarOpen ? '关闭菜单' : '打开菜单'">
          <Menu v-if="!isSidebarOpen" :size="18" />
          <X v-else :size="18" />
        </button>
        <span class="mobile-brand">Pivot<span class="brand-accent">Stack</span></span>
        <div class="hamburger-spacer" />
      </header>

      <!-- Desktop crumbs -->
      <header class="desktop-header">
        <div class="crumbs">
          <span class="crumb-mute">控制台</span>
          <ChevronRight :size="13" />
          <span class="crumb-active">{{ pageTitle }}</span>
        </div>
        <div class="header-status">
          <span class="status-dot" />
          <span>在线</span>
        </div>
      </header>

      <!-- Content -->
      <div class="admin-content">
        <div class="content-wrap">
          <slot />
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.admin-shell {
  display: flex;
  height: 100vh;
  overflow: hidden;
  color: var(--world-text-primary);
  background: var(--world-bg-main);
  font-family: var(--world-font-sans);
  position: relative;
}

/* === Mobile backdrop === */
.mobile-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
  z-index: 40;
  display: none;
}

/* === Sidebar === */
.sidebar {
  position: fixed;
  inset-block: 0;
  left: 0;
  width: 244px;
  height: 100vh;
  z-index: 50;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  background: var(--world-glass-bg-strong);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  border-right: 1px solid var(--world-glass-border);
  transform: translateX(-100%);
  transition: transform 320ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1));
}
[data-world="daogui"] .sidebar { border-right-color: rgba(184, 134, 11, 0.20); }
.sidebar.is-open { transform: translateX(0); }
@media (min-width: 1024px) {
  .sidebar { position: relative; transform: translateX(0); height: 100vh; }
}

/* Sidebar brand */
.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 64px;
  padding: 0 18px;
  border-bottom: 1px solid var(--world-divider);
  flex-shrink: 0;
}
.brand-mark {
  width: 32px; height: 32px;
  border-radius: var(--world-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  color: white;
}
[data-world="daogui"] .brand-mark { box-shadow: 0 0 14px rgba(196, 30, 58, 0.4); }
.brand-text-wrap { display: flex; flex-direction: column; gap: 2px; }
.brand-name {
  font-size: 1rem;
  font-weight: 800;
  letter-spacing: -0.01em;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
.brand-accent {
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.brand-eyebrow {
  font-size: 0.55rem;
  font-weight: 800;
  letter-spacing: 0.2em;
  color: var(--world-text-dim);
}

.sidebar-switcher {
  display: flex;
  justify-content: center;
  padding: 14px 18px;
  border-bottom: 1px solid var(--world-divider);
  flex-shrink: 0;
}

/* Nav */
.sidebar-nav {
  flex: 1;
  padding: 14px 12px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.nav-eyebrow {
  padding: 4px 12px;
  margin-bottom: 6px;
  font-size: 0.6rem;
  font-weight: 800;
  letter-spacing: 0.24em;
  color: var(--world-text-dim);
  text-transform: uppercase;
}
.nav-eyebrow-mt { margin-top: 14px; }

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border-radius: var(--world-radius-md);
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--world-text-mute);
  text-decoration: none;
  transition: all 200ms var(--world-transition-fast, cubic-bezier(0.4, 0, 0.2, 1));
  position: relative;
}
.nav-item .nav-chevron {
  margin-left: auto;
  opacity: 0;
  transform: translateX(-4px);
  transition: all 200ms ease;
}
.nav-item:hover {
  color: var(--world-text-primary);
  background: var(--world-overlay-light);
}
.nav-item:hover .nav-chevron {
  opacity: 0.5;
  transform: translateX(0);
}
.nav-item.router-link-active {
  color: white;
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
}
.nav-item.router-link-active .nav-chevron { opacity: 0.7; transform: translateX(0); }
[data-world="daogui"] .nav-item.router-link-active {
  box-shadow: 0 4px 14px -4px rgba(196, 30, 58, 0.5);
}

/* User card */
.sidebar-user {
  padding: 14px;
  border-top: 1px solid var(--world-divider);
  flex-shrink: 0;
}
.user-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: var(--world-radius-md);
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
}
.user-avatar {
  width: 32px; height: 32px;
  border-radius: var(--world-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--world-paper-aged, #b8860b), var(--world-accent-deep, #0c4a6e));
  color: white;
  font-weight: 800;
  font-size: 0.7rem;
}
.user-info { flex: 1; min-width: 0; }
.user-name {
  font-size: 0.75rem;
  font-weight: 800;
  color: var(--world-text-primary);
  margin-bottom: 1px;
}
.user-status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.65rem;
  color: var(--world-success);
  font-weight: 700;
}
.status-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--world-success);
  box-shadow: 0 0 6px var(--world-success);
  animation: pulse 1.8s ease-in-out infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}

.logout-btn {
  width: 28px; height: 28px;
  border-radius: var(--world-radius-sm);
  background: transparent;
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 200ms ease;
}
.logout-btn:hover {
  color: var(--world-error);
  background: rgba(239, 68, 68, 0.10);
  border-color: rgba(239, 68, 68, 0.35);
}

/* === Main === */
.admin-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  height: 100vh;
  overflow: hidden;
}

.mobile-header {
  display: none;
  align-items: center;
  justify-content: space-between;
  padding: 0 14px;
  height: 56px;
  background: var(--world-glass-bg-strong);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  border-bottom: 1px solid var(--world-divider);
  position: sticky;
  top: 0;
  z-index: 30;
}
.hamburger {
  width: 38px; height: 38px;
  border-radius: var(--world-radius-sm);
  background: transparent;
  border: none;
  color: var(--world-text-mute);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.hamburger:hover { color: var(--world-accent); background: var(--world-overlay-light); }
.hamburger-spacer { width: 38px; }
.mobile-brand {
  font-weight: 800;
  font-size: 1rem;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}

.desktop-header {
  display: none;
  align-items: center;
  justify-content: space-between;
  padding: 0 28px;
  height: 56px;
  background: var(--world-glass-bg);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  border-bottom: 1px solid var(--world-divider);
  position: sticky;
  top: 0;
  z-index: 20;
}
@media (min-width: 1024px) {
  .desktop-header { display: flex; }
  .mobile-header { display: none !important; }
  .mobile-backdrop { display: none !important; }
}
@media (max-width: 1023px) {
  .mobile-header { display: flex; }
  .mobile-backdrop { display: block; }
}

.crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.78rem;
  font-weight: 700;
}
.crumb-mute { color: var(--world-text-mute); }
.crumb-active { color: var(--world-accent); letter-spacing: 0.04em; }

.header-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--world-success);
}

/* Content */
.admin-content {
  flex: 1;
  overflow-y: auto;
  position: relative;
}
.content-wrap {
  max-width: 1600px;
  margin: 0 auto;
  padding: 24px 28px 64px;
}
@media (max-width: 768px) {
  .content-wrap { padding: 16px 16px 64px; }
}
</style>
