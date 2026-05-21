import { api } from '../admin'

export interface NewAPIProvider {
  id: string
  name?: string
  baseUrl?: string
  userId?: number
  tokenCount?: number
  channelCount?: number
  modelCount?: number
  enabled?: boolean
  lastSyncAt?: number
  lastSyncError?: string
  quotaPerUnitDollar?: number
  yuanPerUpstreamDollar?: number
  syncIntervalSec?: number
}

export interface NewAPIProviderUpsert {
  id: string
  name?: string
  baseUrl: string
  username?: string
  password?: string
  quotaPerUnitDollar?: number
  yuanPerUpstreamDollar?: number
  syncIntervalSec?: number
  enabled?: boolean
}

export async function listProviders(): Promise<NewAPIProvider[]> {
  const r = await api('/providers')
  return (await r.json()) as NewAPIProvider[]
}

export async function createProvider(req: NewAPIProviderUpsert): Promise<NewAPIProvider> {
  return await (await api('/providers', { method: 'POST', body: JSON.stringify(req) })).json()
}

export async function updateProvider(id: string, req: Partial<NewAPIProviderUpsert>): Promise<NewAPIProvider> {
  return await (await api(`/providers/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(req) })).json()
}

export async function getProvider(id: string): Promise<NewAPIProvider> {
  return await (await api(`/providers/${encodeURIComponent(id)}`)).json()
}

export async function getProviderMetadata(id: string): Promise<{
  groups?: Array<{ name: string; description?: string }>
  models?: string[]
  tokens?: Array<{ id: number; name?: string; status?: number }>
}> {
  return await (await api(`/providers/${encodeURIComponent(id)}/metadata`)).json()
}

export async function syncProvider(id: string): Promise<unknown> {
  const r = await api(`/providers/${encodeURIComponent(id)}/sync`, { method: 'POST' })
  return await r.json()
}

export async function deleteProvider(id: string): Promise<void> {
  await api(`/providers/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
