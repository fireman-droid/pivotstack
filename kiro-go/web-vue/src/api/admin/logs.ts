import { api } from '../admin'

export interface CallLog {
  request_id?: string
  time?: string
  timestamp?: number
  api_key_id?: string
  api_key_note?: string
  channel_id?: string
  channel_alias?: string
  channel_type?: string
  original_model?: string
  actual_model?: string
  input_tokens?: number
  output_tokens?: number
  duration_ms?: number
  status?: string
  error?: string
  cost_usd?: number
}

export interface CallLogsResponse {
  logs: CallLog[]
  total?: number
  page?: number
}

export async function listLogs(params: Record<string, string | number | undefined>): Promise<CallLogsResponse> {
  const qs = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') qs.set(key, String(value))
  })
  const r = await api(`/logs?${qs.toString()}`)
  return (await r.json()) as CallLogsResponse
}
