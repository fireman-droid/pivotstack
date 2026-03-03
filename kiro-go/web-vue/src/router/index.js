import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },
  { path: '/', name: 'Dashboard', component: () => import('../views/Dashboard.vue'), meta: { auth: true } },
  { path: '/accounts', name: 'Accounts', component: () => import('../views/Accounts.vue'), meta: { auth: true } },
  { path: '/settings', name: 'Settings', component: () => import('../views/Settings.vue'), meta: { auth: true } },
  { path: '/api', name: 'ApiInfo', component: () => import('../views/ApiInfo.vue'), meta: { auth: true } },
  { path: '/logs', name: 'Logs', component: () => import('../views/Logs.vue'), meta: { auth: true } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.meta.auth) {
    const auth = useAuthStore()
    if (!auth.password) return '/login'
  }
})

export default router
