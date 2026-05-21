import { api } from '../admin'

export interface ActivationCode {
  code: string
  type?: string
  amount?: number
  salePriceCNY?: number
  batch?: string
  note?: string
  used: boolean
  usedBy?: string
  usedAt?: number
  createdAt?: number
}

export async function listCodes(used = false): Promise<ActivationCode[]> {
  const q = used ? '?used=true' : ''
  const r = await api(`/codes${q}`)
  return (await r.json()) as ActivationCode[]
}

export interface CreateCodesRequest {
  /** 'balance' = 余额型 / 'days' = 按天 / 'time' = 按秒 */
  type: 'balance' | 'days' | 'time'
  /** balance 时单位 ¥；days 时单位 天；time 时单位 秒 */
  amount: number
  /** 仅 days/time 需要 */
  tier?: 'free' | 'pro'
  /** 单次生成数量，后端上限 100 */
  count: number
  note?: string
  /** 仅天卡：admin 卖给客户的价格（¥），用于利润计算 */
  salePriceCNY?: number
}

export interface CreateCodesResponse {
  success: boolean
  codes: string[]
  count: number
}

export async function createCodes(req: CreateCodesRequest): Promise<CreateCodesResponse> {
  return await (await api('/codes', { method: 'POST', body: JSON.stringify(req) })).json()
}

export async function deleteCode(code: string): Promise<void> {
  await api(`/codes/${encodeURIComponent(code)}`, { method: 'DELETE' })
}
