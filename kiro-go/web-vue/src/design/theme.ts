// PivotStack v6 Naive-UI Theme Override
// 单一 dark theme，参 plan §5.2。注入到 NConfigProvider 给全 naive-ui 组件用。

import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'
import { COLOR, RADIUS, TYPE } from './tokens'

export const baseTheme = darkTheme

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    baseColor: COLOR.bgBase,
    bodyColor: COLOR.bgBase,
    cardColor: COLOR.bgSurface,
    popoverColor: COLOR.bgOverlay,

    primaryColor: COLOR.textPrimary,
    primaryColorHover: COLOR.primaryHover,
    primaryColorPressed: COLOR.primaryPressed,
    primaryColorSuppl: COLOR.textPrimary,

    successColor: COLOR.success,
    warningColor: COLOR.warning,
    errorColor: COLOR.error,
    infoColor: COLOR.info,

    textColorBase: COLOR.textPrimary,
    textColor1: COLOR.textPrimary,
    textColor2: COLOR.textSecondary,
    textColor3: COLOR.textTertiary,
    textColorDisabled: COLOR.textDisabled,
    placeholderColor: COLOR.textTertiary,

    dividerColor: COLOR.divider,
    borderColor: COLOR.borderStrong,
    inputColor: COLOR.bgSurface,
    inputColorDisabled: COLOR.bgSurface,
    actionColor: COLOR.bgSurface,
    hoverColor: COLOR.bgHover,

    borderRadius: `${RADIUS.md}px`,
    borderRadiusSmall: `${RADIUS.sm}px`,

    fontFamily: TYPE.font.sans,
    fontFamilyMono: TYPE.font.mono,
    fontSize: `${TYPE.scale.base.size}px`,
    fontWeight: '400',
    fontWeightStrong: '600',
  },
  DataTable: {
    thColor: COLOR.bgSurface,
    thColorHover: COLOR.bgElevated,
    tdColor: 'transparent',
    tdColorHover: COLOR.bgHover,
    borderColor: COLOR.divider,
    fontSizeSmall: `${TYPE.scale.sm.size}px`,
  },
  Tabs: {
    tabTextColorActiveLine: COLOR.textPrimary,
    tabTextColorHoverLine: COLOR.textPrimary,
    barColor: COLOR.textPrimary,
  },
  Button: {
    // Primary 类型：白底黑字，hover/focus 微暗（避免越变越白）
    textColorPrimary: COLOR.textInverse,
    colorPrimary: COLOR.textPrimary,
    colorHoverPrimary: COLOR.primaryHover,
    colorPressedPrimary: COLOR.primaryPressed,
    colorFocusPrimary: COLOR.primaryHover,
    borderPrimary: `1px solid ${COLOR.textPrimary}`,
    borderHoverPrimary: `1px solid ${COLOR.primaryHover}`,
    borderFocusPrimary: `1px solid ${COLOR.primaryHover}`,
    borderPressedPrimary: `1px solid ${COLOR.primaryPressed}`,

    // Default 类型：透明底，hover 升一档 surface
    color: 'transparent',
    colorHover: COLOR.surfaceHover,
    colorPressed: COLOR.surfacePressed,
    colorFocus: COLOR.surfaceHover,
    border: `1px solid ${COLOR.borderStrong}`,
    borderHover: `1px solid ${COLOR.borderHover}`,
    borderFocus: `1px solid ${COLOR.borderHover}`,
    borderPressed: `1px solid ${COLOR.borderHover}`,

    // 文字 hover 用浅灰升级而非 #fff
    textColorHover: COLOR.textPrimary,
    textColorPressed: COLOR.textPrimary,
    textColorFocus: COLOR.textPrimary,
  },
  Input: {
    color: COLOR.bgSurface,
    colorFocus: COLOR.bgSurface,
    border: `1px solid ${COLOR.borderDefault}`,
    borderHover: `1px solid ${COLOR.borderHover}`,
    borderFocus: `1px solid ${COLOR.info}`,
    boxShadowFocus: `0 0 0 2px ${COLOR.focusRing}`,
  },
  // Select trigger 复用 InternalSelection 的 border tokens（naive-ui 内部映射）。
  // 不覆盖时 focus 边框会显示纯白，看着廉价。统一用品牌 info 蓝 + focus ring。
  InternalSelection: {
    color: COLOR.bgSurface,
    colorActive: COLOR.bgSurface,
    border: `1px solid ${COLOR.borderDefault}`,
    borderHover: `1px solid ${COLOR.borderHover}`,
    borderFocus: `1px solid ${COLOR.info}`,
    borderActive: `1px solid ${COLOR.info}`,
    boxShadowFocus: `0 0 0 2px ${COLOR.focusRing}`,
    boxShadowActive: `0 0 0 2px ${COLOR.focusRing}`,
  },
  // NRadio / NCheckbox 聚焦时同样禁纯白
  Radio: {
    boxShadowFocus: `inset 0 0 0 1px ${COLOR.info}, 0 0 0 2px ${COLOR.focusRing}`,
    boxShadowActive: `inset 0 0 0 1px ${COLOR.info}`,
  },
  Card: {
    color: COLOR.bgSurface,
    borderColor: COLOR.borderDefault,
    titleTextColor: COLOR.textPrimary,
  },
  Menu: {
    itemTextColor: COLOR.textSecondary,
    itemTextColorHover: COLOR.textPrimary,
    itemTextColorActive: COLOR.textPrimary,
    itemColorActive: COLOR.bgHover,
    itemColorActiveHover: COLOR.bgHover,
    itemIconColor: COLOR.textTertiary,
    itemIconColorActive: COLOR.textPrimary,
    itemIconColorHover: COLOR.textPrimary,
    arrowColor: COLOR.textTertiary,
    arrowColorActive: COLOR.textPrimary,
  },
  Layout: {
    color: COLOR.bgBase,
    siderColor: COLOR.bgSurface,
    siderBorderColor: COLOR.borderDefault,
    headerColor: COLOR.bgBase,
    headerBorderColor: COLOR.borderDefault,
  },
  Tag: {
    color: COLOR.bgElevated,
    textColor: COLOR.textSecondary,
    border: `1px solid ${COLOR.borderDefault}`,
  },
}
