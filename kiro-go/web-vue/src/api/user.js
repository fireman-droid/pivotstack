const BASE = '/user/api'

export async function userApi(path, opts = {}) {
  const { method = 'GET', body } = opts
  const apiKey = localStorage.getItem('user_api_key') || sessionStorage.getItem('user_api_key') || ''
  const headers = { 'Authorization': `Bearer ${apiKey}` }
  if (body) headers['Content-Type'] = 'application/json'
  const res = await fetch(BASE + path, { method, headers, body: body ? JSON.stringify(body) : undefined })
  // 401 全局拦截：key 被禁用 / session 过期 → 清 storage + 跳 /login 提示原因
  if (res.status === 401) {
    const reason = res.headers.get('X-Auth-Reason') || 'expired'
    localStorage.removeItem('user_api_key')
    sessionStorage.removeItem('user_api_key')
    if (typeof location !== 'undefined' && location.pathname.startsWith('/user/')) {
      location.replace('/login?reason=' + encodeURIComponent(reason))
    }
  }
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    let data = {}
    try {
      data = await res.clone().json()
      if (data.error) msg = data.error
    } catch {}
    const err = new Error(msg)
    err.status = res.status
    err.retryAfter = res.headers.get('Retry-After')
    err.data = data
    throw err
  }
  return res.json()
}

// v5: per-series 渠道偏好。响应里 availableSeries 已经 mask 过敏感字段（不含 upstreamKey 等）。
export async function getPreferences() {
  return userApi('/preferences')
}
// 替换式更新 channelPreferences 映射；空字符串 channelId 表示移除该 series 的偏好。
export async function updatePreferences(channelPreferences) {
  return userApi('/preferences', { method: 'PUT', body: { channelPreferences } })
}

// v7: user-side ApiKey CRUD（ownership 严格校验，禁止改 balance / isReseller）
export async function listUserKeys() {
  return userApi('/keys')
}
// v7.1: 创建 key 时一次性配置全部参数（名称 / 路由偏好 / 过期时间 / 速率限制）
export async function createUserKey(payload) {
  return userApi('/keys', { method: 'POST', body: payload || {} })
}
// 可选 group/channel 选项（用于创建 Key 表单的下拉）
export async function getChannelOptions() {
  return userApi('/channel-options')
}
export async function patchUserKey(id, patch) {
  return userApi(`/keys/${id}`, { method: 'PATCH', body: patch })
}
export async function deleteUserKey(id) {
  return userApi(`/keys/${id}`, { method: 'DELETE' })
}

// v7: bind 老 key 到新建账号（用户名/邮箱/密码）
export async function bindAccount(email, password, username) {
  return userApi('/bind-account', { method: 'POST', body: { email, password, username } })
}
