// Admin API · 全平台充值流水
import { api } from '../admin'

export interface AdminRechargeRecord {
  time: string
  timestamp: number
  key_id: string
  key_note?: string
  type: 'code_redeem' | 'code_redeem_days' | 'admin_balance' | 'admin_gift' | 'admin_adjust'
  code?: string
  amount_usd: number
  amount_cny: number
  balance_before: number       // 虚拟 $ 单位（user.Balance 同口径）
  balance_after: number        // 虚拟 $ 单位
  balance_before_cny?: number  // 后端换算好的 ¥（balance_before / psdpy）
  balance_after_cny?: number
  gift_before?: number
  gift_after?: number
  operator: 'user' | 'admin'
  note?: string
  ip?: string
}

export interface AdminRechargesSummary {
  todayCNY: number
  monthCNY: number
  avgCNY: number
  returningRate: number
}

export interface AdminRechargesResult {
  records: AdminRechargeRecord[]
  total: number
  summary: AdminRechargesSummary
}

export interface AdminRechargesQuery {
  limit?: number
  offset?: number
  type?: string
  search?: string
  from?: number
  to?: number
}

export async function listAdminRecharges(q: AdminRechargesQuery = {}): Promise<AdminRechargesResult> {
  const params = new URLSearchParams()
  if (q.limit != null) params.set('limit', String(q.limit))
  if (q.offset != null) params.set('offset', String(q.offset))
  if (q.type) params.set('type', q.type)
  if (q.search) params.set('search', q.search)
  if (q.from) params.set('from', String(q.from))
  if (q.to) params.set('to', String(q.to))
  const qs = params.toString()
  const path = qs ? `/recharges?${qs}` : '/recharges'
  return (await api(path)).json()
}
