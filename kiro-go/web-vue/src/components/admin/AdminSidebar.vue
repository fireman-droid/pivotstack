<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { LogOut } from 'lucide-vue-next'
import { adminSidebar } from '../../design/sidebar'
import { useAuthStore } from '../../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

// 单段 path（去掉 query），用于匹配 active item。
const activeBase = computed(() => {
  const path = route.path
  let best = ''
  for (const item of adminSidebar) {
    const base = item.to.split('?')[0]
    if (path === base || path.startsWith(base + '/')) {
      if (base.length > best.length) best = base
    }
  }
  return best
})

async function handleLogout() {
  try {
    await auth.logout()
  } catch {
    /* ignore — 登出失败也跳登录 */
  }
  router.push('/login')
}
</script>

<template>
  <aside class="sidebar">
    <!-- Brand -->
    <RouterLink to="/overview" class="brand">
      <span class="brand__logo">
        <svg viewBox="0 0 24 24" width="20" height="20" fill="none" aria-hidden="true">
          <path d="M3 7l9-4 9 4-9 4-9-4z" fill="currentColor" opacity="0.9" />
          <path d="M3 12l9 4 9-4" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
          <path d="M3 17l9 4 9-4" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" opacity="0.5" />
        </svg>
      </span>
      <span class="brand__name">PivotStack</span>
      <span class="brand__badge">v6</span>
    </RouterLink>

    <!-- Nav -->
    <nav class="nav" aria-label="主菜单">
      <template v-for="(item, i) in adminSidebar" :key="item.to">
        <div v-if="item.divider && i > 0" class="nav__divider" aria-hidden="true" />
        <RouterLink
          :to="item.to"
          class="nav__item"
          :class="{ 'nav__item--active': activeBase === item.to.split('?')[0] }"
        >
          <component :is="item.icon" :size="16" class="nav__icon" aria-hidden="true" />
          <span class="nav__label">{{ item.label }}</span>
        </RouterLink>
      </template>
    </nav>

    <!-- Footer (logout) -->
    <div class="footer">
      <button type="button" class="footer__btn" @click="handleLogout">
        <LogOut :size="14" aria-hidden="true" />
        <span>退出登录</span>
      </button>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 232px;
  min-width: 232px;
  height: 100vh;
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  background: #0a0a0a;
  border-right: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 13px;
  overflow: hidden;
}

/* ===== Brand ===== */
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 18px;
  color: #ededed;
  text-decoration: none;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  height: 56px;
  flex-shrink: 0;
}
.brand__logo {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #4F46E5 0%, #06B6D4 100%);
  color: #ffffff;
}
.brand__name {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.brand__badge {
  margin-left: auto;
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #707070;
  border: 1px solid rgba(255, 255, 255, 0.10);
  padding: 1px 6px;
  border-radius: 3px;
}

/* ===== Nav ===== */
.nav {
  flex: 1;
  overflow-y: auto;
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.nav__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  height: 32px;
  border-radius: 5px;
  color: #a1a1a1;
  text-decoration: none;
  position: relative;
  transition: color 150ms cubic-bezier(0.4, 0, 0.2, 1),
              background-color 150ms cubic-bezier(0.4, 0, 0.2, 1);
}
.nav__item:hover {
  color: #ededed;
  background: rgba(255, 255, 255, 0.04);
}
.nav__item--active {
  color: #ededed;
  background: rgba(255, 255, 255, 0.06);
}
.nav__item--active::before {
  content: "";
  position: absolute;
  left: -8px;
  top: 8px;
  bottom: 8px;
  width: 2px;
  background: #ededed;
  border-radius: 0 2px 2px 0;
}
.nav__icon {
  color: currentColor;
  flex-shrink: 0;
  stroke-width: 1.75;
}
.nav__label {
  font-size: 13px;
  letter-spacing: -0.005em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav__divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.06);
  margin: 8px 4px;
}

/* ===== Footer ===== */
.footer {
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding: 10px 12px 14px;
  flex-shrink: 0;
}
.footer__btn {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 32px;
  padding: 0 10px;
  background: transparent;
  border: none;
  border-radius: 5px;
  color: #707070;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: color 150ms, background 150ms;
}
.footer__btn:hover {
  color: #ff4d4d;
  background: rgba(255, 77, 77, 0.06);
}
.footer__btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.20);
}

/* 滚动条 */
.nav::-webkit-scrollbar { width: 4px; }
.nav::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.08); border-radius: 2px; }
.nav::-webkit-scrollbar-thumb:hover { background: rgba(255, 255, 255, 0.16); }
</style>
