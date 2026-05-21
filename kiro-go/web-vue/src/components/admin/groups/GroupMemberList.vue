<script setup lang="ts">
import { h } from 'vue'
import { NDataTable, NTag, NButton, NRadio, NEmpty, NSpace, NTooltip, type DataTableColumns } from 'naive-ui'
import { X, HelpCircle } from 'lucide-vue-next'
import { type CandidateWithPricing } from '../../../composables/useChannelGroupContext'
import { formatRange } from '../../../composables/useNewAPIPricing'
import ChannelEditPopover from './ChannelEditPopover.vue'
import ChannelModelDetail from './ChannelModelDetail.vue'

const props = defineProps<{
  members: CandidateWithPricing[]
  defaultRuntimeId: string
  // 每条 runtime channel id → 卖价倍率（markup × yuanPerUpstreamDollar × pivotStackDollarsPerYuan）
  sellMultiplierByRuntime: Record<string, number>
}>()

const emit = defineEmits<{
  (e: 'remove', runtimeId: string): void
  (e: 'set-default', runtimeId: string): void
  (e: 'channel-changed'): void
}>()

const columns: DataTableColumns<CandidateWithPricing> = [
  {
    type: 'expand',
    expandable: row => !!row.upstreamPricing && row.upstreamPricing.modelsCount > 0,
    renderExpand: row => h(ChannelModelDetail, {
      modelRows: row.upstreamPricing?.modelRows || [],
      sellMultiplier: props.sellMultiplierByRuntime[row.runtimeId] || 20,
    }),
  },
  {
    title: () => h(NTooltip, { trigger: 'hover', placement: 'top', style: 'max-width: 280px' }, {
      trigger: () => h('div', { style: 'display:inline-flex;align-items:center;gap:4px;cursor:help' }, [
        h('span', null, '默认渠道'),
        h(HelpCircle, { size: 11, style: 'opacity:0.6' }),
      ]),
      default: () => 'user 没在 Dashboard 上挑具体渠道时，本分组的调用走这条「默认」。每组只能选一条。',
    }),
    key: 'default',
    width: 90,
    align: 'center',
    render: row => h(NRadio, {
      checked: props.defaultRuntimeId === row.runtimeId,
      'onUpdate:checked': (v: boolean) => { if (v) emit('set-default', row.runtimeId) },
    }, () => null),
  },
  {
    title: '渠道名',
    key: 'alias',
    width: 200,
    render: row => h('div', { class: 'cell' }, [
      h(NTag, { size: 'small', bordered: false, type: row.sourceType === 'newapi' ? 'info' : 'success' }, () => row.sourceType === 'newapi' ? 'NewAPI' : '直连'),
      h('span', { class: 'cell__alias' }, row.alias),
    ]),
  },
  {
    title: '上游分组',
    key: 'detail',
    width: 240,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'mono dim small' }, row.providerId ? `${row.providerId} · ${row.groupName}` : row.sourceDetail),
  },
  {
    title: '上游入价 in',
    key: 'inPrice',
    width: 140,
    align: 'center',
    render: row => {
      const p = row.upstreamPricing
      if (!p || p.modelsCount === 0) return h('span', { class: 'dim small' }, row.sourceType === 'direct' ? (row.billing || '-') : '-')
      return h('span', { class: 'mono small' }, formatRange(p.inputMin, p.inputMax))
    },
  },
  {
    title: '上游入价 out',
    key: 'outPrice',
    width: 140,
    align: 'center',
    render: row => {
      const p = row.upstreamPricing
      if (!p || p.modelsCount === 0) return h('span', { class: 'dim small' }, '-')
      return h('span', { class: 'mono small dim' }, formatRange(p.outputMin, p.outputMax))
    },
  },
  {
    title: '对外卖价 in',
    key: 'sellIn',
    width: 150,
    align: 'center',
    render: row => {
      const p = row.upstreamPricing
      if (!p || p.modelsCount === 0) return h('span', { class: 'dim small' }, '-')
      const m = props.sellMultiplierByRuntime[row.runtimeId] || 20
      return h('span', { class: 'mono sell' }, formatRange(p.inputMin * m, p.inputMax * m))
    },
  },
  {
    title: '对外卖价 out',
    key: 'sellOut',
    width: 150,
    align: 'center',
    render: row => {
      const p = row.upstreamPricing
      if (!p || p.modelsCount === 0) return h('span', { class: 'dim small' }, '-')
      const m = props.sellMultiplierByRuntime[row.runtimeId] || 20
      return h('span', { class: 'mono small dim' }, formatRange(p.outputMin * m, p.outputMax * m))
    },
  },
  {
    title: 'Markup',
    key: 'markup',
    width: 90,
    align: 'center',
    render: row => row.sourceType === 'newapi'
      ? h('span', { class: 'mono' }, `${(row.markup ?? 1).toFixed(2)}×`)
      : h('span', { class: 'small dim' }, '-'),
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    align: 'center',
    render: row => h(NTag, { size: 'small', bordered: false, type: row.status === 'enabled' ? 'success' : 'default' }, () => row.status === 'enabled' ? '启用' : '禁用'),
  },
  {
    title: '操作',
    key: 'actions',
    width: 170,
    align: 'center',
    render: row => h(NSpace, {
      size: 4, justify: 'center',
      onClick: (e: MouseEvent) => e.stopPropagation(),
    }, () => [
      h(ChannelEditPopover, {
        sourceType: row.sourceType,
        channelId: row.channelId,
        alias: row.alias,
        markup: row.markup,
        enabled: row.status === 'enabled',
        onSaved: () => emit('channel-changed'),
      }),
      h(NButton, {
        size: 'tiny', quaternary: true, type: 'error',
        onClick: (e: MouseEvent) => { e.stopPropagation(); emit('remove', row.runtimeId) },
      }, { default: () => '移除', icon: () => h(X, { size: 11 }) }),
    ]),
  },
]

function rowProps(row: CandidateWithPricing) {
  return {
    style: 'cursor: pointer',
    onClick: () => emit('set-default', row.runtimeId),
  }
}
</script>

<template>
  <section class="ml">
    <header class="ml__head">
      <h3 class="ml__title">已挂载渠道</h3>
      <span class="ml__sub">{{ members.length }} 条 · 默认 <span class="mono">{{ defaultRuntimeId || '未设置' }}</span></span>
    </header>
    <n-data-table
      v-if="members.length"
      :columns="columns"
      :data="members"
      :row-key="(r: CandidateWithPricing) => r.runtimeId"
      :row-props="rowProps"
      :pagination="false"
      :max-height="380"
      :scroll-x="1500"
      size="small"
      striped
    />
    <n-empty v-else description="还没挂任何渠道" />
  </section>
</template>

<style scoped>
.ml { display: flex; flex-direction: column; gap: 10px; }
.ml__head { display: flex; flex-direction: column; gap: 2px; }
.ml__title { color: #ededed; font-size: 14px; font-weight: 500; margin: 0; }
.ml__sub { color: #707070; font-size: 11px; }
.cell { display: flex; align-items: center; gap: 8px; }
.cell__alias { color: #ededed; font-size: 13px; font-weight: 500; }
.dim { color: #707070; }
.small { font-size: 11px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; font-variant-numeric: tabular-nums; font-size: 12px; }
.sell { color: #ededed; }
</style>
