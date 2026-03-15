import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { userApi } from '../api/user'

export const useUserAuth = defineStore('userAuth', () => {
  const apiKey = ref(localStorage.getItem('user_api_key') || sessionStorage.getItem('user_api_key') || '')
  const userInfo = ref(null)
  const loading = ref(false)
  const error = ref('')

  const isLoggedIn = computed(() => !!apiKey.value && !!userInfo.value)
  const balance = computed(() => userInfo.value?.balance || 0)
  const plan = computed(() => userInfo.value?.plan || '')
  const status = computed(() => userInfo.value?.status || '')

  async function login(key, remember = true) {
    loading.value = true
    error.value = ''
    try {
      if (remember) {
        localStorage.setItem('user_api_key', key)
      } else {
        sessionStorage.setItem('user_api_key', key)
        localStorage.removeItem('user_api_key')
      }
      apiKey.value = key
      const data = await userApi('/me')
      userInfo.value = data
      return true
    } catch (e) {
      error.value = e.message
      localStorage.removeItem('user_api_key')
      sessionStorage.removeItem('user_api_key')
      apiKey.value = ''
      userInfo.value = null
      return false
    } finally {
      loading.value = false
    }
  }

  async function refresh() {
    if (!apiKey.value) return
    try {
      const data = await userApi('/me')
      userInfo.value = data
    } catch {}
  }

  function logout() {
    localStorage.removeItem('user_api_key')
    sessionStorage.removeItem('user_api_key')
    apiKey.value = ''
    userInfo.value = null
  }

  // Auto-login on store init
  if (apiKey.value) {
    refresh()
  }

  return { apiKey, userInfo, loading, error, isLoggedIn, balance, plan, status, login, refresh, logout }
})
