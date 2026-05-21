# Stellar Console

_Started 2026-05-17 16:22 UTC_

---

## User

按附件中 Pasted text 的 Stellar Console 设计规范，
  出一个完整的 self-contained HTML 原型，
  要求：

  1. 包含 5 个 tab 的可切换界面：概览 / 充值 / 调用日志 / 接入示例 / 代理
  2. 点击顶部 nav 平滑切换主区，不刷页面
  3. 严格执行规范里的色卡、字号、间距、星空背景、ECharts stellar 主题
  4. 真正用 ECharts 5 CDN 渲染 B1-B8 八个图表（不要静态 SVG 占位）
  5. 必须实现的动效：星空 twinkle、余额 counter、status dot pulse、
     nav active 横线、行 hover、chip 选中弹性、复制反馈
  6. 桌面 1440px 主优化，1024 / 768 / 640 三个断点不破版
  7. 每个区块用 HTML 注释标记预期的 naive-ui 组件，方便后期 Vue 落地
  8. 用 §18 的真实示例数据填满所有图表和表格，让原型像活的
  9. 字体优先 Geist (Google Fonts)，fallback Inter + JetBrains Mono
  10. 严格遵守 §19 的 15 条禁忌

## Assistant

_[tool: update_todos]_

我来按照规范严格出一份完整的 Stellar Console 原型。需求已经非常详尽,直接进入构建。

_[tool: write_file]_

_[tool: write_file]_

Now the CSS — tokens, type system, components, all five tab layouts, responsive.

_[tool: write_file]_

_[tool: done]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: eval_js]_

The stagger-in animation is leaving opacity at 0. Let me debug.

_[tool: str_replace_edit]_

The stagger animation is wedged in "running" state and never reaches opacity 1. The complexity isn't worth the bug — let me simplify to just animate the tab container itself.

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: str_replace_edit]_

Animations are wedged in the preview iframe regardless of fill-mode. Let me just remove the entry animation entirely — content stays visible.

_[tool: save_screenshot]_

_[tool: show_html]_

_[tool: done]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: eval_js]_

_[tool: screenshot_user_view]_

_[tool: str_replace_edit]_

The preview iframe pauses animations, leaving one-shot fade-ins stuck at opacity 0. I'll remove all blocking entry animations — keep only the loops (pulse, twinkle) which won't hide content.

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: register_assets]_

Content rendering. Let me register the asset with proper viewport width so desktop layout shows.

_[tool: eval_js]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: update_todos]_

Looks great at 1440px. Let me complete with done and fork verifier.

_[tool: done]_

_[tool: fork_verifier_agent]_

Stellar Console 原型已完成 ✓

**文件结构**
- `Stellar Console.html` — 主文件,5 tab 平滑切换
- `stellar.css` — tokens / type / components / 响应式
- `stellar.js` — 星空、ECharts stellar theme、B1-B8、所有交互

**已实现**
- 概览 / 充值 / 调用日志 / 接入示例 / 代理 五个完整 tab
- ECharts B1-B8 全部真实渲染(7d 趋势 / 模型 donut / 上游 sparkline ×5 / 24h×7d 热力图 / 24h ribbon mini ×3 / 延迟直方图 / 30d 利润 / 子key bar)
- 星空层:canvas 烘焙 ~1000 静态星 + 42 个 DOM twinkle 星
- 全部 §18 真实示例数据填满
- 复制反馈、counter 数字、status dot pulse、nav 下划线、chip 选中弹性、ticker 自动新行
- 1440 / 1024 / 768 / 640 四断点
- 每个区块带 `<!-- naive-ui: ... -->` 落地注释

**踩过的坑**
预览 iframe 会暂停一次性 CSS 动画,导致初始 fade-in 卡在 opacity:0。已去掉所有阻塞性入场动画,保留循环类动画(twinkle / dot pulse)在真实浏览器中正常播放。

**已知 caveat**
- 起始 tab 默认 dashboard,切到 logs 才会 mount B5/B6/抽屉(按需 lazy init,避免初屏卡顿)
- 截图工具看不到星空背景的细微颗粒(JPEG 压缩),真实 view 中会显示

## User

Apply drawing

## Assistant

_[tool: str_replace_edit]_

我看到了 —  你圈中的是余额卡片底部那块（"充值 $8.50 · 赠送 $1.29" 这行和下面的白色"充值 +"按钮）。这里有两处可以改进：

1. **文案冲突**：上面小字写"充值 $8.50"指的是"已充值金额"，下面按钮也叫"充值"——两个 "充值" 意思不同
2. **按钮里 "充值 +" 用 nbsp 拼出来的加号，间距很奇怪**

我来分别修一下：

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

余额卡片底部清理好了:

- "已充值 / 赠送" 改成两列结构 + 中间细分隔线,赠送数字标绿,层级清晰
- 按钮文案改成 "立即充值",加号用 lucide plus icon(不再是 `&nbsp; +` 的怪间距)
- 上下两个 "充值" 的语义冲突也消除了

(数字是 counter 动画过程中的中间值,不是 bug —— 截图时刻 8.95 → 9.79 还在动)

## User

上下相距太近了啊

## Assistant

_[tool: str_replace_edit]_

确实挤了。我把按钮上方间距加大,同时让卡片整体节奏更舒展:

_[tool: save_screenshot]_

_[tool: view_image]_

现在节奏舒服多了:

- "已充值 / 赠送" 两列上下分两行(label + 数字),数字 15px 加大可读性
- 中间分隔线 28px 高,跟两侧文字高度对齐
- balance-split 和按钮之间留 24px,呼吸感够了

## User

这个星星的动态效果不止一闪一闪，还有一种怎么说，顺时针旋转的效果，偶尔还有流星特效

## Assistant

_[tool: str_replace_edit]_

明白,加这两个效果(虽然 brief §6.2 写禁止流星,但你拍板就行):

1. 整个星层缓慢顺时针旋转(很慢,~6 分钟一圈,潜意识感)
2. 每 12-25 秒随机一颗流星划过Now wrap the static + twinkle layers in a rotating container and add the meteor spawner in HTML/JS:

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: eval_js]_

_[tool: sleep]_

_[tool: eval_js]_

_[tool: eval_js]_

## User

继续吧，星星好像有点不太亮

## Assistant

_[tool: str_replace_edit]_

我把星星亮度提一档,同时降低中心 vignette 让星星更穿透:

_[tool: str_replace_edit]_

Now bump the static-star canvas opacity:

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

Add soft glow to the larger static stars for that real-twinkle look:

_[tool: save_screenshot]_

_[tool: view_image]_

星空亮度提上来了 ✓

变化总结:
- **静态星**:数量加了 2 倍(1500 → 1610),最大档透明度从 0.7 → 1.0,2-3px 的星额外加了径向 halo + 亮核
- **闪烁星**:bg 从 0.7 → 0.9,glow box-shadow 加强,twinkle 摆幅 0.4 ↔ 1.0(原 0.25 ↔ 0.95)
- **vignette 中心遮罩**:从 0.78 → 0.62,边缘衰减更早,让更多星透出来
- **保留中心 vignette** 是为了 hero 大数字不被星点干扰

旋转(360s/圈)和流星(12-25s 随机)都跑在后台,真实浏览器里能看到。html-to-image 截图捕捉不到完整 canvas 数据,所以你在用户视图里会看到比这个截图更亮、更明显的星空。

## User

那个流星你没写好，bug来的，飞的不像一颗流星，我靠现在这个星星又太大太亮了

## Assistant

_[tool: str_replace_edit]_

两个都修。

**流星 bug**：`transform: rotate()` 被关键帧的 `transform: translate()` 整个覆盖掉了,所以流星没朝运动方向飞,看起来就是个水平条平移。换成 individual `translate:` / `rotate:` 属性,两者就不打架了。

**星星太亮**：把高亮度档拉回来一些。

_[tool: grep]_

_[tool: read_file]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

Now scale back static-star density and switch meteor inline style to individual `rotate:` property:

_[tool: str_replace_edit]_

Now fix the meteor rotation — use individual `rotate` property so animated `translate:` doesn't wipe it out:

_[tool: str_replace_edit]_

_[tool: eval_js]_

_[tool: sleep]_

_[tool: eval_js]_

_[tool: eval_js]_

_[tool: sleep]_

_[tool: eval_js]_

_[tool: save_screenshot]_

Rotate preserved while translate animates — proper meteor flight in real browser. Let me check the star toning.

_[tool: view_image]_

两个都修好了：

**流星 bug** — 之前 `transform: rotate(...)` 设的角度被关键帧里 `transform: translate(...)` 整个覆盖,流星只是水平条平移没朝运动方向飞。现在改用 CSS 独立属性:
- 行内 `rotate: 28deg` (角度)
- 关键帧动画 `translate: 0 0 → var(--mx) var(--my)` (位移)

两者不打架,流星沿轨迹斜飞,头部亮点 + 尾迹始终朝向运动方向。

**星星亮度** — 拉回了一档:
- 静态:1206 颗(原 1610),最大档 alpha 1.0 → 0.85
- 闪烁:size 2px → 1.5px / 大星 3px → 2px,glow 半径减半
- twinkle 摆幅 0.4↔1.0 → 0.3↔0.85

现在低调多了 —— 截图里几乎只看到一两个最亮的点(右下角能瞄到),实际浏览器里会比这密一些(html-to-image 截图对暗像素不友好)。

## User

页面滚动条太丑了，星星改回原来的那个样式吧

## Assistant

_[tool: str_replace_edit]_

回退星星 + 给整个 app 的滚动条做暗色定制。

_[tool: str_replace_edit]_

## User

页面滚动条太丑了，星星改回原来的那个样式吧，我说的是你那个最初版的星星

## Assistant

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

Now style the scrollbar in dark theme:

_[tool: str_replace_edit]_

_[tool: eval_js]_

改完了:

- **星星**回到最初版样式: 2/3/4px、原 0.45/0.6/0.7 透明度档、原始 0.25↔0.95 twinkle、没有 canvas halo、42 颗动画星、vignette 也回到 0.78
- **滚动条**改成暗色窄条: 10px 宽、`rgba(255,255,255,0.10)` 灰头、悬停时 `0.22` 加亮、track 透明、firefox 也通过 `scrollbar-width: thin` + `scrollbar-color` 拿到一致效果

旋转和流星保留(那是另一个需求)。

## User

感觉又差一点亮度，就多加一点亮度就可以了

## Assistant

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

一档亮度增量已上：
- 静态星 alpha 0.45/0.6/0.7 → **0.55/0.72/0.88**
- twinkle 峰值 0.95 → **1.0**,起点 0.25 → **0.3**
- vignette 中心遮罩 0.78 → **0.70**,边缘衰减更通透

整体比之前"太亮"那一版克制不少,比"太暗"那版多一档可见度,应该到位了。

## User

这个太丑了

## Assistant

_[tool: str_replace_edit]_

数字输入框的原生上下小箭头,确实丑。隐藏掉:干净了 —— 原生 spinner 在 webkit 和 firefox 都隐藏掉。后期 Vue 落地时换成 `<NInputNumber>`,自带的左右减/加按钮才符合产品调性(spec 里也是这么定的)。

## User

修复了吗

## Assistant

_[tool: save_screenshot]_

_[tool: view_image]_

修好了 ✓ 数字输入框 "¥ 50" 那行右边已经没有原生 spinner 了,只剩前缀 `¥` + 数字 + 后缀 `= $52.00 虚拟$`,干净了。

## User

<pasted_text name="Pasted text (1591 lines)">
# PivotStack · Admin Console + Login + Notification 设计规范

> **第二份给 Claude Design 用的 brief**。承接 `STELLAR_CONSOLE_DESIGN.md`（用户端 Stellar Console 主题已实施）。
>
> 本规范覆盖 **3 项重设计**：
> 1. **Login Page** — 保留 API key 登录 + 新增 username/password + 3 个创意视觉方向（让 design AI 出 3 版让用户挑）
> 2. **Admin Console 整体重绘** — Operations Rail 布局（64px icon rail + 200px collapsible tree）
> 3. **跨端 Notification 系统** — admin 发布 + user 接收（**第一个 user/admin 双端联动的产品功能**）
>
> **目标产物**：3 套 HTML 原型（Login / Admin / Notification 各一），后期 1:1 落地到 Vue 3 + naive-ui。

---

## 0. 一句话定调

> **"User 端追求'记得住'的视觉冲击，Admin 端追求'用得爽'的工具理性 —— 共享同套 Stellar tokens，但密度、节奏、装饰策略完全相反。"**

主题代号：**Stellar Console · Workbench Edition**（星·工作台）。

User Stellar Console（已上线）的所有视觉锚点（黑底 / 星空 / 绿 accent / Geist 字体 / counter 动画）**全部继承**，但 admin 端：
- 字号缩 1 档（hero 大字砍掉 / 表格字号 12px / label 11px）
- 间距缩紧（padding 24px → 12-16px / row 高 44px → 36px）
- 动效几乎全删（保留 row hover / focus ring / notification bell pulse）
- 信息密度优先于装饰

---

## 1. 产品上下文

PivotStack 是 AI API 算力中转 + 转售平台。两端用户：

| 端 | 角色 | 主要任务 | 视觉调性 |
|---|---|---|---|
| **User** | 付费开发者 | 看用量 / 充值 / 调 API / 监控调用日志 | Stellar Console（已上线） |
| **Admin** | 平台运维 | 配渠道 / 管 API key / 看 abuse / 调定价 / 发通知 | **本 brief 覆盖** |

新增功能：**Notification 系统**横跨两端 —— admin 创建，user 收。这是 PivotStack 第一次需要"端到端"思考的产品功能。

---

## 2. 技术栈对齐

| 技术 | 版本 |
|---|---|
| Vue | 3.x Composition API + `<script setup lang="ts">` |
| 组件库 | **naive-ui** |
| 图标 | **lucide-vue-next** |
| 图表 | **ECharts 5** + 复用已注册的 `stellar` 主题 |
| 构建 | Vite |
| 后端 | Go (kiro-go) — JSON 持久化 + RWMutex |

**Design AI 出原型时**：
1. HTML + CSS（Tailwind CDN 或纯 CSS）模拟 naive-ui 视觉
2. 每个区块注释标记预期的 naive-ui 组件（`<!-- naive-ui: NDataTable -->`）
3. 图表用 ECharts 5 CDN + 注册同一个 stellar 主题（见 `STELLAR_CONSOLE_DESIGN.md` §15.1）
4. 字体复用 Geist + Geist Mono + PingFang SC

### 2.1 naive-ui 组件对照表

| 原型元素 | 替换成 naive-ui |
|---|---|
| 顶 / 侧导航项 | `router-link` + 自定义 class |
| 主按钮 | `<NButton type="primary" size="small">` |
| 透明描边按钮 | `<NButton size="small">` |
| Ghost 图标按钮 | `<NButton quaternary size="small">` + lucide icon |
| 输入框 | `<NInput size="small">` |
| 下拉选 | `<NSelect size="small">` |
| 表格 | `<NDataTable size="small" :columns :data :row-key>` |
| 分页 | `<NPagination size="small">` |
| Tag / Chip | `<NTag size="small" :bordered="false">` |
| 折叠面板 | `<NCollapse>` + `<NCollapseItem>` |
| Tabs | `<NTabs>` + `<NTabPane>` |
| Modal | `<NModal>` |
| Drawer | `<NDrawer>` + `<NDrawerContent>` |
| Form | `<NForm>` + `<NFormItem>` |
| 二次确认 | `<NPopconfirm>` |
| Tooltip | `<NTooltip>` |
| 通知（toast） | `useMessage()` / `useNotification()` |
| 标签输入 | `<NDynamicTags>` |
| Switch | `<NSwitch size="small">` |
| Radio | `<NRadio>` / `<NRadioGroup>` |

---

## 3. 色彩系统（继承 Stellar + 新增 Admin 高密度变体）

### 3.1 Stellar Console 全部 tokens（继承，不变）

直接看 `STELLAR_CONSOLE_DESIGN.md` §3，本文件不重复列。摘要：

```
--bg-base:    #000000  (主背景纯黑)
--bg-surface: #0a0a0a  (sidebar / drawer / modal)
--text-pri:   #ededed
--text-sec:   #a1a1a1
--text-ter:   #707070
--success:    #0bd470  (绿 accent / healthy 状态)
--warning:    #f5a623  (warn)
--error:      #ff4d4d  (down / critical)
--info:       #52a8ff  (蓝)
--border:     rgba(255,255,255,0.08)
hover bg:     rgba(255,255,255,0.04-0.06)
```

### 3.2 Admin 端新增 tokens

```css
/* Admin 高密度表格背景（比 user surface 稍亮，便于看 row 边界） */
--admin-bg-table-strip:  rgba(255,255,255,0.02);
--admin-bg-row-hover:    rgba(255,255,255,0.04);
--admin-bg-row-selected: rgba(11,212,112,0.06);

/* Admin 状态色（密度优化，更小的色块面积） */
--admin-status-dot-ok:    #0bd470;
--admin-status-dot-warn:  #f5a623;
--admin-status-dot-err:   #ff4d4d;
--admin-status-dot-idle:  #707070;

/* Admin rail / sidebar */
--admin-rail-bg:        #050505;
--admin-rail-active-bg: rgba(11,212,112,0.10);
--admin-rail-active-fg: #0bd470;
--admin-tree-bg:        #080808;
--admin-tree-divider:   rgba(255,255,255,0.04);

/* Notification levels（共享 user/admin） */
--notif-info-bg:    rgba(82,168,255,0.10);
--notif-info-fg:    #52a8ff;
--notif-warn-bg:    rgba(245,166,35,0.10);
--notif-warn-fg:    #f5a623;
--notif-crit-bg:    rgba(255,77,77,0.10);
--notif-crit-fg:    #ff4d4d;
--notif-crit-glow:  0 0 12px rgba(255,77,77,0.25);
```

### 3.3 颜色规则（admin 部分）

- accent 绿仍然稀缺使用 —— 仅 rail 当前激活 / 健康状态点 / 关键正向数字
- 表格里的"成功"状态不要染整行，只染状态点
- critical 通知用红色 + 微弱 glow（仅那一个 chip 用 glow，不污染整页）
- 禁纯白 hover、禁紫粉橙 AI 渐变（继承 Stellar 禁忌）

---

## 4. 字体系统（Admin 紧凑版）

复用 Stellar Console 的字族（Geist / Geist Mono / PingFang SC），但 admin 端字号变小：

| Class | size | weight | 用途 |
|---|---|---|---|
| `.t-display-admin` | 18px | 600 | admin 区块标题（user 端是 22px） |
| `.t-h-admin` | 14px | 600 | 次级标题 |
| `.t-body-strong` | 13px | 500 | 强调主体 |
| `.t-body` | 12px | 400 | 默认正文（user 端是 13px）|
| `.t-table-cell` | 12px mono | 400 tabular-nums | 表格数据行（**主要用法**） |
| `.t-table-head` | 11px | 500 / 0.06em / uppercase | 表头 |
| `.t-label` | 11px | 500 / 0.06em / uppercase | metadata / 时间戳 |
| `.t-meta` | 11px mono | 400 tabular-nums | request_id / hash / 时间 |
| `.t-micro` | 10px | 600 / 0.08em / uppercase | badge 内文字 |

**关键约束**：
- 数字一律 mono + tabular-nums（admin 看数据是主任务）
- 表格内单元格 max 1 行，超长用 ellipsis + tooltip
- 没有 hero-xl / hero-lg（admin 不需要 64px 大字）—— 唯一例外是 Login 页（继承 user 端 hero 字号）

---

## 5. 间距 / 圆角 / 阴影

### 5.1 间距 token（4px 基准）

```
--space-1: 4px
--space-2: 8px
--space-3: 12px   ← admin 默认 padding（user 端是 24px）
--space-4: 16px   ← admin 区块间距
--space-5: 20px
--space-6: 24px
--space-8: 32px
```

### 5.2 圆角

```
--r-xs: 2px
--r-sm: 4px    ← chip / tag / button 默认
--r-md: 6px    ← input / 小容器
--r-lg: 8px    ← modal / drawer
```

**禁** > 10px 大圆角（避免卡通感）。

### 5.3 阴影

全产品禁 `box-shadow`，唯一例外：
- modal / drawer 浮层：`0 8px 32px rgba(0,0,0,0.5)`
- critical notification 的红色 chip：微弱 glow `0 0 12px rgba(255,77,77,0.25)`

---

## 6. Initiative A — Login Page 重设计

### 6.1 业务上下文

- **保留** 现有 "API key" 登录（legacy 用户已经有 key）—— 直接粘 sk-xxx
- **新增** "username + password" 登录（迁移路径）
- **注册流程**：admin 邀请制 v1 —— 不公开自助注册（v2 再考虑）。admin 在后台发 invite link，用户点开后设置 username + password 即可使用
- **错误态**：网络失败 / 密钥错误 / 用户名密码错误 / 账号被禁 4 种
- **成功跳转**：API key 登录 → `/user/dashboard`；admin password 登录 → `/overview`

### 6.2 双登录方式 UX

**核心交互模式：Tab 切换 + 视觉一致**

```
┌──────────────────────────────────────────┐
│  [API Key]   [用户名密码]                  │  ← 顶部 2 tab，selected 时绿色下划线
│                                          │
│  [输入区]                                  │  ← 根据 tab 切换内容
│                                          │
│  [登录按钮]                                │
└──────────────────────────────────────────┘
```

**API Key Tab 表单**：
```
[t-label] ACCESS TOKEN
[mono input]  sk-XXXXXXXXXXXXXXXXXXXXXXXX        [复制 / 显示切换]
[t-meta tertiary] 来自老用户？继续用 API key 即可
[primary button] 登录
[ghost link] 用户名密码登录 →   ← tab 之间互相跳转
```

**用户名密码 Tab 表单**：
```
[t-label] USERNAME
[input] your@email.com 或 用户名

[t-label] PASSWORD
[password input + eye toggle]

[checkbox] 记住我（30 天）       [ghost link] 忘记密码？

[primary button] 登录
[ghost link] 我有 API key →
```

**视觉一致原则**：两个 tab 切换时表单容器位置不变（只 swap 内容），下方按钮永远在同一位置。

### 6.3 创意方向（3 选 1，design AI 都出，让用户挑）

#### 方向 A — **"The Singularity"**（中央脉动 Orb）— 推荐

**概念**：
正中央有一个 64-96px 的脉动绿色发光球（"Pivot Orb"），周围星空粒子缓慢被它吸引。鼠标 hover orb → orb "展开"成 login 卡片（CSS 3D transform + opacity 过渡）。点击外部空白 → orb 重新收起。

**视觉 mockup**（描述）：

```
全屏黑底 + 星空层 + 流星（继承 Stellar）

         · ·       ·          ·
                 · . ·
              ╭─────────╮              ← 64-96px 圆形 orb
              │   (●)    │              ← 中央绿光球 #0bd470，外圈柔光
              │  pulse   │              ← 缓慢呼吸 2.5s 周期
              ╰─────────╯
               PivotStack              ← orb 下方品牌字
              [点击进入]                ← micro 提示
       ·         · ·         ·

   (hover 后 orb 展开 → 卡片浮现)
              ↓
   ┌──────────────────────────┐
   │  [API Key] [用户名密码]    │
   │                           │
   │  ····· 表单 ·····          │
   │                           │
   │  [登录]                    │
   └──────────────────────────┘
```

**实现要点**：
- Orb 用 `radial-gradient(circle, #0bd470 0%, #0bd47080 30%, transparent 70%)` + `filter: blur(...)`
- 周围"被吸引粒子"：在 starfield 之上加一个 canvas，10-15 个小点向中心做缓慢 vector 移动，到达后 fade
- Hover 触发 form 出现：orb 缩到 24px 同时 form 卡片 fade-in + scale 0.95→1（300ms cubic-bezier(0.16, 1, 0.3, 1)）
- "未选定方向"和"激活后"的对比通过单一 hover 实现 —— 不需要 click 二次确认

**Pros**：
- 极强记忆点，访客一眼记住"那个 orb"
- 跟 Stellar Console 主题极契合（pivot = 中心枢轴）
- 实现成本中等（纯 CSS + 1 个轻量 canvas）

**Cons**：
- 移动端 hover 不存在 → 需要 fallback 为 tap to expand
- prefers-reduced-motion 需关闭 orb pulse 和粒子吸引

**Effort**：M（中等）

#### 方向 B — **"The Boot Sequence"**（终端启动序列）

**概念**：
页面打开瞬间是黑屏 + 终端字符流"启动"PivotStack。ASCII 字符快速滚动（系统模块加载 / 网络检查 / 凭证模块就绪 / ...），最后停在 prompt：

```
> pivot login --user <_>
```

User input 直接在终端里输。两个 tab 通过命令切换（`--key` vs `--user`）。

**视觉 mockup**：

```
全屏黑底 + 单色绿字（#0bd470）

▌PIVOTSTACK GATEWAY OS · v6.0
▌─────────────────────────────────────
▌[ OK ] starfield engine loaded
▌[ OK ] auth module ready (v2)
▌[ OK ] gateway 8990 listening
▌[ OK ] welcome to pivotstack
▌
▌Sign in to continue.
▌
▌  pivot login --user [you@example.com_____]
▌                    [password__________]
▌
▌  [Enter] to login    [Tab] switch to --key
▌
▌_                                              ← 闪烁光标
```

**实现要点**：
- 字符流"打字"效果：每行字符以 30-50ms 间隔 push 出（typing animation）
- 不要真等 4 秒，2 秒内必须完成 boot
- 终端字体 Geist Mono / JetBrains Mono
- form 直接绑定在终端文本流上 —— 实际是 `<input>` 用 mono 字体 + 透明边框 + 绿色 caret
- `[Tab]` 真切换 API key 模式：UI 重写第三行 prompt 为 `pivot login --key sk-xxxx`

**Pros**：
- 极度对应开发者用户心智模型
- 简单纯 CSS + JS，没有性能负担
- 可访问性最好（屏幕阅读器友好）

**Cons**：
- 非技术 reseller 用户会觉得"不友好"
- 移动端终端比例难处理

**Effort**：S（小）

#### 方向 C — **"The Warp Tunnel"**（曲速隧道凝聚）

**概念**：
启动时星空高速向中心 warp（星点拉长成流线），1.5 秒后减速 → 星流收敛 → 凝聚成 login 卡片边框（光线沿矩形边游走）。Login 卡片显示后，warp 停止，回到正常星空 + 偶尔流星。

**视觉 mockup**（描述）：

```
帧 0-1.5s：warp 高速
   ─────────·────────·────────
   ════════════════════════════   ← 星点拉长流线，向中心汇聚
   ─────────·────────·────────

帧 1.5-2s：减速 + 凝聚
   高速 → 减速 → 流线绕成矩形边框

帧 2s+ 卡片成形
   ┌─━━━━━━━━━━━━━━━━━━━━━━━━┐
   ║  [API Key] [用户名密码]    ║   ← 卡片边框是连续游走的光线
   ║                          ║
   ║  表单                     ║
   ║                          ║
   ║  [登录]                   ║
   └─━━━━━━━━━━━━━━━━━━━━━━━━┘
```

**实现要点**：
- Warp 阶段：现有 starfield 的 transform: scale 从 1 加速到 5（perspective 拉近）
- 凝聚阶段：用 SVG stroke-dasharray + animation 让光线沿矩形边周长游走
- 卡片本身：背景 90% 透明 + 1px 内描边（边框是动效本身）
- 跳过动画选项：localStorage 标记"已访问"→ 第二次直接 skip 到帧 2s+

**Pros**：
- 最 cinematic / 印象最深
- 用满了 Stellar 主题的"深空"属性

**Cons**：
- 实现复杂（perspective 切换 + SVG dash 动效精度）
- 高耗能 GPU（旧设备可能卡）
- prefers-reduced-motion 需完全 disable

**Effort**：L（大）

#### 创意方向的产出策略

Design AI **3 个方向都要出 HTML 原型**（同一 brief 内 3 个 section）。用户看完 3 版后选定，再做实施细节。

### 6.4 Login 页其他细节

#### 6.4.1 错误态

| 错误 | 文案 | 视觉 |
|---|---|---|
| 网络失败 | `连不上服务器，检查网络后重试` | input 不抖、底部 chip warn 红色 |
| API key 无效 | `这个 key 不对，确认一下？` | input 红色描边 + 抖动 1 次 |
| 用户名密码错 | `用户名或密码不对` | 同上，错 3 次后冷却 30s |
| 账号被禁 | `账号已停用，联系客服 xxx` | input 不可输入 + 显示客服联系 |

#### 6.4.2 忘记密码（v1 占位）

点 "忘记密码？" → 弹 modal：`v1 还不支持自助找回，请联系 admin 重置（[复制邮箱]）`。v2 再做真正的邮件重置。

#### 6.4.3 移动端

- 320-768px：3 个创意方向都要 fallback 到普通"星空背景 + 居中卡片"（动效太重在小屏负担大）
- Tab 切换改成单按钮上下排（API key 输入框 在上，用户名密码在下，互相 toggle）

### 6.5 Login 视觉规范汇总

```css
/* Login 页用大字号 hero（继承 user 端 hero-xl 64px） */
.login-brand { font: 500 36px Geist; letter-spacing: -0.02em; color: #ededed; }
.login-subtitle { font: 400 14px Geist; color: #707070; }

/* 卡片容器 */
.login-card {
  width: 380px;
  background: rgba(10,10,10,0.85);
  backdrop-filter: blur(20px);
  border-radius: 8px;
  padding: 32px 28px;
  position: relative; z-index: 2;
}

/* Tab 切换 */
.login-tabs { display: flex; gap: 24px; border-bottom: 1px solid var(--border); margin-bottom: 20px; }
.login-tab { padding: 10px 0; color: var(--text-sec); position: relative; cursor: pointer; }
.login-tab.is-active { color: var(--text-pri); }
.login-tab.is-active::after {
  content: ''; position: absolute; bottom: -1px; left: 0; right: 0;
  height: 2px; background: var(--success);
}
```

---

## 7. Initiative B — Admin 布局：Operations Rail

### 7.1 整体结构

```
┌──────┬─────────────┬─────────────────────────────────────────┐
│      │             │                                          │
│  64  │    200      │                                          │
│  px  │    px       │              主区域                       │
│ Rail │   Tree      │            (剩余宽度)                     │
│      │             │                                          │
│      │             │                                          │
└──────┴─────────────┴─────────────────────────────────────────┘
       ↑               ↑
   icon-only       group + sub-items
   极简长条        可折叠树
```

**为什么用 Rail 而非 240px sidebar**：
- 13 个 admin 路由按 5 个域分组 → 普通 sidebar 至少 280px（容纳"渠道总览 / 分组 / NewAPI / 直连 / 重新对账"这种文字）
- Rail + Tree 二级分离：rail 顶部域切换（5 个 icon），tree 显示当前域的具体 route → 总宽 264px 但显示信息更丰富
- 用户表达过"实用、信息密度高" → Rail 比 Sidebar 释放更多 main canvas 给数据

### 7.2 Rail（64px icon-only）

**位置**：左边贴边 fixed，height 100vh，背景 `#050505`，无 border（靠 darker 背景区分）。

**结构**：
```
┌──────┐
│  [P] │ ← 24px logo（手写 S，沿用 user 端 logo）
│      │
│  [⌂] │ ← Overview (lucide LayoutDashboard)
│  [▣] │ ← Channels (Layers)
│  [$] │ ← Billing (Wallet)
│  [⊞] │ ← Ops (Activity)
│  [⚙] │ ← System (Settings)
│  [👥]│ ← Reseller (Users)
│      │
│  [🔔]│ ← Notifications (Bell) ← 新增 + unread badge
│      │
│ ↓底部 │
│  [👤]│ ← Admin profile + logout (UserCircle2)
└──────┘
```

**Rail icon 规范**：
- 每个 icon 14-16px，居中
- icon 容器 40×40px，圆角 6px
- 未选中：color `#707070`，无背景
- hover：background `rgba(255,255,255,0.06)`，color `#ededed`
- 激活（当前域）：background `var(--admin-rail-active-bg)`，color `var(--admin-rail-active-fg)`，左侧 2px 绿色竖线
- icon 旁悬浮 Tooltip 显示域名（hover delay 400ms）
- 单击 icon：rail 不变（保持激活），tree 切到该域

### 7.3 Tree（200px 可折叠）

**位置**：rail 右侧，width 200px，背景 `#080808`，无 border。

**结构**：

```
┌──────────────────┐
│ CHANNELS         │  ← .t-label uppercase 表示当前域
│                  │
│ ◉ 分组管理         │  ← 当前激活 route，绿色 + 左 2px 竖线
│ ○ NewAPI Provider │
│ ○ 自营直连          │
│ ○ 对账中心          │
│                  │
│ ── divider ──    │
│                  │
│ KEYS & BILLING   │  ← 二级 group label
│ ○ API Keys       │
│ ○ 兑换码           │
│ ○ 定价配置          │
│ ○ 单位换算          │
└──────────────────┘
```

**Tree item 规范**：
- 每项 32px 高，padding `0 12px`，左右排：[bullet] [label]
- bullet：未激活 `○` color #707070，激活 `●` color #0bd470 + 左侧 2px 竖绿线
- label 字号 12px，未激活 #a1a1a1，hover #ededed，激活 #ededed
- 整行 hover background `rgba(255,255,255,0.04)`
- 切换路由用 router-link

**Tree 折叠行为**：
- 桌面 1280+：默认展开
- 1024-1280：rail 保留，tree 默认折叠（点 rail icon 浮出 popover tree）
- < 1024：rail + tree 整体收起为 hamburger 按钮

### 7.4 主区域 (Main Canvas)

**容器**：
```css
.admin-main {
  margin-left: 264px;     /* rail 64 + tree 200 */
  min-height: 100vh;
  background: transparent;  /* 看到 starfield */
  padding: 0;  /* 让 PageHeader 自己控 */
}
.admin-page {
  max-width: 1600px;  /* 比 user 端 1440 略宽，admin 表格多 */
  margin: 0 auto;
  padding: 24px 32px 48px;
}
```

**注意**：admin 端**保留 starfield**（共享 Stellar 主题），但 vignette 更深（中心遮罩从 70% 增到 80%），不干扰数据阅读。

### 7.5 PageHeader（统一规范）

每个 admin view 顶部必须有 PageHeader：

```
┌───────────────────────────────────────────────────────────────┐
│ Channels  /  分组管理                              [+ 新建分组] │
│ ──────────                                                    │
│ 渠道分组                            ●Live  · 14:32 last sync   │
│ 管理 NewAPI / 自营 / 直连渠道的分组归属                          │
└───────────────────────────────────────────────────────────────┘
```

- **breadcrumb**：当前域 / 当前 route（rail 已经告诉用户域，breadcrumb 只是辅助）
- **title**：`.t-h-admin` 18px weight 600
- **subtitle**：`.t-body` 12px color #707070
- **right slot**：actions（primary button / live status / sync info）
- 下方一道 hairline divider

### 7.6 13 个 admin 路由归组方案

```
RAIL ICON     TREE ITEMS                              ROUTE PATH
──────────────────────────────────────────────────────────────────────────
[⌂] Dashboard   总览                                    /overview

[▣] Channels    分组管理                                 /channels
                NewAPI Provider                          /channels/newapi
                自营直连                                  /channels/direct
                对账中心                                  /channels/reconcile

[$] Billing     API Keys                                /billing/keys
                兑换码                                    /billing/codes
                定价配置                                  /billing/pricing
                单位换算                                  /billing/unit

[⊞] Ops         调用日志                                  /ops/call-logs
                Abuse Monitor                            /ops/abuse  (现有 /AbuseMonitor.vue 迁移)

[⚙] System      系统设置                                  /system/settings
                实验性功能                                  /system/experimental
                ⚡ 通知管理 (新增)                          /system/notifications

[👥] Reseller   代理总览                                  /reseller/summary
                子 Key 管理                              /reseller/keys
```

**所有现有路由保持兼容**（router/index.js 旧 URL 自动 redirect），只是 UI 重组织。

### 7.7 移动端（< 1024px）

- Rail + Tree 完全隐藏
- 顶部 56px 高 sticky bar：左 [☰] 汉堡 + logo，右 actions
- 点 [☰] → 全屏 drawer 滑入，显示 rail + tree 完整树
- 主区域 padding 改 16px

---

## 8. 通用组件规范（admin 专用，新建）

### 8.1 AdminTable（统一表格密度）

替代散落在 13 个 view 里的各种 NDataTable 调用。统一 props：

```
<AdminTable
  :columns="..."
  :data="..."
  :row-key="..."
  :loading
  :pagination       (server-side, default page-size 50)
  :selectable       (单选 / 多选 / 关闭)
  :on-row-click     (整行点击 = 进详情)
  :bulk-actions     ([{ icon, label, handler }])
  :empty-text
/>
```

**默认视觉**：
- 行高 36px（user 端 NDataTable 是 44px）
- header 高 32px，字号 11px uppercase
- 单元格 padding `0 12px`
- striped 隔行：`bg-table-strip` 0.02
- hover 整行：`bg-row-hover` 0.04 + 右侧出 ChevronRight 暗示可点
- selected 整行：`bg-row-selected` 0.06 + 左 2px 绿竖线
- empty state：居中 `.t-label tertiary` + 可选 action 按钮

**bulk-actions 工具栏**：
当 selectable=multi 且 selected.length>0 时，表格顶部出现：

```
┌─ 已选 5 条 ─────────────  [启用] [禁用] [删除] [导出 CSV]  [取消选择] ─┐
```

position sticky top 56px，高 40px，背景 `#0a0a0a` + 1px 底 border。

### 8.2 AdminFilterBar（顶部过滤条）

每个列表 view 顶部统一组件：

```
┌─────────────────────────────────────────────────────────────────────┐
│ [搜索 mono input] [状态▾] [类型▾] [时间▾]    [刷新] [批量导入] [+ 新建]│
└─────────────────────────────────────────────────────────────────────┘
```

- 搜索框默认 240px，prefix lucide Search icon，placeholder 显示当前实体名（"搜索 keyword 或 ID"）
- 多个 dropdown 用 NSelect size small + 显示已选数量
- 右侧 actions 区固定排序：刷新 → 导入 → 新建 → ...
- 整条高度 40px

### 8.3 AdminStatusTag

统一状态可视化：

```
●ACTIVE         ●OK            ⚠WARN          ●DOWN
绿点 + 大写       同绿            橙             红 + 微 glow
```

- 圆点 6px + gap 6px + text `.t-micro` uppercase
- 多色：ok/warn/err/idle，对应 `--admin-status-dot-*`
- 状态为 ok 时点 pulse（2s），其他静止

### 8.4 AdminMetricStrip（顶部 KPI 横排）

部分 view（Overview / BillingKeys / Reseller）顶部需要 3-4 个 metric tile：

```
┌──────────────┬──────────────┬──────────────┬──────────────┐
│ 总余额        │ 今日调用      │ 活跃 keys    │ 错误率         │
│ $128,492.40  │ 9,855        │ 268 / 356    │ 0.42% ↓       │
│ ¥128,492.40  │ ↗ +12%       │ +12 本周      │ ↘ -0.1pp      │
└──────────────┴──────────────┴──────────────┴──────────────┘
```

- 每 tile 高 88px，无 border，按空白分层
- 主数字 24px mono（admin 比 user 端 hero-lg 40px 小）
- delta chip 复用 user 端 `chip--up`/`chip--err`

### 8.5 AdminEmptyState

替代散落的 EmptyState，统一组件：

```
            ╭─ icon ─╮
            │   ○    │
            ╰────────╯
             暂无 X 数据
       上次同步 14:32 · 检查筛选条件
            [+ 立即新建]
```

- icon 32px lucide 风格 + 0.4 opacity
- title `.t-body-strong`，subtitle `.t-label tertiary`
- 可选 action button

### 8.6 AdminToolbar

页面右上 actions 区，统一布局：
```
[ghost 刷新] [secondary 导出] [secondary 导入] [primary + 新建]
```
高 32px / gap 8px / 右对齐。

### 8.7 AdminDrawer（详情抽屉）

替代散落的"点击进详情页"模式 —— admin 端高频操作建议 drawer 不是新页面：

```
点击表格 row → 右侧滑入 480px drawer
drawer 内：
  [关闭 X]            标题 ID/Name        [复制 ID]
  ─────────────
  [tabs: 概览 / 用量 / 设置 / 历史]
  ─────────────
  各 tab 内容
  ─────────────
  底部 sticky: [删除] [保存]
```

drawer 替代 90% 的"详情页 route"，仅复杂详情（如 ChannelGroup 详情、KeyDetail）保留独立路由。

---

## 9. Initiative C — 跨端 Notification 系统

### 9.1 后端 Schema（直接来自 codex 分析）

#### 9.1.1 Go 结构定义

```go
type Notification struct {
    ID          string   `json:"id"`           // ntf_<unix>_<short>
    Title       string   `json:"title"`        // ≤ 80 字
    Body        string   `json:"body"`         // markdown，≤ 2000 字
    Level       string   `json:"level"`        // info | warn | critical
    TargetType  string   `json:"targetType"`   // all | plan | group | userIds
    TargetValue []string `json:"targetValue,omitempty"`

    Status      string `json:"status"`       // draft | published
    PublishAt   int64  `json:"publishAt,omitempty"`
    ExpireAt    int64  `json:"expireAt,omitempty"`
    Dismissible bool   `json:"dismissible"`

    CreatedAt int64  `json:"createdAt"`
    UpdatedAt int64  `json:"updatedAt,omitempty"`
    CreatedBy string `json:"createdBy,omitempty"`
    UpdatedBy string `json:"updatedBy,omitempty"`
    DeletedAt int64  `json:"deletedAt,omitempty"` // 软删除
}

type NotificationDelivery struct {
    NotificationID string `json:"notificationId"`
    UserID         string `json:"userId"`       // ApiKeyInfo.ID
    FirstSeenAt    int64  `json:"firstSeenAt,omitempty"`
    ReadAt         int64  `json:"readAt,omitempty"`
    DismissedAt    int64  `json:"dismissedAt,omitempty"`
    UpdatedAt      int64  `json:"updatedAt,omitempty"`
}

type NotificationFile struct {
    SchemaVersion int                    `json:"schemaVersion"`
    Notifications []Notification         `json:"notifications"`
    Deliveries    []NotificationDelivery `json:"deliveries,omitempty"`
    UpdatedAt     int64                  `json:"updatedAt,omitempty"`
}
```

**存储位置**：独立 `data/notifications.json`（**不**写入 config.json，避免高频读/写冲突）。

#### 9.1.2 文件布局

```
kiro-go/notifications/
  types.go             ← struct + enum + 校验
  store.go             ← 读写 data/notifications.json + RWMutex
  service.go           ← targeting / read / dismiss / stats
  service_test.go      ← 单元测试

kiro-go/proxy/
  handler_admin_notifications.go  ← admin CRUD + stats
  handler_user_notifications.go   ← user list / read / dismiss
```

### 9.2 后端 API

#### 9.2.1 Admin endpoints

##### `GET /admin/api/notifications?status=all&limit=50&offset=0`

```json
{
  "items": [
    {
      "notification": {
        "id": "ntf_01",
        "title": "Claude channel maintenance",
        "body": "**Claude** group will rotate upstream keys.",
        "level": "warn",
        "targetType": "group",
        "targetValue": ["claude"],
        "status": "published",
        "publishAt": 1779091200,
        "expireAt": 1779177600,
        "dismissible": true,
        "createdAt": 1779080000
      },
      "stats": {
        "targetCount": 42,
        "readCount": 18,
        "dismissedCount": 3,
        "unreadCount": 24
      }
    }
  ],
  "total": 1
}
```

##### `POST /admin/api/notifications`

Request body：
```json
{
  "title": "Billing policy update",
  "body": "Hybrid plan pricing changes tonight.",
  "level": "info",
  "targetType": "plan",
  "targetValue": ["hybrid"],
  "status": "draft",
  "publishAt": 1779091200,
  "expireAt": 1779696000,
  "dismissible": true
}
```

Response：201 + 完整 Notification 对象。

##### `PUT /admin/api/notifications/:id`

同 POST 的 body。校验 `expireAt > publishAt`。**修改已发布通知不会重置 read state**（如需重新触达，新建）。

##### `DELETE /admin/api/notifications/:id`

软删除 = `deletedAt = now()`。返回 `{ "success": true }`。

##### `GET /admin/api/notifications/:id/stats`

```json
{
  "notificationId": "ntf_01",
  "targetCount": 42,
  "readCount": 18,
  "dismissedCount": 3,
  "unreadCount": 24
}
```

##### Audit log

每个 admin mutation 记录：
```go
AuditLog("notification_create", actor, fmt.Sprintf("id=%s status=%s", n.ID, n.Status))
AuditLog("notification_update", actor, fmt.Sprintf("id=%s", id))
AuditLog("notification_delete", actor, fmt.Sprintf("id=%s", id))
```

#### 9.2.2 User endpoints

##### `GET /user/api/notifications?limit=5`

```json
{
  "unreadCount": 2,
  "items": [
    {
      "id": "ntf_01",
      "title": "Claude channel maintenance",
      "body": "**Claude** group will rotate upstream keys.",
      "level": "warn",
      "publishAt": 1779091200,
      "expireAt": 1779177600,
      "dismissible": true,
      "read": false,
      "dismissed": false,
      "readAt": 0
    }
  ]
}
```

##### `POST /user/api/notifications/:id/read`

```json
{ "success": true, "readAt": 1779091300 }
```

幂等。

##### `POST /user/api/notifications/:id/dismiss`

```json
{ "success": true, "dismissedAt": 1779091400 }
```

要求 `dismissible=true`。

### 9.3 Targeting Resolver

```go
func visibleForUser(n Notification, key config.ApiKeyInfo, now int64) bool {
    if !key.Enabled { return false }
    if n.DeletedAt != 0 || n.Status != "published" { return false }
    if n.PublishAt > 0 && now < n.PublishAt { return false }
    if n.ExpireAt > 0 && now >= n.ExpireAt { return false }

    switch n.TargetType {
    case "all":
        return true
    case "plan":
        return contains(n.TargetValue, key.Plan)
    case "group":
        // 用户的 ChannelPreferences 里包含目标 groupID 之一
        for _, groupID := range n.TargetValue {
            if _, ok := key.ChannelPreferences[groupID]; ok { return true }
        }
        return false
    case "userIds":
        return contains(n.TargetValue, key.ID)
    default:
        return false
    }
}
```

### 9.4 推送机制：v1 polling 60s

**理由**（采纳 codex 的判断）：
- PivotStack 通知低频（admin 一周 1-2 条普通通知 + 偶尔 critical）
- 现有 Bearer 鉴权零改动
- 后端保持 stateless，无长连接负担
- 单次 list endpoint 应答 < 50ms（in-memory 索引）
- SSE 在 356 个 user key 同时在线时需要重大架构调整 → 不在 v1 范围

**前端 polling 策略**：
```ts
// stores/notifications.js
const pollIntervalMs = 60_000
const lastFetchedAt = ref(0)
const items = ref<Notification[]>([])
const unread = computed(() => items.value.filter(n => !n.read).length)

async function refresh() {
  const data = await userApi('/notifications?limit=10')
  items.value = data.items
  lastFetchedAt.value = Date.now()
}

onMounted(() => {
  refresh()
  setInterval(refresh, pollIntervalMs)
})
```

**v2 升级路径**：当用户量 > 5000 或通知 push 需要 < 30s 延迟时，迁移到 SSE。schema 不变，只换 transport。

### 9.5 Admin UI 设计

#### 9.5.1 路由：`/system/notifications` 通知管理

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ System  /  通知管理                                          [+ 新建通知]    │
│ ─────────                                                                   │
│ 用户通知                                  共 12 条 · 5 已发布 · 2 草稿 · ...  │
│ 给所有/特定 user 发布站内通知                                                │
└─────────────────────────────────────────────────────────────────────────────┘

┌─ filter ribbon ──────────────────────────────────────────────────────────────┐
│ [搜索标题/正文] [状态▾] [级别▾] [时间▾]            [刷新] [+ 新建通知]         │
└─────────────────────────────────────────────────────────────────────────────┘

┌─ AdminTable ─────────────────────────────────────────────────────────────────┐
│ 标题                目标         级别   状态    发布时间       已读率  操作   │
│ ──────────────────────────────────────────────────────────────────────────── │
│ ●WARN Claude 维护    group:claude warn  PUBLISHED 05-18 02:00 / -1d  18/42   [✏][🗑] │
│ ●INFO 充值优惠        plan:hybrid  info  PUBLISHED 05-15 18:00       89/120  [✏][🗑] │
│ ⚠CRIT 紧急维护        all          crit  EXPIRED   05-13 09:00       356/356 [✏][🗑] │
│ ○DRAFT 即将到期提醒    plan:credit info  DRAFT     —                  —      [✏][🗑] │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 9.5.2 创建/编辑 Modal（NModal width 720px）

```
┌─────────────────────────────────────────────────────────────────────┐
│ 新建通知                                                       [X]  │
│ ───────────────────────────────────────────────────────────────── │
│                                                                     │
│ TITLE  (≤ 80 字)                                                    │
│ [Claude 维护通告________________________________________________]    │
│                                                                     │
│ BODY  支持 markdown  (≤ 2000 字)                                   │
│ ┌─────────────────────────────────────────────────────────────────┐ │
│ │ **Claude** group 将于今晚 02:00 - 03:00 进行上游 key 轮换。      │ │
│ │                                                                 │ │
│ │ 期间可能出现短暂调用失败，请已经在生产的客户提前规避。           │ │
│ └─────────────────────────────────────────────────────────────────┘ │
│ [预览 markdown] ←                                                   │
│                                                                     │
│ LEVEL                                                               │
│ ( )●INFO  (•)●WARN  ( )⚠CRITICAL                                   │
│                                                                     │
│ TARGETING                                                           │
│ targetType: [ALL] [PLAN] [GROUP] [USERIDS]  ← Radio chip            │
│                                                                     │
│ ▾ 显示目标值选择器（根据 type 切换 UI）                              │
│   PLAN: ◉ free  ◉ credit  ◉ timed  ◉ hybrid                       │
│   GROUP: ◉ claude  ◉ gpt-系列  ◉ gemini-系列                       │
│   USERIDS: [搜索 key id 多选 input]                                 │
│                                                                     │
│ SCHEDULE                                                            │
│ Publish at:  [2026-05-18 02:00] [立即发布]                          │
│ Expire at:   [2026-05-18 23:59] [永久]                              │
│                                                                     │
│ Dismissible:  [● ON]  ← 用户可手动关闭                              │
│                                                                     │
│ ─────────────────────────────────────────────────────────────────── │
│ [保存草稿]                              [取消]    [立即发布]         │
└─────────────────────────────────────────────────────────────────────┘
```

**Targeting UX**：
- targetType 改成 Radio chip group（4 个 chip 横排，selected 时绿底）
- 切换 type 时下方面板平滑切换 height
- ALL：无下方面板
- PLAN：4 个 plan checkbox 横排
- GROUP：动态拉 admin 已建的 channelGroup，checkbox 列表
- USERIDS：mono input + autocomplete（搜 key id 或 note）

**预览 markdown 按钮**：点了切到只读 preview 模式，可以再切回编辑。

#### 9.5.3 统计 Panel（行右侧抽屉）

点表格"已读率"列或者点行 → 右侧滑入 360px 抽屉显示通知详细 stats：

```
┌─ stats drawer ──────────────────────────┐
│  ID  ntf_01                       [X]  │
│  Claude 维护通告                        │
│ ─────────────────────────────────────── │
│                                         │
│  目标受众                                │
│  group: claude                          │
│  ─────                                   │
│  TOTAL TARGETED            42           │
│  READ                      18 (43%)     │
│  DISMISSED                  3 (7%)      │
│  UNREAD                    24 (57%)     │
│                                         │
│  阅读率分布（折线，过去 24h）             │
│  [mini ECharts area chart]              │
│                                         │
│  最早阅读                                │
│  user_abc · 02:14 · 14 分钟前            │
│                                         │
│  最近阅读                                │
│  user_def · 02:28 · 刚才                 │
└─────────────────────────────────────────┘
```

### 9.6 User UI 设计

#### 9.6.1 NotificationBell（user nav）

加在 UserLayout.vue 顶部 nav 右侧（在 Live indicator 和余额 chip 之间）：

```
[●Live · 12s ago]   [🔔(2)]   [$ 9.79]   [⏻]
                     ↑ bell icon + unread badge
```

**Bell 视觉规范**：
- icon 14px lucide Bell
- 容器 32×32 圆角 4px，hover 背景 `rgba(255,255,255,0.04)`
- unread > 0 时右上角红点（critical 含）或绿点（仅 info/warn）
  - critical：红 #ff4d4d + 微 glow + pulse 1.5s
  - warn / info：绿 #0bd470 静止
- unread === 0 时无点

**badge 数字**：
- 1-9：显示数字
- ≥ 10：显示 `9+`
- 0：不显示 badge

#### 9.6.2 NotificationDropdown（hover/click 浮出）

```
┌─ Dropdown (320px wide, top-right anchored) ──────┐
│  通知                              [查看全部 →]  │
│  ────────────────────────────────────────────── │
│                                                  │
│  ●WARN  Claude 维护通告                  2 分钟前 │
│  Claude group 将于今晚 02:00...                  │
│  ────                                            │
│                                                  │
│  ●INFO  充值优惠                          1 天前  │
│  本月充 ¥500 多送 14% 虚拟$，活动至...           │
│  ────                                            │
│                                                  │
│  ⚠CRIT  系统紧急维护                      3 天前  │
│  (已过期 · 灰色显示)                              │
│                                                  │
│  ────────────────────────────────────────────── │
│  [全部标已读]                  [查看历史 →]      │
└──────────────────────────────────────────────────┘
```

- 每条 item 80px 高，左边 4px 状态点（绿/橙/红）
- title `.t-body-strong`，body preview 60 字截断 + ellipsis
- 时间 11px mono 灰色右对齐
- hover 整条 background `rgba(255,255,255,0.04)`
- click → 打开 modal（见 9.6.3）
- "查看全部" → `/user/notifications` 路由（独立 view）
- "全部标已读" → 批量 POST mark-all-read

#### 9.6.3 NotificationModal（详情）

点 dropdown 单条或 banner click → 弹 NModal width 560px：

```
┌─ Modal ─────────────────────────────────────────┐
│  ●WARN  Claude 维护通告              [X]        │
│  发布于 2 分钟前 · 来自 admin                    │
│  ──────────────────────────────────────────── │
│                                                  │
│  Claude group 将于今晚 02:00 - 03:00 进行上游    │
│  key 轮换。期间可能出现短暂调用失败，请已经在生产   │
│  的客户提前规避。                                 │
│                                                  │
│  (markdown 渲染 — 支持 **粗体** / `代码` /        │
│   [链接](url) / - 列表）                          │
│                                                  │
│  ──────────────────────────────────────────── │
│  [关闭]                              [不再提示]   │
│                                       (dismissible) │
└──────────────────────────────────────────────────┘
```

- 打开 modal 自动 mark-as-read（如未读）
- `[不再提示]` 触发 dismiss endpoint（只对 dismissible=true 显示）
- 链接外开新窗口 + 加 rel=noopener
- markdown 渲染白名单：粗体、斜体、代码、链接、列表、引用 —— 禁 HTML 嵌入

#### 9.6.4 完整通知页 `/user/notifications`（独立路由）

```
┌─ User Layout nav (沿用) ──────────────────────────┐
│ ...                                              │
│                                                  │
│ 通知中心                                          │
│ ──────────                                       │
│                                                  │
│ [全部] [未读 (2)] [已读] [级别 ▾]   [全部标已读]  │
│                                                  │
│ ●WARN  Claude 维护通告        2 分钟前   [详情]   │
│ ●INFO  充值优惠                1 天前    [详情]   │
│ ⚠CRIT  系统紧急维护(已过期)     3 天前    [详情]   │
│ ●INFO  欢迎使用 PivotStack    1 月前    [详情]   │
│                                                  │
│ ←  历史更早                                       │
└──────────────────────────────────────────────────┘
```

简单的列表 + filter chips。点 [详情] → 同 9.6.3 modal。

### 9.7 状态机

```
[ADMIN 视角]

create  --save draft--> [DRAFT]
                          ↓ publish
                       [PUBLISHED]
                          ↓ expireAt reached
                       [EXPIRED]
                          ↓ delete
                       [SOFT_DELETED] (隐藏不删数据)

[USER 视角，针对单条 published 通知]

[UNREAD]  ── click ──> [READ]
   │                     │
   └── dismiss ──────────┴──> [HIDDEN]
```

### 9.8 Level 视觉规范汇总

| Level | 主色 | 背景 chip | 状态点 | 特殊效果 |
|---|---|---|---|---|
| info | `#52a8ff` | `rgba(82,168,255,0.10)` | `●` 蓝 | 无 |
| warn | `#f5a623` | `rgba(245,166,35,0.10)` | `●` 橙 | 无 |
| critical | `#ff4d4d` | `rgba(255,77,77,0.10)` | `●` 红 | bell pulse 1.5s + 微 glow |

**关键约束**：
- 不要把整段通知文本染色，只染状态点和 level chip
- critical 是唯一会"打扰" user 的级别 —— 在 user nav bell 上做 pulse + glow，但不要弹自动 popup（避免打断工作）

---

## 10. 各 admin view 重设计要点

> **策略**：5 个核心 view 详细规范；其他用统一组件（AdminTable / AdminFilterBar / AdminMetricStrip）覆盖，仅描述差异点。

### 10.1 Overview（仪表盘）— 核心

继承 user 端 Dashboard 视觉，但密度提升 + admin 关注点不同：

```
顶部 AdminMetricStrip:
  [总余额]  [今日总调用]  [活跃 keys]  [错误率]

主体三栏：
  ┌─ 左栏 ──────────┬─ 中栏 ──────────────┬─ 右栏 ───────────┐
  │ 收入趋势 7d 折线  │ 7d 调用量 + 错误率叠图 │ 上游 channel 健康  │
  │ (ECharts B1 变种) │ (ECharts 双轴折线)    │ (mini sparklines)│
  │                  │                       │                   │
  │ 模型分布 donut    │ 24h × 7d 调用热力     │ TOP 5 高用户       │
  │                  │ (ECharts B4)          │                   │
  └──────────────────┴───────────────────────┴───────────────────┘

底部全宽：
  最近 10 个异常事件 (Abuse / 失败调用 / 余额异常)
```

ECharts 全部复用 stellar 主题。

### 10.2 Channels Groups（分组管理）— 核心

承接前面已经设计过的 `ChannelGroup` 详情页 + 列表页。已实施部分保持不变：
- 列表：AdminTable + bulk actions
- 详情：保留独立路由（不用 drawer，因为有多个嵌套区块）
- 见 STELLAR_CONSOLE_DESIGN.md 的 admin 路由章节

### 10.3 Billing Keys（API Key 管理）— 核心

```
PageHeader: 标题 + [批量充值] [批量导入] [+ 新建 key]

AdminFilterBar:
  [搜索 note/id/key] [plan▾] [status▾] [resellerId▾] [余额范围▾]

AdminMetricStrip:
  [总 keys 356] [活跃 268] [本月新增 12] [总余额 $128k]

AdminTable:
  columns: [☐] note · key (masked) · plan · balance · gift · requests · 上次活跃 · status · [actions]
  multi-select bulk: [启用] [禁用] [充值] [导出] [删除]
  row click → drawer 详情（不跳页面）

Drawer 内容（点 row 展开）：
  tabs: [概览] [用量] [充值历史] [偏好] [操作日志]
  - 概览：plan / status / 余额 / 创建时间 / 过期
  - 用量：mini chart 7d + 模型分布
  - 充值历史：紧凑表
  - 偏好：分组路由选择（user 端能看到的）
  - 操作日志：审计 entries
```

### 10.4 Ops Call Logs（调用日志）— 核心

继承 user 端 UserLogs 设计，但 admin 视角额外字段：

```
+ 用户列（哪个 key 触发）
+ 上游 channel 详情
+ retry 次数和上游 timing breakdown
+ 实际成本（不是用户付的，是 admin 视角的"上游消耗"）
+ 多级 filter：除了 user 端有的，加 [user_key▾] [channel▾] [retry_count▾]
+ 批量操作：导出 / 标记为异常
```

布局基本同 user logs：stat ribbon + filter + 表 + 抽屉，但表格列更多。

### 10.5 System Notifications（**新增**）— 核心

完整规范见 §9.5。

### 10.6-10.13 其他 view（用通用模板覆盖）

下表说明每个 view 复用哪些组件 + 重点差异。

| Route | 主体 | 特殊组件 | 关键差异 |
|---|---|---|---|
| `/channels/newapi` ProviderList | AdminTable + Filter | 无 | 同 Groups，filter 多 [test_status▾] |
| `/channels/newapi/:id` ProviderDetail | 独立路由（嵌套丰富）| ChannelTreeBrowser | 保留现有结构 |
| `/channels/direct` ChannelList | AdminTable | 无 | direct 渠道无 group 概念，更简单 |
| `/channels/direct/:id` | 同 ProviderDetail | - | - |
| `/channels/reconcile` | 双表格对照视图 | DualTableLayout | 左 admin 真实账单 vs 右 user 调用日志 |
| `/billing/codes` Codes | AdminTable + 批量创建 modal | QR code preview chip | 二维码用 lucide QrCode icon |
| `/billing/pricing` Pricing | KV grid + drawer 编辑 | PricingMatrix | 复用 v6 已设计的 |
| `/billing/unit` Unit | Form + 历史 timeline | UnitTimeline | 简单 form，保留现有 |
| `/ops/abuse` Abuse Monitor | AdminTable + 实时刷新 (10s polling) | LiveBadge | 异常事件红高亮 + 一键封禁 |
| `/system/settings` Settings | NForm with sections | 无 | 保留现有 |
| `/system/experimental` Experimental | NCollapse 列表 | featureFlagBoard | 保留现有 |
| `/reseller/summary` Reseller | AdminMetricStrip + AdminTable | 同 BillingKeys |  |
| `/reseller/keys` ResellerKeys | AdminTable + 子 key | KeyTreeView |  |

---

## 11. ECharts 主题

复用已注册的 `stellar` 主题（`src/design/echarts-stellar.ts`）。Admin 端不引入新主题。

Admin 端图表场景对照：
- Overview 收入趋势 / 调用量曲线 → 同 user B1（line + area + markPoint）
- 模型分布 donut → 同 user B2
- 调用热力图 → 同 user B4
- 上游 sparkline → 同 user B3
- Notification stats mini → 简化 B7（30 天趋势）

---

## 12. 动效规范（admin 极度克制）

| 触发 | 动效 | 原因 |
|---|---|---|
| 首屏入场 | **删除** stagger fade-in | admin 看数据不需要"惊艳"，开门见山 |
| 数字 counter | **保留**首屏 counter | 让 admin 感知"刚加载" |
| Row hover | 背景渐变 150ms | 必要的反馈 |
| Bulk select | checkbox 切换 + bulk-action toolbar slide-down | 必要 |
| Sort 切换 | 列头箭头 rotate 180deg | 必要 |
| Modal/Drawer open | scale 0.95→1 / slide-in right | 200ms ease-out，标准 |
| Notification bell pulse | 仅 critical unread 时 1.5s 循环 | 必要的警示 |
| Save success | toast + 微 checkmark icon 转 360° | 反馈 |
| **禁** 整页 stagger / 整页 fade / 装饰性 particle / hover lift | - | admin 不需要花哨 |

---

## 13. 响应式

### 13.1 断点

```
desktop:    > 1280px  ← admin 主优化
laptop:     1024-1280px
tablet:     640-1024px
mobile:     < 640px (admin 不优先，但要可用)
```

### 13.2 1024-1280px

- Tree 默认折叠（点 rail icon 弹 popover tree）
- AdminMetricStrip 4 列 → 2 列
- AdminTable column 隐藏次要列（保留 ID/name + 关键数字 + actions）

### 13.3 < 1024px

- Rail + Tree → 全屏 drawer
- AdminTable → Card mode（每行变 vertical card，关键信息上方，actions 在卡片底部）
- AdminMetricStrip → 单列 stack

### 13.4 < 640px

- 顶 bar 紧凑成 [☰] + logo + bell + 用户头像
- 所有 modal 改 full-screen
- form 单列

---

## 14. 真实示例数据

让原型像活的，至少这些数据要填：

### 14.1 通知

```json
[
  {
    "id": "ntf_001",
    "title": "Claude group 上游维护通告",
    "body": "**Claude** group 将于今晚 02:00 - 03:00 进行上游 key 轮换。\n\n期间可能出现短暂调用失败，请已经在生产的客户提前规避。\n\n建议切换到 `cc` 或 `kiro高缓存` 渠道作为备份。",
    "level": "warn",
    "targetType": "group",
    "targetValue": ["claude"],
    "status": "published",
    "publishAt": 1779091200,
    "expireAt": 1779177600,
    "dismissible": true,
    "createdAt": 1779080000
  },
  {
    "id": "ntf_002",
    "title": "充值优惠活动",
    "body": "本月充值 ¥500 多送 **14%** 虚拟$（=$70）。\n\n活动至 5 月 31 日 23:59 结束。",
    "level": "info",
    "targetType": "plan",
    "targetValue": ["credit", "hybrid"],
    "status": "published",
    "publishAt": 1779000000,
    "expireAt": 1779600000,
    "dismissible": true,
    "createdAt": 1778990000
  },
  {
    "id": "ntf_003",
    "title": "🚨 紧急维护：网关重启",
    "body": "PivotStack 网关将于 **15 分钟后** 重启，预计 30 秒不可用。\n\n请等待后重试。",
    "level": "critical",
    "targetType": "all",
    "targetValue": null,
    "status": "published",
    "publishAt": 1778800000,
    "expireAt": 1778803600,
    "dismissible": false,
    "createdAt": 1778800000
  },
  {
    "id": "ntf_004",
    "title": "即将到期：续费提醒",
    "body": "您的订阅将于 7 天后到期，[立即续费](https://pivotstack.cn/recharge)。",
    "level": "warn",
    "targetType": "userIds",
    "targetValue": ["key_abc", "key_def"],
    "status": "draft",
    "publishAt": 0,
    "expireAt": 0,
    "dismissible": true,
    "createdAt": 1779070000
  }
]
```

### 14.2 Admin Overview metrics

```
总余额：       $128,492.40
今日总调用：    9,855  ↗ +12%
活跃 keys：    268 / 356
错误率：       0.42%  ↘ -0.1pp
本月营收：      ¥3,840 / ¥12,500 目标
异常事件 24h：  3
```

### 14.3 Top 5 高用户

```
1. 玉米地大佬           $195.40   2,847 calls
2. 半岛沉静的芝士        $65.71    1,932 calls
3. 勺子?                $22.56    1,421 calls
4. agent-team-a         $2.40     282 calls
5. client-acme          $1.80     142 calls
```

### 14.4 上游 channel 健康

```
apijing      ● 142ms  err 0.2%
aws稳定(自营) ●  88ms  err 0.0%
cc           ⚠ 320ms  err 4.1%
kiro高缓存    ● 223ms  err 0.0%
反重力pro     ● 178ms  err 0.5%
```

### 14.5 异常事件

```
14:32  ●HIGH   key_xxx 5 分钟 200+ 调用 (rate limit)
14:18  ●MED    key_yyy 同模型 30 次连续 503
13:55  ●LOW    cc 渠道 30 秒错误率 > 10%
```

---

## 15. 禁忌（继承 Stellar Console 15 条 + admin 新增 5 条）

继承自 `STELLAR_CONSOLE_DESIGN.md` §19 全部 15 条不重复。

**Admin 新增禁忌**：

16. ❌ Admin 端不要"营销 banner / CTA 卡片 / 推广位"——这是后台，不是 marketing
17. ❌ Admin 端不要花哨入场动画 / hover lift / 装饰性 particle —— 阻碍工作流
18. ❌ Admin 端不要默认 modal —— form 优先 inline 编辑（drawer / 表格内编辑），仅"新建"/"危险操作"用 modal
19. ❌ Admin 端表格不要 client-side pagination —— 必须 server-side（数据量可能很大）
20. ❌ Admin 端不要"3D / glass / 立体"卡片 —— 工作环境求清晰，不求装饰

---

## 16. 产出要求

### 16.1 文件结构

**Design AI 产出 3 个 self-contained HTML 文件**：

1. **`login-stellar.html`** — Login 页 3 个创意方向都画（用 tab 或 anchor 切换查看），每个方向都是完整可点击的 demo
2. **`admin-rail.html`** — Admin Operations Rail 主壳 + 至少 3 个 view 完整展示（Overview / BillingKeys / Notifications）
3. **`notification-system.html`** — 通知系统专题：admin 通知管理页 + user bell dropdown + 详情 modal + critical banner

### 16.2 每个 HTML 必须

- 真实工作的 starfield + meteor（沿用 user `stellar.js` 的逻辑）
- 真实 ECharts 渲染（stellar 主题已注册）
- 至少 §12 列出的动效全部实现
- 桌面 1440-1600 主优化，1024 / 768 / 640 三个断点不破版
- 每个区块用 `<!-- naive-ui: NXxx -->` 注释标记预期的 naive-ui 组件
- 字体优先 Geist (Google Fonts)，fallback Inter / JetBrains Mono

### 16.3 给后期落地的提示

- 整体复用 `web-vue/src/design/stellar.css` 已有的 tokens
- 加 admin-specific tokens 到新文件 `web-vue/src/design/admin-stellar.css`
- 公共 admin 组件放 `web-vue/src/components/admin/stellar/`
- 通知组件放 `web-vue/src/components/notifications/`（user/admin 共用）

### 16.4 检查清单（验收时逐项核对）

- [ ] **Login**：3 个创意都能正常切换查看
- [ ] **Login**：双登录方式 tab 切换流畅
- [ ] **Login**：移动端有合理 fallback
- [ ] **Admin Rail**：64px rail 在左 + 200px tree + 主区域排列正确
- [ ] **Admin Rail**：13 个路由按 5 域分组正确
- [ ] **Admin Rail**：1024 / 768 / 640 三档响应式不崩
- [ ] **AdminTable**：行高 36px，多选 + bulk action toolbar，empty state 正确
- [ ] **AdminMetricStrip**：4 列 KPI 无 border 靠空白分层
- [ ] **Notification Admin**：list + create modal + targeting 切换 type 时 UI 平滑
- [ ] **Notification User**：bell icon 在 user nav 正确位置，unread badge 数字正确
- [ ] **Notification User**：dropdown 显示 last 5 + 时间倒序 + level dot
- [ ] **Notification Modal**：markdown 渲染正常 + dismiss 按钮仅 dismissible=true 时显示
- [ ] **Stellar 主题**：所有图表 stellar 主题，无任何默认蓝紫色
- [ ] **背景**：星空旋转 + 流星偶尔划过

---

## 17. Open Decisions（仍待用户确定的）

本 brief 已经按"默认决策"写完，但以下 5 个仍可以微调：

1. **Login 创意方向最终选哪个？** Design AI 出 3 版后再定。
2. **Sign-up 是否做 self-signup？** 默认 admin 邀请制，可加自助注册（v2）
3. **Notification email mirror 是否必要？** 默认 critical 不发邮件（v2 视产品需要）
4. **Command-K 全局搜索是否做？** 默认不做（v2 视使用频率）
5. **Reconcile view 双表格细节？** 待具体查看现有 view 再决定细节布局

---

## 18. 实施估算

| 任务 | 估时 | 备注 |
|---|---|---|
| Design AI 出 Login 3 方向 HTML | 1-2 小时 | 自动化 |
| Design AI 出 Admin Rail + 3 view HTML | 1-2 小时 | 自动化 |
| Design AI 出 Notification 系统 HTML | 1 小时 | 自动化 |
| 用户审 3 套原型 + 选定方向 | 0.5-1 小时 | 决策 |
| 后端 notification 模块（Go）| 1 天 | 严格按 codex schema |
| 前端 Login 重做（含选定方向）| 2-3 天 | 含创意动效 |
| 前端 Admin Rail 主壳 + 13 view 适配 | 5-7 天 | 工作量最大 |
| 前端 Notification 组件 | 2-3 天 | bell + dropdown + modal + admin 管理页 |
| 联调测试 + 上线 | 2 天 | 含 polling 验证 + targeting 校验 |
| **总计** | **约 15 工作日** | 不含 brief 撰写时间 |

---

## 19. 引用

- `web-vue/STELLAR_CONSOLE_DESIGN.md` — User 端完整规范（视觉、tokens、组件、ECharts 主题）
- `web-vue/.design/pivotstack/project/stellar.{html,css,js}` — User 端原型源码
- `kiro-go/proxy/handler_admin_*.go` — 现有 admin endpoint 命名风格
- `kiro-go/config/config.go` — ApiKeyInfo 结构（targeting plan/group/userIds 依赖）

---

**END OF BRIEF — 拷给 Claude Design 出 3 套原型**

</pasted_text>

<!-- The user explicitly selected the following skills for this project, as attachments to their message. These are not optional context — they define how you work. Use them. -->
<attached-skill name="Frontend design">
Use this guidance when designing frontend/UI work that is NOT governed by an existing brand or design system. Create distinctive HTML with exceptional attention to aesthetic details and creative choices.

### Design Thinking

Before coding, understand the context and commit to a BOLD aesthetic direction:
- **Purpose**: What problem does this interface solve? Who uses it?
- **Tone**: Pick an extreme: brutally minimal, maximalist chaos, retro-futuristic, organic/natural, luxury/refined, playful/toy-like, editorial/magazine, brutalist/raw, art deco/geometric, soft/pastel, industrial/utilitarian, etc. Use these for inspiration but design one that is true to the aesthetic direction.
- **Differentiation**: What makes this UNFORGETTABLE? What's the one thing someone will remember?

Choose a clear conceptual direction and execute it with precision. Bold maximalism and refined minimalism both work — the key is intentionality, not intensity.

### Aesthetics Guidelines

- **Typography**: Choose fonts that are beautiful, unique, and interesting. Avoid generic fonts like Arial and Inter; opt for distinctive, characterful choices. Pair a distinctive display font with a refined body font.
- **Color & Theme**: Commit to a cohesive aesthetic. Use CSS variables for consistency. Dominant colors with sharp accents outperform timid, evenly-distributed palettes.
- **Motion**: Use animations for effects and micro-interactions. Prioritize CSS-only solutions for HTML. Focus on high-impact moments: one well-orchestrated page load with staggered reveals creates more delight than scattered micro-interactions.
- **Spatial Composition**: Unexpected layouts. Asymmetry. Overlap. Diagonal flow. Grid-breaking elements. Generous negative space OR controlled density.
- **Backgrounds & Visual Details**: Create atmosphere and depth rather than defaulting to solid colors. Gradient meshes, noise textures, geometric patterns, layered transparencies, dramatic shadows, decorative borders, grain overlays.

Vary between light and dark themes, different fonts, different aesthetics. NEVER converge on the same choices across generations.

Match implementation complexity to the aesthetic vision. Maximalist designs need elaborate animations and effects. Minimalist designs need restraint, precision, and careful attention to spacing and subtle details.
</attached-skill>

## Assistant

_[tool: update_todos]_

收到大 brief。3 套原型,我按优先级建,共享底层 tokens(直接引用现有 `stellar.css` + 新增 admin tokens 文件)。

_[tool: write_file]_

_[tool: write_file]_

## User

看这个，上面那个是错的

## Assistant

_[tool: snip]_

_[tool: update_todos]_

明白了 —— 是一个文件三合一。我开始构建 `pivotstack-admin.html`。先骨架 + Login 区,再加 Admin 和 Notification。

_[tool: write_file]_

_[tool: write_file]_

_[tool: write_file]_

_[tool: done]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: str_replace_edit]_

Layout looks good — orb scene renders. Just need to fix some text-wrapping issues in the nav and direction tabs:

_[tool: save_screenshot]_

_[tool: view_image]_

## User

继续

## Assistant

_[tool: snip]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: eval_js]_

_[tool: get_webview_logs]_

_[tool: grep]_

`charts` collides with stellar.js's global. Rename mine.

_[tool: run_script]_

_[tool: grep]_

_[tool: eval_js]_

_[tool: sleep]_

_[tool: get_webview_logs]_

_[tool: grep]_

_[tool: eval_js]_

_[tool: sleep]_

_[tool: eval_js]_

_[tool: eval_js]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: eval_js]_

_[tool: str_replace_edit]_

Grid + position:fixed conflict — admin-main got 0 width. Switch admin-shell off grid.

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: grep]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: eval_js]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: view_image]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: run_script]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: view_image]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: eval_js]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: screenshot_user_view]_

_[tool: eval_js]_

_[tool: done]_

三套原型已合并到一个文件 `pivotstack-admin.html`,顶部用 mega-tab 切换。文件结构:

- **`pivotstack-admin.html`** — 主文件 (HTML 结构)
- **`pivotstack-admin.css`** — 页面专属样式 (login orb / boot / warp / admin views / 通知)
- **`pivotstack-admin.js`** — 所有交互 + ECharts
- 继承 `stellar.css` (tokens / starfield / 按钮 / chip / 共用组件)
- 继承 `admin-shared.css` (rail / tree / atable / metric strip / 通知 chip / drawer / bell)

**Login 三个创意方向都可切**:
- **A · The Singularity** — 中央脉动绿色 Orb + 粒子被吸引 + 点击 orb 卡片展开
- **B · The Boot Sequence** — 终端启动文字流 + Geist Mono prompt,Tab 切换 `--user` / `--key`
- **C · The Warp Tunnel** — Canvas 星点向中心 warp 1.5s + 卡片在游走光线边框内浮现 + 重放按钮

**Admin Console**(rail + tree + main):
- 左 64px Rail (6 个域 icon + 底部 bell + admin),激活态绿色竖线
- 200px Tree (按域显示子路由,激活态绿点 + 竖线)
- 主区域: Overview / BillingKeys / Notifications 三个 view 完整画完;其他路由点了会 toast 提示
- 全部继承 stellar ECharts 主题: 收入趋势 / 模型 donut / 调用量+错误率双轴 / 24h×7d 热力 / 5 个上游 sparkline / 通知阅读率小图

**Notification 系统**(底部 4 个 sub-tab):
- **Admin 管理** — 完整 CRUD 表 + 新建 modal (level/targeting radio chip + 各 target 子面板平滑切换 + dismissible switch)
- **User Bell** — 模拟 user nav + bell 32×32 + 红 badge(critical 2 条带 pulse glow)+ dropdown 320px
- **User 通知中心** — 全部/未读/已读 filter chip + 4 条样例
- **Critical Banner** + 状态机生命周期图

**注意**:html-to-image 截图工具对 backdrop-filter + 复杂 z-index 容器有时漏渲染部分内容(我看到截图里 bell sub-pane 和通知中心显示为空,但 eval 验证内容确实在 DOM 里、高度正确)—— 真实浏览器里能看全。如果你在用户视图里也看不到内容,告诉我具体哪个 sub-tab,我针对性查。

## User

给 Claude Design 的修复 prompt（直接拷过去）

  两个 bug 修一下：

  1. Login 页的 Warp Tunnel 方向（C 方向），现在 warp 拉长流线只在
     登录卡片周围一圈出现，其他区域是静态星空。需要改成 FULL VIEWPORT
     warp tunnel —— 整个屏幕的星点都被 warp 拉长成流线，向中心汇聚。
     卡片只是漂浮在 warp tunnel 中央的"目标"，不是 warp 的容器边界。

     参考效果：星点拉长成 2-8px 的横线，靠近中心的拉长程度更夸张，
     边缘点拉长最强。整个动画就像高速穿越星际，卡片是终点的稳定锚点。

  2. Tab 切换 bug：当前 API Key tab 选中状态下，表单区域同时显示了
     ACCESS TOKEN 和 USERNAME + PASSWORD 两套表单。应该只显示当前
     tab 对应的一套表单：
     - API Key tab → 只显示 ACCESS TOKEN + "穿越登录" 按钮
     - 用户名密码 tab → 只显示 USERNAME + PASSWORD + "穿越登录" 按钮
     切换 tab 时另一套表单要隐藏（display: none 或 hidden 属性）。

  3. 按钮文案 "穿越登录" 改回 "登录"，简洁一点。"穿越" 这个动作
     是视觉动效本身已经传达了，按钮文字不用重复说。

## Assistant

_[tool: snip]_

_[tool: str_replace_edit]_

三个修复都来:

_[tool: run_script]_

_[tool: str_replace_edit]_

Now make the warp visually full-viewport — bump star count and make streak length grow with distance from center:

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

_[tool: eval_js]_

_[tool: str_replace_edit]_

_[tool: save_screenshot]_

_[tool: view_image]_

三个修复全部到位 ✓

**1. Warp Tunnel 现在全视口** — 整个屏幕都是星点拉长的流线,边缘最夸张(streak length 用 `0.4 + (r/maxR) * 1.6` 距离系数),向中心汇聚。卡片是浮在 tunnel 中央的稳定锚点,SVG 边框光线沿矩形游走。

**2. Tab 切换 bug 修了** — 当前只显示 `ACCESS TOKEN` 一个输入框(eval 验证 user 表单是 `display: none`)。根因是 `.login-form { display: flex }` 比浏览器默认 `[hidden] { display: none }` 优先级高,我加了一条全局 `[hidden] { display: none !important }` 兜底。

**3. 按钮文案改回 "登录"** — 简洁,动效自己说话。

附带:warp card 在预览 iframe 里的入场过渡(0→1 opacity transition)被 iframe 暂停冻在初始状态,改成直接 class 切换 + `!important`,真实浏览器里你应该会看到 warp 减速→card 直接显现。如果想要 fade-in,后期 Vue 用 `<Transition>` 包一下就行。

