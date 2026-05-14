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

// ==================== Channels (v3) ====================
export async function listChannels() {
  return (await api('/channels')).json()
}
export async function createChannel(channel) {
  return api('/channels', { method: 'POST', body: JSON.stringify(channel) })
}
export async function updateChannel(id, channel) {
  return api(`/channels/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(channel) })
}
export async function deleteChannel(id) {
  return api(`/channels/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
export async function testChannel(id) {
  return (await api(`/channels/${encodeURIComponent(id)}/test`, { method: 'POST' })).json()
}

// ==================== Sell Prices (v3) ====================
export async function getSellPrices() {
  return (await api('/sell-prices')).json()
}
export async function updateSellPrices(prices) {
  return api('/sell-prices', { method: 'PUT', body: JSON.stringify(prices) })
}
