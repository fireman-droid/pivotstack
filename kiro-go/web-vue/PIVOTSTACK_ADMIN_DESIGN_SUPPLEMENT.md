# PivotStack Admin v2 增量改稿单（给 claude-design）

**对象**：`pivotstack-admin.html`（现有 prototype）
**原则**：**增量改 / 增量补 / 不重写**

---

## 0. 给你的执行守则（必读）

PivotStack 本地项目**功能完整、逻辑稳定**——这次找你不是改功能，是**改排版/视觉 + 补漏画的页面**。

### 0.1 必须做

1. 在现有 `pivotstack-admin.html` 内部**增量编辑**
2. 保留现有的 `data-view-panel` 切换机制 + `ADMIN_TREE` + `ROUTE_TO_VIEW` + `switchAdminView(route)`
3. 保留现有的黑底视觉 token、ECharts stellar 主题、表格密度、4 大块已实现 UI
4. 删除一切 prototype 演示元素（见 §6）

### 0.2 严禁做

1. **不要重写整份 HTML**
2. **不要推翻已实现的 4 大块**（Login Warp / Notification 4 子页 / Overview / BillingKeys）
3. **不要再造**"消费溯源"、"用户 360"、"运营分群"这些 v1 编造的视图
4. **不要发明业务**——所有字段必须用 §9 的后端字段表
5. **不要把 Billing 拆成 8 子菜单**——只要 5 个

---

## 1. 概念锁定（最重要，必读）

v1 把三个概念搞混了，v2 必须锁死：

| 概念 | 数据源 | 本质 | 对应路由 | 必显字段 | 视觉差异 |
|---|---|---|---|---|---|
| **调用日志** | `CallLog` / `data/call_logs.jsonl` | **出账记录** = 每次 API 请求扣费明细 | `/ops/call-logs`（admin 全平台）<br>`/billing/keys/:id` 调用日志 Tab<br>`/user/logs`（user 自己的） | time / request_id / api_type / original_model / actual_model / api_key_id / channel_alias / total_tokens / paid_credits / gifted_credits / cost_usd / status / error / duration_ms / billing_mode / billing_status | 运维排障表。高密度（36px 行高）+ status chip + 详情 Drawer。重点：request_id / 错误 / 耗时 / token / 成本拆解 |
| **充值流水** | `RechargeRecord` / `data/recharge_records.jsonl` | **入账记录** = 用户用兑换码或 admin 调余额生成的流水 | `/billing/recharges`（admin 全平台 ⚠️ 新增）<br>`/billing/keys/:id` 充值流水 Tab<br>`/user/recharge` 充值历史 | time / key_id / key_note / type (`code_redeem` \| `code_redeem_days` \| `admin_balance` \| `admin_gift` \| `admin_adjust`) / code / amount_usd / amount_cny / balance_before / balance_after / gift_before / gift_after / operator / note / ip | 财务流水表。正向金额（绿色入账，橙色调整）+ 余额 before→after + operator + IP |
| **激活码** | `ActivationCode` / codes 票根池 | **库存**（未兑换前不是流水） | `/billing/codes` | code / type (`balance` \| `days` \| `time`) / amount / salePriceCNY / used / createdAt / batch / note | 票根池。未用/已用 Tab + ticket 风格 + code mono。**不要**显示 balance before/after |

### 1.1 概念禁令

- ❌ **不要画**独立"消费溯源"页面 → 它就是 `/ops/call-logs` 的详情 Drawer
- ❌ **不要把** `/billing/codes` 画成充值流水
- ❌ **不要把** `/billing/recharges` 画成激活码列表
- ❌ **不要把** `/billing/keys` 改名为"用户 360"

---

## 2. 视觉规范（必须延续 v1 + 补强）

### 2.1 色彩

- **背景**: 主背景 `#000` / 卡片 `#0a0a0a` / 悬浮表面 `#141414`
- **边框**: 基础 `rgba(255,255,255,0.08)` / 强调 `rgba(255,255,255,0.15)`
- **文本**: 主 `#ededed` / 次 `#a1a1a1` / 弱 `#707070`
- **品牌绿**: `#0bd470` / 弱化背景 `rgba(11,212,112,0.10)`
- **状态色**: success `#0bd470` / warn `#f5a623` / error `#ff4d4f` / info `#3b82f6`

### 2.2 字体（Geist 字体族）

| 字号 | 用途 | 字重 |
|---|---|---|
| `11px` Mono | ID / Token 尾号 / 时间戳 | 400 |
| `12px` | Meta 信息 / 表头 / 次要标签 | 500 |
| `13px` | UI 标准（表格/表单默认） | 400 |
| `14px` | 正文段落 | 400 |
| `16px` | 卡片标题 / 模块小标题 | 500 |
| `18px` | PageHead 次级标题 / 弹窗标题 | 600 |
| `24px` | PageHead 主标题 / KPI 数字 | 600 |
| `32px` | 大盘指标 / Hero 文本 | 600 |

数字一律 `font-variant-numeric: tabular-nums`。

### 2.3 间距（4pt grid，禁止 15/21 等随意值）

- `4px / 8px`: 元素内间距（icon + 文字、Tag 内边距）
- `12px / 16px`: 组件间距（List Item / 表单字段 / 默认 padding）
- `24px`: 模块级间距（Card padding / Section 之间）
- `32px / 48px`: 页面级布局间距

### 2.4 圆角阶

- `3px`: Checkbox / 小 Chip
- `4px`: Button / Input / Select
- `6px`: 内部数据卡 / 图表容器
- `8px`: 外层主卡 / Modal / Drawer

### 2.5 玻璃磨砂（仅 Popover/Modal/Notification Dropdown）

```css
background: rgba(10, 10, 10, 0.65);
backdrop-filter: blur(16px);
-webkit-backdrop-filter: blur(16px);
border: 1px solid rgba(255, 255, 255, 0.10);
```

### 2.6 三种小标签的差异

| 类型 | 视觉 | 用例 |
|---|---|---|
| **Chip** | 实色极暗背景 + 细边框 | 模型名 `gpt-4o` / 渠道名 |
| **Badge** | 带状态色微光背景 + 无边框 | Status `ACTIVE`（绿色微光）/ `EXPIRED`（灰色） |
| **Tag** | 纯线框 / 空心 | 用户手工打标签 |

### 2.7 表格风格

- **不要 striped**，用 hover-only（hover 背景 `#141414`）
- 普通行高 `44px`；密集数据（调用日志）行高 `36px`
- 表头加粗 + 全大写 + 字号 `12px`
- 第一列固定 checkbox，最后一列固定操作（quaternary 按钮组）
- 长文本 `text-overflow: ellipsis` + tooltip
- ID/请求号用 `11px Mono`

### 2.8 按钮变体

| 变体 | 视觉 | hover |
|---|---|---|
| `primary` | `#0bd470` 底 + `#000` 字 | 亮度提升 |
| `secondary` | `#1f1f1f` 底 + `rgba(255,255,255,0.10)` 边框 | `#2a2a2a` |
| `ghost` | 透明 + 无边框 | `#1f1f1f` 浅底 |
| `quaternary` | 仅文字 / icon | 文字色变 |
| `icon-only` | 28×28 或 32×32 正方形 | 浅灰底 |

### 2.9 禁忌

- ❌ 紫色渐变
- ❌ 霓虹大光晕
- ❌ hover 纯白底（用 `rgba(255,255,255,0.06)` 灰）
- ❌ 营销 banner
- ❌ 卡片套卡片
- ❌ admin 页面用 32px 以上 hero 字号

---

## 3. 复用组件清单（design 只画一次，全局复用）

| 组件 | 结构 |
|---|---|
| **PageHead** | kicker（小灰字）+ Title（24px）+ Description + 右侧 Actions（按钮组）。下边距 24px |
| **MetricStrip** | 4 列 grid。每 Tile: Label（12px 灰）+ Value（24px）+ 可选 sparkline / 同环比 chip |
| **FilterBar** | 左：Search input（带 🔍 icon）+ 多个 Select；右：高级过滤 / 视图切换。紧贴表格上方 |
| **BulkActionBar** | 选中数 > 0 时从底部滑出或原地替换 FilterBar；显示"已选 N 项" + 批量动作按钮 |
| **AdminTable** | 第一列 checkbox 固定，最后列操作固定；selection + sort + ellipsis tooltip |
| **DetailDrawer** | 右侧划出，宽 ~600px，`#0a0a0a` 底。顶部大标题 + X 关闭；内容分 Section + 左右两栏 label-value |
| **LogRowExpand** | 表格行下方展开，`#111` 深灰区块，展示 Cost 拆解（prompt/completion 占比条）+ 请求摘要 + Trace ID 复制 |
| **CopyableMono** | 灰底圆角框 + Mono 字体，hover 右侧浮现 Copy icon |
| **LevelChip** | info（蓝微光）/ warn（橙微光）/ critical（红微光 pulse）/ success（绿微光）/ idle（灰） |
| **StatusBadge** | success（绿）/ error（红）/ warn（橙）/ pending（橙）/ disabled（灰） |

---

## 4. 保留 / 改 / 补 / 删 矩阵

### 4.1 保留（v1 已画且符合本地实际，仅做微调）

| 模块 | v1 内容 | 保留方式 | 微调点 |
|---|---|---|---|
| Login Warp | 曲速隧道 + 绿色 L 角框 + 简洁卡 | 唯一登录视觉方向 | **删** Singularity/Boot 切换器；**加** 用户名/密码登录 + 邀请码注册入口 |
| Notification Admin | 通知列表 + 创建 modal + targeting + stats drawer | 保留 | 字段对齐 `Notification`；**删** Lifecycle 图 |
| User Bell | bell badge + dropdown + modal | 保留 | critical 只 pulse 不自动弹 |
| User 通知中心 | 列表 | 保留 | 确认为 `/user/notifications` 路由 |
| Critical Banner | user/admin 顶部 sticky | 保留 | 仅 critical + published + 未过期才显示 |
| Admin Overview | KPI + 图表 + 热力 + 排行 + 异常 | 保留 | 文案对齐 `/admin/api/stats`；数字 mono |
| BillingKeys | metric strip + filter + bulk + table | 保留 | **标题改回**"API Key 管理"；**加** 详情入口 |

### 4.2 改

| 现状问题 | 改成 | 设计要求 |
|---|---|---|
| "用户 360" 作为 Billing 主入口 | `/billing/keys` · **API Key 管理** | 标题/kicker 必须是 "API Key 管理"。用户画像放 KeyDetail，不放列表 |
| Billing 拆 8 子菜单 | **5 个**：API Key 管理 / 充值流水 / 激活码 / 定价中心 / 单位换算 | Tree 只保留这 5 个 |
| "消费溯源" 独立视图 | `/ops/call-logs` 详情 Drawer | Cost breakdown 放进 drawer |
| 激活码混成充值流水 | `/billing/codes` 只做码池 | 未用/已用 Tab + ticket 风格 |
| KeyDetail 太薄 | `/billing/keys/:id` 强化 | hero 4 卡 + 自动 tags + overview charts + logs + recharges |

### 4.3 补（本地有但 v1 没画的 16 个 view）—— 详见 §5

### 4.4 删（演示性元素）

| 元素 | 删除原因 |
|---|---|
| 顶部 Mega-tab (Login / Admin Console / Notification 系统) | prototype 演示壳，不是产品 IA |
| Login 三方向切换器（Singularity / Boot / Warp） | 已选 Warp |
| Notification Lifecycle 图 | 文档级，不入页面 |
| "PROTOTYPE" badge | 生产视觉不应出现 |
| "CREATIVE DIRECTION" 标签 | 设计评审残留 |
| "消费溯源" 独立入口 | 概念错误 |

注意：删除的是**可见 UI**，不是 JS helper。`switchAdminView` / `ADMIN_TREE` 等保留并扩展。

---

## 5. 必补 view 逐项要求

### 5.1 调用日志 `/ops/call-logs` ⭐ admin 运维核心

- **PageHead**: kicker "运营" / title "调用日志" / desc "共 N 条 · 最近 200 条"
- **FilterBar**: 搜索（request_id / key / channel / model / error） · 状态下拉（成功/失败） · 时间范围 · billing_status · channel
- **Table（密集 36px 行高）**:
  - Timestamp (11px Mono 弱化)
  - Entity（Key note + id 灰）
  - Model (Chip)
  - Tokens (P:120 / C:45 / T:165 紧凑展示)
  - Latency（如 `1.2s`，超 3s 标黄）
  - Cost（Mono `$0.0045`，对齐）
  - Status (200 绿 / 4xx 黄 / 5xx 红)
  - 操作 [...]
- **详情 Drawer（左右两栏 label-value）**:
  - 顶部 hero: StatusBadge + 时间 + request_id + api_type
  - **Key/账号**: Key note + key id + 上游账号
  - **渠道**: alias / id / type
  - **模型**: original / actual / price_model
  - **Token**: input / output / total
  - **Cost 拆解**: paid_credits / gifted_credits / total / cost_usd / billing_mode chip (token/subscription/legacy_credits) / billing_status chip (paid 绿 / free 灰 / estimated 橙)
  - **元信息**: duration / stream / stop_reason
  - **error 文本**
  - **复制原始 JSON** 按钮
- **API**: `GET /admin/api/logs?search=&limit=200`

### 5.2 充值流水 `/billing/recharges` ⭐ admin 财务核心

- **PageHead**: kicker "销售 & 计费" / title "充值流水" / desc "全平台入账记录"
- **MetricStrip 4 卡**: 今日入账 / 本月入账 / 平均充值额 / 复充率
- **FilterBar**: 搜索 code 或 key note · 类型多选 · operator · 金额范围 · 时间范围
- **Table 列**:
  - Txn ID (Mono Copyable)
  - Time
  - User/Account（key note + id 灰，链接到 KeyDetail）
  - Amount（绿色 `+ $50.00`，显著字重）
  - Method (Chip：`code_redeem` / `admin_balance` / `admin_gift` / `admin_adjust`)
  - Code (Mono，仅 code_redeem 类型显示)
  - Balance After
  - Operator (user / admin)
  - IP
- **详情 Drawer**: 完整 `RechargeRecord` JSON 展开 + 复制按钮
- **API**: `GET /admin/api/recharges` ⚠️ 后端待补，design 先按字段出图

### 5.3 API Key 详情强化 `/billing/keys/:id` ⭐ 用户画像核心

- **Hero**: 大字号 Key Name + 自动 tags chip 区（🔥活跃 / 💎VIP / 💤沉睡 / 🔄回头客）+ 右侧停用/删除按钮
- **Hero 4 卡（保留）**: 余额 ¥ / 套餐 / 启用 switch / 累计请求
- **Key 字符串卡（保留）**: CopyableMono
- **Tabs**: 概览 / 调用日志 / 充值流水
- **概览 Tab 新增**:
  - **模块 A（左上）**: 7d 消耗趋势 sparkline（黑底 + 绿色面积线）
  - **模块 B（右上）**: 模型调用占比 donut（如 GPT-4 60% / Claude 30%）+ 极简图例
  - **模块 C（下方）**: 24h 调用时段热力图（不同亮度绿方块）
  - **常用模型 top3** + **最爱渠道**
  - 现有 kv 行（累计 Token / Credits / 代理商 / 父 Key / 创建 / 最近调用）保留
- **调用日志 Tab**: 顶部 mini stats + 表格复用 `/ops/call-logs` 子集；点行打开同款 drawer
- **充值流水 Tab**: 顶部 mini bar chart + 表格复用 RechargeRecord 子集
- **API**: `/admin/api/apikeys`、`/apikeys/:id/logs`、`/apikeys/:id/recharges`

### 5.4 登录策略 `/system/auth` ⭐ v6 新功能

- **PageHead**: kicker "系统 · 认证" / title "登录策略"
- **Section 1: Policy Toggles**（列表式，每行 icon + Title 粗体 + 灰字描述 + 右侧 Switch）:
  - API Key 登录
  - 用户名/密码登录
  - 自助注册
  - 注册必须邀请码
- **Section 2: 邀请码管理**:
  - 小型 FilterBar + AdminTable
  - 列：code (Copyable Mono 大字) / 配额 (usedCount / maxUses Badge) / expiresAt / createdBy / note / disabled
- **创建邀请码 Drawer**: maxUses / expiresAt / note / generate count
- **API**: `/admin/api/users/policy`、`/admin/api/invite-codes`

### 5.5 用户管理 `/system/users` ⭐ v6 新功能

- **PageHead**: kicker "系统" / title "用户管理" / desc "User 实体，区别于 API Key"
- **Table 列**:
  - User ID (Mono)
  - Email / Username（强调）
  - Bound Keys（绑定 key 数 Badge）
  - Balance（总余额，跨 keys 合计）
  - Invited By
  - Last Login
  - Created
  - Disabled Switch
- **详情 Drawer**:
  - Profile block
  - Bound Keys table（小表，跨链到 KeyDetail）
  - Invite metadata
  - Last login
  - Actions（禁用 / 设默认 Key / 充值快捷弹窗）
- **API**: `/admin/api/users`、`/admin/api/users/:id/disable|enable`

### 5.6 用户通知中心 `/user/notifications` ⭐ v6 新功能

- **PageHead**: kicker "用户" / title "通知中心"
- **布局**: 邮箱客户端式双栏（左侧 sidebar + 右侧列表）
- **左侧 Sidebar**: filter chip + 未读数 Badge:
  - 全部
  - 未读 (N)
  - 已读
  - 按级别筛选下拉
- **右侧 List**: 宽卡片
  - level chip + 标题加粗 + 时间右对齐
  - body excerpt
  - 未读条目左侧绿点
- **点条目**: 打开 NotifModal（复用现有）
- **顶部**: "全部标已读" 按钮
- **API**: `/user/api/notifications`

### 5.7 其他必补 view（标准模板）

| view | kicker / title | 必有字段 | 必有操作 | 视觉重点 |
|---|---|---|---|---|
| `/channels` 分组管理 | 渠道 / 分组管理 | id / name / description / members count / defaultChannelID / createdAt | 新建/编辑/删除/进详情 | 路由分组，不是 provider；行内 member 数 + default chip |
| `/channels/groups/:id` 分组详情 | 渠道 · 分组 / 分组详情 | meta + members + defaultChannelID | 添加/移除渠道 + 设默认 | 左 meta card + 右 mounted table；默认 channel 绿色 DEFAULT chip |
| `/channels/newapi` NewAPI 列表 | 渠道 · NewAPI / NewAPI 上游 | alias / baseURL / token count / userID / status / lastSyncAt | 新建/同步/删除/进详情 | status dot + baseURL mono |
| `/channels/newapi/:id` NewAPI 详情 | 渠道 · NewAPI · 详情 / 上游详情 | meta + tokens + models | 重新同步/拉模型/同步定价/手动加 token/物化 | hero meta + Tabs（概览/Token/同步记录/定价映射）；token 表高密度 |
| `/channels/direct` 自营直连 | 渠道 · 自营 / 自营直连 | type / alias / baseURL / 模型数 / 售价摘要 / enabled | 新建 openai/编辑/删除/健康检查 | **内建 Kiro 置顶 + BUILTIN lock chip 不可删** |
| `/channels/direct/:id` 自营详情 | 渠道 · 自营 · 详情 | meta + per-model in/out 售价 + recent logs | 改 baseURL/enabled/单模型售价/健康检查 | hero + Tabs（概览/模型售价/调用日志） |
| `/channels/reconcile` 对账 | 渠道 · 对账 / 上游对账 | time / provider / upstream quota / local estimated / delta / status | 重试 reconcile/展开详情/导出 | 上下分割：上对比图（差异条形），下明细 table，差额 +/- 绿红但**不要整行红底** |
| `/billing/codes` 激活码 | 销售 & 计费 / 激活码 | code / type / amount / salePriceCNY / used / batch / note | 批量生成/复制/导出/删除 | Tab 未用/已用；code Mono；ticket 风格；**绝不显示 balance** |
| `/billing/pricing` 定价中心 | 销售 & 计费 / 定价中心 | model / input price / output price / channel override / updatedAt | 改单价/批量/覆盖渠道 | 矩阵表 IN/OUT 双列；价格 Mono；inline edit |
| `/billing/unit` 单位换算 | 销售 & 计费 / 单位换算 | current ratio / new ratio / rebalance / admin password | 保存（二次确认） | 表单 + 橙色 warn strip（高风险）+ history timeline |
| `/ops/abuse` Abuse 监控 | 运营 / Abuse 监控 | event time / severity / api_key_id / rule / count / window / status | 一键封禁/清标/查相关日志 | severity chip warn/error；**不要整行红底** |
| `/ops/profit` 计费分析 | 运营 / 计费分析 | revenue / cost / gross profit / margin + 切片 | 时间范围/切片/导出 | **三层 KPI**：用户收入 / 上游成本 / 毛利 + 趋势图 |
| `/system/settings` 系统设置 | 系统 / 系统设置 | thinking 后缀 / debug / proxy / 默认 plan 等 | 保存/重置/敏感项二次确认 | sectioned form + hairline divider；**不卡片套卡片** |
| `/system/experimental` 实验功能 | 系统 · 实验 / 实验功能 | feature flag / enabled / scope / note | 开关/保存/查看影响范围 | collapse sections + switch table；橙色 `EXPERIMENTAL` chip |
| `/system/accounts` Kiro 账号池 | 系统 / Kiro 账号池 | alias / region / status / lastRefresh / weight / enabled | 新建（SSO/BuilderID）/ 刷新 token / 批量导入凭证 | 账号池≠用户账号；status dot |
| `/reseller` 代理总览 | 代理商 / 代理商总览 | note / key / balance / 子 key 数 / 7d / revenue | 查看子 Key / 导出 / 调整额度 | 复用 BillingKeys；代理层级 chip |
| `/reseller/keys/:id` 子 Key 池 | 代理商 · 子 KEY / 子 Key 池 | note / key / balance / requests / 7d / expiresAt / enabled | 启用/禁用/删/导出/详情 | parent key hero + child key table |

---

## 6. 应该删的演示元素清单（汇总）

| 元素 | 位置 | 删除原因 | 替代 |
|---|---|---|---|
| Mega-tab | 顶部 (Login / Admin Console / Notification 系统) | prototype 演示壳 | 实际 route / rail tree |
| Login direction switcher | Login 上方 | 已选 Warp | 保留 Warp 单页 |
| Lifecycle diagram | Notification | 文档级 | 状态 chip + 操作即可 |
| "CREATIVE DIRECTION" 标签 | 各处 | 设计评审残留 | 无 |
| "PROTOTYPE" badge | 各处 | 生产视觉 | 无 |
| 独立"消费溯源"入口 | Billing | 概念错误 | `/ops/call-logs` Drawer |

---

## 7. Login 页面具体修改要求

保留 Warp 视觉。卡片内容**新增**：

- **登录 Tab**:
  - API Key 登录（保留）
  - 用户名/密码登录（新增）
- **注册入口**（卡片底部链接 "立即注册"）→ 切换为注册表单：
  - invite code（input + 红描边校验态）
  - email
  - username（可选）
  - password
  - confirm password

**注意**：
- 邀请码无效 → input 红描边 + 错误文案
- 注册成功 → 跳 dashboard 或提示绑定 key
- 账号 disabled → 显示停用提示
- **不要恢复** Singularity / Boot 方向

---

## 8. Admin 端 Critical Banner 位置

User 端已经在顶部 nav 下 sticky。Admin 端**同样**需要：

- AdminLayoutRail 顶部全宽（rail + tree + main 之上）
- 仅当有未读 critical 通知时显示
- 红色微光 + level chip pulse + dismissible（取决于 Notification.dismissible 字段）

---

## 9. 后端字段对照表（design 视觉时取字段用，不要自创）

```go
// CallLog（/admin/api/logs）
type CallLog struct {
  Time, RequestID, APIType, OriginalModel, ActualModel, Account, ApiKeyID string
  InputTokens, OutputTokens, TotalTokens int
  PaidCredits, GiftedCredits, CostUSD, UpstreamCredits float64
  Status, Error, StopReason, PriceModel string
  ChannelID, ChannelAlias, ChannelType string
  BillingMode  // "token" | "subscription" | "legacy_credits"
  BillingStatus // "paid" | "free" | "estimated"
  Stream bool
  DurationMs int64
}

// RechargeRecord（/admin/api/recharges 待补；/user/api/recharges 已有 per-key 版本）
type RechargeRecord struct {
  Time string; Timestamp int64
  KeyID, KeyNote string
  Type  // "code_redeem" | "code_redeem_days" | "admin_balance" | "admin_gift" | "admin_adjust"
  Code string
  AmountUSD, AmountCNY, BalanceBefore, BalanceAfter, GiftBefore, GiftAfter float64
  Operator  // "user" | "admin"
  Note, IP string
}

// ActivationCode（/admin/api/codes）
type ActivationCode struct {
  Code string
  Type // "balance" | "days" | "time"
  Amount float64
  SalePriceCNY float64
  Used bool
  CreatedAt int64
  Batch, Note string
}

// ApiKeyInfo（/admin/api/apikeys）
type ApiKeyInfo struct {
  ID, Key, Note, Plan, Tier string
  Balance, GiftBalance, TotalRecharged, TotalGifted, Credits float64
  CreatedAt, LastUsed, ExpiresAt int64
  Requests, Errors, Tokens int64
  Enabled, IsReseller bool
  ParentKeyID string
  ChannelPreferences map[string]string
}

// User（v6 新；/admin/api/users）
type User struct {
  ID, Email, Username, PasswordHash string
  ApiKeyIDs []string
  DefaultKeyID, InvitedBy, InviterUserID string
  CreatedAt, LastLoginAt int64
  Disabled bool
}

// InviteCode（v6 新；/admin/api/invite-codes）
type InviteCode struct {
  Code, Note, CreatedBy string
  MaxUses, UsedCount int
  CreatedAt, ExpiresAt int64
  Disabled bool
}

// Notification（已实现）
type Notification struct {
  ID, Title, Body string
  Level // "info" | "warn" | "critical"
  TargetType // "all" | "plan" | "group" | "userIds"
  TargetValue []string
  Status // "draft" | "published"
  PublishAt, ExpireAt, CreatedAt int64
  Dismissible bool
}

// ChannelGroup（/admin/api/channel-groups）
type ChannelGroup struct {
  ID, Name, Description string
  Members []GroupMember
  DefaultChannelID string
}
```

---

## 10. API 端点对照表

| View | Endpoint |
|---|---|
| `/overview` | `/admin/api/stats`（聚合用 `/admin/api/logs`） |
| `/channels` | `/admin/api/channel-groups` |
| `/channels/groups/:id` | `/admin/api/channel-groups` |
| `/channels/newapi` | `/admin/api/providers` |
| `/channels/newapi/:id` | `/admin/api/providers`、`/admin/api/newapi/channels` |
| `/channels/direct` | `/admin/api/channels` |
| `/channels/direct/:id` | `/admin/api/channels`、`/admin/api/logs` |
| `/channels/reconcile` | `/admin/api/newapi/reconcile-status` |
| `/billing/keys` | `/admin/api/apikeys` |
| `/billing/keys/:id` | `/admin/api/apikeys`、`/apikeys/:id/logs`、`/apikeys/:id/recharges` |
| `/billing/recharges` ⚠️ | `/admin/api/recharges`（后端待补） |
| `/billing/codes` | `/admin/api/codes` |
| `/billing/pricing` | `/admin/api/sell-prices` |
| `/billing/unit` | `/admin/api/system/unit-config` |
| `/ops/call-logs` | `/admin/api/logs` |
| `/ops/abuse` | `/admin/api/abuse` |
| `/ops/profit` | `/admin/api/profit` |
| `/system/notifications` | `/admin/api/notifications` |
| `/system/auth` | `/admin/api/users/policy`、`/admin/api/invite-codes` |
| `/system/users` | `/admin/api/users` |
| `/system/accounts` | `/admin/api/accounts` |
| `/user/notifications` | `/user/api/notifications` |

---

## 11. Action Items（claude-design 交付清单）

1. **删除可见演示元素**：Mega-tab / Login direction switcher / Lifecycle 图 / "PROTOTYPE" badge / "CREATIVE DIRECTION" 标签
2. **Login Warp 单页**：删除方向切换器；卡片内增加 用户名密码登录 + 邀请码注册
3. **Billing tree 改为 5 项**：API Key 管理 / 充值流水 / 激活码 / 定价中心 / 单位换算
4. **BillingKeys 标题改回**："API Key 管理"
5. **新增 `/ops/call-logs` panel**，作为 Ops 第一入口（密集行高 36px + 详情 Drawer）
6. **新增 `/billing/recharges` panel**（全平台充值流水，按 RechargeRecord 字段出图）
7. **强化 `/billing/keys/:id` KeyDetail**（hero 大字 + tags + 概览 Tab 加 sparkline/donut/heatmap）
8. **新增 `/system/auth`**（policy toggles + 邀请码管理）
9. **新增 `/system/users`**（用户实体管理，强调≠API Key）
10. **新增 `/user/notifications`**（双栏布局：sidebar filter + list）
11. **补齐所有 channels / ops / system / reseller 缺失 panels**（按 §5.7 标准模板）
12. **扩展 `ROUTE_TO_VIEW`**，每个 tree route 切到真实 `data-view-panel`，不再 toast stub
13. **Admin Critical Banner** 装到 AdminLayoutRail 顶部（与 user 端一致）
14. **最终自检**：
    - 所有 view 都有 PageHead
    - 所有列表都有 FilterBar
    - 所有大表都有 BulkActionBar
    - 所有详情用 Drawer 或 Tabs
    - 不发明新业务概念
    - 字段名严格按 §9 后端 struct

---

## 12. 占位数据要求

填具有**真实业务感**的数据：

- 模型：`claude-3-opus-20240229` / `gpt-4o-2024-08-06` / `gemini-2.5-pro-preview`
- 错误：`429 Too Many Requests - Upstream quota exhausted` / `503 Upstream timeout`
- Key note：玉米地大佬 / agent-team-a / client-acme
- 充值类型：code_redeem / admin_balance / admin_gift
- IP：`192.168.1.42` / `10.0.0.15`
- request_id：`req_a8d2f3...`（mono short）

---

## 13. 老用户迁移策略 ⭐（v6 关键决策）

### 13.1 背景

PivotStack 老用户模型 = 一个 API Key 即一个"用户"，无 email/password。v6 引入 `User` 实体后需要**平滑迁移**——零破坏 + 自助升级 + admin 不手动建账号。

### 13.2 升级流程：软强制

| 阶段 | 行为 |
|---|---|
| 老 key 用户用 API Key 登录 | 正常通过（永久兼容，不动 Login 视觉） |
| UserLayout 检测 `userInfo.userId` 为空 | 自动弹出 **绑定账号 Modal** |
| Modal 行为 | 提供 `[立即升级]` / `[暂时跳过]` 二选 |
| 跳过次数 < 3 次 | 显示 `[暂时跳过]` 按钮，localStorage 计数 +1 |
| 跳过次数 ≥ 3 次 | 隐藏跳过按钮，必须升级才能进入 |
| 升级成功 | 创建 `User` 实体绑定当前 key；下次可用 email 或任一 bound key 登录 |
| 已升级的用户 | 不再弹 Modal；可在 `/user/recharge` 旁加"账号设置"入口管理多 key |

**关键性质**：
- API Key **永久不失效**（老 key 永远能登）
- 升级是**加层**不是**换层**（key 还是那个 key）
- email **不发激活邮件**（仅作登录凭证，简化）
- 余额、套餐、调用记录**全部不动**

### 13.3 绑定账号 Modal 视觉规范（新增设计需求）

**复用** `NotifModal` 的 backdrop（玻璃磨砂）+ 容器（`#0a0a0a` 8px 圆角）+ 关闭按钮风格。

**结构**：

```
┌─────────────────────────────────────────────┐
│                                       [X]   │
│  ┌────┐                                     │
│  │ 🔐 │  升级账号                            │
│  └────┘  为您的账号绑定邮箱和密码             │
│                                             │
│  ─────────────────────────────────────────  │
│                                             │
│  EMAIL                                      │
│  ┌─────────────────────────────────────┐   │
│  │ your@email.com                       │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  密码  ≥ 8 位                               │
│  ┌─────────────────────────────────────┐   │
│  │ ••••••••                          👁 │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  确认密码                                    │
│  ┌─────────────────────────────────────┐   │
│  │ ••••••••                          👁 │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  💡 升级后您原有的 API Key 不变，余额不动    │
│  💡 下次可用邮箱+密码或原 API Key 登录       │
│                                             │
│  ─────────────────────────────────────────  │
│                                             │
│  [暂时跳过]                  [立即升级 →]   │
│  (跳过 < 3 次时显示)         (primary 绿色) │
└─────────────────────────────────────────────┘
```

**字段规范**：
- Email: type=email，输入时 trim + lowercase；后端唯一性校验失败时红描边
- 密码 / 确认密码：≥ 8 位前端校验；后端 bcrypt
- 两次密码不一致：实时红色提示

**Modal 出现时机**：
- 老 key 用户登录成功 + `userInfo.userId == null` → 自动弹
- 用户主动从 `/user/recharge` 旁的"升级账号"链接打开（强制升级前）

**Modal 关闭逻辑**：
- 跳过 < 3 次：底部显示 `[暂时跳过]` + 右上角 X 关闭按钮均可
- 跳过 ≥ 3 次：隐藏 `[暂时跳过]` + 右上角 X 也禁用，仅升级按钮可用

### 13.4 后端新增接口（execute 阶段实现）

```go
// POST /user/api/bind-account
// Headers: Authorization: Bearer <api-key>
// Body: { email, password }
// Response: { success: true, userId, email }
//
// 行为：
//   1. 通过 Bearer key 拿到当前 ApiKeyInfo
//   2. 检查该 key 还没绑定过 User（key.id 不在任何 User.apiKeyIds 中）
//   3. 验证 email 全局唯一
//   4. ValidatePassword + bcrypt
//   5. 创建 User { apiKeyIds: [key.id], defaultKeyId: key.id, email, passwordHash }
//   6. 落盘
//   7. 返回 { userId, email }
```

**users 包加函数**：`BindKeyToNewUser(keyID, email, password) (User, error)`
**handler_user_auth.go 加路由**：`handleUserBindAccount`

### 13.5 前端 UserLayout 集成（execute 阶段实现）

```typescript
// UserLayout.vue 新增
import UpgradeAccountModal from '@/components/user/UpgradeAccountModal.vue'

const showUpgradeModal = ref(false)
const upgradeForcedNoSkip = ref(false)

onMounted(async () => {
  await auth.refresh()
  notif.startPolling()
  checkUpgradeNeeded()
})

function checkUpgradeNeeded() {
  // 已绑定 User 的不弹
  if (auth.userInfo?.userId) return
  // 新用户（无 key）不弹
  if (!auth.apiKey) return
  // 老 key 用户：检查跳过次数
  const skipCount = parseInt(localStorage.getItem('upgrade_skip_count') || '0', 10)
  upgradeForcedNoSkip.value = skipCount >= 3
  showUpgradeModal.value = true
}

function onSkip() {
  const n = parseInt(localStorage.getItem('upgrade_skip_count') || '0', 10) + 1
  localStorage.setItem('upgrade_skip_count', String(n))
  showUpgradeModal.value = false
}

function onUpgradeSuccess() {
  localStorage.removeItem('upgrade_skip_count')
  showUpgradeModal.value = false
  auth.refresh() // 重新拉 userInfo（含 userId）
}
```

**新组件** `UpgradeAccountModal.vue`：复用 NotifModal 样式 + form 字段 + `POST /user/api/bind-account` 调用。

### 13.6 admin 端无需手动操作

- 老 key 自动升级 → admin 不用一个个建账号
- admin `/system/users` 仅做：列表查看、禁用问题账号、邀请码管理
- 真正"高频干预"是邀请码生成（仅新用户注册需要）
- 完全自助流程，admin 后台是观察者

### 13.7 极端情况兜底

| 情况 | 处理 |
|---|---|
| 老 key 用户死活不愿升级（清 localStorage 重置计数） | 接受。3 次跳过后强制，但用户仍可清缓存重置。最终通过禁止 key 创建新 child key、不提供新功能等方式自然淘汰 |
| 用户填了 email 但记不住密码 | admin `/system/users` 详情可"重置密码"（生成临时密码）；或加 `POST /user/api/reset-password` 流程（后期） |
| 升级中网络断 | 后端事务保证：要么 User 没建、key 没绑定；要么完整建好。前端失败重试 |
| 升级时 email 被别人占了 | 后端返回 409 + "邮箱已注册"；前端显示在 email input 下方 |

---

## END

> 完成后告诉我 "v2 done"，我会逐 view 拉回去落地。
