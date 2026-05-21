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

// ==================== Series (v4) ====================
export async function listSeries() {
  return (await api('/series')).json()
}
export async function createSeries(series) {
  return (await api('/series', { method: 'POST', body: JSON.stringify(series) })).json()
}
export async function updateSeries(id, series) {
  return (await api(`/series/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(series) })).json()
}
export async function deleteSeries(id) {
  return api(`/series/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// ==================== Channels (v3 + v4 health-check) ====================
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
// v4 健康检查：探测 /v1/models + 1-token chat probe，per-channel 30s 冷却。
export async function checkChannelHealth(id) {
  return (await api(`/channels/${encodeURIComponent(id)}/health-check`, { method: 'POST' })).json()
}

// ==================== Sell Prices (v3) ====================
export async function getSellPrices() {
  return (await api('/sell-prices')).json()
}
export async function updateSellPrices(prices) {
  return api('/sell-prices', { method: 'PUT', body: JSON.stringify(prices) })
}

// ==================== v5 NewAPI Providers ====================
export async function listProviders() {
  return (await api('/providers')).json()
}
export async function getProvider(id) {
  return (await api(`/providers/${encodeURIComponent(id)}`)).json()
}
export async function createProvider(provider) {
  return (await api('/providers', { method: 'POST', body: JSON.stringify(provider) })).json()
}
export async function updateProvider(id, provider) {
  return (await api(`/providers/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(provider) })).json()
}
export async function deleteProvider(id, purge = false) {
  const q = purge ? '?purge=true' : ''
  return api(`/providers/${encodeURIComponent(id)}${q}`, { method: 'DELETE' })
}
// 立即触发同步（pricing + groups + tokens）。返回 sync 摘要。
export async function syncProvider(id) {
  return (await api(`/providers/${encodeURIComponent(id)}/sync`, { method: 'POST' })).json()
}
// 拉取 provider 缓存（groups / models / tokens）— 用于 admin 查看上游元数据现状。
export async function getProviderMetadata(id) {
  return (await api(`/providers/${encodeURIComponent(id)}/metadata`)).json()
}
// 把现有 v4 手工配的 openai channel 一键迁到 v5（baseURL 命中 + APIKey 命中上游 token）。
// dryRun=true 先看 plan，false 才真正写入。
export async function migrateProviderManualChannels(id, dryRun = true) {
  const qs = dryRun ? '?dryRun=true' : ''
  return (await api(`/providers/${encodeURIComponent(id)}/migrate-manual-channels${qs}`, { method: 'POST' })).json()
}

// ==================== v5 NewAPI Channels（同步物化出来的渠道）====================
// admin 只能改 alias/markup/seriesId/enabled；其他字段由同步流程覆盖。
export async function listNewAPIChannels() {
  return (await api('/newapi/channels')).json()
}
export async function patchNewAPIChannel(id, patch) {
  return (await api(`/newapi/channels/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(patch) })).json()
}
// POST /admin/api/newapi/channels — 在上游创建 token + 同步落 channel（一步到位）
export async function createNewAPIChannel(payload) {
  return (await api('/newapi/channels', { method: 'POST', body: JSON.stringify(payload) })).json()
}
// v5 渠道健康检查：双探针 GET /v1/models + 1-token chat probe。per-channel 30s 冷却。
export async function healthCheckNewAPIChannel(id) {
  return (await api(`/newapi/channels/${encodeURIComponent(id)}/health-check`, { method: 'POST' })).json()
}
// 删除 channel：deleteUpstream=true 同步删上游 token，false 只删 PivotStack 本地（软删）。
export async function deleteNewAPIChannel(id, { deleteUpstream = true } = {}) {
  const qs = deleteUpstream ? '?deleteUpstream=true' : '?deleteUpstream=false'
  return (await api(`/newapi/channels/${encodeURIComponent(id)}${qs}`, { method: 'DELETE' })).json()
}

// ==================== v5 Phase 4b Reconcile Monitor ====================
// providers[].recentEvents 已经是 reverse-chronological（最新在前）。
export async function getReconcileStatus() {
  return (await api('/newapi/reconcile-status')).json()
}
export async function retryReconcileRequest(requestId) {
  return (await api(`/newapi/reconcile-status/retry/${encodeURIComponent(requestId)}`, { method: 'POST' })).json()
}

// ==================== v5 System Unit Config（改全局 PivotStackDollarsPerYuan）====================
// 改值会影响所有 user 的虚拟$余额显示与后续 reservation 计费。必须二次输入 admin 密码。
export async function getSystemUnitConfig() {
  return (await api('/system/unit-config')).json()
}
export async function postSystemUnitConfig({ newValue, rebalanceUserBalances, adminPassword }) {
  return (await api('/system/unit-config', {
    method: 'POST',
    body: JSON.stringify({ newValue, rebalanceUserBalances, adminPassword }),
  })).json()
}
