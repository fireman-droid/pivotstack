const BASE = '/user/api'

export async function userApi(path, opts = {}) {
  const { method = 'GET', body } = opts
  const apiKey = localStorage.getItem('user_api_key') || ''
  const headers = { 'Authorization': `Bearer ${apiKey}` }
  if (body) headers['Content-Type'] = 'application/json'
  const res = await fetch(BASE + path, { method, headers, body: body ? JSON.stringify(body) : undefined })
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = await res.clone().json()
      if (data.error) msg = data.error
    } catch {}
    throw new Error(msg)
  }
  return res.json()
}
