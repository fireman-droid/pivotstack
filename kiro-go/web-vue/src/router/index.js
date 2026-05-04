import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },

  // Admin routes
  { path: '/',         name: 'Dashboard',      component: () => import('../views/Dashboard.vue'),       meta: { auth: true } },
  { path: '/accounts', name: 'Accounts',       component: () => import('../views/Accounts.vue'),        meta: { auth: true } },
  { path: '/apikeys',  name: 'ApiKeys',        component: () => import('../views/ApiKeys.vue'),         meta: { auth: true } },
  { path: '/settings', name: 'Settings',       component: () => import('../views/Settings.vue'),        meta: { auth: true } },
  { path: '/api',      name: 'ApiInfo',        component: () => import('../views/ApiInfo.vue'),         meta: { auth: true } },
  { path: '/logs',     name: 'Logs',           component: () => import('../views/Logs.vue'),            meta: { auth: true } },
  { path: '/pricing',  name: 'Pricing',        component: () => import('../views/Pricing.vue'),         meta: { auth: true } },
  { path: '/codes',    name: 'CodeManagement', component: () => import('../views/CodeManagement.vue'),  meta: { auth: true } },
  { path: '/stealth',  name: 'StealthConfig',  component: () => import('../views/StealthConfig.vue'),   meta: { auth: true } },
  { path: '/leaderboard', name: 'Leaderboard', component: () => import('../views/Leaderboard.vue'),     meta: { auth: true } },

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
      { path: 'recharge',  name: 'UserRecharge',  component: () => import('../views/user/UserRecharge.vue') },
      { path: 'logs',      name: 'UserLogs',      component: () => import('../views/user/UserLogs.vue') },
      // 用户端排行榜暂时隐藏，路由 + 导航全部下线；UserLeaderboard.vue 文件保留供后续启用
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    return savedPosition || { top: 0 }
  },
})

router.beforeEach((to) => {
  if (to.meta.auth) {
    const auth = useAuthStore()
    if (!auth.password) return '/login'
  }
  if (to.meta.userAuth || to.matched.some(r => r.meta.userAuth)) {
    const apiKey = localStorage.getItem('user_api_key')
    if (!apiKey) return '/login'
  }
})

export default router
