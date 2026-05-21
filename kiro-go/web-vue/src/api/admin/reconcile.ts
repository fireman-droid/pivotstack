import { api } from '../admin'

export interface ReconcileEvent {
  id?: string
  requestId?: string
  time?: string
  createdAt?: number
  providerId?: string
  channelId?: string
  channelAlias?: string
  upstreamCost?: number
  localCost?: number
  diff?: number
  status?: 'pending' | 'success' | 'failed' | 'retrying' | string
  action?: string
}

export interface ReconcileStatus {
  pending?: number
  success?: number
  failed?: number
  retrying?: number
  queue?: ReconcileEvent[]
  retry?: ReconcileEvent[]
  anomalies?: ReconcileEvent[]
  recent?: ReconcileEvent[]
  providers?: Array<{ providerId: string; recentEvents?: ReconcileEvent[] }>
}

export async function getReconcileStatus(): Promise<ReconcileStatus> {
  const r = await api('/newapi/reconcile-status')
  return (await r.json()) as ReconcileStatus
}

export async function retryReconcileRequest(requestId: string): Promise<unknown> {
  const r = await api(`/newapi/reconcile-status/retry/${encodeURIComponent(requestId)}`, { method: 'POST' })
  return await r.json()
}
