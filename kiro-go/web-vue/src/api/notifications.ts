// User & admin notification API。
//
// User 端走 Bearer + /user/api/notifications；admin 端复用 admin api 封装。
import { api as adminApi } from './admin'

const USER_BASE = '/user/api/notifications'

function userKey(): string {
  return localStorage.getItem('user_api_key') || sessionStorage.getItem('user_api_key') || ''
}

async function userFetch(path: string, opts: { method?: string; body?: unknown } = {}) {
  const { method = 'GET', body } = opts
  const headers: Record<string, string> = { Authorization: `Bearer ${userKey()}` }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const res = await fetch(USER_BASE + path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const d = await res.clone().json()
      if (d?.error) msg = d.error
    } catch {}
    throw new Error(msg)
  }
  return res.json()
}

// ───────── User types ─────────
export interface UserNotification {
  id: string
  title: string
  body: string
  level: 'info' | 'warn' | 'critical'
  publishAt?: number
  expireAt?: number
  dismissible: boolean
  read: boolean
  dismissed: boolean
  readAt?: number
}
export interface UserListResult {
  unreadCount: number
  items: UserNotification[]
}

export async function listUserNotifications(limit = 10): Promise<UserListResult> {
  return userFetch(`?limit=${limit}`)
}
export async function markRead(id: string): Promise<{ success: true; readAt: number }> {
  return userFetch(`/${encodeURIComponent(id)}/read`, { method: 'POST' })
}
export async function markDismiss(id: string): Promise<{ success: true; dismissedAt: number }> {
  return userFetch(`/${encodeURIComponent(id)}/dismiss`, { method: 'POST' })
}
export async function markAllRead(): Promise<{ success: true; marked: number }> {
  return userFetch('/read-all', { method: 'POST' })
}

// ───────── Admin types ─────────
export interface AdminNotification {
  id: string
  title: string
  body: string
  level: 'info' | 'warn' | 'critical'
  targetType: 'all' | 'plan' | 'group' | 'userIds'
  targetValue?: string[]
  status: 'draft' | 'published'
  publishAt?: number
  expireAt?: number
  dismissible: boolean
  createdAt: number
  updatedAt?: number
  createdBy?: string
  updatedBy?: string
  deletedAt?: number
}
export interface AdminStats {
  notificationId: string
  targetCount: number
  readCount: number
  dismissedCount: number
  unreadCount: number
}
export interface AdminItem {
  notification: AdminNotification
  stats: AdminStats
}
export interface AdminListResult {
  items: AdminItem[]
  total: number
}

export interface NotificationInput {
  title: string
  body: string
  level: 'info' | 'warn' | 'critical'
  targetType: 'all' | 'plan' | 'group' | 'userIds'
  targetValue?: string[]
  status: 'draft' | 'published'
  publishAt?: number
  expireAt?: number
  dismissible: boolean
}

export async function adminListNotifications(
  status = 'all',
  limit = 50,
  offset = 0,
): Promise<AdminListResult> {
  const qs = new URLSearchParams({
    status,
    limit: String(limit),
    offset: String(offset),
  })
  const r = await adminApi(`/notifications?${qs}`)
  return r.json()
}

export async function adminCreateNotification(input: NotificationInput): Promise<AdminNotification> {
  const r = await adminApi('/notifications', { method: 'POST', body: JSON.stringify(input) })
  return r.json()
}

export async function adminUpdateNotification(
  id: string,
  input: NotificationInput,
): Promise<AdminNotification> {
  const r = await adminApi(`/notifications/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
  return r.json()
}

export async function adminDeleteNotification(id: string): Promise<{ success: true }> {
  const r = await adminApi(`/notifications/${encodeURIComponent(id)}`, { method: 'DELETE' })
  return r.json()
}

export async function adminGetNotificationStats(id: string): Promise<AdminStats> {
  const r = await adminApi(`/notifications/${encodeURIComponent(id)}/stats`)
  return r.json()
}
