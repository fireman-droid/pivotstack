import { api } from '../admin'

export interface AdminSettings {
  requireApiKey?: boolean
  apiKey?: string
  host?: string
  port?: number
  runtimeHost?: string   // main.go 启动 listener 后回写的实际监听地址
  runtimePort?: string
  maxConcurrentPerKey?: number
  maxInFlightPerAccountFree?: number
  maxInFlightPerAccountPro?: number
  timedKeyRPM?: number
  abuseEnabled?: boolean
  [key: string]: unknown
}

export async function getSettings(): Promise<AdminSettings> {
  const r = await api('/settings')
  return (await r.json()) as AdminSettings
}

export async function updateSettings(payload: AdminSettings): Promise<AdminSettings> {
  // 后端只接 POST /settings（plan 一致），PUT 会 404。
  const r = await api('/settings', { method: 'POST', body: JSON.stringify(payload) })
  return (await r.json()) as AdminSettings
}
