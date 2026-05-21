<script setup lang="ts">
// 渠道展开后显示该 group 下所有按量模型的具体价格。
// 入参：modelRows（来自 useChannelGroupContext）+ optional sellMultiplier
import { computed, h } from 'vue'
import { NDataTable, NEmpty, type DataTableColumns } from 'naive-ui'
import { type ModelPriceRow } from '../../../composables/useChannelGroupContext'
import { formatPrice } from '../../../composables/useNewAPIPricing'

const props = defineProps<{
  modelRows: ModelPriceRow[]
  // 卖价倍率（admin 视角：markup × yuanPerUpstreamDollar × pivotStackDollarsPerYuan）。传 0 / 不传 = 只显入价
  sellMultiplier?: number
}>()

const showSell = computed(() => typeof props.sellMultiplier === 'number' && props.sellMultiplier > 0)
const mult = computed(() => props.sellMultiplier || 1)

interface DisplayRow extends ModelPriceRow {
  sellInput: number
  sellOutput: number
  sellCache: number
}

const rows = computed<DisplayRow[]>(() => {
  return props.modelRows.map(r => ({
    ...r,
    sellInput: r.inputPrice * mult.value,
    sellOutput: r.outputPrice * mult.value,
    sellCache: r.cachePrice * mult.value,
  }))
})

const columns = computed<DataTableColumns<DisplayRow>>(() => {
  const base: DataTableColumns<DisplayRow> = [
    { title: '模型', key: 'name', width: 260, ellipsis: { tooltip: true }, render: r => h('span', { class: 'mono' }, r.name) },
    { title: '上游入价 in', key: 'inputPrice', width: 110, align: 'center', render: r => h('span', { class: 'mono dim' }, formatPrice(r.inputPrice)) },
    { title: '上游入价 out', key: 'outputPrice', width: 110, align: 'center', render: r => h('span', { class: 'mono dim' }, formatPrice(r.outputPrice)) },
    { title: '缓存读', key: 'cachePrice', width: 100, align: 'center', render: r => h('span', { class: 'mono dim small' }, r.cachePrice > 0 ? formatPrice(r.cachePrice) : '-') },
  ]
  if (showSell.value) {
    base.push(
      { title: '对外卖价 in', key: 'sellInput', width: 120, align: 'center', render: r => h('span', { class: 'mono sell' }, formatPrice(r.sellInput)) },
      { title: '对外卖价 out', key: 'sellOutput', width: 120, align: 'center', render: r => h('span', { class: 'mono sell' }, formatPrice(r.sellOutput)) },
    )
  }
  base.push({
    title: 'model_ratio',
    key: 'modelRatio',
    width: 100,
    align: 'center',
    render: r => h('span', { class: 'mono dim small' }, `${r.modelRatio}×`),
  })
  return base
})
</script>

<template>
  <div class="model-detail">
    <header class="model-detail__head">
      <span class="model-detail__title">该渠道按量模型明细</span>
      <span class="model-detail__sub">{{ modelRows.length }} 个 · 按 model_ratio 降序</span>
    </header>
    <n-data-table
      v-if="modelRows.length"
      :columns="columns"
      :data="rows"
      :row-key="(r: DisplayRow) => r.name"
      :pagination="modelRows.length > 8 ? { pageSize: 8 } : false"
      size="small"
      :scroll-x="800"
    />
    <n-empty v-else description="该渠道无按量模型（可能是按次计费 group）" />
  </div>
</template>

<style scoped>
.model-detail { padding: 10px 16px; background: rgba(0,0,0,0.25); border-radius: 4px; }
.model-detail__head { display: flex; align-items: baseline; gap: 10px; margin-bottom: 8px; }
.model-detail__title { color: #ededed; font-size: 12px; font-weight: 500; }
.model-detail__sub { color: #707070; font-size: 11px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; font-variant-numeric: tabular-nums; font-size: 12px; color: #ededed; }
.dim { color: #a3a3a3; }
.small { font-size: 11px; }
.sell { color: #0bd470; }
</style>
