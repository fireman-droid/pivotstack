// PivotStack v6 ChannelGroup CRUD + 候选 channel 池 API。
//
// 后端契约（kiro-go proxy/handler_admin_groups.go）：
//   GET    /admin/api/groups            → ChannelGroup 列表
//   POST   /admin/api/groups            → 新建
//   GET    /admin/api/groups/channels   → 候选 channel 池
//   GET    /admin/api/groups/:id        → 单条
//   PATCH  /admin/api/groups/:id        → 改元数据
//   PUT    /admin/api/groups/:id/channels → 替换成员 + 默认 channel
//   DELETE /admin/api/groups/:id        → 软删 + 清偏好

import { api } from '../admin'

export interface ChannelGroupChannelEntry {
  runtimeId: string
  sourceType: 'newapi' | 'direct'
  channelId: string
  alias: string
  sourceDetail?: string
  billing?: string
  enabled: boolean
  missing?: boolean
}

export interface ChannelGroupView {
  id: string
  name: string
  description?: string
  enabled: boolean
  modelPatterns?: string[]
  defaultRuntimeChannelId?: string
  sortOrder?: number
  createdAt?: number
  updatedAt?: number
  channels: ChannelGroupChannelEntry[]
  channelCount: number
  enabledChannelCount: number
}

// 旧 v5 candidate 视图（每条 channel 一行），现在挪到 /groups/channels。
export interface AdminGroupView {
  alias: string
  sourceType: 'newapi' | 'direct'
  sourceDetail: string
  billing: string
  status: 'enabled' | 'disabled'
  channelId: string
  runtimeId: string
  route: string
  markup?: number // NewAPI 才有
}

export interface ChannelGroupCreatePayload {
  id: string
  name: string
  description?: string
  enabled?: boolean
  modelPatterns?: string[]
  sortOrder?: number
}

export interface ChannelGroupUpdatePayload {
  name?: string
  description?: string
  enabled?: boolean
  modelPatterns?: string[]
  sortOrder?: number
}

export interface ChannelGroupChannelRef {
  sourceType: 'newapi' | 'direct'
  channelId: string
}

export interface ChannelGroupMembersPayload {
  channels: ChannelGroupChannelRef[]
  defaultRuntimeChannelId?: string
}

// ===== v6 ChannelGroup CRUD =====

export async function listChannelGroups(): Promise<ChannelGroupView[]> {
  const r = await api('/groups')
  return (await r.json()) as ChannelGroupView[]
}

export async function getChannelGroup(id: string): Promise<ChannelGroupView> {
  const r = await api(`/groups/${encodeURIComponent(id)}`)
  return (await r.json()) as ChannelGroupView
}

export async function createChannelGroup(payload: ChannelGroupCreatePayload): Promise<ChannelGroupView> {
  const r = await api('/groups', { method: 'POST', body: JSON.stringify(payload) })
  return (await r.json()) as ChannelGroupView
}

export async function updateChannelGroup(id: string, payload: ChannelGroupUpdatePayload): Promise<ChannelGroupView> {
  const r = await api(`/groups/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(payload) })
  return (await r.json()) as ChannelGroupView
}

export async function replaceChannelGroupMembers(id: string, payload: ChannelGroupMembersPayload): Promise<ChannelGroupView> {
  const r = await api(`/groups/${encodeURIComponent(id)}/channels`, { method: 'PUT', body: JSON.stringify(payload) })
  return (await r.json()) as ChannelGroupView
}

export async function deleteChannelGroup(id: string): Promise<void> {
  await api(`/groups/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// ===== 候选 channel 池（admin 在 GroupDrawer 里挑成员用） =====

export async function listGroupCandidateChannels(): Promise<AdminGroupView[]> {
  const r = await api('/groups/channels')
  return (await r.json()) as AdminGroupView[]
}

// ===== 向后兼容（前端仍有地方用 listGroups），等所有调用迁完再删 =====

export async function listGroups(): Promise<AdminGroupView[]> {
  return listGroupCandidateChannels()
}
