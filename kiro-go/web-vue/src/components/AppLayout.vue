<script setup>
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useTheme } from '../composables/useTheme'
import { 
  LayoutDashboard, 
  Key, 
  FileText, 
  Users, 
  Settings, 
  LogOut, 
  Sun, 
  Moon, 
  Laptop,
  Search,
  Bell,
  Menu,
  X,
  ChevronRight,
  Shield
} from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const { theme, setTheme } = useTheme()
const isSidebarOpen = ref(true)

const navItems = [
  { name: '数据面板', path: '/', icon: LayoutDashboard },
  { name: 'API 令牌', path: '/api', icon: Key },
  { name: '使用日志', path: '/logs', icon: FileText },
  { name: '账号管理', path: '/accounts', icon: Users },
  { name: '个人设置', path: '/settings', icon: Settings },
]

const pageTitle = computed(() => {
  return navItems.find(item => item.path === route.path)?.name || '控制台'
})

function cycleTheme() {
  const next = { light: 'dark', dark: 'system', system: 'light' }
  setTheme(next[theme.value] || 'light')
}

function handleLogout() {
  auth.logout()
  router.push('/login')
}

const ThemeIcon = computed(() => {
  if (theme.value === 'light') return Sun
  if (theme.value === 'dark') return Moon
  return Laptop
})
</script>

<template>
  <div class="flex h-screen bg-[var(--bg)] text-[var(--text)] overflow-hidden font-sans">
    <!-- Sidebar -->
    <aside 
      class="fixed inset-y-0 left-0 z-50 w-64 bg-[var(--sidebar-bg)] border-r border-[var(--border)] transition-transform duration-300 ease-in-out lg:translate-x-0 lg:static lg:inset-0 flex flex-col"
      :class="isSidebarOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <!-- Logo Area -->
      <div class="h-16 flex items-center px-6 border-b border-[var(--border)] shrink-0">
        <div class="w-8 h-8 rounded-lg bg-primary flex items-center justify-center mr-3 shadow-lg shadow-primary/20">
          <Shield class="w-4 h-4 text-white" />
        </div>
        <span class="font-black text-xl tracking-tighter">Kiro-Stack</span>
      </div>

      <!-- Search Bar -->
      <div class="px-4 py-4 shrink-0">
        <div class="relative group">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-secondary)] group-focus-within:text-primary transition-colors" />
          <input
            type="text"
            placeholder="搜索... (Ctrl+K)"
            readonly
            class="w-full h-10 pl-9 pr-4 bg-[var(--card)] border border-[var(--border)] rounded-xl text-xs cursor-pointer hover:border-primary/50 transition-all outline-none"
          />
        </div>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 px-3 space-y-1 overflow-y-auto custom-scrollbar">
        <div class="px-3 mb-2 text-[10px] font-black uppercase tracking-widest text-[var(--text-secondary)] opacity-50">主菜单</div>
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="flex items-center px-4 py-3 rounded-xl text-sm font-bold transition-all group mb-1 text-[var(--text-secondary)] hover:bg-primary/5 hover:text-primary"
          active-class="!bg-primary !text-white shadow-xl shadow-primary/20"
        >
          <component :is="item.icon" class="w-4 h-4 mr-3 group-hover:scale-110 transition-transform" />
          {{ item.name }}
          <ChevronRight class="w-3.5 h-3.5 ml-auto opacity-0 group-[.router-link-active]:opacity-50" />
        </router-link>
      </nav>

      <!-- User Section -->
      <div class="p-4 border-t border-[var(--border)] bg-[var(--card)]/50">
        <div class="flex items-center p-3 rounded-2xl bg-[var(--card)] border border-[var(--border)] shadow-sm">
          <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-primary to-indigo-600 flex items-center justify-center text-white font-black text-sm mr-3 shadow-lg shadow-primary/10">AD</div>
          <div class="flex-1 min-w-0">
            <div class="text-xs font-black truncate">系统管理员</div>
            <div class="text-[10px] text-[var(--text-secondary)] truncate font-medium">admin@kiro.io</div>
          </div>
          <button @click="handleLogout" class="p-2 hover:bg-rose-500/10 hover:text-rose-500 rounded-lg transition-all" title="退出">
            <LogOut class="w-4 h-4" />
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 flex flex-col min-w-0 relative bg-[var(--bg)]">
      <!-- Mobile Header -->
      <header class="lg:hidden h-16 flex items-center justify-between px-4 bg-[var(--card)] border-b border-[var(--border)] shrink-0 z-40">
        <button @click="isSidebarOpen = !isSidebarOpen" class="p-2">
          <Menu v-if="!isSidebarOpen" class="w-6 h-6" />
          <X v-else class="w-6 h-6" />
        </button>
        <span class="font-black text-lg">Kiro-Stack</span>
        <div class="w-10"></div>
      </header>

      <!-- Glass Header -->
      <header class="hidden lg:flex h-16 items-center justify-between px-8 bg-[var(--card)]/60 backdrop-blur-xl border-b border-[var(--border)] sticky top-0 z-30 shrink-0">
        <div class="flex items-center gap-3 text-xs font-bold">
          <span class="text-[var(--text-secondary)] opacity-50">控制台</span>
          <ChevronRight class="w-3 h-3 text-[var(--text-secondary)] opacity-30" />
          <span class="text-primary tracking-wide">{{ pageTitle }}</span>
        </div>

        <div class="flex items-center gap-2">
          <button class="p-2.5 text-[var(--text-secondary)] hover:bg-primary/5 hover:text-primary rounded-xl transition-all relative">
            <Bell class="w-5 h-5" />
            <span class="absolute top-2.5 right-2.5 w-2 h-2 bg-rose-500 rounded-full border-2 border-[var(--card)] shadow-[0_0_8px_rgba(244,63,94,0.5)]"></span>
          </button>
          <button @click="cycleTheme" class="p-2.5 text-[var(--text-secondary)] hover:bg-primary/5 hover:text-primary rounded-xl transition-all">
            <component :is="ThemeIcon" class="w-5 h-5" />
          </button>
          <div class="w-px h-4 bg-[var(--border)] mx-2 opacity-50"></div>
          <div class="flex items-center gap-3 px-3 py-1.5 rounded-xl hover:bg-primary/5 transition-all cursor-pointer group">
            <div class="text-right">
              <div class="text-[11px] font-black leading-none mb-1 group-hover:text-primary transition-colors">系统管理员</div>
              <div class="text-[10px] text-emerald-500 flex items-center justify-end gap-1.5 font-bold">
                <span class="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.5)]"></span>
                在线
              </div>
            </div>
          </div>
        </div>
      </header>

      <!-- Scrollable Content -->
      <div class="flex-1 overflow-y-auto custom-scrollbar">
        <div class="max-w-[1600px] mx-auto p-4 lg:p-8">
          <slot />
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.bg-primary {
  background-color: var(--primary);
}
.text-primary {
  color: var(--primary);
}

/* 导航活动状态微光 */
.router-link-active {
  position: relative;
}
</style>

<style scoped>
.bg-primary {
  background-color: var(--primary);
}
.text-primary {
  color: var(--primary);
}

/* 导航活动状态微光 */
.router-link-active {
  position: relative;
}
.router-link-active::before {
  content: '';
  position: absolute;
  left: -3px;
  top: 20%;
  bottom: 20%;
  width: 3px;
  background-color: white;
  border-radius: 0 4px 4px 0;
  opacity: 0.5;
}
</style>
