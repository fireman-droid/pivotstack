import { api } from '../admin'

export interface KiroAccount {
  id: string
  email?: string
  nickname?: string
  userId?: string
  authMethod?: string
  provider?: string
  region?: string
  enabled: boolean
  weight?: number
  banStatus?: string
  banReason?: string
  hasToken?: boolean
  subscriptionType?: string
  subscriptionTitle?: string
  daysRemaining?: number
  usageCurrent?: number
  usageLimit?: number
  usagePercent?: number
  nextResetDate?: number
  lastRefresh?: number
  requestCount?: number
  errorCount?: number
  totalTokens?: number
  totalCredits?: number
  inFlight?: number
}

export interface AddAccountRequest {
  email?: string
  refreshToken?: string
  accessToken?: string
  authMethod?: 'idc' | 'social'
  region?: string
  nickname?: string
  weight?: number
}

export async function listAccounts(): Promise<KiroAccount[]> {
  return await (await api('/accounts')).json()
}

export async function addAccount(req: AddAccountRequest): Promise<{ success: boolean; id: string }> {
  return await (await api('/accounts', { method: 'POST', body: JSON.stringify(req) })).json()
}

export async function batchAddAccounts(items: AddAccountRequest[]): Promise<unknown> {
  return await (await api('/accounts/batch', { method: 'POST', body: JSON.stringify({ accounts: items }) })).json()
}

// === OAuth: BuilderId device code flow ===
export interface BuilderIdStartResp {
  sessionId: string
  userCode: string
  verificationUri: string
  interval: number
}
export async function startBuilderIdLogin(region: string): Promise<BuilderIdStartResp> {
  return await (await api('/auth/builderid/start', { method: 'POST', body: JSON.stringify({ region }) })).json()
}

export interface BuilderIdPollResp {
  success: boolean
  completed: boolean
  status?: string
  interval?: number
  isNew?: boolean
  account?: { id: string; email?: string }
  error?: string
}
export async function pollBuilderIdAuth(sessionId: string): Promise<BuilderIdPollResp> {
  // pending 时也是 200 + completed:false，通用 api() 不会 throw
  return await (await api('/auth/builderid/poll', { method: 'POST', body: JSON.stringify({ sessionId }) })).json()
}

export async function updateAccount(id: string, patch: Partial<KiroAccount>): Promise<unknown> {
  return await (await api(`/accounts/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(patch) })).json()
}

export async function deleteAccount(id: string): Promise<void> {
  await api(`/accounts/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export async function refreshAccount(id: string): Promise<unknown> {
  return await (await api(`/accounts/${encodeURIComponent(id)}/refresh`, { method: 'POST' })).json()
}
