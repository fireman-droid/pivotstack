// 单位换算 composable —— 全局共享 pivotStackDollarsPerYuan
// 来源（优先级）：
//   1. /user/api/me 响应里的 pivotStackDollarsPerYuan（v7+，用户端默认走这里）
//   2. /admin/api/system/unit-config（admin 端备用）
// 默认 20（与后端 DefaultPivotStackDollarsPerYuan 对齐），避免在还没拿到接口数据时回退到 1:1 误导用户。
import { ref } from 'vue'

const DEFAULT_RATE = 20

const dollarsPerYuan = ref(DEFAULT_RATE)
const loaded = ref(false)

let inflight: Promise<void> | null = null

function setRate(v: unknown) {
  if (typeof v === 'number' && Number.isFinite(v) && v > 0) {
    dollarsPerYuan.value = v
    loaded.value = true
  }
}

// 用户端登录后调用一次 /user/api/me 就能拿到最新 rate；任何组件都可以主动注入。
export function applySystemUnitFromMe(meResp: unknown) {
  if (meResp && typeof meResp === 'object') {
    setRate((meResp as Record<string, unknown>).pivotStackDollarsPerYuan)
  }
}

async function tryLoad() {
  if (loaded.value) return
  if (inflight) return inflight
  inflight = (async () => {
    try {
      const ctrl = new AbortController()
      const t = window.setTimeout(() => ctrl.abort(), 1500)
      // 1) 优先走 user 端：公开给所有登录用户
      const userKey = localStorage.getItem('user_api_key') || ''
      if (userKey) {
        const meRes = await fetch('/user/api/me', {
          headers: { Authorization: `Bearer ${userKey}` },
          signal: ctrl.signal,
        })
        if (meRes.ok) {
          const data = await meRes.json()
          setRate(data?.pivotStackDollarsPerYuan)
        }
      }
      // 2) admin 端备用（已登录 admin 时也能拿到）
      if (!loaded.value) {
        const adminPwd = localStorage.getItem('admin_password') || ''
        if (adminPwd) {
          const headers: Record<string, string> = { 'X-Admin-Password': adminPwd }
          const res = await fetch('/admin/api/system/unit-config', { headers, signal: ctrl.signal })
          if (res.ok) {
            const data = await res.json()
            setRate(data?.pivotStackDollarsPerYuan)
          }
        }
      }
      window.clearTimeout(t)
    } catch {
      /* 静默：保持默认 DEFAULT_RATE */
    } finally {
      loaded.value = true
      inflight = null
    }
  })()
  return inflight
}

export function useSystemUnit() {
  if (!loaded.value && !inflight) tryLoad()
  return {
    dollarsPerYuan,
    /** virtual $ → ¥ */
    toCny: (usd: number) => usd / dollarsPerYuan.value,
    /** ¥ → virtual $ */
    toUsd: (cny: number) => cny * dollarsPerYuan.value,
    formatCny: (usd: number, decimals = 2) => (usd / dollarsPerYuan.value).toFixed(decimals),
    reload: () => { loaded.value = false; return tryLoad() },
  }
}
