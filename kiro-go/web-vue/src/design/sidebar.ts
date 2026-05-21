// PivotStack v6 Admin Sidebar — 按 plan §3.1 daily-frequency 排序，§3.3 URL 结构。
// 全部指 v6 URL。未迁完的页面用 PlaceholderView 顶替（router 配 legacyTo prop）。

import type { Component } from 'vue'
import {
  LayoutDashboard,
  Layers,
  Server,
  Plug,
  CheckCircle2,
  KeyRound,
  Ticket,
  Tag,
  Calculator,
  ScrollText,
  Settings as SettingsIcon,
} from 'lucide-vue-next'

export interface SidebarItem {
  label: string
  to: string
  icon: Component
  /** true: 在此 item 之前画一根 1px divider */
  divider?: boolean
}

export const adminSidebar: SidebarItem[] = [
  // === DASHBOARD（含 insights / leaderboard 通过 ?tab=trend|rank） ===
  { label: '仪表盘', to: '/overview', icon: LayoutDashboard },

  // === CHANNELS ===
  { label: '分组总览', to: '/channels',           icon: Layers,       divider: true },
  { label: 'NewAPI 上游', to: '/channels/newapi', icon: Server },
  { label: '自营直连',   to: '/channels/direct',  icon: Plug },
  { label: '对账监控',   to: '/channels/reconcile', icon: CheckCircle2 },

  // === SALES & BILLING ===
  { label: 'API Key',  to: '/billing/keys',    icon: KeyRound,    divider: true },
  { label: '激活码',   to: '/billing/codes',   icon: Ticket },
  { label: '单位换算', to: '/billing/unit',    icon: Calculator },

  // === OPS ===
  { label: '调用日志', to: '/ops/call-logs', icon: ScrollText, divider: true },
  { label: '经营看板', to: '/ops/business-board', icon: Tag },
  // 「API 接入说明」已迁到 user 端（/user/api-docs），admin 无需该入口

  // === SYSTEM ===
  { label: '系统设置', to: '/system/settings', icon: SettingsIcon, divider: true },
]
