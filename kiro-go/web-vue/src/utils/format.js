export function formatNum(n) {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return n.toString()
}

export function formatTokenExpiry(ts) {
  if (!ts) return '-'
  // 转换为北京时间（UTC+8）
  const date = new Date(ts * 1000)
  const bjTime = new Date(date.getTime() + 8 * 60 * 60 * 1000)
  const now = new Date(Date.now() + 8 * 60 * 60 * 1000)
  const diff = (bjTime - now) / 1000

  if (diff <= 0) return '已过期'
  if (diff < 3600) return Math.floor(diff / 60) + '分钟'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时'
  return Math.floor(diff / 86400) + '天'
}

export function maskEmail(email, enabled = true) {
  if (!enabled || !email || !email.includes('@')) return email
  const [local, domain] = email.split('@')
  const maskedLocal = local.length <= 2 ? local : local.substring(0, 2) + '***'
  const parts = domain.split('.')
  if (parts.length >= 2) {
    const tld = parts[parts.length - 1]
    const sld = parts[parts.length - 2]
    const maskedSld = sld.length <= 2 ? sld : sld.substring(0, 2) + '***'
    const subs = parts.slice(0, -2).map(s => s.length <= 2 ? s : s.substring(0, 2) + '***')
    return maskedLocal + '@' + [...subs, maskedSld, tld].join('.')
  }
  return maskedLocal + '@' + domain
}

export function formatTrialExpiry(timestamp) {
  if (!timestamp) return ''
  const date = new Date(timestamp * 1000)
  const now = new Date()
  const diffDays = Math.ceil((date - now) / (1000 * 60 * 60 * 24))
  if (diffDays < 0) return '(已过期)'
  if (diffDays === 0) return '(今天到期)'
  if (diffDays <= 7) return `(${diffDays}天后到期)`
  return ''
}

export function getSubBadge(type) {
  const t = (type || '').toUpperCase()
  if (t.includes('POWER')) return { label: 'POWER', color: 'amber' }
  if (t.includes('PRO_PLUS') || t.includes('PROPLUS')) return { label: 'PRO+', color: 'violet' }
  if (t.includes('PRO')) return { label: 'PRO', color: 'blue' }
  return { label: 'FREE', color: 'gray' }
}

// 金额自适应精度：避免 $0.000048 被 toFixed(4) round 成 $0.0000。
// ≥0.01 → 2 位（'$1.23'），<0.01 → 4 位（'$0.0048'），<0.0001 → 6 位 trim 尾 0（'$0.000048'）。
export function fmtCost(v) {
  if (v == null || v === 0 || Number.isNaN(v)) return '$0'
  const a = Math.abs(v)
  if (a < 0.0001) {
    const s = v.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')
    return '$' + s
  }
  if (a < 0.01) return '$' + v.toFixed(4)
  return '$' + v.toFixed(2)
}

// 后端返回的 *_CNY 字段直接展示 ¥；不做单位换算（后端已用 PivotStackDollarsPerYuan 换算）。
export function fmtMoneyCny(v) {
  if (v == null || Number.isNaN(v)) return '¥-'
  return '¥' + v.toFixed(2)
}

// API Key 套餐 plan 枚举值 → 中文 label。后端 value 仍是 credit/timed/hybrid，仅 UI 显示用中文。
export function planLabel(plan) {
  if (plan === 'credit') return '余额卡'
  if (plan === 'timed') return '天卡'
  if (plan === 'hybrid') return '混合'
  return plan || '-'
}
