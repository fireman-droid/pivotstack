// Operations Rail 配置 — v6 IA 单一数据源。
//
// 维护规则：
//   - 所有 rail/tree 信息 **只** 在这里维护，组件不硬编码
//   - rail ≤ 6 个（视觉上 7-8 个会拥挤），底部 bell+profile+logout 单独走 layout
//   - 每个 tree item 至少 1 个；占位路由也照常列出
//   - `to` 必须是 router 已注册路径
//   - `matchPrefixes` 用来识别当前激活的 rail（路径前缀匹配）

import type { Component } from 'vue'
import {
  LayoutDashboard,
  Layers,
  Wallet,
  Activity,
  Settings2,
  Users,
} from 'lucide-vue-next'

/** Tree 子菜单项 */
export interface TreeItem {
  /** 显示文本（中文） */
  label: string
  /** router 绝对路径 */
  to: string
  /** 可选副标题（一行小灰字） */
  hint?: string
  /** 在该项前画一条 divider */
  divider?: boolean
  /** 标记新增功能（视觉上 NEW 角标） */
  isNew?: boolean
}

/** Rail 一级菜单 */
export interface RailItem {
  /** 唯一标识 */
  id: string
  /** Tooltip 文本（英文短） */
  label: string
  /** lucide icon */
  icon: Component
  /** 用于识别激活 rail 的路径前缀 */
  matchPrefixes: string[]
  /** 子菜单列表 */
  tree: TreeItem[]
}

export const adminRail: RailItem[] = [
  // ─── 📊 Dashboard ───
  {
    id: 'dashboard',
    label: 'Dashboard',
    icon: LayoutDashboard,
    matchPrefixes: ['/overview'],
    tree: [
      { label: '总览', to: '/overview', hint: '实时运营 · 60s 刷新' },
    ],
  },

  // ─── 🔌 Channels ───
  {
    id: 'channels',
    label: 'Channels',
    icon: Layers,
    matchPrefixes: ['/channels'],
    tree: [
      { label: '分组管理', to: '/channels', hint: '渠道路由分组' },
      { label: 'NewAPI 上游', to: '/channels/newapi', hint: '聚合网关' },
      { label: '自营直连', to: '/channels/direct', hint: '含内建 Kiro 账号池' },
      { label: '对账监控', to: '/channels/reconcile', hint: '上游账单 vs 本地' },
    ],
  },

  // ─── 💰 Billing ───
  {
    id: 'billing',
    label: 'Billing',
    icon: Wallet,
    matchPrefixes: ['/billing'],
    tree: [
      { label: 'API Key 管理', to: '/billing/keys', hint: '用户与计费单元' },
      { label: '充值流水', to: '/billing/recharges', hint: '全平台入账', isNew: true },
      { label: '激活码', to: '/billing/codes' },
      { label: '单位换算', to: '/billing/unit' },
    ],
  },

  // ─── 📈 Ops ───
  {
    id: 'ops',
    label: 'Ops',
    icon: Activity,
    matchPrefixes: ['/ops'],
    tree: [
      { label: '调用日志', to: '/ops/call-logs', hint: '行级追溯 · 出账' },
      { label: '异常监控', to: '/ops/abuse', hint: 'Abuse + Rate limit' },
      { label: '经营看板', to: '/ops/business-board', hint: '收入 / 成本 / 利润' },
    ],
  },

  // ─── ⚙️ System ───
  {
    id: 'system',
    label: 'System',
    icon: Settings2,
    matchPrefixes: ['/system'],
    tree: [
      { label: '用户管理', to: '/system/users', hint: 'User 实体（≠Key）', isNew: true },
      { label: '登录策略', to: '/system/auth', hint: '注册策略', isNew: true },
      { label: '通知管理', to: '/system/notifications', hint: '跨端发布' },
      { label: '系统设置', to: '/system/settings' },
    ],
  },

  // ─── 👥 Reseller ───
  {
    id: 'reseller',
    label: 'Reseller',
    icon: Users,
    matchPrefixes: ['/reseller'],
    tree: [
      { label: '代理商总览', to: '/reseller', hint: 'admin 视角' },
    ],
  },
]

/** 根据当前路径返回激活的 rail id；找不到回退到首个 rail */
export function resolveActiveRail(path: string): string {
  for (const rail of adminRail) {
    for (const prefix of rail.matchPrefixes) {
      if (path === prefix || path.startsWith(prefix + '/')) {
        return rail.id
      }
    }
  }
  return adminRail[0].id
}

/** 按 id 取 rail，找不到返回首个 */
export function getRailById(id: string): RailItem {
  return adminRail.find(r => r.id === id) ?? adminRail[0]
}
