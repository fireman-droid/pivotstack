import { api } from '../admin'

export interface DirectSellPriceRow {
  inputPerM?: number
  outputPerM?: number
  costInputPerM?: number
  costOutputPerM?: number
}

export interface DirectChannel {
  id: string
  type: 'openai' | 'kiro'
  alias: string
  baseUrl?: string
  hasAPIKey?: boolean
  models?: string[]
  sellPrice?: { default?: DirectSellPriceRow; models?: Record<string, DirectSellPriceRow> }
  modelMapping?: Record<string, string>
  extraHeaders?: Record<string, string>
  enabled: boolean
  status?: string
  createdAt?: number
  updatedAt?: number
  deletedAt?: number
}

export interface DirectChannelCreateRequest {
  type: 'openai' | 'kiro'
  alias: string
  baseUrl?: string
  apiKey?: string
  models?: string[]
  sellPrice?: { default?: DirectSellPriceRow }
  modelMapping?: Record<string, string>
  extraHeaders?: Record<string, string>
  enabled?: boolean
}

export async function listDirectChannels(): Promise<DirectChannel[]> {
  const r = await api('/direct-channels')
  return (await r.json()) as DirectChannel[]
}

export async function createDirectChannel(req: DirectChannelCreateRequest): Promise<DirectChannel> {
  return await (await api('/direct-channels', { method: 'POST', body: JSON.stringify(req) })).json()
}

export async function patchDirectChannel(id: string, patch: Partial<DirectChannel>): Promise<DirectChannel> {
  const r = await api(`/direct-channels/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  })
  return (await r.json()) as DirectChannel
}

export async function deleteDirectChannel(id: string): Promise<void> {
  await api(`/direct-channels/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
