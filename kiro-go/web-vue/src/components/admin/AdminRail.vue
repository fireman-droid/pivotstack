<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Bell, UserCircle2, LogOut } from 'lucide-vue-next'
import { adminRail, resolveActiveRail } from '../../design/rail'
import { useAuthStore } from '../../stores/auth'
import logoUrl from '../../assets/pivotstack-logo.png'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const activeId = computed(() => resolveActiveRail(route.path))

function selectRail(id: string) {
  const rail = adminRail.find(r => r.id === id)
  if (!rail) return
  // 进入此 rail 时跳转到第一个 tree 节点（保证 tree 高亮 + 视图刷新）
  const first = rail.tree[0]
  if (first) router.push(first.to)
}

async function handleLogout() {
  try { await auth.logout() } catch {}
  router.push('/login')
}

function goNotifications() {
  router.push('/system/notifications')
}
</script>

<template>
  <aside class="rail">
    <div class="rail__brand" title="PivotStack Admin">
      <img :src="logoUrl" alt="PivotStack" />
    </div>

    <div class="rail__group">
      <button
        v-for="item in adminRail"
        :key="item.id"
        class="rail__item"
        :class="{ 'is-active': activeId === item.id }"
        @click="selectRail(item.id)"
        :aria-label="item.label"
        type="button"
      >
        <component :is="item.icon" :size="18" stroke-width="1.75" />
        <span class="rail-tip">{{ item.label }}</span>
      </button>
    </div>

    <div class="rail__bottom">
      <button
        class="rail__item"
        type="button"
        title="Notifications"
        aria-label="Notifications"
        @click="goNotifications"
        :class="{ 'is-active': route.path.startsWith('/system/notifications') }"
      >
        <Bell :size="18" stroke-width="1.75" />
        <span class="rail-tip">Notifications</span>
      </button>
      <button class="rail__item" type="button" title="Admin profile" aria-label="Admin">
        <UserCircle2 :size="18" stroke-width="1.75" />
        <span class="rail-tip">Admin</span>
      </button>
      <button class="rail__item rail__item--danger" type="button" @click="handleLogout" title="退出" aria-label="退出">
        <LogOut :size="18" stroke-width="1.75" />
        <span class="rail-tip">退出</span>
      </button>
    </div>
  </aside>
</template>

<style scoped>
.rail {
  width: 64px;
  min-width: 64px;
  height: 100vh;
  background: #050505;
  border-right: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  flex-shrink: 0;
  z-index: 10;
}

.rail__brand {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #000;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 24px;
  flex-shrink: 0;
  overflow: hidden;
}
.rail__brand img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.rail__group { display: flex; flex-direction: column; gap: 4px; }
.rail__bottom {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-top: 12px;
}

.rail__item {
  position: relative;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  border-radius: 6px;
  color: #707070;
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease;
  flex-shrink: 0;
}
.rail__item:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #ededed;
}
.rail__item.is-active {
  background: rgba(11, 212, 112, 0.10);
  color: #0bd470;
}
.rail__item.is-active::before {
  content: '';
  position: absolute;
  left: -12px;
  top: 8px;
  bottom: 8px;
  width: 2px;
  border-radius: 0 2px 2px 0;
  background: #0bd470;
}
.rail__item--danger:hover {
  color: #ff7a7a;
}

.rail__badge {
  position: absolute;
  top: 4px;
  right: 4px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  background: #ff4d4d;
  color: #fff;
  font-size: 10px;
  font-weight: 600;
  line-height: 16px;
  text-align: center;
  border-radius: 8px;
  border: 1px solid #050505;
}

/* Tooltip：hover 时出现，绝对定位在 rail 右侧 */
.rail-tip {
  position: absolute;
  left: calc(100% + 8px);
  top: 50%;
  transform: translateY(-50%);
  padding: 4px 8px;
  background: #0a0a0a;
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 4px;
  color: #ededed;
  font-size: 11px;
  letter-spacing: 0.04em;
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transition: opacity 120ms ease;
  z-index: 30;
}
.rail__item:hover .rail-tip { opacity: 1; }
</style>
