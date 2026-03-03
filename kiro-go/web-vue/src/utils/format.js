export function formatNum(n) {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return n.toString()
}

export function formatTokenExpiry(ts) {
  if (!ts) return '-'
  const diff = ts - Date.now() / 1000
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
