// Pinia store · 跨端通知 polling。
//
// User 端：60s 一次 polling /user/api/notifications；放在 UserLayout 顶部 setup 里
// 启动一次 polling，路由切换不影响（store 单例）。
//
// 不写入 sessionStorage，每次刷新页面重新拉一次足够。
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listUserNotifications,
  markRead,
  markDismiss,
  markAllRead,
  type UserNotification,
} from '../api/notifications'

const POLL_MS = 60_000
const TOP_LIMIT = 10

export const useNotificationStore = defineStore('notifications', () => {
  const items = ref<UserNotification[]>([])
  const lastFetchedAt = ref(0)
  const loading = ref(false)
  const error = ref<string>('')
  let timer: ReturnType<typeof setInterval> | null = null

  const unreadCount = computed(() => items.value.filter(n => !n.read).length)
  const hasCritical = computed(() =>
    items.value.some(n => n.level === 'critical' && !n.dismissed),
  )
  const topCritical = computed(() =>
    items.value.find(n => n.level === 'critical' && !n.dismissed) ?? null,
  )

  async function refresh() {
    if (loading.value) return
    loading.value = true
    try {
      const data = await listUserNotifications(TOP_LIMIT)
      items.value = data.items
      lastFetchedAt.value = Date.now()
      error.value = ''
    } catch (e: any) {
      error.value = e?.message || 'refresh failed'
    } finally {
      loading.value = false
    }
  }

  function startPolling() {
    if (timer) return
    refresh()
    timer = setInterval(refresh, POLL_MS)
  }

  function stopPolling() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  async function read(id: string) {
    const item = items.value.find(n => n.id === id)
    if (!item || item.read) return
    try {
      const res = await markRead(id)
      item.read = true
      item.readAt = res.readAt
    } catch (e: any) {
      error.value = e?.message || 'read failed'
    }
  }

  async function dismiss(id: string) {
    const item = items.value.find(n => n.id === id)
    if (!item) return
    try {
      await markDismiss(id)
      item.dismissed = true
      item.read = true
    } catch (e: any) {
      error.value = e?.message || 'dismiss failed'
    }
  }

  async function readAll() {
    try {
      await markAllRead()
      for (const it of items.value) {
        if (!it.read) it.read = true
      }
    } catch (e: any) {
      error.value = e?.message || 'mark-all failed'
    }
  }

  return {
    items,
    lastFetchedAt,
    loading,
    error,
    unreadCount,
    hasCritical,
    topCritical,
    refresh,
    startPolling,
    stopPolling,
    read,
    dismiss,
    readAll,
  }
})
