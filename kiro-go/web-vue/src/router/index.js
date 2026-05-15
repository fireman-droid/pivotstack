import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },

  // Admin routes
  { path: '/',         name: 'Dashboard',      component: () => import('../views/Dashboard.vue'),       meta: { auth: true } },
  { path: '/channels', name: 'Channels',       component: () => import('../views/Channels.vue'),        meta: { auth: true } },
  { path: '/accounts', name: 'Accounts',       component: () => import('../views/Accounts.vue'),        meta: { auth: true } },
  { path: '/apikeys',  name: 'ApiKeys',        component: () => import('../views/ApiKeys.vue'),         meta: { auth: true } },
  { path: '/settings', name: 'Settings',       component: () => import('../views/Settings.vue'),        meta: { auth: true } },
  { path: '/api',      name: 'ApiInfo',        component: () => import('../views/ApiInfo.vue'),         meta: { auth: true } },
  { path: '/logs',     name: 'Logs',           component: () => import('../views/Logs.vue'),            meta: { auth: true } },
  { path: '/pricing',  name: 'Pricing',        component: () => import('../views/Pricing.vue'),         meta: { auth: true } },
  { path: '/codes',    name: 'CodeManagement', component: () => import('../views/CodeManagement.vue'),  meta: { auth: true } },
  { path: '/stealth',  name: 'StealthConfig',  component: () => import('../views/StealthConfig.vue'),   meta: { auth: true } },
  { path: '/leaderboard', name: 'Leaderboard', component: () => import('../views/Leaderboard.vue'),     meta: { auth: true } },
  { path: '/insights',    name: 'Insights',    component: () => import('../views/Insights.vue'),        meta: { auth: true } },

  // Legacy redirects (keep so old bookmarks/links don't 404)
  { path: '/pricing-config',   redirect: '/pricing' },
  { path: '/pricing-analysis', redirect: '/pricing' },
  { path: '/abuse',            redirect: '/settings' },

  // User portal
  {
    path: '/user',
    component: () => import('../views/user/UserLayout.vue'),
    meta: { userAuth: true },
    children: [
      { path: '',          redirect: '/user/dashboard' },
      { path: 'dashboard', name: 'UserDashboard', component: () => import('../views/user/UserDashboard.vue') },
      { path: 'recharge',  name: 'UserRecharge',  component: () => import('../views/user/UserRecharge.vue'), meta: { blockChildKey: true } },
      { path: 'logs',      name: 'UserLogs',      component: () => import('../views/user/UserLogs.vue') },

      // Reseller 代理面板（仅 isReseller=true 可访问，守卫在 router.beforeEach）
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
  if (to.meta.auth) {
    const auth = useAuthStore()
    const ok = await auth.ensureSession()
    if (!ok) return '/login'
  }
  if (to.meta.userAuth || to.matched.some(r => r.meta.userAuth)) {
    const apiKey = localStorage.getItem('user_api_key') || sessionStorage.getItem('user_api_key')
    if (!apiKey) return '/login'
  }
  // Reseller guard: requires isReseller=true on the user info
  if (to.matched.some(r => r.meta.requireReseller)) {
    const { useUserAuth } = await import('../stores/userAuth')
    const userAuth = useUserAuth()
    if (!userAuth.userInfo) {
      await userAuth.refresh()
    }
    if (!userAuth.userInfo?.isReseller) {
      return '/user/dashboard'
    }
  }
  // Child-key guard: child key 不能访问标记 blockChildKey 的路由（充值页等）
  if (to.matched.some(r => r.meta.blockChildKey)) {
    const { useUserAuth } = await import('../stores/userAuth')
    const userAuth = useUserAuth()
    if (!userAuth.userInfo) {
      await userAuth.refresh()
    }
    if (userAuth.userInfo?.isChildKey) {
      return '/user/dashboard'
    }
  }
})

export default router
