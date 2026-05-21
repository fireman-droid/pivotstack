<script setup lang="ts">
import { ref, computed, h, watch } from 'vue'
import {
  NDataTable, NInput, NSelect, NButton, NTag, NCollapse, NCollapseItem, NSwitch, NSpin,
  type DataTableColumns, type DataTableRowKey,
} from 'naive-ui'
import { Plus, Search, RefreshCw } from 'lucide-vue-next'
import { type CandidateWithPricing } from '../../../composables/useChannelGroupContext'
import { formatRange } from '../../../composables/useNewAPIPricing'
import type { ChannelGroupChannelRef } from '../../../api/admin/groups'
import ChannelEditPopover from './ChannelEditPopover.vue'
import ChannelModelDetail from './ChannelModelDetail.vue'

const props = defineProps<{
  candidates: CandidateWithPricing[]
  excludedRuntimeIds: Set<string>
  loading?: boolean
  // 全局单位换算 1¥=N虚拟$（admin /billing/unit 配置）；不传默认 1
  pivotStackDollarsPerYuan?: number
}>()

const emit = defineEmits<{
  (e: 'add', refs: ChannelGroupChannelRef[]): void
  (e: 'channel-changed'): void
}>()

const search = ref('')
const statusFilter = ref<'all' | 'enabled' | 'disabled'>('all')
const checked = ref<DataTableRowKey[]>([])

const statusOptions = [
  { label: '全部状态', value: 'all' },
  { label: '已启用', value: 'enabled' },
  { label: '已禁用', value: 'disabled' },
]

// 按 provider / 来源 type 分组：NewAPI 各 provider 独立组；direct 归 "自营直连"
const grouped = computed(() => {
  const q = search.value.trim().toLowerCase()
  const groups = new Map<string, { label: string; sourceType: 'newapi' | 'direct'; items: CandidateWithPricing[] }>()
  for (const c of props.candidates) {
    if (statusFilter.value !== 'all' && c.status !== statusFilter.value) continue
    if (q && !c.alias.toLowerCase().includes(q) &&
        !c.sourceDetail.toLowerCase().includes(q) &&
        !c.channelId.toLowerCase().includes(q) &&
        !(c.topModelExamples || []).some(m => m.toLowerCase().includes(q)) &&
        !(c.groupName || '').toLowerCase().includes(q))
      continue
    const groupKey = c.sourceType === 'direct' ? '__direct__' : (c.providerId || c.sourceType)
    if (!groups.has(groupKey)) {
      groups.set(groupKey, {
        label: c.sourceType === 'direct' ? '自营直连' : c.providerId || c.sourceType,
        sourceType: c.sourceType,
        items: [],
      })
    }
    groups.get(groupKey)!.items.push(c)
  }
  return Array.from(groups.entries()).map(([key, g]) => ({ key, ...g }))
})

const visibleCount = computed(() => grouped.value.reduce((s, g) => s + g.items.length, 0))

// 默认展开所有分组
const expandedNames = ref<string[]>([])
watch(grouped, (g) => {
  if (!expandedNames.value.length) expandedNames.value = g.map(x => x.key)
}, { immediate: true })

watch(() => props.excludedRuntimeIds, (next) => {
  checked.value = checked.value.filter(k => !next.has(String(k)))
}, { deep: true })

const visibleAddableRuntimeIds = computed(() => {
  const ids = new Set<string>()
  for (const g of grouped.value) {
    for (const c of g.items) {
      if (!props.excludedRuntimeIds.has(c.runtimeId)) ids.add(c.runtimeId)
    }
  }
  return ids
})

// 全选 toggle：当前可见且可加入的渠道是否已经全部勾选
const allVisibleChecked = computed(() => {
  const ids = visibleAddableRuntimeIds.value
  if (!ids.size) return false
  const cs = new Set(checked.value.map(String))
  for (const id of ids) if (!cs.has(id)) return false
  return true
})

function toggleSelectAll() {
  if (allVisibleChecked.value) {
    // 取消选择当前可见
    const visible = visibleAddableRuntimeIds.value
    checked.value = checked.value.filter(k => !visible.has(String(k)))
  } else {
    const next = new Set(checked.value.map(String))
    for (const id of visibleAddableRuntimeIds.value) next.add(id)
    checked.value = Array.from(next)
  }
}
function clearSelection() { checked.value = [] }

function toggleRow(runtimeId: string) {
  if (props.excludedRuntimeIds.has(runtimeId)) return
  const arr = checked.value.map(String)
  const idx = arr.indexOf(runtimeId)
  if (idx >= 0) arr.splice(idx, 1)
  else arr.push(runtimeId)
  checked.value = arr
}

function rowProps(row: CandidateWithPricing) {
  const excluded = props.excludedRuntimeIds.has(row.runtimeId)
  if (excluded) return { style: 'opacity: 0.55; cursor: not-allowed' }
  return {
    style: 'cursor: pointer',
    onClick: (e: MouseEvent) => {
      const t = e.target as HTMLElement
      // 内置控件自处理 → 行不再重复 toggle
      if (
        t.closest('.n-checkbox') ||
        t.closest('button') ||
        t.closest('a') ||
        t.closest('.n-data-table-expand-trigger') ||
        t.closest('.n-base-icon')
      ) return
      toggleRow(row.runtimeId)
    },
  }
}

function add() {
  const refs: ChannelGroupChannelRef[] = []
  const keys = new Set(checked.value.map(String))
  for (const c of props.candidates) {
    if (keys.has(c.runtimeId) && !props.excludedRuntimeIds.has(c.runtimeId)) {
      refs.push({ sourceType: c.sourceType, channelId: c.channelId })
    }
  }
  emit('add', refs)
  checked.value = []
}

const checkedAddableCount = computed(() => {
  let n = 0
  const keys = new Set(checked.value.map(String))
  for (const c of props.candidates) {
    if (keys.has(c.runtimeId) && !props.excludedRuntimeIds.has(c.runtimeId)) n++
  }
  return n
})

// 每个 provider 子表的 columns
function makeColumns(): DataTableColumns<CandidateWithPricing> {
  return [
    {
      type: 'expand',
      expandable: row => !!row.upstreamPricing && row.upstreamPricing.modelsCount > 0,
      renderExpand: row => h(ChannelModelDetail, {
        modelRows: row.upstreamPricing?.modelRows || [],
        // 候选池里 markup 是 channel 现有的 markup × 全局单位换算系数
        sellMultiplier: (row.markup ?? 1) * (props.pivotStackDollarsPerYuan ?? 1),
      }),
    },
    {
      type: 'selection',
      width: 36,
      disabled: row => props.excludedRuntimeIds.has(row.runtimeId),
    },
    {
      title: '渠道名',
      key: 'alias',
      width: 200,
      render: row => h('span', { class: 'alias' }, row.alias),
    },
    {
      title: '上游分组',
      key: 'groupName',
      width: 260,
      ellipsis: { tooltip: true },
      render: row => h('span', { class: 'mono dim' }, row.groupName || row.sourceDetail),
    },
    {
      title: '上游入价 (in / out) $/Mtok',
      key: 'priceIn',
      width: 220,
      align: 'center',
      render: row => {
        const p = row.upstreamPricing
        if (row.sourceType === 'direct') return h('span', { class: 'mono small dim' }, row.billing || '-')
        if (!p) return h('span', { class: 'small dim' }, '⚠ 上游分组数据不一致')
        if (p.modelsCount === 0) return h('span', { class: 'small dim' }, '该分组无模型')
        return h('div', { class: 'price-cell' }, [
          h('span', { class: 'mono price-in' }, `${formatRange(p.inputMin, p.inputMax)}`),
          h('span', { class: 'mono price-out' }, `${formatRange(p.outputMin, p.outputMax)}`),
          h('span', { class: 'price-meta' }, `${p.modelsCount} 模型 · 倍率 ${p.groupRatio}×`),
        ])
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
      render: row => h(NTag, {
        size: 'small', bordered: false,
        type: row.status === 'enabled' ? 'success' : 'default',
      }, () => row.status === 'enabled' ? '启用' : '禁用'),
    },
    {
      title: '',
      key: 'mounted',
      width: 80,
      align: 'center',
      render: row => props.excludedRuntimeIds.has(row.runtimeId)
        ? h(NTag, { size: 'small', bordered: false, type: 'warning' }, () => '已挂载')
        : null,
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      align: 'center',
      render: row => h(ChannelEditPopover, {
        sourceType: row.sourceType,
        channelId: row.channelId,
        alias: row.alias,
        markup: row.markup,
        enabled: row.status === 'enabled',
        onSaved: () => emit('channel-changed'),
      }),
    },
  ]
}
</script>

<template>
  <section class="picker">
    <header class="picker__head">
      <div class="picker__head-left">
        <h3 class="picker__title">候选渠道池</h3>
        <span class="picker__sub">{{ candidates.length }} 条总 · {{ visibleCount }} 条筛选 · 分 {{ grouped.length }} 组</span>
      </div>
      <div class="picker__head-right">
        <n-button size="small" :disabled="!visibleAddableRuntimeIds.size" @click="toggleSelectAll">
          {{ allVisibleChecked ? '取消全选' : '全选当前结果' }}
        </n-button>
        <n-button size="small" :disabled="!checked.length" @click="clearSelection">清空</n-button>
        <n-button size="small" type="primary" :disabled="checkedAddableCount === 0" @click="add">
          <template #icon><Plus :size="14" /></template>
          加入 {{ checkedAddableCount }} 条
        </n-button>
      </div>
    </header>
    <div class="picker__filters">
      <n-input v-model:value="search" placeholder="搜索 渠道名 / 上游分组 / 模型 / channelId" size="small" clearable style="flex: 1; min-width: 240px">
        <template #prefix><Search :size="13" /></template>
      </n-input>
      <n-select v-model:value="statusFilter" :options="statusOptions" size="small" style="width: 130px" />
    </div>
    <n-spin :show="!!loading">
      <n-collapse v-model:expanded-names="expandedNames" :trigger-areas="['main', 'arrow']">
        <n-collapse-item v-for="group in grouped" :key="group.key" :name="group.key" :title="''">
          <template #header>
            <div class="grp">
              <n-tag size="small" :bordered="false" :type="group.sourceType === 'newapi' ? 'info' : 'success'">
                {{ group.sourceType === 'newapi' ? 'NewAPI' : '直连' }}
              </n-tag>
              <span class="grp__name">{{ group.label }}</span>
              <span class="grp__count mono">{{ group.items.length }} 条</span>
            </div>
          </template>
          <n-data-table
            v-model:checked-row-keys="checked"
            :columns="makeColumns()"
            :data="group.items"
            :row-key="(r: CandidateWithPricing) => r.runtimeId"
            :row-props="rowProps"
            :pagination="false"
            :scroll-x="1116"
            size="small"
            striped
          />
        </n-collapse-item>
      </n-collapse>
    </n-spin>
  </section>
</template>

<style scoped>
.picker { display: flex; flex-direction: column; gap: 12px; }
.picker__head { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; }
.picker__head-left { display: flex; flex-direction: column; gap: 2px; }
.picker__title { color: #ededed; font-size: 14px; font-weight: 500; margin: 0; }
.picker__sub { color: #707070; font-size: 11px; font-family: "Geist Mono", ui-monospace, monospace; }
.picker__head-right { display: flex; gap: 6px; }
.picker__filters { display: flex; gap: 8px; align-items: center; }
.grp { display: flex; align-items: center; gap: 10px; }
.grp__name { color: #ededed; font-size: 13px; font-weight: 500; }
.grp__count { color: #707070; font-size: 11px; margin-left: auto; }
.alias { color: #ededed; font-size: 13px; font-weight: 500; }
.dim { color: #707070; }
.small { font-size: 11px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; font-variant-numeric: tabular-nums; }
.price-cell { display: flex; flex-direction: column; gap: 1px; line-height: 1.35; }
.price-in { color: #ededed; font-size: 12px; }
.price-in::after { content: ' /in'; color: #707070; font-size: 10px; }
.price-out { color: #a3a3a3; font-size: 11px; }
.price-out::after { content: ' /out'; color: #707070; font-size: 10px; }
.price-meta { color: #707070; font-size: 10px; margin-top: 1px; }
</style>
