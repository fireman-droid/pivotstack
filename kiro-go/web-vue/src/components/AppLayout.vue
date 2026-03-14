<script setup>
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import WorldSwitcher from './WorldSwitcher.vue'
import {
  LayoutDashboard, Key, FileText, Users, Settings,
  LogOut, Menu, X, ChevronRight, Shield, TrendingUp, KeyRound
} from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const isSidebarOpen = ref(true)

const navItems = [
  { name: '数据面板', path: '/', icon: LayoutDashboard },
  { name: 'Key 管理', path: '/apikeys', icon: KeyRound },
  { name: 'API 令牌', path: '/api', icon: Key },
  { name: '使用日志', path: '/logs', icon: FileText },
  { name: '定价分析', path: '/pricing', icon: TrendingUp },
  { name: '账号管理', path: '/accounts', icon: Users },
  { name: '个人设置', path: '/settings', icon: Settings },
]

const pageTitle = computed(() => {
  return navItems.find(item => item.path === route.path)?.name || '控制台'
})

function handleLogout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <div class="flex h-screen text-[var(--text)] overflow-hidden font-sans relative z-10">
    <!-- Sidebar -->
    <aside 
      class="fixed inset-y-0 left-0 z-50 w-60 bg-[var(--card)]/95 backdrop-blur-xl border-r border-[var(--border)] transition-transform duration-300 ease-in-out lg:translate-x-0 lg:static lg:inset-0 flex flex-col"
      :class="isSidebarOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <!-- Logo -->
      <div class="h-16 flex items-center px-5 border-b border-[var(--border)] shrink-0">
        <div class="w-9 h-9 rounded-lg bg-gradient-to-br from-[var(--primary)] to-[var(--primary)] flex items-center justify-center mr-3 shadow-lg shadow-[var(--primary)]/20 relative">
          <Shield class="w-4 h-4 text-white" />
          <!-- 微光 -->
          <div class="absolute inset-0 rounded-lg bg-[var(--primary)] opacity-20 blur-sm animate-rune-pulse"></div>
        </div>
        <div>
          <span class="font-black text-lg tracking-tighter text-[var(--text)]">Kiro<span class="text-[var(--primary)]">Stack</span></span>
          <div class="text-[8px] font-bold tracking-[0.2em] text-[var(--text)]-secondary uppercase">ADMIN PANEL</div>
        </div>
      </div>

      <!-- World Switcher -->
      <div class="flex items-center justify-center py-3 border-b border-[var(--border)] shrink-0">
        <WorldSwitcher />
      </div>

      <!-- Navigation -->
      <nav class="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
        <div class="px-3 mb-3 text-[9px] font-black uppercase tracking-[0.3em] text-[var(--world-accent-alt)]/50">主 菜 单</div>
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="nav-item flex items-center px-4 py-3 rounded-xl text-sm font-bold transition-all group mb-1 text-[var(--text)]-secondary hover:bg-[var(--primary)]/5 hover:text-[var(--primary)]"
          active-class="!bg-[var(--primary)]/15 !text-[var(--primary)] !border-l-2 !border-[var(--primary)] shadow-lg shadow-[var(--primary)]/5"
        >
          <component :is="item.icon" class="w-4 h-4 mr-3 group-hover:scale-110 transition-transform" />
          {{ item.name }}
        </router-link>
      </nav>

      <!-- User Section -->
      <div class="p-3 border-t border-[var(--border)]">
        <div class="flex items-center p-3 rounded-xl bg-[var(--card)] border border-[var(--border)]">
          <div class="w-9 h-9 rounded-lg bg-gradient-to-br from-[var(--world-accent-alt)] to-[var(--world-accent-alt)] flex items-center justify-center text-white font-black text-xs mr-3 shadow-lg shadow-[var(--world-accent-alt)]/10">AD</div>
          <div class="flex-1 min-w-0">
            <div class="text-[11px] font-black truncate text-[var(--text)]">系统管理员</div>
            <div class="text-[9px] text-[var(--world-accent-alt)] flex items-center gap-1.5 font-bold">
              <span class="w-1.5 h-1.5 bg-[var(--world-accent-alt)] rounded-full animate-pulse shadow-md"></span>
              在线
            </div>
          </div>
          <button @click="handleLogout" class="p-2 hover:bg-[var(--primary)]/10 hover:text-[var(--primary)] rounded-lg transition-all text-[var(--text)]-secondary" title="退出登录">
            <LogOut class="w-4 h-4" />
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 flex flex-col min-w-0 relative">
      <!-- Mobile Header -->
      <header class="lg:hidden h-14 flex items-center justify-between px-4 bg-[var(--card)]/90 border-b border-[var(--border)] shrink-0 z-40 backdrop-blur-xl">
        <button @click="isSidebarOpen = !isSidebarOpen" class="p-2 text-[var(--text)]-secondary">
          <Menu v-if="!isSidebarOpen" class="w-5 h-5" />
          <X v-else class="w-5 h-5" />
        </button>
        <span class="font-black text-base text-[var(--text)]">Kiro<span class="text-[var(--primary)]">Stack</span></span>
        <div class="w-10"></div>
      </header>

      <!-- Glass Header -->
      <header class="hidden lg:flex h-14 items-center justify-between px-8 bg-[var(--card)]/50 backdrop-blur-xl border-b border-[var(--border)]/50 sticky top-0 z-30 shrink-0">
        <div class="flex items-center gap-3 text-xs font-bold">
          <span class="text-[var(--text)]-secondary">控制台</span>
          <ChevronRight class="w-3 h-3 text-[var(--text)]-secondary" />
          <span class="text-[var(--primary)] tracking-wide">{{ pageTitle }}</span>
        </div>

        <div class="flex items-center gap-2">
          <div class="text-[10px] text-[var(--world-accent-alt)] flex items-center gap-1.5 font-bold">
            <span class="w-1.5 h-1.5 bg-[var(--world-accent-alt)] rounded-full animate-pulse shadow-md"></span>
            在线
          </div>
        </div>
      </header>

      <!-- Scrollable Content -->
      <div class="flex-1 overflow-y-auto">
        <div class="max-w-[1600px] mx-auto p-4 lg:p-8">
          <slot />
        </div>
      </div>
    </main>
  </div>
</template>
