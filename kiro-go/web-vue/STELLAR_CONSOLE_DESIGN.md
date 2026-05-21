# Stellar Console — PivotStack User 端设计规范

> **一份完整的 brief，给 design AI 直接出 HTML 原型用。从色卡到 ECharts 到每一个区块的像素尺寸，全部明确。**
>
> **目标产物**：单个 self-contained HTML 文件，包含 5 个 tab 的可切换原型，跑起来即可看效果。
>
> **后期落地**：原型审完后，按"naive-ui 组件对照表"1:1 替换成 Vue 3 + naive-ui 实际组件。

---

## 0. 一句话定调

> **"在 Cloudflare Dashboard 和 SuperGrok 起始页之间找平衡 —— 信息密度像专业 API 网关后台，背景调性像深空控制塔。"**

主题代号叫 **Stellar Console**（星·控制台）。每次拿不准时回头读这句。

---

## 1. 产品上下文

**PivotStack** 是 AI API 算力中转 + 转售平台。User 在这里做的事：

- 看自己的钱、配额、用量
- 看流量被路由到哪个上游渠道（claude 组 → aws稳定 / cc / kiro高缓存 等）
- 看上游健康度（延迟、错误率、是否可用）
- 拉调用日志 debug
- 复制 API endpoint / key 接入自己的项目
- （可选）作为 reseller 转售给子用户

**不是 chat 产品，是 live console**。视觉语言要像 Cloudflare、Vercel、Stripe、Anthropic Console 那样有专业感。

---

## 2. 技术栈对齐（重要 — 决定原型怎么出）

PivotStack 前端实际跑的技术栈：

| 技术 | 版本 | 备注 |
|---|---|---|
| Vue | 3.x | Composition API + `<script setup lang="ts">` |
| 组件库 | **naive-ui** | 全局 dark theme + theme overrides |
| 图标 | **lucide-vue-next** | 一律用 lucide，不混用其他图标库 |
| 图表 | **ECharts 5** | 必须注册自定义 `stellar` 主题 |
| 构建 | Vite |  |
| 样式 | scoped CSS + CSS vars |  |

**design AI 出原型时按以下规则**：
1. 用 **HTML + CSS（或 Tailwind CDN）模拟 naive-ui 的视觉**，**不要**引入真正的 naive-ui（会跑不起来）
2. 出原型时每个区块用 `<!-- naive-ui: NButton, type="primary" -->` 之类的注释标记，让后期 1:1 替换有据可依
3. 图表必须真用 ECharts 5（CDN）+ 注册下面提供的 `stellar` 主题
4. 字体优先 Geist（Google Fonts），fallback 到 Inter / JetBrains Mono

### 2.1 naive-ui 组件对照表（原型 → Vue 替换时用）

| 原型出现的元素 | 后期替换成 naive-ui |
|---|---|
| nav 中的导航按钮 | router-link + 自定义 class（不用 NMenu） |
| 顶部右上余额胶囊 | `<NTag size="small" :bordered="false" type="success">` |
| primary 主按钮 | `<NButton type="primary" size="small">` |
| 透明描边次要按钮 | `<NButton size="small">`（默认 outline 风） |
| 图标按钮 | `<NButton quaternary size="small">` + `<lucide-icon />` |
| 输入框 | `<NInput size="small">` |
| 数字输入框 | `<NInputNumber size="small">` |
| 下拉选 / multi-select | `<NSelect size="small">` |
| 表格 | `<NDataTable :columns :data :row-key size="small">` |
| 单选 chip | `<NRadioGroup>` + `<NRadio>` |
| 开关 | `<NSwitch size="small">` |
| 状态 tag | `<NTag size="small" :bordered="false" :type="...">` |
| 提示弹层 | `<NTooltip>` |
| 抽屉 | `<NDrawer>` + `<NDrawerContent>` |
| modal | `<NModal>` |
| 二次确认 | `<NPopconfirm>` |
| 折叠面板 | `<NCollapse>` + `<NCollapseItem>` |
| 标签页 | `<NTabs>` + `<NTabPane>` |
| 空状态 | `<NEmpty>` 或自定义 EmptyState 组件 |
| 加载 | `<NSpin>` |
| 分割线 | `<NDivider>` |
| 日期选择 | `<NDatePicker size="small">` |
| Toast 消息 | `useMessage()`（注入式） |

---

## 3. 色彩系统（直接对齐项目 `src/design/tokens.ts`）

### 3.1 色卡

| Token | Hex / RGBA | 用途 |
|---|---|---|
| `bg.base` | `#000000` | 主背景（星空层底） |
| `bg.surface` | `#0a0a0a` | sidebar / 抽屉 / modal 背景 |
| `bg.elevated` | `#141414` | popover / tooltip 背景 |
| `bg.overlay` | `#1a1a1a` | 二级浮层 |
| `bg.hover` | `rgba(255,255,255,0.05)` | 默认 hover |
| `surface.hover` | `rgba(255,255,255,0.06)` | 卡片/行 hover |
| `surface.pressed` | `rgba(255,255,255,0.10)` | 按下态 |
| `border.default` | `rgba(255,255,255,0.08)` | 默认 1px hairline |
| `border.strong` | `rgba(255,255,255,0.16)` | 强分割 |
| `border.hover` | `rgba(255,255,255,0.24)` | hover 描边 |
| `divider` | `rgba(255,255,255,0.10)` | divider |
| `focus.ring` | `rgba(82,168,255,0.32)` | 焦点环 |
| `primary.hover` | `#dadada` | 白色按钮 hover |
| `primary.pressed` | `#bdbdbd` | 白色按钮按下 |
| `text.primary` | `#ededed` | 主文字、关键数字 |
| `text.secondary` | `#a1a1a1` | 次级文字 |
| `text.tertiary` | `#707070` | 弱化信息、metadata |
| `text.disabled` | `#4d4d4d` | 禁用 |
| `text.inverse` | `#0a0a0a` | 白底上的黑字（primary button） |
| `success` | `#0bd470` | 健康、正常、有钱 |
| `warning` | `#f5a623` | 警告、低余额 |
| `error` | `#ff4d4d` | 错误、down |
| `info` | `#52a8ff` | 信息蓝（少用） |

### 3.2 颜色使用规则

- 主背景永远是 `#000000`，不要灰
- **禁用任何渐变色**（线性/径向都不要），仅 sparkline 下方 fade-out 例外
- success 绿 `#0bd470` 用得稀缺、克制 —— 只标记"健康"和"关键正向数字"
- 不要把整段文字染绿，绿色只用在**单个状态点**或**单个关键数字**
- hover 背景永远用 `rgba(255,255,255,0.05-0.06)`，**绝对不能用 #ffffff/#ededed**

---

## 4. 字体系统

### 4.1 字族

```css
--font-sans:    "Geist Sans", Inter, -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", sans-serif;
--font-mono:    "Geist Mono", "JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, monospace;
--font-display: "Geist Sans", Inter, sans-serif;
```

数字相关元素 **必须** 加 `font-variant-numeric: tabular-nums`。

### 4.2 字号系统

| Class | size | weight | letter-spacing | line-height | 用途 |
|---|---|---|---|---|---|
| `.t-hero-xl` | 64px | 500 | -0.025em | 1.0 | 余额超大数字 |
| `.t-hero-lg` | 40px | 500 | -0.02em | 1.05 | 次级大数字 |
| `.t-display` | 22px | 600 | -0.01em | 1.2 | 区块标题（"上游健康"） |
| `.t-h` | 16px | 600 | -0.005em | 1.3 | 次级标题 |
| `.t-body-strong` | 14px | 500 | 0 | 1.5 | 强调主体 |
| `.t-body` | 13px | 400 | 0 | 1.5 | 默认正文 |
| `.t-label` | 11px | 500 | 0.06em（uppercase）| 1.2 | label / metadata（**全大写**） |
| `.t-micro` | 10px | 600 | 0.08em | 1 | badge 内容 |
| `.t-num` | 13px mono | 400 | 0 (tabular-nums) | 1.5 | 表格数字 |
| `.t-num-strong` | 14px mono | 500 | 0 (tabular-nums) | 1.4 | 重要数字 |

### 4.3 排版规则

- hero 数字下方紧跟一行 `.t-label`（如 `余额 BALANCE`），不在数字上面
- 区块标题 `.t-display` 下方常配 `.t-label` 作小标语
- 中文混排时，中文字号比英文字号大 1-2 级

---

## 5. 间距 / 圆角

### 5.1 间距 token（4px 基准，对齐项目）

```
--space-1: 4px
--space-2: 8px
--space-3: 12px
--space-4: 16px
--space-5: 20px
--space-6: 24px
--space-8: 32px
--space-10: 40px
--space-12: 48px
--space-16: 64px
```

### 5.2 圆角

```
--radius-xs: 2px    （chip 内嵌徽标）
--radius-sm: 4px    （chip / tag / 按钮）
--radius-md: 6px    （input / button 默认）
--radius-lg: 8px    （抽屉 / modal）
--radius-xl: 12px   （仅 hero 用）
```
**禁止 12px 以上的大圆角**（避免卡通感）。

### 5.3 阴影

**全产品禁用 box-shadow**。仅 modal / drawer 浮层例外：`0 8px 32px rgba(0,0,0,0.5)`。

---

## 6. 全局背景：星空层

这是 Stellar Console 主题的灵魂。

### 6.1 实现规范

1. 全屏容器 `position: fixed; inset: 0; z-index: -1;`
2. 星点分两层：
   - **底层 800-1200 个静态星**：多层 `radial-gradient` 在 `::before` 合成。点大小 1-2px，颜色 `rgba(255,255,255,0.45)`，分布偏向四角，中心带（正文区域）密度降 70%
   - **顶层 30-50 个动态星**：每个用一个 `<i class="star">` 元素，独立 twinkle 动画
3. twinkle keyframes：
   ```css
   @keyframes twinkle {
     0%, 100% { opacity: 0.25; transform: scale(1); }
     50%      { opacity: 0.95; transform: scale(1.3); }
   }
   ```
   `animation-duration: 3-8s` 随机、`animation-delay: 0-5s` 随机
4. 5-8 个"大星"用 3-4px + 微弱径向 glow：`box-shadow: 0 0 8px rgba(255,255,255,0.4)`
5. 鼠标移动**不做视差**（控制台主题求稳）

### 6.2 不可以做

- 流星划过 / 星点连线 / 星点变色 / 大量蓝色或紫色星
- 星点不能遮挡正文（与大字号数字重叠时变花）

---

## 7. 全局组件视觉规范

### 7.1 StatusDot

```
●  6px 圆点 + 字色 = 该状态颜色
   后接 8px gap + 状态文字（.t-label uppercase）
```

| 状态 | 颜色 | 用途 |
|---|---|---|
| `●` 绿 | `#0bd470` | HEALTHY / ONLINE / OK |
| `●` 黄 | `#f5a623` | WARN / DEGRADED |
| `●` 红 | `#ff4d4d` | DOWN / ERROR |
| `●` 灰 | `#707070` | DISABLED / IDLE |

**只有绿色 healthy 状态点带 pulse 动画**（2s 循环 opacity 0.3↔1）。

### 7.2 Chip

胶囊形容器，用于：tag、status badge、router selection、过滤项

```
border-radius: 4px
padding: 4px 8px
font-size: 11px / weight 500
height: 22px
display: inline-flex
```

四种变体：

| 变体 | 样式 |
|---|---|
| default | `color: #a1a1a1; background: rgba(255,255,255,0.06)` |
| selected | `color: #0bd470; background: rgba(11,212,112,0.10); inset shadow 1px rgba(11,212,112,0.4)` |
| warning | `color: #f5a623; background: rgba(245,166,35,0.10)` |
| mono | `font-family: Geist Mono; color: #ededed; background: rgba(255,255,255,0.04)` |

### 7.3 Metric Tile

bento grid 里的单元，**没有边框**：

```
padding: var(--space-6)
hover: background rgba(255,255,255,0.02)
内部结构：
  ┌─────────────────────────────┐
  │ 余额 BALANCE          ●Live │  ← .t-label + 右上次要标识
  │                             │
  │ $9.79                       │  ← .t-hero-xl 或 .t-hero-lg
  │   ≈ ¥9.79 · 够 ~12 天        │  ← .t-body 灰 #707070
  │                             │
  │ ─────────                   │  ← 1px hairline
  │ 充值 $8.50 · 赠送 $1.29      │  ← .t-label tertiary
  └─────────────────────────────┘
```

### 7.4 Table Row

```
height: 44px
padding: 0 12px
border-bottom: 1px solid rgba(255,255,255,0.06)
hover: background rgba(255,255,255,0.04); cursor: pointer
```

对齐：
- 时间 / id：左对齐 mono
- 文字描述：左对齐 sans
- 数字：右对齐 mono tabular-nums
- 状态：居中 status dot

### 7.5 Sparkline（mini 折线）

```
高度 32-48px / 宽度自适应
线宽 1px / 颜色按状态（#0bd470 或 #f5a623 或 #ff4d4d）
节点 2px 圆点（仅最新节点点亮 + glow）
下方 fill 渐变 rgba(color,0.15) → rgba(color,0)
不要坐标轴 / grid / tooltip（保持极简）
```

### 7.6 Bar（横向条）

```
高度 6-8px / border-radius: 3px
背景 rgba(255,255,255,0.06)
fill 默认 #ededed，可指定颜色
不要数字标注（数字在 bar 旁边）
```

### 7.7 Mono Value（可复制值）

```
font-family: Geist Mono / 13px / #ededed
background: rgba(255,255,255,0.04)
padding: 6px 10px / radius: 4px
后接 lucide Copy icon button (11px)
hover: background rgba(255,255,255,0.08)
复制后图标变 Check，停留 1.5s 再变回
```

### 7.8 Button

3 种变体：

| 变体 | 样式 |
|---|---|
| **primary** | 背景 `#ededed` 文字 `#0a0a0a`，hover 背景 `#dadada`，按下 `#bdbdbd`。仅主 CTA |
| **secondary** | 背景透明，1px hairline 描边，文字 `#ededed`，hover 描边 `rgba(255,255,255,0.18)` |
| **ghost** | 背景透明无描边，文字 `#a1a1a1`，hover 文字 `#ededed` + 背景 `rgba(255,255,255,0.04)` |

```
height: 28px (small) / 32px (default) / 36px (large)
padding: 0 12px / radius: 6px
font: 13px weight 500
```

---

## 8. 顶部 Nav

### 8.1 容器

```
高度: 56px
背景: rgba(0,0,0,0.85) + backdrop-filter: blur(12px)
底部: 1px hairline #1a1a1a
sticky top: 0 / z-index: 100
内部 max-width: 1440px / padding: 0 32px
```

### 8.2 结构

```
┌────────────────────────────────────────────────────────────────────┐
│ [P] PivotStack [USER]    概览 充值 日志 接入 代理      ●Live  $9.79 [⏻]│
└────────────────────────────────────────────────────────────────────┘
```

**左侧 brand 区（240px）**：
- 24px 方块 logo：背景 `#ededed`，文字 `P` 黑色 700 weight 12px
- "PivotStack" 14px weight 600 letter-spacing -0.01em
- "USER" 10px label uppercase letter-spacing 0.12em color `#707070`，1px 描边小框

**中部 nav（flex 1，居中）**：
- 5 个 nav 项：**概览 / 充值 / 调用日志 / 接入示例 / 代理**（代理仅 reseller 可见）
- 每项：
  ```
  display: inline-flex; gap: 6px (icon + label)
  padding: 6px 12px / height: 32px
  font: 13px / weight 400
  color: #a1a1a1
  border-radius: 4px
  hover: color #ededed; background rgba(255,255,255,0.04)
  active: color #ededed; 底部 2px 横线 #0bd470 (绝对定位，居中可缩放展开)
  ```

**右侧 actions 区**（gap 12px）：

1. **Live indicator**：
   ```
   ● 6px 绿点（pulse 2s）
   "Live" 11px label uppercase color #a1a1a1
   "· 12s ago" 11px label color #707070
   ```
2. **余额 chip**：
   ```
   font-family: Geist Mono / 13px weight 500
   color: #ededed (余额 < 阈值时变 #f5a623)
   background: rgba(255,255,255,0.06)
   padding: 6px 12px / radius: 4px
   前缀: lucide DollarSign 12px
   ```
3. **退出按钮**：ghost button + LogOut icon 14px

---

## 9. 主区三栏布局

```
主容器:
  max-width: 1440px
  margin: 0 auto
  padding: 32px 32px 64px

三栏 grid:
  display: grid
  grid-template-columns: minmax(280px, 1fr) minmax(380px, 1.4fr) minmax(280px, 1fr)
  gap: 24px

  左栏 (1fr):   余额 / 主指标
  中栏 (1.4fr): 主图表 / 业务核心
  右栏 (1fr):   实时状态 / 健康度

栏内 grid-template-rows: auto auto auto 流式堆叠
模块间 32px 间距 + 可选 1px hairline 分割
```

---

## 10. Tab 1：概览 `/user/dashboard`

### 10.1 左栏（1fr）：余额与配额

**模块 A：余额 Hero**
- `.t-label` `余额 BALANCE`  右侧 `●Live` micro indicator
- 32px gap
- 主余额：`$9.79` 用 `.t-hero-xl` 64px，色 `#ededed`（余额 ≤ 0 时变 `#ff4d4d`）
- 8px gap
- 副文字：`≈ ¥9.79  ·  够 ~12 天` 用 `.t-body` color `#707070`
- 24px gap + 1px hairline
- 充值/赠送拆分：`充值 $8.50  ·  赠送 $1.29` 用 `.t-label` color `#a1a1a1`
- 16px gap
- primary button "充值 +"，宽度 100%

**模块 B：今日 / 本月 / 累计 metrics**
- `.t-label` `USAGE`
- 16px gap
- 3 行垂直堆叠，每行 horizontal：
  ```
  左 (50px 固宽): 今日 / 本月 / 累计 (.t-body color #a1a1a1)
  右: 数字 .t-num-strong + trend chip
     142  [↗ +12%]
     4,283  [$42.50]
     12,847
  ```
- 每行底部 1px hairline

### 10.2 中栏（1.4fr）：核心业务可视化

**模块 A：7 日调用趋势**（ECharts 大图，详见 §16）
- 顶部：`.t-display` `调用趋势` + `.t-label` `LAST 7 DAYS` | 右 `.t-num-strong` 总和 `743 calls` + `.t-label` `TOTAL`
- 24px gap
- ECharts line chart 高度 200px，配置见 §16 B1

**模块 B：模型分布**（ECharts donut + HTML legend，详见 §16 B2）
- 顶部 `.t-display` `模型分布`
- 16px gap
- 左半 160px donut + 右半 HTML legend

**模块 C：分组路由配置**（核心业务交互）
- 顶部 `.t-display` `分组路由` + 8px gap + `.t-label` `PREFERENCES · 配置走哪个上游`
- 每个 group 一行（56px 高）：
  ```
  左 (200px):
    .t-body-strong "claude"
    .t-label tertiary "10x cache · 4 渠道可选"
  中 (flex 1):
    chip 横排 gap 6px
    第一个 selected: [● aws稳定]
    其他 default: [cc] [kiro高缓存] [反重力pro]
  右 (24px):
    ChevronRight 11px color #707070 (hover #ededed)
  ```
- 行间 1px hairline，整行 hover 背景 `rgba(255,255,255,0.02)`

### 10.3 右栏（1fr）：上游健康监控

**模块 A：上游健康度**
- `.t-display` `上游健康` + `.t-label` `REAL-TIME`
- 每个上游一行（48px）：
  ```
  状态点 ● 6px (绿/黄/红)
  16px gap
  channel 名 (.t-body-strong, 100px)
  延迟 (.t-num color #a1a1a1, 64px 右对齐)
  错误率 chip (warn if >2%)
  右侧 48×24 mini sparkline (ECharts，§16 B3)
  ```

**模块 B：API 接入快速复制**
- `.t-display` `接入`
- `.t-label` `ENDPOINT` + Mono value with copy: `https://api.pivotstack.cn/v1`
- 8px gap
- `.t-label` `API KEY` + Mono value with copy: `sk-•••••••••••e3a2`
- 12px gap
- secondary button `查看接入示例 →`

**模块 C：系统公告（仅有内容时显示）**
- 1px hairline 顶
- padding 12px
- `.t-label` `NOTICE`
- `.t-body` 内容
- 例：`cc 渠道今日延迟偏高（>300ms），延迟敏感请切换 aws稳定`

### 10.4 底部全宽：24h × 7d 调用热力图

**位置**：三栏下方，gap 32px。

- `.t-display` `调用热力图` + `.t-label` `WHEN YOU ARE BUSY`
- 24px gap
- ECharts heatmap 高 180px 全宽，配置见 §16 B4
- 底部自定义 legend：`少 ░░▒▒▓▓██ 多`

### 10.5 底部全宽：最近调用 Ticker

- `.t-display` `最近调用` + `.t-label` `LATEST 10`
- 24px gap
- 表格 10 行（每行 44px）：
  ```
  时间 (mono 13px #707070, 80px)
  request_id 后8位 + 复制 (mono 11px, 100px)
  模型 (.t-body 13px #ededed, 160px)
  上游 (chip mono, 100px)
  in/out tokens (mono num, 各 64px 右对齐)
  花费 (mono num green, 72px 右对齐)
  延迟 (mono num color by speed, 72px 右对齐)
  状态点 (居中, 32px)
  ```
- hover 行高亮 + 右侧出 ChevronRight 暗示可点
- 新行从顶部 slide-in (200ms ease-out)，整列上移

---

## 11. Tab 2：充值 `/user/recharge`

### 11.1 布局：1.2fr | 1fr | 1fr

### 11.2 左栏：充值入口

**模块 A：金额输入**
- `.t-display` `充值`
- `.t-label` `选择金额`
- 4 个预设金额 chip 横排 (gap 6px)：`[¥10] [¥50] [¥100] [¥500]`，selected 时绿底
- 8px gap
- 自定义金额输入框（高度 44px / font 16px mono / 前缀 ¥）
- 实时显示折算：`= $10.00 虚拟$`

**模块 B：支付方式**
- `.t-label` `支付方式`
- 三个方式横排，每个 tile 120×80：
  ```
  ┌──────────┐
  │ [icon]   │  ← lucide 图标
  │ 微信支付  │  ← .t-body-strong
  │ 推荐     │  ← .t-label tertiary
  └──────────┘
  ```
  selected: 1px 绿描边 + 角标
- 微信 / 支付宝 / 卡券码

**模块 C：兑换码**
- `.t-label` `或使用兑换码`
- mono input + secondary button `兑换`

**底部 CTA**：primary button 100% 宽 `确认充值 ¥50 →`

### 11.3 中栏：当前余额 + 套餐推荐

**模块 A：当前余额**
- `$9.79` 用 `.t-hero-lg` 40px
- 同 dashboard 余额 hero 结构但稍小

**模块 B：充值优惠表**
- `.t-display` `套餐`
- 4 行表格：
  ```
  ¥10  →  $10  (无优惠)
  ¥50  →  $52  (赠送 $2 · 4%)
  ¥100 →  $108 (赠送 $8 · 8%)
  ¥500 →  $570 (赠送 $70 · 14%)  [最划算]
  ```
- 最划算行：selected 绿底 + 角标 `BEST`

### 11.4 右栏：充值历史

- `.t-display` `充值记录` + `.t-label` `RECENT`
- 表格行高 36px：
  ```
  时间 (mono 11px, 80px)
  方式 (chip, 64px)
  金额 (mono num green, 72px)
  状态点
  ```
- 显示最近 8 条，底部 ghost button `查看全部 →`

---

## 12. Tab 3：调用日志 `/user/logs`

### 12.1 布局：上 stat ribbon + filter ribbon + 左 7fr 表 + 右 3fr 抽屉

### 12.2 顶部 Stat Ribbon

3 个 metric tile 横排，每个 320×96（详见 §16 B5）：
- **今日调用数** `142` + 24h 调用柱状（mini ECharts bar）
- **平均延迟** `142ms` + 24h 延迟折线
- **错误率** `0.4%` + 24h 错误尖刺图（多数 0 偶尔红色突起）

### 12.3 Filter Ribbon

```
高度 64px / padding 16px 0
横向 flex，左对齐 + 右对齐分组：

左:
  [时间] dropdown (今天 / 7d / 30d / 自定义)
  [模型] multi-select
  [状态] (全部 / 仅错误)
  [上游] dropdown
  搜索框 (mono, placeholder "搜索 request_id 或关键词")

右:
  [导出 CSV] secondary
  [自动刷新 ●] switch (on 时绿)
```

### 12.4 主表格

列：
```
时间        (mono 12px, 100px) - HH:MM:SS
request_id  (mono 11px, 100px) - 后 8 位 + 复制
模型        (.t-body, 160px)
上游        (chip mono, 100px)
in          (mono num, 64px)
out         (mono num, 64px)
total       (mono num strong, 72px)
花费        (mono num green, 72px)
延迟        (mono num color by speed, 72px)
状态        (status dot, 32px)
```

行高 40px，整行点击 → 右抽屉滑出。

### 12.5 右抽屉（点行展开）

```
宽 420px / 从右滑入 300ms ease-out
背景 #0a0a0a
内容:
  顶部: X 关闭 + request_id 完整 mono
  时间戳完整 + 时区
  模型 / 实际路由上游 / 是否 retry
  token 拆分: input / cached / output
  cost breakdown: input cost + output cost + cache discount
  错误时: 完整 error message + 错误码（可点查文档）
  延迟分布直方图 (mini ECharts, §16 B6)
  请求体预览 (折叠 json 高亮)
  响应体预览 (折叠)
```

---

## 13. Tab 4：接入示例 `/user/api-docs`

### 13.1 布局：上 hero + 左 4fr 代码 + 右 3fr 测试

### 13.2 Hero 区（200px 高）

```
.t-label "你的 ENDPOINT"
.t-hero-lg mono "https://api.pivotstack.cn/v1"  [复制]
24px gap
.t-label "你的 API KEY"
.t-hero-lg mono "sk-•••••••••••••••e3a2"  [显示完整] [复制]
```

### 13.3 左侧：代码示例

- 顶部 tab 切换：`curl` / `python (openai-sdk)` / `python (anthropic-sdk)` / `node` / `cline 配置`
  - 用 naive-ui NTabs 视觉模拟
  - selected tab 底部 2px 绿线
- 代码区：
  ```
  height 自适应 / max 480px
  background rgba(255,255,255,0.02)
  radius: 6px / padding: 16px
  font: Geist Mono 12.5px / line-height 1.6
  ```
- 高亮配色：
  - keyword: `#52a8ff`
  - string: `#0bd470`
  - comment: `#707070`
  - 其他: `#ededed`
- 右上角悬浮 `复制` button

### 13.4 右侧：测试 + 推荐

**模块 A：测试连通**
- `.t-display` `测试连通`
- 表单：模型 dropdown + prompt textarea (2 行) + primary `发送`
- 发送后下方显示真实响应（折叠 json + 延迟 + 花费）

**模块 B：你可用的模型**
- `.t-display` `你可用的模型`
- 表格：
  ```
  模型名             input/M  output/M  推荐场景
  opus-4-7         $10      $50       长任务 / agent
  sonnet-4-6       $6       $30       日常对话
  haiku-4-5        $2       $10       高频快任务
  ```

---

## 14. Tab 5：代理 `/user/reseller`（reseller 才可见）

### 14.1 布局：1fr | 1.4fr | 1fr

### 14.2 左栏：池子状态

3 个 metric tile 垂直堆叠：
- **我的余额** `$9.79`
- **已分发** `$12.40` 给 8 个子 key
- **累计利润** `$3.21`  `[+24% vs 上月]`

### 14.3 中栏：子 key 列表

```
顶部:
  .t-display "我的子 key" + .t-label "8 个"
  右上角: primary button "+ 创建子 key"
表格:
  alias           (.t-body, 160px)
  余额            (mono num, 80px)
  累计调用         (mono num, 80px)
  最近活跃         (mono 11px tertiary, 100px) - 相对时间
  状态点
  操作            (icon buttons: 充值 / 暂停 / 删除)
```

### 14.4 右栏：利润 + 操作

**模块 A：利润 30 日趋势**（ECharts area + 均值 markLine，§16 B7）
- 高度 160px

**模块 B：子 key 余额分布**（ECharts horizontal bar，§16 B8）
- 每条 bar 末端显示金额 label
- 余额 0 的 bar 用 `#4d4d4d`

**模块 C：批量操作**
- secondary `批量充值`
- secondary `导出明细`
- ghost `代理规则设置`

---

## 15. ECharts 集成

### 15.1 全局 Stellar Theme（必须先注册）

```html
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
<script>
const stellarTheme = {
  color: ['#0bd470', '#52a8ff', '#f5a623', '#ff4d4d', '#a1a1a1', '#ededed'],
  backgroundColor: 'transparent',
  textStyle: {
    color: '#a1a1a1',
    fontFamily: 'Geist Sans, Inter, "PingFang SC", sans-serif',
    fontSize: 11,
  },
  title: { show: false },
  legend: {
    textStyle: { color: '#a1a1a1', fontSize: 11 },
    icon: 'circle',
    itemWidth: 6,
    itemHeight: 6,
    itemGap: 16,
  },
  grid: {
    left: 0, right: 0, top: 12, bottom: 20,
    containLabel: true,
    borderColor: 'transparent',
  },
  xAxis: {
    axisLine: { lineStyle: { color: 'rgba(255,255,255,0.08)' } },
    axisTick: { show: false },
    axisLabel: { color: '#707070', fontSize: 10, margin: 12 },
    splitLine: { show: false },
  },
  yAxis: {
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: { color: '#707070', fontSize: 10 },
    splitLine: { lineStyle: { color: 'rgba(255,255,255,0.04)', type: 'dashed' } },
  },
  tooltip: {
    backgroundColor: 'rgba(10,10,10,0.96)',
    borderColor: 'rgba(255,255,255,0.10)',
    borderWidth: 1,
    padding: [8, 12],
    textStyle: {
      color: '#ededed',
      fontSize: 12,
      fontFamily: 'Geist Mono, monospace',
    },
    extraCssText: 'backdrop-filter: blur(8px); border-radius: 6px;',
  },
  line: {
    smooth: true,
    lineStyle: { width: 1.5 },
    symbol: 'circle',
    symbolSize: 3,
    showSymbol: false,
    emphasis: { itemStyle: { borderColor: '#000', borderWidth: 2 } },
  },
  bar: { itemStyle: { borderRadius: [3, 3, 0, 0] } },
};
echarts.registerTheme('stellar', stellarTheme);
</script>
```

**所有图表必须用 `echarts.init(dom, 'stellar')` 初始化。**

### 15.2 B1 — 7 日调用趋势（Tab 1 中栏）

- 容器 高 200px / 全栏宽
- 类型 `line` + `areaStyle`
- 数据 `[82, 95, 120, 88, 142, 156, 142]` / x `['周一'..'周日']`
- 关键配置：

```js
series: [{
  type: 'line',
  smooth: true,
  data: [82, 95, 120, 88, 142, 156, 142],
  lineStyle: { width: 1.5, color: '#0bd470' },
  showSymbol: false,
  emphasis: { showSymbol: true, itemStyle: { color: '#0bd470', borderColor: '#000', borderWidth: 2 } },
  areaStyle: {
    color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
      { offset: 0, color: 'rgba(11,212,112,0.20)' },
      { offset: 1, color: 'rgba(11,212,112,0)' },
    ]),
  },
  markPoint: {
    data: [{ type: 'max', name: '峰值' }],
    symbol: 'circle', symbolSize: 4,
    itemStyle: { color: '#0bd470' },
    label: {
      show: true, position: 'top',
      color: '#ededed', fontSize: 11, fontFamily: 'Geist Mono',
      formatter: '{c}',
    },
  },
}]
```
- tooltip 例：`周五  142 calls  ↗ +18% vs 周四`
- 不显示 legend / title

### 15.3 B2 — 模型分布 Donut（Tab 1 中栏）

- 容器高 240px
- **左 160px donut**：
  - `pie` type / `radius: ['55%', '78%']`
  - 中央 graphic 文字：
    ```js
    graphic: {
      type: 'group',
      left: 'center', top: 'middle',
      children: [
        { type: 'text', style: { text: '850', fill: '#ededed', font: '600 28px Geist Mono', textAlign: 'center' }, top: -8 },
        { type: 'text', style: { text: 'CALLS', fill: '#707070', font: '500 9px Geist Sans', letterSpacing: 1 }, top: 24 },
      ],
    }
    ```
  - 扇区间隙 `itemStyle.borderColor: '#000', borderWidth: 2`
  - hover 扇区 `scale(1.04)`
- **右 HTML legend**：
  ```
  [● #0bd470] opus-4-7       528 (62%)  $25.40
  [● #52a8ff] sonnet-4-6     238 (28%)  $11.20
  [● #f5a623] haiku-4-5       84 (10%)   $0.85
  ```
  hover legend 行 → 高亮 donut 对应扇区（`dispatchAction({type:'highlight', dataIndex:i})`)

### 15.4 B3 — 上游延迟 Mini Sparkline（Tab 1 右栏）

- 容器 48×24px
- 极简 line，无 axis 无 tooltip：

```js
{
  grid: { left: 0, right: 0, top: 2, bottom: 2 },
  xAxis: { show: false, type: 'category', data: Array(24).fill('') },
  yAxis: { show: false },
  series: [{
    type: 'line', smooth: true, symbol: 'none',
    data: latencyArray, // 24 个延迟值
    lineStyle: { width: 1, color: channelColor },
    areaStyle: { color: channelColorTransparent },
  }],
  tooltip: { show: false },
  animation: false,
}
```
- 颜色按健康度：绿/黄/红

### 15.5 B4 — 24h × 7d 调用热力图（Tab 1 底部）

- 容器 高 180px / 全宽（跨三栏）
- 类型 `heatmap`
- 数据 7×24 = 168 个点

```js
{
  xAxis: { type: 'category', data: Array.from({length:24}, (_,i) => String(i).padStart(2,'0')), splitArea: { show: false } },
  yAxis: { type: 'category', data: ['周日','周六','周五','周四','周三','周二','周一'] },
  visualMap: {
    min: 0, max: 30, calculable: false,
    show: false,
    inRange: { color: ['rgba(11,212,112,0.05)', 'rgba(11,212,112,0.4)', '#0bd470'] },
  },
  series: [{
    type: 'heatmap',
    data: heatmapData,
    itemStyle: { borderRadius: 2, borderColor: '#000', borderWidth: 1 },
    emphasis: { itemStyle: { borderColor: '#0bd470', borderWidth: 1 } },
  }]
}
```

示例数据生成：
```js
const heatmapData = [];
for (let d = 0; d < 7; d++) {
  for (let h = 0; h < 24; h++) {
    let base = h >= 9 && h <= 23 ? 8 : 1;
    let val = Math.floor(base + Math.random() * 15);
    heatmapData.push([h, d, val]);
  }
}
```
- 底部自定义 HTML legend：`少 ░░▒▒▓▓██ 多`

### 15.6 B5 — Stat Ribbon Mini Charts（Tab 3 顶部）

3 个 320×96 的 metric tile，每个内部含 mini chart：

- **今日调用**：大数字 `142` + 下方 24h 柱状（24 bar / 4px wide / `#0bd470` 透明梯度）
- **平均延迟**：大数字 `142ms` + 24h 延迟折线
- **错误率**：大数字 `0.4%` + 24h 错误尖刺图（`#ff4d4d` bar，多数 0）

每个图同 B3 风格：极简 / animation:false / 无 tooltip 无 legend。

### 15.7 B6 — 延迟分布直方图（Tab 3 抽屉）

- 容器高 100px
- 类型 `bar` / 10 个 bucket
- 配置：bar 默认 `#a1a1a1`，本次调用所在 bucket 高亮 `#0bd470` + label `本次`

```js
series: [{
  type: 'bar',
  data: bucketData.map((v, i) => ({
    value: v,
    itemStyle: { color: i === currentBucket ? '#0bd470' : '#a1a1a1' },
    label: i === currentBucket ? {
      show: true, position: 'top',
      color: '#0bd470', fontSize: 10, formatter: '本次',
    } : { show: false },
  })),
}]
```

### 15.8 B7 — 利润 30 日趋势（Tab 5 右栏）

- 容器高 160px
- 类型 `line` + `areaStyle`（同 B1）
- 数据 30 天利润，加均值 markLine：

```js
markLine: {
  symbol: 'none',
  lineStyle: { color: 'rgba(255,255,255,0.20)', type: 'dashed', width: 1 },
  label: {
    position: 'end',
    color: '#a1a1a1', fontSize: 10, fontFamily: 'Geist Mono',
    formatter: '均值 ${c}',
  },
  data: [{ type: 'average' }],
}
```

### 15.9 B8 — 子 key 余额 Horizontal Bar（Tab 5 右栏）

- 高度自适应（按 key 数）/ 宽 280px
- 类型 `bar` horizontal（`yAxis.type: 'category'`）
- 每条 bar 末端 label 显示金额
- 颜色统一 `#0bd470`，余额 0 用 `#4d4d4d`

### 15.10 ECharts 交互规则

1. 所有 tooltip 用 mono 字体显示数字
2. **不要 legend toggle 让用户隐藏系列**（控制台不该让数据可关）
3. 图表区域 `backgroundColor: transparent`
4. emphasis 颜色比常态亮 1 档，**不能用纯白**
5. **必须监听 `window.resize` 调用 `chart.resize()`**
6. 数据为空：显示 `.t-label tertiary "暂无数据 · 调用一次就会出现"`，**不用** echarts 自带 noData
7. 加载状态：`chart.showLoading('default', { text: '', color: '#0bd470', maskColor: 'transparent', spinnerRadius: 8, lineWidth: 1.5 })`
8. mini sparkline（B3 / B5 那种）`animation: false`
9. 大图表（B1 / B2 / B4 / B7）保留 400-600ms 动画

---

## 16. 微动效清单

| 触发 | 动效 | 时长 | 缓动 |
|---|---|---|---|
| 页面入场 | 三栏区块 stagger fade-in + slide-up 8px | 400ms total / 60ms stagger | cubic-bezier(0.16, 1, 0.3, 1) |
| 余额数字首屏 | counter 0 → 实际值 | 800ms | ease-out |
| Live indicator | 绿点 opacity 0.4 ↔ 1.0 pulse | 2s infinite | ease-in-out |
| Nav item hover | 底部 2px 横线从中心向两侧展开 | 200ms | ease |
| Nav item active | 横线常驻 #0bd470 | - | - |
| 按钮 hover | 背景渐变 + 微 lift translateY(-1px) | 150ms | ease |
| Chip 选中 | scale 0.96 → 1.02 → 1 弹性 | 240ms | cubic-bezier(0.34, 1.56, 0.64, 1) |
| 行 hover | 背景 rgba(255,255,255,0) → 0.04 | 150ms | ease |
| 抽屉滑出 | translateX 100% → 0 + 背景 0 → 1 | 300ms | cubic-bezier(0.16, 1, 0.3, 1) |
| 复制按钮点击 | Copy 旋转 360° + 1.5s 后变 Check 再变回 | 400ms 旋转 | ease |
| Ticker 新行 | 顶部 slide-down + 整体上移 | 200ms | ease-out |
| 数字变化 | 旧值 fade-out + 新值 slide-up | 240ms | ease |
| Status dot pulse | 仅绿色 healthy 0.3 ↔ 1 opacity | 2s | ease-in-out |
| ECharts 大图入场 | 自带 line/area 抽出动画 | 600ms | -- |

---

## 17. 响应式

### 17.1 断点

```
mobile:     < 640px
tablet:     640 - 1024px
desktop:    1024 - 1440px
wide:       > 1440px
```

### 17.2 < 1024 变化

- 三栏 grid → 单栏 stack
- 顶部 nav → sticky 底部 tabbar / nav 文字换 icon-only
- hero 数字 → 48px
- 表格 → card 视图（每行一个 card）

### 17.3 < 640

- padding 全部 16px
- hero 数字 → 36px
- 表格 / sparkline 横向滚动
- 抽屉 → 全屏 modal

---

## 18. 真实示例数据

让原型像活的，必须填这套：

### 顶部 nav
- 余额 chip：`$9.79`
- Live：`12s ago`

### Tab 1 概览
- 余额 hero：`$9.79`，折合 `≈ ¥9.79`，`够 12 天`
- 充值/赠送：`$8.50 + $1.29`
- 今日：`142 calls` `↗ +12%`
- 本月：`4,283 calls` `$42.50`
- 累计：`12,847 calls`

- 7 天趋势数据：`[82, 95, 120, 88, 142, 156, 142]`

- 模型分布：
  ```
  claude-opus-4-7     528 calls   62%   $25.40
  claude-sonnet-4-6   238 calls   28%   $11.20
  claude-haiku-4-5     84 calls   10%    $0.85
  ```

- 分组路由：
  - `claude` → 当前 aws稳定，可选 cc / kiro高缓存 / 反重力pro
  - `GPT 系列` → 当前 apijing-codex

- 上游健康：
  ```
  apijing       ● 142ms   err 0.2%
  aws稳定(自营)  ●  88ms   err 0.0%
  cc           ⚠ 320ms   err 4.1%   ← 黄
  kiro高缓存    ● 223ms   err 0.0%
  反重力pro     ● 178ms   err 0.5%
  ```

- 接入：
  - endpoint：`https://api.pivotstack.cn/v1`
  - api key：`sk-XYZ987654321ABCDEFGHe3a2`

- 最近调用（10 条）：
  ```
  10:42:31  abc12345  opus-4-7     aws稳定  3,512/728/4,240   $0.12   142ms  ●
  10:39:18  def67890  sonnet-4-6   cc         824/252/1,076   $0.03    95ms  ●
  10:38:02  ghi23456  opus-4-7     aws稳定  6,521/1,234/7,755 $0.25   188ms  ●
  10:32:45  jkl78901  opus-4-7     aws稳定  2,432/689/3,121   $0.09   126ms  ●
  10:30:11  mno34567  haiku-4-5    apijing    482/158/640    $0.006   62ms  ●
  10:28:53  pqr89012  opus-4-7     aws稳定  4,820/982/5,802   $0.18   204ms  ●
  10:25:01  stu45678  sonnet-4-6   cc       1,201/320/1,521   $0.05   287ms  ⚠
  10:22:14  vwx90123  opus-4-7     aws稳定  3,889/712/4,601   $0.14   151ms  ●
  10:19:47  yz123456  haiku-4-5    apijing    321/98/419     $0.004   58ms  ●
  10:18:32  abc78901  opus-4-7     aws稳定  5,210/1,012/6,222 $0.22   178ms  ●
  ```

### Tab 2 充值
- 充值历史：
  ```
  2026-05-15  微信   +¥50.00   $52    ✓
  2026-05-12  兑换码 +$10.00   $10    ✓
  2026-05-08  支付宝 +¥100.00  $108   ✓
  ```

### Tab 5 代理（reseller）
- 我的池子：`$9.79`
- 已分发：`$12.40` 给 8 个子 key
- 累计利润：`$3.21`
- 子 key 列表：
  ```
  agent-team-a    $2.40  282 calls   2 分钟前   ●
  client-acme     $1.80  142 calls   5 分钟前   ●
  test-sandbox    $0.50   88 calls   2 小时前   ●
  client-zenith   $3.20  421 calls   1 天前     ●
  archive-2026q1  $0.00    0 calls   30 天前    ○
  ```

---

## 19. 禁忌（每条都不能违反）

1. ❌ 不要任何 `border: 1px solid` 把面板框起来 —— 必须靠空白 + 字色分层
2. ❌ 不要 box-shadow（modal 浮层除外）
3. ❌ 不要 hover 用纯白 #ffffff/#ededed 高亮 —— 必须 `rgba(255,255,255,0.04-0.06)`
4. ❌ 不要紫粉橙渐变 / AI SaaS 通病配色
5. ❌ 不要玻璃质感 / blur 卡片
6. ❌ 不要 emoji 装饰（lucide icon 可以）
7. ❌ 不要把整段大文字染绿 —— 绿色只用在状态点和单个关键数字
8. ❌ 数字必须 mono + tabular-nums 对齐
9. ❌ 圆角不超过 12px（避免卡通感）
10. ❌ Tab 切换不能页面跳转刷新 —— 必须平滑过渡
11. ❌ 不要混用 Material / AntD 风图标 —— 全 lucide-react/lucide-vue
12. ❌ 主区域之外不加任何"营销 banner"或"推广位"
13. ❌ ECharts 不能出现默认的蓝紫色 / 白底 tooltip / 内置 legend toggle
14. ❌ 不要让 legend 可点击隐藏系列
15. ❌ 不要做流星 / 星点连线 / 鼠标视差

---

## 20. 产出要求

### 20.1 文件结构

- **单个 HTML 文件**包含 **5 个 tab 的可切换原型**
- 点击 nav 切换主区内容，每个 tab 都完整展示（不是只占位）
- self-contained：CSS 内嵌或 Tailwind CDN，ECharts CDN
- 字体 Google Fonts 加载 Geist Sans + Geist Mono（不可用时 Inter + JetBrains Mono）
- lucide icons：`https://unpkg.com/lucide@latest` 或内联 SVG

### 20.2 必须实现

- [x] 完整工作的星空背景（不是占位 svg）
- [x] 真实工作的 ECharts 图表（B1 / B2 / B3 / B4 / B5 / B6 / B7 / B8 全部）
- [x] 至少这些动效：fade-in 入场 / counter 数字 / status dot pulse / nav active 横线 / 行 hover / chip 选中弹性 / 复制反馈
- [x] 桌面 1440px 主优化，1024 / 768 / 640 三个断点都适配
- [x] 在 HTML 顶部注释：`<!-- PivotStack User Console · Stellar Console theme v1 -->`

### 20.3 给后期落地的提示

每个区块用 `<!-- naive-ui: NDataTable -->` 之类注释标记预期的 naive-ui 组件，方便 1:1 替换。例如：

```html
<!-- naive-ui: NTag size="small" :bordered="false" type="success" -->
<span class="chip chip--selected">● aws稳定</span>
```

### 20.4 检查清单（出完原型我会逐项验）

- [ ] 整页背景是纯黑 + 真实星空（不是 svg 静图）
- [ ] 所有 ECharts 用 stellar 主题，无任何默认蓝紫色
- [ ] tooltip 黑底毛玻璃 + 1px hairline
- [ ] line chart 都开 smooth + areaStyle 渐变
- [ ] donut 中央有 graphic 文字（不是空心）
- [ ] heatmap 颜色梯度淡绿 → 饱和绿
- [ ] 没有任何图表显示 echarts 内置 legend
- [ ] mini sparkline 是 48×24px 这种 mini，**不是** 200px 高大图
- [ ] 24h heatmap x 轴 24 小时 y 轴周一到周日
- [ ] 所有可点行 hover 时背景变色
- [ ] 切 tab 不会页面刷新
- [ ] 1024 / 768 / 640 三个断点不破版

---

**END OF BRIEF · 拷给 design AI 出原型**
