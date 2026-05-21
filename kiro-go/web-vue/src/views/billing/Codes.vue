<script setup lang="ts">
import { ref, computed, onMounted, watch, h } from 'vue'
import { NButton, NDataTable, NTabs, NTabPane, NTag, NPopconfirm, useMessage, type DataTableColumns, type DataTableRowKey } from 'naive-ui'
import { Plus, Trash2, Download, Copy } from 'lucide-vue-next'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import CopyableText from '../../components/common/CopyableText.vue'
import EmptyState from '../../components/common/EmptyState.vue'
import BatchActionBar from '../../components/common/BatchActionBar.vue'
import CodeBatchDrawer from '../../components/admin/codes/CodeBatchDrawer.vue'
import CodesGeneratedDialog from '../../components/admin/codes/CodesGeneratedDialog.vue'
import { listCodes, deleteCode, type ActivationCode, type CreateCodesResponse } from '../../api/admin/codes'
import { useTablePagination } from '../../composables/useTablePagination'
import { useRowClickToggle } from '../../composables/useRowClickToggle'

const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const rows = ref<ActivationCode[]>([])
const tab = ref<'unused' | 'used'>('unused')
const checkedRowKeys = ref<DataTableRowKey[]>([])
const batchRunning = ref(false)
const selectedCodes = computed(() => rows.value.filter(r => checkedRowKeys.value.includes(r.code)))
const rowProps = useRowClickToggle<ActivationCode>(checkedRowKeys, r => r.code)

async function bulkDelete() {
  const items = selectedCodes.value
  batchRunning.value = true
  const results = await Promise.allSettled(items.map(c => deleteCode(c.code)))
  const ok = results.filter(r => r.status === 'fulfilled').length
  const fail = results.length - ok
  batchRunning.value = false
  if (fail === 0) message.success(`已删除 ${ok} 条`)
  else message.warning(`${ok} 成功 / ${fail} 失败`)
  await reload()
  checkedRowKeys.value = []
}
async function bulkCopy() {
  const text = selectedCodes.value.map(c => c.code).join('\n')
  await navigator.clipboard.writeText(text)
  message.success(`已复制 ${selectedCodes.value.length} 条激活码`)
}
function bulkExport() {
  const list = selectedCodes.value.length ? selectedCodes.value : rows.value
  const head = ['code', 'type', 'amount', 'salePriceCNY', 'used', 'createdAt']
  const lines = [head.join(',')]
  for (const c of list) {
    lines.push([c.code, c.type || '', c.amount ?? 0, c.salePriceCNY ?? 0, c.used ? 1 : 0,
      c.createdAt ? new Date(c.createdAt * 1000).toISOString() : ''].join(','))
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `codes-${tab.value}-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
  message.success(`已导出 ${list.length} 条`)
}

const drawerShow = ref(false)
const resultShow = ref(false)
const resultCodes = ref<string[]>([])
const resultMeta = ref<{ type: string; amount: number; tier?: string; salePriceCNY?: number } | null>(null)

// 切 tab 后直接重新拉对应集合，所以本地数组就是当前 tab 的列表
const filtered = computed(() => rows.value)

watch(tab, () => reload())

const columns: DataTableColumns<ActivationCode> = [
  { type: 'selection' },
  { title: 'code', key: 'code', width: 200, render: row => h(CopyableText, { text: row.code, mono: true }) },
  { title: '类型', key: 'type', width: 90, align: 'center', render: row => typeTag(row) },
  { title: '面额 / 时长', key: 'amount', width: 130, align: 'center', render: row => h('span', { class: 'mono' }, amountLabel(row)) },
  { title: '售价 ¥', key: 'salePrice', width: 110, align: 'center', render: row => h('span', { class: 'mono' }, row.salePriceCNY ? `¥${row.salePriceCNY.toFixed(2)}` : '-') },
  { title: '批次 / 备注', key: 'batch', width: 140, ellipsis: { tooltip: true }, render: row => row.batch || row.note || '-' },
  {
    title: '状态',
    key: 'used',
    width: 100,
    align: 'center',
    render: row => h(NTag, { size: 'small', bordered: false, type: row.used ? 'default' : 'success' }, () => row.used ? '已使用' : '未使用'),
  },
  { title: '创建', key: 'createdAt', width: 170, align: 'center', render: row => h('span', { class: 'mono' }, formatTime(row.createdAt)) },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    align: 'center',
    render: row => h(NPopconfirm, {
      onPositiveClick: () => doDelete(row),
      positiveText: '删除',
      negativeText: '取消',
    }, {
      trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error', disabled: row.used }, { icon: () => h(Trash2, { size: 13 }) }),
      default: () => `删除激活码 ${row.code.slice(0, 8)}…？${row.used ? '该码已使用，建议保留。' : ''}`,
    }),
  },
]

function typeTag(row: ActivationCode) {
  const t = row.type || 'balance'
  const map: Record<string, { label: string; type: 'success' | 'info' | 'warning' | 'default' }> = {
    balance: { label: '余额', type: 'success' },
    days: { label: '天卡', type: 'info' },
    time: { label: '时长', type: 'warning' },
  }
  const m = map[t] || { label: t, type: 'default' as const }
  return h(NTag, { size: 'small', bordered: false, type: m.type }, () => m.label)
}
function amountLabel(row: ActivationCode) {
  const a = Number(row.amount ?? 0)
  if (row.type === 'days') return `${a} 天`
  if (row.type === 'time') return `${a} 秒`
  return `¥${a.toFixed(2)}`
}

function formatTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}
async function reload() {
  loading.value = true
  try {
    rows.value = await listCodes(tab.value === 'used')
  } catch (e: any) {
    message.error(e?.message || '加载激活码失败')
  } finally {
    loading.value = false
  }
}

function onGenerated(resp: CreateCodesResponse, meta: typeof resultMeta.value) {
  resultCodes.value = resp.codes
  resultMeta.value = meta
  resultShow.value = true
  reload()
}

async function doDelete(row: ActivationCode) {
  try {
    await deleteCode(row.code)
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
    <PageHeader kicker="销售 & 计费" :kicker-dot="'#707070'" title="激活码">
      <template #actions>
        <n-button type="primary" size="small" @click="drawerShow = true">
          <template #icon><Plus :size="14" /></template>
          批量生成
        </n-button>
      </template>
    </PageHeader>

    <n-tabs v-model:value="tab" size="small" class="sub-tabs">
      <n-tab-pane name="unused" tab="未使用" />
      <n-tab-pane name="used" tab="已使用" />
    </n-tabs>

    <BatchActionBar :count="checkedRowKeys.length" @clear="checkedRowKeys = []">
      <n-button size="small" @click="bulkCopy">
        <template #icon><Copy :size="13" /></template>批量复制
      </n-button>
      <n-button size="small" quaternary @click="bulkExport">
        <template #icon><Download :size="13" /></template>导出 CSV
      </n-button>
      <n-popconfirm @positive-click="bulkDelete" positive-text="删除" negative-text="取消">
        <template #trigger>
          <n-button size="small" type="error" :loading="batchRunning">
            <template #icon><Trash2 :size="13" /></template>批量删除
          </n-button>
        </template>
        删除选中的 {{ checkedRowKeys.length }} 张激活码？
      </n-popconfirm>
    </BatchActionBar>

    <n-data-table v-if="filtered.length || loading" v-model:checked-row-keys="checkedRowKeys" :columns="columns" :data="filtered" :loading="loading" :row-key="row => row.code" :row-props="rowProps" :pagination="pagination" :scroll-x="1350" size="small" striped />
    <EmptyState v-else icon="○" :title="tab === 'used' ? '没有已使用激活码' : '没有未使用激活码'" />

    <CodeBatchDrawer v-model:show="drawerShow" @generated="onGenerated" />
    <CodesGeneratedDialog v-model:show="resultShow" :codes="resultCodes" :meta="resultMeta" />
  </PageContainer>
</template>

<style scoped>
.sub-tabs { margin-bottom: 16px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; color: #ededed; }
</style>
