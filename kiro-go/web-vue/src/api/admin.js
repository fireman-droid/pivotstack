import { useAuthStore } from '../stores/auth'

const BASE = '/admin/api'

export async function api(path, opts = {}) {
  const auth = useAuthStore()
  const { method = 'GET', body, password } = opts
  const headers = { 'X-Admin-Password': password || auth.password }
  if (body) headers['Content-Type'] = 'application/json'
  return fetch(BASE + path, { method, headers, body })
}
