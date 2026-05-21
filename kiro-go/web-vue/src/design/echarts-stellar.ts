// ECharts "stellar" theme for PivotStack user console.
// 注册一次（main.ts 启动时），后续 echarts.init(dom, 'stellar') 直接用。
import * as echarts from 'echarts'

const stellarTheme = {
  color: ['#0bd470', '#52a8ff', '#f5a623', '#ff4d4d', '#a1a1a1', '#ededed'],
  backgroundColor: 'transparent',
  textStyle: {
    color: '#a1a1a1',
    fontFamily: 'Geist, Inter, "PingFang SC", sans-serif',
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
    textStyle: { color: '#ededed', fontSize: 12, fontFamily: 'Geist Mono, monospace' },
    extraCssText: 'backdrop-filter: blur(8px); border-radius: 6px;',
  },
  line: {
    smooth: true,
    lineStyle: { width: 1.5 },
    symbol: 'circle',
    symbolSize: 3,
    showSymbol: false,
  },
  bar: { itemStyle: { borderRadius: [3, 3, 0, 0] } },
}

let registered = false
export function ensureStellarTheme() {
  if (registered) return
  echarts.registerTheme('stellar', stellarTheme)
  registered = true
}

export function hexToRgba(hex: string, a: number): string {
  const m = hex.replace('#', '')
  const r = parseInt(m.substring(0, 2), 16)
  const g = parseInt(m.substring(2, 4), 16)
  const b = parseInt(m.substring(4, 6), 16)
  return `rgba(${r},${g},${b},${a})`
}

export { echarts }
