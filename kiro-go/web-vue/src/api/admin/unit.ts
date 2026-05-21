import { api } from '../admin'

export interface UnitHistory {
  time?: number
  oldValue?: number
  newValue?: number
  actor?: string
}

export interface SystemUnitConfig {
  pivotStackDollarsPerYuan?: number
  yuanPerUSD?: number
  history?: UnitHistory[]
}

export async function getSystemUnitConfig(): Promise<SystemUnitConfig> {
  const r = await api('/system/unit-config')
  return (await r.json()) as SystemUnitConfig
}

export async function updateSystemUnitConfig(payload: {
  newValue: number
  adminPassword?: string
  rebalanceUserBalances?: boolean
}): Promise<SystemUnitConfig> {
  const r = await api('/system/unit-config', { method: 'POST', body: JSON.stringify(payload) })
  return (await r.json()) as SystemUnitConfig
}
