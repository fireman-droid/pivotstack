<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'
import { useSystemUnit } from '../../composables/useSystemUnit'
import { useNotificationStore } from '../../stores/notifications'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { LayoutDashboard, Wallet, ScrollText, Code2, Users, LogOut, DollarSign, Key } from 'lucide-vue-next'
import Starfield from '../../components/user/stellar/Starfield.vue'
import LiveIndicator from '../../components/user/stellar/LiveIndicator.vue'
import NotifBell from '../../components/notif/NotifBell.vue'
import NotifModal from '../../components/notif/NotifModal.vue'
import NotifCritBanner from '../../components/notif/NotifCritBanner.vue'
import UpgradeAccountModal from '../../components/user/UpgradeAccountModal.vue'
import type { UserNotification } from '../../api/notifications'

const UPGRADE_SKIP_KEY = 'pivotstack_upgrade_skip_count'
import logoUrl from '../../assets/pivotstack-logo.png'

const router = useRouter()
const route = useRoute()
const auth = useUserAuth() as any
const { toCny } = useSystemUnit()

const navItems = computed(() => {
  const items: { path: string; label: string; icon: any }[] = [
    { path: '/user/dashboard', label: '概览', icon: LayoutDashboard },
  ]
  if (!auth.userInfo?.isChildKey) items.push({ path: '/user/recharge', label: '充值', icon: Wallet })
  items.push({ path: '/user/logs', label: '调用日志', icon: ScrollText })
  items.push({ path: '/user/api-docs', label: '接入示例', icon: Code2 })
  items.push({ path: '/user/keys', label: 'API Key', icon: Key })
  if (auth.userInfo?.isReseller) items.push({ path: '/user/reseller', label: '代理', icon: Users })
  return items
})

const balance = computed(() => Number(auth.balance || 0))
const giftBalance = computed(() => Number(auth.userInfo?.giftBalance || 0))
const totalBalance = computed(() => balance.value + giftBalance.value)
const isCreditPlan = computed(() => auth.plan === 'credit' || auth.plan === 'hybrid')
const balanceLow = computed(() => isCreditPlan.value && totalBalance.value < 1)
const balanceCny = computed(() => toCny(totalBalance.value).toFixed(2))

const balanceTitle = computed(() => {
  if (!isCreditPlan.value) return ''
  return `充值 $${balance.value.toFixed(2)} · 赠送 $${giftBalance.value.toFixed(2)} · ≈¥${balanceCny.value}`
})

function isActive(path: string) {
  return route.path === path || route.path.startsWith(path + '/')
}

function handleLogout() {
  auth.logout()
  router.replace('/login')
}

// Notification: 启动 polling，layout 卸载时关掉。
const notif = useNotificationStore()
const openNotif = ref<UserNotification | null>(null)
function onOpenNotif(n: UserNotification) { openNotif.value = n }
function openCritBannerNotif() { openNotif.value = notif.topCritical }
function closeNotif() { openNotif.value = null }
function goNotifCenter() { router.push('/user/notifications') }

// v6: 老 key 升级账号 Modal（软强制）
const upgradeShow = ref(false)
const upgradeSkipCount = ref(0)
function checkUpgradeNeeded() {
  if (!auth.apiKey) return
  if (auth.userInfo?.userId) return // 已绑定，不弹
  upgradeSkipCount.value = parseInt(localStorage.getItem(UPGRADE_SKIP_KEY) || '0', 10) || 0
  upgradeShow.value = true
}
function onUpgradeSkip() {
  upgradeSkipCount.value += 1
  localStorage.setItem(UPGRADE_SKIP_KEY, String(upgradeSkipCount.value))
  upgradeShow.value = false
}
async function onUpgradeSuccess() {
  localStorage.removeItem(UPGRADE_SKIP_KEY)
  upgradeShow.value = false
  await auth.refresh()
}

onMounted(async () => {
  await auth.refresh()
  notif.startPolling()
  checkUpgradeNeeded()
})
onUnmounted(() => notif.stopPolling())
</script>

<template>
  <div class="user-shell stellar-scope">
    <Starfield />
    <NotifCritBanner @open="openCritBannerNotif" />
    <NotifModal :notif="openNotif" @close="closeNotif" />
    <UpgradeAccountModal
      v-model:show="upgradeShow"
      :skip-count="upgradeSkipCount"
      @skip="onUpgradeSkip"
      @success="onUpgradeSuccess"
    />

    <header class="st-nav">
      <div class="st-nav__inner">
        <router-link to="/user/dashboard" class="st-nav__brand">
          <img :src="logoUrl" class="st-nav__logo" alt="PivotStack" />
          <span class="st-nav__brand-name">PivotStack</span>
          <span class="st-nav__brand-role">USER</span>
        </router-link>

        <nav class="st-nav__center" aria-label="用户导航">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            :class="['st-nav__item', { 'is-active': isActive(item.path) }]"
          >
            <component :is="item.icon" :size="14" />
            <span>{{ item.label }}</span>
          </router-link>
        </nav>

        <div class="st-nav__right">
          <LiveIndicator class="st-nav__live" />
          <NotifBell @open="onOpenNotif" @see-all="goNotifCenter" />
          <div
            v-if="isCreditPlan"
            class="balance-chip"
            :class="{ 'is-low': balanceLow }"
            :title="balanceTitle"
          >
            <DollarSign :size="12" />
            <span class="t-num-strong">{{ totalBalance.toFixed(2) }}</span>
          </div>
          <button class="btn btn--ghost btn--icon" title="退出" @click="handleLogout">
            <LogOut :size="14" />
          </button>
        </div>
      </div>
    </header>

    <main class="st-main" id="main-content">
      <router-view v-slot="{ Component }">
        <component :is="Component" />
      </router-view>
    </main>

    <nav class="st-mobile-tabbar" aria-label="移动端导航">
      <router-link
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        :class="['st-mobile-tabbar__item', { 'is-active': isActive(item.path) }]"
      >
        <component :is="item.icon" :size="18" />
        <span>{{ item.label }}</span>
      </router-link>
    </nav>
  </div>
</template>

<style>
.user-shell {
  min-height: 100vh;
  color: var(--st-text-pri);
  font-family: var(--st-font-sans);
  position: relative;
}
.st-nav {
  position: sticky; top: 0; z-index: 100;
  height: 56px;
  background: rgba(0, 0, 0, 0.85);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-bottom: 1px solid #1a1a1a;
}
.st-nav__inner {
  height: 100%;
  max-width: 1440px;
  margin: 0 auto;
  padding: 0 32px;
  display: flex;
  align-items: center;
  gap: 24px;
}
.st-nav__brand {
  display: flex; align-items: center; gap: 12px;
  text-decoration: none;
  width: 240px;
  flex-shrink: 0;
}
.st-nav__logo {
  width: 24px; height: 24px;
  border-radius: 5px;
  object-fit: cover;
  background: #fff;
}
.st-nav__brand-name {
  font-size: 14px; font-weight: 600; letter-spacing: -0.01em;
  color: var(--st-text-pri);
}
.st-nav__brand-role {
  font-size: 10px; font-weight: 500;
  letter-spacing: 0.12em; text-transform: uppercase;
  color: var(--st-text-ter);
  border: 1px solid var(--st-border);
  padding: 2px 6px;
  border-radius: 2px;
}

.st-nav__center {
  flex: 1; display: flex; align-items: center; justify-content: center; gap: 4px;
}
.st-nav__item {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 12px; height: 32px;
  font-size: 13px; font-weight: 400;
  color: var(--st-text-sec);
  border-radius: 4px;
  text-decoration: none;
  position: relative;
  transition: color 150ms ease, background 150ms ease;
}
.st-nav__item:hover { color: var(--st-text-pri); background: rgba(255,255,255,0.04); }
.st-nav__item.is-active { color: var(--st-text-pri); }
.st-nav__item::after {
  content: ""; position: absolute; left: 50%; bottom: -10px;
  height: 2px; width: 0; background: var(--st-success);
  transform: translateX(-50%);
  transition: width 200ms ease;
}
.st-nav__item.is-active::after { width: 24px; }

.st-nav__right { display: flex; align-items: center; gap: 12px; flex-shrink: 0; }

.balance-chip {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 6px 12px;
  background: rgba(255,255,255,0.06);
  border-radius: 4px;
  color: var(--st-text-pri);
  font-family: var(--st-font-mono); font-size: 13px; font-weight: 500;
  font-variant-numeric: tabular-nums;
  cursor: help;
}
.balance-chip svg { opacity: 0.8; }
.balance-chip.is-low { color: var(--st-warning); }

.st-main {
  max-width: 1440px;
  margin: 0 auto;
  padding: 32px 32px 64px;
  position: relative;
  z-index: 1;
}

.st-mobile-tabbar { display: none; }

@media (max-width: 1024px) {
  .st-nav__inner { padding: 0 20px; }
  .st-nav__brand { width: auto; }
  .st-nav__brand-role { display: none; }
  .st-nav__center { gap: 0; }
  .st-nav__item span { display: none; }
  .st-nav__item { padding: 6px 10px; }
  .st-main { padding: 24px 20px 48px; }
}
@media (max-width: 768px) {
  .st-nav__inner { padding: 0 16px; }
  .st-nav__brand-name { display: none; }
  .st-nav__live { display: none; }
  .st-main { padding: 20px 16px 88px; }
  .st-nav__center { display: none; }
  .st-mobile-tabbar {
    display: flex;
    position: fixed;
    bottom: 0; left: 0; right: 0;
    z-index: 50;
    background: rgba(0,0,0,0.92);
    backdrop-filter: blur(8px);
    border-top: 1px solid var(--st-border);
    padding: 6px 0;
  }
  .st-mobile-tabbar__item {
    flex: 1;
    display: flex; flex-direction: column; align-items: center; gap: 2px;
    padding: 6px 4px;
    color: var(--st-text-ter);
    text-decoration: none;
    font-size: 10px;
  }
  .st-mobile-tabbar__item.is-active { color: var(--st-text-pri); }
  .st-mobile-tabbar__item.is-active svg { color: var(--st-success); }
}
</style>
