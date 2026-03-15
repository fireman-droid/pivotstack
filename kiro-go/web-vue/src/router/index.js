import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },
  { path: '/', name: 'Dashboard', component: () => import('../views/Dashboard.vue'), meta: { auth: true } },
  { path: '/accounts', name: 'Accounts', component: () => import('../views/Accounts.vue'), meta: { auth: true } },
  { path: '/apikeys', name: 'ApiKeys', component: () => import('../views/ApiKeys.vue'), meta: { auth: true } },
  { path: '/settings', name: 'Settings', component: () => import('../views/Settings.vue'), meta: { auth: true } },
  { path: '/api', name: 'ApiInfo', component: () => import('../views/ApiInfo.vue'), meta: { auth: true } },
  { path: '/logs', name: 'Logs', component: () => import('../views/Logs.vue'), meta: { auth: true } },
  { path: '/pricing', name: 'Pricing', component: () => import('../views/PricingAnalysis.vue'), meta: { auth: true } },
  { path: '/pricing-config', name: 'PricingConfig', component: () => import('../views/PricingConfig.vue'), meta: { auth: true } },
  { path: '/codes', name: 'CodeManagement', component: () => import('../views/CodeManagement.vue'), meta: { auth: true } },
  { path: '/abuse', name: 'AbuseMonitor', component: () => import('../views/AbuseMonitor.vue'), meta: { auth: true } },

  // User portal routes
  { path: '/user/login', name: 'UserLogin', component: () => import('../views/user/UserLogin.vue') },
  {
    path: '/user',
    component: () => import('../views/user/UserLayout.vue'),
    meta: { userAuth: true },
    children: [
      { path: '', redirect: '/user/dashboard' },
      { path: 'dashboard', name: 'UserDashboard', component: () => import('../views/user/UserDashboard.vue') },
      { path: 'recharge', name: 'UserRecharge', component: () => import('../views/user/UserRecharge.vue') },
      { path: 'logs', name: 'UserLogs', component: () => import('../views/user/UserLogs.vue') },
    ]
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    return savedPosition || { top: 0 }
  }
})

router.beforeEach((to) => {
  // Admin auth guard
  if (to.meta.auth) {
    const auth = useAuthStore()
    if (!auth.password) return '/login'
  }
  // User auth guard
  if (to.meta.userAuth || to.matched.some(r => r.meta.userAuth)) {
    const apiKey = localStorage.getItem('user_api_key')
    if (!apiKey) return '/login'
  }
})

export default router
