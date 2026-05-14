import { defineStore } from 'pinia'
import { ref } from 'vue'

const BASE = '/admin/api'

// v2 auth store:
// - 不再在 localStorage 存任何明文密码
// - 服务端用 HttpOnly admin_session cookie 维持登录态
// - csrfToken 仅存内存（不进 localStorage），随会话过期自然失效
export const useAuthStore = defineStore('auth', () => {
  const authenticated = ref(false)
  const csrfToken = ref('')
  const checked = ref(false)
  let inflight = null

  // 一次性清理旧版 localStorage 残留（防止泄露已经发生的明文密码继续存在硬盘）
  if (localStorage.getItem('admin_password') || localStorage.getItem('admin_login_time')) {
    localStorage.removeItem('admin_password')
    localStorage.removeItem('admin_login_time')
  }

  /**
   * 登录。失败时返回 { ok: false, error, remainingAttempts?, locked?, retryAfter? }
   */
  async function login(pwd) {
    try {
      const res = await fetch(`${BASE}/login`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: pwd }),
      })
      let data = {}
      try { data = await res.json() } catch {}
      if (res.ok) {
        authenticated.value = true
        csrfToken.value = data.csrfToken || ''
        checked.value = true
        return { ok: true }
      }
      return {
        ok: false,
        error: data.error || `HTTP ${res.status}`,
        remainingAttempts: data.remainingAttempts,
        locked: !!data.locked,
        retryAfter: data.retryAfter,
      }
    } catch (e) {
      return { ok: false, error: '连接服务器失败' }
    }
  }

  /**
   * 用 cookie 探活 + 拿 csrfToken。被并发调用时合流到同一 promise，避免冲量。
   */
  async function ensureSession() {
    if (checked.value && authenticated.value) return true
    if (inflight) return inflight
    inflight = (async () => {
      try {
        const res = await fetch(`${BASE}/session`, { credentials: 'same-origin' })
        if (!res.ok) {
          clearLocal()
          return false
        }
        const data = await res.json()
        authenticated.value = true
        csrfToken.value = data.csrfToken || ''
        return true
      } catch {
        clearLocal()
        return false
      } finally {
        checked.value = true
        inflight = null
      }
    })()
    return inflight
  }

  async function logout() {
    try {
      await fetch(`${BASE}/logout`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'X-CSRF-Token': csrfToken.value },
      })
    } catch {}
    clearLocal()
  }

  function clearLocal() {
    authenticated.value = false
    csrfToken.value = ''
    checked.value = true
    // 防御性清旧版残留
    localStorage.removeItem('admin_password')
    localStorage.removeItem('admin_login_time')
  }

  return { authenticated, csrfToken, checked, login, logout, ensureSession, clearLocal }
})
