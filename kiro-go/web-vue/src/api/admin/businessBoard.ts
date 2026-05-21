import { api } from '../admin'

export type BoardPeriod = 'today' | '7d' | '30d' | 'custom'

export interface BusinessBoardQuery {
  period?: BoardPeriod
  from?: number
  to?: number
  includeGift?: boolean
  channel?: string
  topN?: number
}

export interface TrendPoint {
  date: string
  revenue_cny: number
  cost_cny: number
}

export interface BusinessKpi {
  revenue_cny: number
  cost_cny: number
  profit_cny: number
  margin_percent: number
}

export interface RevenueBreakdown {
  balance_cny: number
  time_cards_cny: number
  gift_cny: number
  total_cny: number
}

export interface ChannelRow {
  channel_id: string
  channel_type: string
  alias: string
  requests: number
  errors: number
  tokens_in: number
  tokens_out: number
  tokens: number
  charged_cny: number
  cost_cny: number
  revenue_share_cny: number
  profit_cny: number
  margin_percent: number
}

export interface ModelRow {
  model: string
  channel_id: string
  requests: number
  tokens_in: number
  tokens_out: number
  tokens: number
  charged_cny: number
  cost_cny: number
  revenue_share_cny: number
  profit_cny: number
  margin_percent: number
}

export interface BusinessBoardResponse {
  period: string
  from: number
  to: number
  include_gift: boolean
  kpi: BusinessKpi
  revenue_breakdown: RevenueBreakdown
  channels: ChannelRow[]
  models: ModelRow[]
  trend: TrendPoint[]
  warnings?: string[]
}

export async function getBusinessBoard(q: BusinessBoardQuery = {}): Promise<BusinessBoardResponse> {
  const qs = new URLSearchParams()
  if (q.period) qs.set('period', q.period)
  if (q.from !== undefined) qs.set('from', String(q.from))
  if (q.to !== undefined) qs.set('to', String(q.to))
  if (q.includeGift !== undefined) qs.set('include_gift', String(q.includeGift))
  if (q.channel) qs.set('channel', q.channel)
  if (q.topN) qs.set('top_n', String(q.topN))
  const suffix = qs.toString()
  const r = await api(suffix ? `/business-board?${suffix}` : '/business-board')
  return (await r.json()) as BusinessBoardResponse
}
