import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

// PivotStack v6 路由（plan §3.3 / §10.2）。
// 所有 admin 路径都用 AdminLayout shell；v6 view 已全部建好，路径直接指实文件。
// 旧 v5 view 暂保留在 /legacy/* 路径作为 fallback，Stage 13 集成验证后再清。

const AdminLayout = () => import('../layouts/AdminLayoutRail.vue')
const UserLayout = () => import('../views/user/UserLayout.vue')

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },

  // ====== Admin shell ======
  {
    path: '/',
    component: AdminLayout,
    meta: { auth: true },
    children: [
      { path: '', redirect: '/overview' },

      // === Dashboard ===
      { path: 'overview', name: 'Overview', component: () => import('../views/overview/Overview.vue') },

      // === Channels ===
      { path: 'channels', name: 'ChannelsGroups', component: () => import('../views/channels/Groups.vue') },
      { path: 'channels/groups/:id', name: 'ChannelsGroupDetail', component: () => import('../views/channels/GroupDetail.vue'), props: true },
      { path: 'channels/newapi',     name: 'ChannelsNewAPI',       component: () => import('../views/channels/newapi/ProviderList.vue') },
      { path: 'channels/newapi/:id', name: 'ChannelsNewAPIDetail', component: () => import('../views/channels/newapi/ProviderDetail.vue'), props: true },
      { path: 'channels/direct',     name: 'ChannelsDirect',       component: () => import('../views/channels/direct/ChannelList.vue') },
      { path: 'channels/direct/:id', name: 'ChannelsDirectDetail', component: () => import('../views/channels/direct/ChannelDetail.vue'), props: true },
      { path: 'channels/reconcile',  name: 'ChannelsReconcile',    component: () => import('../views/channels/Reconcile.vue') },

      // === Sales & Billing ===
      { path: 'billing/keys',      name: 'BillingKeys',      component: () => import('../views/billing/Keys.vue') },
      { path: 'billing/keys/:id',  name: 'BillingKeyDetail', component: () => import('../views/billing/KeyDetail.vue'), props: true },
      { path: 'billing/recharges', name: 'BillingRecharges', component: () => import('../views/billing/Recharges.vue') },
      { path: 'billing/codes',     name: 'BillingCodes',     component: () => import('../views/billing/Codes.vue') },
      // v9：旧定价中心 → 经营看板（计费/利润分析统一入口）
      { path: 'billing/pricing', redirect: '/ops/business-board' },
      { path: 'billing/unit',      name: 'BillingUnit',      component: () => import('../views/billing/Unit.vue') },

      // === Ops ===
      { path: 'ops/call-logs', name: 'OpsCallLogs', component: () => import('../views/ops/CallLogs.vue') },
      { path: 'ops/abuse',     name: 'OpsAbuse',    component: () => import('../views/AbuseMonitor.vue') },
      { path: 'ops/business-board', name: 'OpsBusinessBoard', component: () => import('../views/ops/BusinessBoard.vue') },
      { path: 'ops/profit', redirect: '/ops/business-board' },
      { path: 'ops/api-docs',  redirect: '/user/api-docs' }, // 兼容旧 URL → 改到 user 端

      // === System ===
      { path: 'system/users',         name: 'SystemUsers',         component: () => import('../views/system/Users.vue') },
      { path: 'system/auth',          name: 'SystemAuth',          component: () => import('../views/system/Auth.vue') },
      { path: 'system/notifications', name: 'SystemNotifications', component: () => import('../views/system/Notifications.vue') },
      { path: 'system/settings',      name: 'SystemSettings',      component: () => import('../views/system/Settings.vue') },

      // === Reseller (admin 视角) ===
      { path: 'reseller', name: 'AdminReseller', component: () => import('../views/admin/AdminReseller.vue') },

      // === Legacy v5 view 入口（Stage 13 已清理，仅留 Accounts 管理上游 kiro 账号池）===
      { path: 'legacy/accounts',        component: () => import('../views/Accounts.vue') },

      // === Stage 12：v5 旧 URL → v6 URL frontend redirect ===
      // 这些只是历史 bookmark 收藏的兜底，admin 仍能输入 /apikeys 自动跳到 /billing/keys。
      { path: 'apikeys',         redirect: '/billing/keys' },
      { path: 'providers',       redirect: '/channels/newapi' },
      { path: 'newapi-channels', redirect: '/channels/newapi' },
      { path: 'reconcile',       redirect: '/channels/reconcile' },
      { path: 'codes',           redirect: '/billing/codes' },
      { path: 'pricing',         redirect: '/ops/business-board' },
      { path: 'system-unit',     redirect: '/billing/unit' },
      { path: 'logs',            redirect: '/ops/call-logs' },
      { path: 'api',             redirect: '/ops/api-docs' },
      { path: 'settings',        redirect: '/system/settings' },
      { path: 'stealth',         redirect: '/system/settings' },
      { path: 'series',          redirect: '/channels/newapi' },
      { path: 'insights',        redirect: '/overview' },
      { path: 'leaderboard',     redirect: '/overview' },
      { path: 'dashboard',       redirect: '/overview' },
      { path: 'accounts',        redirect: '/channels/newapi' },
    ],
  },

  // ====== User portal ======
  {
    path: '/user',
    component: UserLayout,
    meta: { userAuth: true },
    children: [
      { path: '', redirect: '/user/dashboard' },
      { path: 'dashboard', name: 'UserDashboard', component: () => import('../views/user/UserDashboard.vue') },
      { path: 'recharge',  name: 'UserRecharge',  component: () => import('../views/user/UserRecharge.vue'), meta: { blockChildKey: true } },
      { path: 'logs',      name: 'UserLogs',      component: () => import('../views/user/UserLogs.vue') },
      { path: 'api-docs',  name: 'UserApiDocs',   component: () => import('../views/user/UserApiDocs.vue') },
      { path: 'keys',      name: 'UserKeys',      component: () => import('../views/user/UserKeys.vue') },
      {
        path: 'reseller',
        component: () => import('../views/reseller/ResellerLayout.vue'),
        meta: { requireReseller: true },
        children: [
          { path: '',         redirect: '/user/reseller/summary' },
          { path: 'summary',  name: 'ResellerSummary', component: () => import('../views/reseller/ResellerSummary.vue') },
          { path: 'keys',     name: 'ResellerKeys',    component: () => import('../views/reseller/ResellerKeys.vue') },
        ],
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    return savedPosition || { top: 0 }
  },
})

router.beforeEach(async (to) => {
  if (to.matched.some(r => r.meta.auth)) {
    const auth = useAuthStore()
    const ok = await auth.ensureSession()
    if (!ok) return '/login'
  }
  if (to.meta.userAuth || to.matched.some(r => r.meta.userAuth)) {
    const apiKey = localStorage.getItem('user_api_key') || sessionStorage.getItem('user_api_key')
    if (!apiKey) return '/login'
  }
  if (to.matched.some(r => r.meta.requireReseller)) {
    const { useUserAuth } = await import('../stores/userAuth')
    const userAuth = useUserAuth()
    if (!userAuth.userInfo) await userAuth.refresh()
    if (!userAuth.userInfo?.isReseller) return '/user/dashboard'
  }
  if (to.matched.some(r => r.meta.blockChildKey)) {
    const { useUserAuth } = await import('../stores/userAuth')
    const userAuth = useUserAuth()
    if (!userAuth.userInfo) await userAuth.refresh()
    if (userAuth.userInfo?.isChildKey) return '/user/dashboard'
  }
})

export default router
