import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '../api/admin'

export const useAuthStore = defineStore('auth', () => {
  const password = ref(localStorage.getItem('admin_password') || '')
  const loginTime = ref(parseInt(localStorage.getItem('admin_login_time') || '0'))

  // 72h 过期
  if (loginTime.value && Date.now() - loginTime.value > 72 * 3600 * 1000) {
    password.value = ''
    loginTime.value = 0
    localStorage.removeItem('admin_password')
    localStorage.removeItem('admin_login_time')
  }

  async function login(pwd) {
    const res = await api('/status', { password: pwd })
    if (res.ok) {
      password.value = pwd
      loginTime.value = Date.now()
      localStorage.setItem('admin_password', pwd)
      localStorage.setItem('admin_login_time', loginTime.value.toString())
      return true
    }
    return false
  }

  function logout() {
    password.value = ''
    localStorage.removeItem('admin_password')
    localStorage.removeItem('admin_login_time')
  }

  return { password, login, logout }
})
