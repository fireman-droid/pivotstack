import { useAuthStore } from '../stores/auth'

const BASE = '/admin/api'

export async function api(path, opts = {}) {
  const auth = useAuthStore()
  const { method = 'GET', body, password } = opts
  const headers = { 'X-Admin-Password': password || auth.password }
  if (body) headers['Content-Type'] = 'application/json'
  const res = await fetch(BASE + path, { method, headers, body })
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
