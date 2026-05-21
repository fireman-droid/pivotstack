// Admin API · Users / 邀请码 / 登录策略
import { api } from '../admin'

export interface AdminUser {
  id: string
  email: string
  username?: string
  apiKeyIds: string[]
  defaultKeyId: string
  invitedBy?: string
  inviterUserId?: string
  createdAt: number
  lastLoginAt?: number
  disabled?: boolean
}

export interface UserPolicy {
  allowSelfRegister: boolean
  requireActivationCode: boolean
}

export async function listUsers(): Promise<{ users: AdminUser[]; total: number }> {
  return (await api('/users')).json()
}

export async function disableUser(id: string): Promise<void> {
  await api(`/users/${encodeURIComponent(id)}/disable`, { method: 'POST' })
}

export async function enableUser(id: string): Promise<void> {
  await api(`/users/${encodeURIComponent(id)}/enable`, { method: 'POST' })
}

export async function getUserPolicy(): Promise<UserPolicy> {
  return (await api('/users/policy')).json()
}

export async function setUserPolicy(patch: Partial<UserPolicy>): Promise<UserPolicy> {
  return (await api('/users/policy', { method: 'PUT', body: JSON.stringify(patch) })).json()
}

