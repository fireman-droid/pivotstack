import { useAuthStore } from '../stores/auth'

const BASE = '/admin/api'

// v2 fetch wrapper:
// - 鉴权完全由 admin_session cookie 承担（HttpOnly + Secure + SameSite=Strict）
// - 不安全方法（POST/PUT/DELETE/PATCH）必须带 X-CSRF-Token
// - 401 自动清前端态（防止 dead session 死循环）
export async function api(path, opts = {}) {
  const auth = useAuthStore()
  const { method = 'GET', body } = opts
  const headers = {}

  if (body) headers['Content-Type'] = 'application/json'

  const isSafe = ['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase())
  if (!isSafe && auth.csrfToken) {
    headers['X-CSRF-Token'] = auth.csrfToken
  }

  const res = await fetch(BASE + path, {
    method,
    headers,
    body,
    credentials: 'same-origin',
  })

  if (res.status === 401) {
    auth.clearLocal()
  }
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = await res.clone().json()
      if (data.error) msg = data.error
    } catch {}
    throw new Error(msg)
  }
  return res
}
