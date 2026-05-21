import { api } from '../admin'

export interface ApiKeyRow {
  id: string
  key?: string
  keyMasked?: string
  note?: string
  plan?: string
  tier?: string
  enabled: boolean
  balance?: number          // 虚拟 $
  giftBalance?: number
  totalBalance?: number
  balanceCNY?: number       // 后端换算（balance / psdpy）
  giftBalanceCNY?: number
  totalBalanceCNY?: number
  expiresAt?: number
  createdAt?: number
  lastUsed?: number
  requests?: number
  isReseller?: boolean
  maxChildKeys?: number
  parentKeyId?: string
}

export interface ApiKeyUpdate {
  plan?: string
  expiresAt?: number
  enabled?: boolean
  balance?: number
  giftBalance?: number
  note?: string
  isReseller?: boolean
  maxChildKeys?: number
}

export async function listApiKeys(): Promise<ApiKeyRow[]> {
  const r = await api('/apikeys')
  return (await r.json()) as ApiKeyRow[]
}

export async function createApiKey(note: string): Promise<ApiKeyRow> {
  return await (await api('/apikeys', { method: 'POST', body: JSON.stringify({ note }) })).json()
}

export async function updateApiKey(id: string, patch: ApiKeyUpdate): Promise<ApiKeyRow> {
  return await (await api(`/apikeys/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(patch) })).json()
}

export async function deleteApiKey(id: string): Promise<void> {
  await api(`/apikeys/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export interface ApiKeyLogEntry {
  request_id?: string
  time?: string
  timestamp?: number
  original_model?: string
  actual_model?: string
  channel_id?: string
  channel_alias?: string
  input_tokens?: number
  output_tokens?: number
  duration_ms?: number
  cost_usd?: number
  charged_usd?: number   // v2+：token/newapi 实际扣费总额（paid + gift）
  paid_credits?: number
  gifted_credits?: number
  status?: string
  error?: string
  billing_mode?: string
  billing_status?: string
}

export async function getApiKeyLogs(id: string): Promise<ApiKeyLogEntry[]> {
  const r = await api(`/apikeys/${encodeURIComponent(id)}/logs`)
  const data = (await r.json()) as { logs?: ApiKeyLogEntry[] }
  return data.logs || []
}

export interface RechargeRecord {
  time?: string
  timestamp?: number
  type?: string
  amountUsd?: number
  amountCny?: number
  balanceBefore?: number
  balanceAfter?: number
  giftBefore?: number
  giftAfter?: number
  operator?: string
  note?: string
}

export async function getApiKeyRecharges(id: string, page = 0, limit = 100): Promise<{ records: RechargeRecord[]; total: number }> {
  const r = await api(`/apikeys/${encodeURIComponent(id)}/recharges?page=${page}&limit=${limit}`)
  return (await r.json()) as { records: RechargeRecord[]; total: number }
}
