<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { userApi } from '../../api/user'
import {
  NButton, NDataTable, NInput, NSwitch, NTag, NPopconfirm, NSpace, NModal, useMessage,
  type DataTableColumns, type DataTableRowKey,
} from 'naive-ui'
import { Plus, Search, Pencil, Trash2, Copy, CheckCircle2, XCircle } from 'lucide-vue-next'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import Toolbar from '../../components/common/Toolbar.vue'
import CopyableText from '../../components/common/CopyableText.vue'
import EmptyState from '../../components/common/EmptyState.vue'
import BatchActionBar from '../../components/common/BatchActionBar.vue'
import ResellerChildKeyDrawer, { type ChildKey } from '../../components/admin/reseller/ResellerChildKeyDrawer.vue'
import { useTablePagination } from '../../composables/useTablePagination'
import { useRowClickToggle } from '../../composables/useRowClickToggle'

const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const keys = ref<ChildKey[]>([])
const search = ref('')
const checkedRowKeys = ref<DataTableRowKey[]>([])
const batchRunning = ref(false)
const selectedKeys = computed(() => keys.value.filter(k => checkedRowKeys.value.includes(k.id)))
const rowProps = useRowClickToggle<ChildKey>(checkedRowKeys, r => r.id)

const CNY_PER_USD = 0.05

async function runBatch(items: ChildKey[], op: (k: ChildKey) => Promise<unknown>, label: string) {
  if (!items.length) { message.info(`${label}：无需处理`); return }
  batchRunning.value = true
  const r = await Promise.allSettled(items.map(op))
  const ok = r.filter(x => x.status === 'fulfilled').length
  const fail = r.length - ok
  batchRunning.value = false
  if (fail === 0) message.success(`${label}：${ok} 条成功`)
  else message.warning(`${label}：${ok} 成功 / ${fail} 失败`)
  await load()
  checkedRowKeys.value = []
}
const bulkEnable = () => runBatch(selectedKeys.value.filter(k => !k.enabled), k => userApi(`/reseller/keys/${k.id}`, { method: 'PATCH', body: { enabled: true } }), '批量启用')
const bulkDisable = () => runBatch(selectedKeys.value.filter(k => k.enabled), k => userApi(`/reseller/keys/${k.id}`, { method: 'PATCH', body: { enabled: false } }), '批量禁用')
const bulkDelete = () => runBatch(selectedKeys.value, k => userApi(`/reseller/keys/${k.id}`, { method: 'DELETE' }), '批量删除')

const drawerShow = ref(false)
const drawerRow = ref<ChildKey | null>(null)
const createdKey = ref<ChildKey | null>(null)
const createdShow = ref(false)

async function load() {
  loading.value = true
  try {
    const data = await userApi('/reseller/keys')
    keys.value = data.keys || []
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return keys.value
  return keys.value.filter(k => [k.note, k.keyMasked, k.id].some(v => String(v || '').toLowerCase().includes(q)))
})

const columns: DataTableColumns<ChildKey> = [
  { type: 'selection' },
  { title: '备注', key: 'note', width: 240, ellipsis: { tooltip: true }, render: r => r.note || '-' },
  { title: 'Key', key: 'key', width: 200, render: r => h(CopyableText, { text: r.keyMasked || r.key || r.id, mono: true, mask: true }) },
  { title: '余额 ¥', key: 'balance', width: 130, align: 'center', render: r => h('span', { class: 'mono' }, `¥${((r.totalBalance || 0) * CNY_PER_USD).toFixed(2)}`) },
  { title: '请求', key: 'requests', width: 100, align: 'center', render: r => h('span', { class: 'mono' }, (r.requests || 0).toLocaleString()) },
  { title: '近 7 天', key: 'recent', width: 110, align: 'center', render: r => h('span', { class: 'mono' }, (r.recentCalls7d || 0).toLocaleString()) },
  {
    title: '启用',
    key: 'enabled',
    width: 80,
    align: 'center',
    render: r => h(NSwitch, { size: 'small', value: r.enabled, onUpdateValue: (v: boolean) => toggle(r, v) }),
  },
  { title: '过期', key: 'expiresAt', width: 130, align: 'center', render: r => h('span', { class: 'mono dim' }, r.expiresAt ? new Date(r.expiresAt * 1000).toLocaleDateString('zh-CN') : '永久') },
  {
    title: '操作',
    key: 'actions',
    width: 170,
    align: 'center',
    render: row => h(NSpace, { size: 4, justify: 'center' }, () => [
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => openEdit(row) },
        { default: () => '编辑', icon: () => h(Pencil, { size: 13 }) }),
      h(NPopconfirm, {
        onPositiveClick: () => remove(row),
        positiveText: '删除',
        negativeText: '取消',
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { icon: () => h(Trash2, { size: 13 }) }),
        default: () => `删除子 Key「${row.note || row.id}」？余额 ¥${((row.totalBalance || 0) * CNY_PER_USD).toFixed(2)} 将退回到你的账户。`,
      }),
    ]),
  },
]

function openCreate() {
  drawerRow.value = null
  drawerShow.value = true
}
function openEdit(row: ChildKey) {
  drawerRow.value = row
  drawerShow.value = true
}

function onCreated(row: ChildKey) {
  createdKey.value = row
  createdShow.value = true
  load()
}

async function toggle(row: ChildKey, v: boolean) {
  try {
    await userApi(`/reseller/keys/${row.id}`, { method: 'PATCH', body: { enabled: v } })
    row.enabled = v
    message.success(v ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '操作失败')
  }
}

async function remove(row: ChildKey) {
  try {
    const data = await userApi(`/reseller/keys/${row.id}`, { method: 'DELETE' })
    const cny = (data.refundedUSD || 0) * CNY_PER_USD
    message.success(`已删除${cny > 0 ? ` · 退还 ¥${cny.toFixed(2)}` : ''}`)
    load()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

function copyCreatedKey() {
  if (!createdKey.value?.key) return
  navigator.clipboard.writeText(createdKey.value.key)
  message.success('已复制完整 Key')
}

onMounted(load)
</script>

<template>
  <PageContainer>
    <PageHeader kicker="代理商 · 子 KEY" :kicker-dot="'#707070'" title="子 Key 管理" :desc="`${keys.length} 个子 Key`">
      <template #actions>
        <n-button type="primary" size="small" @click="openCreate">
          <template #icon><Plus :size="14" /></template>
          创建子 Key
        </n-button>
      </template>
    </PageHeader>

    <Toolbar>
      <template #left>
        <n-input v-model:value="search" clearable size="small" placeholder="搜索备注 / key / id" style="width: 280px">
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
        删除选中的 {{ checkedRowKeys.length }} 个子 Key？余额将退回你账户。
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
      :scroll-x="1310"
      size="small"
      striped
    />
    <EmptyState v-else icon="○" title="还没有子 Key" desc="点右上「创建子 Key」开始" />

    <ResellerChildKeyDrawer
      v-model:show="drawerShow"
      :row="drawerRow"
      @saved="load"
      @created="onCreated"
    />

    <n-modal
      v-model:show="createdShow"
      preset="card"
      :style="{ width: '520px' }"
      :mask-closable="false"
      :closable="false"
      title="子 Key 已创建"
    >
      <p class="warn">这是唯一一次能看到完整 Key 的机会，请立即复制保存。</p>
      <div class="key-box">{{ createdKey?.key }}</div>
      <template #footer>
        <n-space justify="end" :size="8">
          <n-button quaternary @click="createdShow = false">我已保存</n-button>
          <n-button type="primary" @click="copyCreatedKey">
            <template #icon><Copy :size="14" /></template>
            复制
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </PageContainer>
</template>

<style scoped>
.mono { font-family: "Geist Mono", ui-monospace, monospace; color: #ededed; }
.warn { color: #f5a623; font-size: 12px; margin: 0 0 12px; }
.key-box {
  padding: 12px 14px;
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 6px;
  background: #0a0a0a;
  font-family: "Geist Mono", ui-monospace, monospace;
  font-size: 13px;
  color: #ededed;
  word-break: break-all;
  user-select: all;
  line-height: 1.5;
}
</style>
