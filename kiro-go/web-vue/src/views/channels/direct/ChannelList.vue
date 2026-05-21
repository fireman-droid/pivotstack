<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NDataTable, NInput, NSpace, NSwitch, NTabs, NTabPane, NTag, NPopconfirm, useMessage, type DataTableColumns, type DataTableRowKey } from 'naive-ui'
import { Plus, Search, Trash2, Pencil, CheckCircle2, XCircle } from 'lucide-vue-next'
import PageContainer from '../../../components/common/PageContainer.vue'
import PageHeader from '../../../components/common/PageHeader.vue'
import Toolbar from '../../../components/common/Toolbar.vue'
import StatusBadge from '../../../components/common/StatusBadge.vue'
import MonoValue from '../../../components/common/MonoValue.vue'
import EmptyState from '../../../components/common/EmptyState.vue'
import BatchActionBar from '../../../components/common/BatchActionBar.vue'
import DirectChannelDrawer from '../../../components/admin/direct/DirectChannelDrawer.vue'
import { listDirectChannels, deleteDirectChannel, patchDirectChannel, type DirectChannel } from '../../../api/admin/directChannels'
import { useTablePagination } from '../../../composables/useTablePagination'
import { useRowClickToggle } from '../../../composables/useRowClickToggle'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const rows = ref<DirectChannel[]>([])
const search = ref('')

const drawerShow = ref(false)
const drawerType = ref<'openai' | 'kiro'>('openai')
const drawerRow = ref<DirectChannel | null>(null)
const togglingId = ref<string>('')
const checkedRowKeys = ref<DataTableRowKey[]>([])
const batchRunning = ref(false)
const selectedRows = computed(() => rows.value.filter(r => checkedRowKeys.value.includes(r.id)))
const rowProps = useRowClickToggle<DirectChannel>(checkedRowKeys, r => r.id)

async function runBatch(items: DirectChannel[], op: (r: DirectChannel) => Promise<unknown>, label: string) {
  if (!items.length) { message.info(`${label}：无需处理的目标`); return }
  batchRunning.value = true
  const r = await Promise.allSettled(items.map(op))
  const ok = r.filter(x => x.status === 'fulfilled').length
  const fail = r.length - ok
  batchRunning.value = false
  if (fail === 0) message.success(`${label}：${ok} 条成功`)
  else message.warning(`${label}：${ok} 成功 / ${fail} 失败`)
  await reload()
  checkedRowKeys.value = []
}
const bulkEnable = () => runBatch(selectedRows.value.filter(r => !r.enabled), r => patchDirectChannel(r.id, { enabled: true }), '批量启用')
const bulkDisable = () => runBatch(selectedRows.value.filter(r => r.enabled), r => patchDirectChannel(r.id, { enabled: false }), '批量禁用')
const bulkDelete = () => runBatch(selectedRows.value.filter(r => r.type !== 'kiro'), r => deleteDirectChannel(r.id), '批量删除')

const tab = computed({
  get: () => String(route.query.type || 'all'),
  set: (value: string) => router.replace({ query: { ...route.query, type: value === 'all' ? undefined : value } }),
})

// 内建 kiro 渠道（共享 kiro 账号池）：列表始终包含一条，不可删。
const KIRO_BUILTIN_ID = 'kiro:builtin'

const allRows = computed<DirectChannel[]>(() => {
  // 后端如果将来真的存了 kiro:default 记录则用它，否则前端 unshift 一条虚拟行
  const hasBuiltin = rows.value.some(r => r.type === 'kiro')
  if (hasBuiltin) return rows.value
  const builtin: DirectChannel = {
    id: KIRO_BUILTIN_ID,
    type: 'kiro',
    alias: 'Kiro 账号池',
    enabled: true,
    models: [],
  }
  return [builtin, ...rows.value]
})

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return allRows.value.filter(row => {
    if (tab.value !== 'all' && row.type !== tab.value) return false
    if (!q) return true
    return [row.alias, row.id, row.baseUrl].some(v => String(v || '').toLowerCase().includes(q))
  })
})

function isKiroBuiltin(row: DirectChannel) {
  return row.type === 'kiro' || row.id === KIRO_BUILTIN_ID
}

function typeLabel(t: string) {
  if (t === 'kiro') return 'Kiro 内建'
  return '自定义上游'
}

const columns: DataTableColumns<DirectChannel> = [
  { type: 'selection', disabled: (row: DirectChannel) => isKiroBuiltin(row) },
  {
    title: 'Alias',
    key: 'alias',
    width: 200,
    render: row => h('span', { class: 'alias', style: isKiroBuiltin(row) ? 'color:#0bd470;font-weight:600' : '' }, row.alias || row.id),
  },
  {
    title: '类型',
    key: 'type',
    width: 100,
    align: 'center',
    render: row => h(NTag, { size: 'small', bordered: false, type: row.type === 'kiro' ? 'info' : 'success' }, () => typeLabel(row.type)),
  },
  { title: 'BaseURL', key: 'baseUrl', width: 240, ellipsis: { tooltip: true }, render: row => row.type === 'kiro' ? h('span', { class: 'dim' }, '共享账号池') : h(MonoValue, { value: row.baseUrl || '-' }) },
  { title: '模型数', key: 'models', width: 90, align: 'center', render: row => row.models?.length || 0 },
  { title: '售价摘要', key: 'sellPrice', width: 320, ellipsis: { tooltip: true }, render: row => priceSummary(row) },
  {
    title: '状态',
    key: 'status',
    width: 100,
    align: 'center',
    render: row => h(StatusBadge, { status: row.enabled ? 'enabled' : 'disabled', label: row.status || (row.enabled ? '启用' : '禁用') }),
  },
  {
    title: '启用',
    key: 'enabled',
    width: 80,
    align: 'center',
    render: row => h(NSwitch, {
      size: 'small',
      value: row.enabled,
      loading: row.id === togglingId.value,
      disabled: row.id === KIRO_BUILTIN_ID,
      onUpdateValue: (v: boolean) => toggleEnabled(row, v),
    }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 220,
    align: 'center',
    render: row => h(NSpace, { size: 4, justify: 'center' }, () => {
      const items = [
        h(NButton, { size: 'tiny', quaternary: true, onClick: () => router.push({ name: 'ChannelsDirectDetail', params: { id: row.id } }) },
          { default: () => '详情' }),
      ]
      if (row.id !== KIRO_BUILTIN_ID) {
        items.push(
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => openEdit(row) },
            { default: () => '编辑', icon: () => h(Pencil, { size: 13 }) }),
        )
      }
      if (!isKiroBuiltin(row)) {
        items.push(
          h(NPopconfirm, {
            onPositiveClick: () => doDelete(row),
            positiveText: '删除',
            negativeText: '取消',
          }, {
            trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { icon: () => h(Trash2, { size: 13 }) }),
            default: () => `删除自定义上游「${row.alias || row.id}」？此操作会软删（保留 tombstone）。`,
          }),
        )
      }
      return items
    }),
  },
]

function priceSummary(row: DirectChannel) {
  const price = row.sellPrice?.default
  if (!price || (!price.inputPerM && !price.outputPerM)) return h('span', { class: 'dim' }, '未设置 / 沿用全局')
  return `¥${Number(price.inputPerM || 0).toFixed(4)}/Mtok in · ¥${Number(price.outputPerM || 0).toFixed(4)}/Mtok out`
}

async function reload() {
  loading.value = true
  try {
    rows.value = await listDirectChannels()
  } catch (e: any) {
    message.error(e?.message || '加载自营渠道失败')
  } finally {
    loading.value = false
  }
}

function openCreate(type: 'openai' | 'kiro') {
  drawerType.value = type
  drawerRow.value = null
  drawerShow.value = true
}
function openEdit(row: DirectChannel) {
  drawerType.value = row.type
  drawerRow.value = row
  drawerShow.value = true
}

async function toggleEnabled(row: DirectChannel, v: boolean) {
  togglingId.value = row.id
  try {
    await patchDirectChannel(row.id, { enabled: v })
    row.enabled = v
    message.success(v ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '切换失败')
  } finally {
    togglingId.value = ''
  }
}

async function doDelete(row: DirectChannel) {
  try {
    await deleteDirectChannel(row.id)
    message.success('已删除')
    reload()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="渠道 · 自营"
      :kicker-dot="'#707070'"
      title="自营直连"
      desc="Kiro 账号池（内建）+ 自定义上游（admin 接 base + key + 模型）"
    >
      <template #actions>
        <n-button type="primary" size="small" @click="openCreate('openai')">
          <template #icon><Plus :size="14" /></template>
          创建自定义上游
        </n-button>
      </template>
    </PageHeader>

    <n-tabs v-model:value="tab" size="small" class="sub-tabs">
      <n-tab-pane name="all" tab="全部" />
      <n-tab-pane name="kiro" tab="Kiro 内建" />
      <n-tab-pane name="openai" tab="自定义上游" />
    </n-tabs>

    <Toolbar>
      <template #left>
        <n-input v-model:value="search" clearable size="small" placeholder="搜索 alias / ID / baseURL" style="width: 280px">
          <template #prefix><Search :size="14" /></template>
        </n-input>
      </template>
    </Toolbar>

    <BatchActionBar :count="checkedRowKeys.length" @clear="checkedRowKeys = []">
      <n-button size="small" :loading="batchRunning" @click="bulkEnable">
        <template #icon><CheckCircle2 :size="13" /></template>批量启用
      </n-button>
      <n-button size="small" :loading="batchRunning" @click="bulkDisable">
        <template #icon><XCircle :size="13" /></template>批量禁用
      </n-button>
      <n-popconfirm @positive-click="bulkDelete" positive-text="删除" negative-text="取消">
        <template #trigger>
          <n-button size="small" type="error" :loading="batchRunning">
            <template #icon><Trash2 :size="13" /></template>批量删除
          </n-button>
        </template>
        删除选中的自定义上游（kiro 内建会被忽略）？
      </n-popconfirm>
    </BatchActionBar>

    <n-data-table
      v-if="filtered.length || loading"
      v-model:checked-row-keys="checkedRowKeys"
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="row => row.id"
      :row-props="rowProps"
      :pagination="pagination"
      :scroll-x="1400"
      size="small"
      striped
    />
    <EmptyState v-else icon="○" title="没有匹配的自营渠道" desc="切换筛选条件或创建自定义上游" />

    <DirectChannelDrawer
      v-model:show="drawerShow"
      :type="drawerType"
      :row="drawerRow"
      @saved="reload"
    />
  </PageContainer>
</template>

<style scoped>
.sub-tabs { margin-bottom: 16px; }
.alias { color: #ededed; font-weight: 500; }
.dim { color: #707070; }
.mini { font-size: 11px; }
</style>
