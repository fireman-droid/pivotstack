<script setup lang="ts">
// 通用 ECharts mount 容器。
// props:
//   option: 完整 echarts option（外部计算好）
//   height: 容器高度（默认 200px）
//   noAnimation: 跳过动画（mini sparkline 用）
// expose: rerender(opt) 手动重画
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { echarts, ensureStellarTheme } from '../../../design/echarts-stellar'

const props = withDefaults(defineProps<{
  option: any
  height?: number | string
  noAnimation?: boolean
}>(), {
  height: 200,
  noAnimation: false,
})

const root = ref<HTMLDivElement | null>(null)
let chart: any = null
let resizeObs: ResizeObserver | null = null

function init() {
  if (!root.value || chart) return
  ensureStellarTheme()
  chart = echarts.init(root.value, 'stellar')
  applyOption()
  resizeObs = new ResizeObserver(() => chart && chart.resize())
  resizeObs.observe(root.value)
}

function applyOption() {
  if (!chart) return
  const opt = props.noAnimation ? { ...props.option, animation: false } : props.option
  chart.setOption(opt, true)
}

watch(() => props.option, () => applyOption(), { deep: true })

onMounted(() => {
  // 等下一帧确保 DOM 有尺寸
  requestAnimationFrame(() => {
    init()
    // 双保险：tab 切换时父容器可能刚显示
    setTimeout(() => chart && chart.resize(), 50)
  })
})

onBeforeUnmount(() => {
  if (resizeObs) resizeObs.disconnect()
  if (chart) {
    chart.dispose()
    chart = null
  }
})

defineExpose({
  getInstance: () => chart,
  resize: () => chart && chart.resize(),
})
</script>

<template>
  <div
    ref="root"
    class="echarts-panel"
    :style="{ height: typeof height === 'number' ? height + 'px' : height }"
  />
</template>

<style>
.echarts-panel { width: 100%; }
</style>
