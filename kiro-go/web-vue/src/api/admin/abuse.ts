import { api } from '../admin'

export interface AbuseFlag {
  keyId: string
  reason: string
  activeStreams: number
  distinctIPs: number
}

export async function listAbuseFlags(): Promise<AbuseFlag[]> {
  const r = await api('/abuse')
  return (await r.json()) as AbuseFlag[]
}

export async function clearAbuseFlag(keyId: string): Promise<void> {
  await api(`/abuse/${encodeURIComponent(keyId)}/clear`, {
    method: 'POST',
  })
}
