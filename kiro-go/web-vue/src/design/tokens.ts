// PivotStack v6 Design Tokens
// 单一暗色主题（Vercel.com 风），全平台一改全改。
// 不要直接在 .vue 里硬编码颜色 — 用 CSS var 或从这里 import。

export const COLOR = {
  // 背景层（叠层从最底到最浅）
  bgBase: '#000000',
  bgSurface: '#0a0a0a',
  bgElevated: '#141414',
  bgOverlay: '#1a1a1a',
  bgHover: 'rgba(255,255,255,0.05)',

  // 描边
  borderDefault: 'rgba(255,255,255,0.08)',
  borderStrong: 'rgba(255,255,255,0.16)',
  borderHover: 'rgba(255,255,255,0.24)',
  divider: 'rgba(255,255,255,0.10)',
  focusRing: 'rgba(82, 168, 255, 0.32)',

  // 交互态（克制 grey，禁止纯白 hover）
  surfaceHover: 'rgba(255,255,255,0.06)',
  surfacePressed: 'rgba(255,255,255,0.10)',
  primaryHover: '#dadada',
  primaryPressed: '#bdbdbd',

  // 文字
  textPrimary: '#ededed',
  textSecondary: '#a1a1a1',
  textTertiary: '#707070',
  textDisabled: '#4d4d4d',
  textInverse: '#0a0a0a',

  // 状态
  success: '#0bd470',
  warning: '#f5a623',
  error: '#ff4d4d',
  info: '#52a8ff',
} as const

export const GRADIENT = {
  brand: 'linear-gradient(90deg, #FF0080 0%, #7928CA 50%, #0070F3 100%)',
  blueCyan: 'linear-gradient(90deg, #4F46E5 0%, #06B6D4 100%)',
  purplePink: 'linear-gradient(90deg, #BD00FF 0%, #FF0080 100%)',
  warmGlow: 'linear-gradient(90deg, #F5A623 0%, #FF0080 100%)',
} as const

export const RADIUS = {
  xs: 2,
  sm: 4,
  md: 6,
  lg: 8,
  xl: 12,
  pill: 9999,
} as const

export const SPACE = [4, 8, 12, 16, 24, 32, 48, 64] as const

export const TYPE = {
  font: {
    sans: '"Geist Sans", Inter, "PingFang SC", "Microsoft YaHei", sans-serif',
    mono: '"Geist Mono", "JetBrains Mono", ui-monospace, monospace',
  },
  scale: {
    xs: { size: 12, line: 16 }, // tertiary / tag
    sm: { size: 13, line: 20 }, // table data / sidebar
    base: { size: 14, line: 24 }, // body / input
    md: { size: 16, line: 24 }, // subheading
    lg: { size: 20, line: 28 }, // section header
    xl: { size: 24, line: 32 }, // page title / KPI
    '2xl': { size: 32, line: 40 }, // big KPI
    '3xl': { size: 48, line: 48 }, // hero (罕用)
  },
  weight: {
    normal: 400,
    medium: 500,
    semibold: 600,
  },
} as const

export const MOTION = {
  easing: 'cubic-bezier(0.4, 0, 0.2, 1)',
  durFast: '150ms',
  durBase: '200ms',
  durSlow: '300ms',
  reduceMotion: '@media (prefers-reduced-motion: reduce)',
} as const

export const Z_INDEX = {
  base: 0,
  raised: 10,
  sticky: 100,
  drawer: 900,
  modal: 1000,
  toast: 1100,
  tooltip: 1200,
} as const
