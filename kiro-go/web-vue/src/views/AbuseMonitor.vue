<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  NDataTable, NInput, NPopconfirm, NButton, useMessage,
  type DataTableColumns, type DataTableRowKey,
} from 'naive-ui'
import { Search, RefreshCw, ShieldCheck, ShieldAlert, Globe, Activity, XCircle } from 'lucide-vue-next'
import PageContainer from '../components/common/PageContainer.vue'
import PageHeader from '../components/common/PageHeader.vue'
import Toolbar from '../components/common/Toolbar.vue'
import EmptyState from '../components/common/EmptyState.vue'
import BatchActionBar from '../components/common/BatchActionBar.vue'
import CopyableText from '../components/common/CopyableText.vue'
import { listAbuseFlags, clearAbuseFlag, type AbuseFlag } from '../api/admin/abuse'
import { useTablePagination } from '../composables/useTablePagination'

const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const rows = ref<AbuseFlag[]>([])
const search = ref('')
const checked = ref<DataTableRowKey[]>([])
const clearingId = ref<string>('')
const bulkBusy = ref(false)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(r =>
    [r.keyId, r.reason].some(v => String(v || '').toLowerCase().includes(q)),
  )
})

const totalFlagged = computed(() => rows.value.length)
const highIPCount = computed(() => rows.value.filter(r => (r.distinctIPs || 0) > 10).length)
const totalStreams = computed(() => rows.value.reduce((s, r) => s + (r.activeStreams || 0), 0))

async function reload() {
  loading.value = true
  try {
    rows.value = await listAbuseFlags()
    checked.value = []
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function clearOne(keyId: string) {
  clearingId.value = keyId
  try {
    await clearAbuseFlag(keyId)
    rows.value = rows.value.filter(r => r.keyId !== keyId)
    checked.value = checked.value.filter(k => k !== keyId)
    message.success('已清除标记')
  } catch (e: any) {
    message.error(e?.message || '清除失败')
  } finally {
    clearingId.value = ''
  }
}

async function clearChecked() {
  if (!checked.value.length) return
  bulkBusy.value = true
  const ids = checked.value.map(String)
  let ok = 0, fail = 0
  for (const id of ids) {
    try {
      await clearAbuseFlag(id)
      ok++
    } catch {
      fail++
    }
  }
  await reload()
  bulkBusy.value = false
  if (fail === 0) message.success(`已清除 ${ok} 条`)
  else message.warning(`成功 ${ok} 条 / 失败 ${fail} 条`)
}

const columns: DataTableColumns<AbuseFlag> = [
  { type: 'selection' },
  {
    title: 'Key ID',
    key: 'keyId',
    width: 250,
    render: row => h(CopyableText, { text: row.keyId, mono: true }),
  },
  {
    title: '原因',
    key: 'reason',
    width: 360,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'am-reason' }, row.reason || '异常行为'),
  },
  {
    title: '活跃流',
    key: 'activeStreams',
    width: 90,
    align: 'center',
    render: row => h('span', { class: 'am-mono' }, String(row.activeStreams || 0)),
  },
  {
    title: 'IP 数',
    key: 'distinctIPs',
    width: 90,
    align: 'center',
    render: row => h('span', {
      class: ['am-mono', (row.distinctIPs || 0) > 10 ? 'am-mono--warn' : ''],
    }, String(row.distinctIPs || 0)),
  },
  {
    title: '操作',
    key: 'actions',
    width: 130,
    align: 'center',
    render: row => h(NPopconfirm, {
      onPositiveClick: () => clearOne(row.keyId),
      positiveText: '清除',
      negativeText: '取消',
    }, {
      trigger: () => h(NButton, {
        size: 'tiny',
        quaternary: true,
        loading: row.keyId === clearingId.value,
      }, {
        icon: () => h(XCircle, { size: 12 }),
        default: () => '清除标记',
      }),
      default: () => `确认清除 ${row.keyId.slice(0, 12)}… 的滥用标记？`,
    }),
  },
]

onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="OPS / 滥用监控"
      title="滥用监控"
      :desc="`异常 API Key 风控标记 · ${totalFlagged} 条`"
    >
      <template #actions>
        <n-button size="small" :loading="loading" @click="reload">
          <template #icon><RefreshCw :size="13" /></template>
          刷新
        </n-button>
      </template>
    </PageHeader>

    <section class="metric-strip">
      <div class="metric-tile">
        <div class="metric-tile__label"><ShieldAlert :size="12" /> 标记总数</div>
        <div class="metric-tile__num">{{ totalFlagged }}</div>
        <div class="metric-tile__delta"><span class="t-meta">含历史未清除</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label"><Globe :size="12" /> 高 IP 离散度</div>
        <div class="metric-tile__num">{{ highIPCount }}</div>
        <div class="metric-tile__delta"><span class="t-meta">IP 数 &gt; 10</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label"><Activity :size="12" /> 活跃流合计</div>
        <div class="metric-tile__num">{{ totalStreams }}</div>
        <div class="metric-tile__delta"><span class="t-meta">所有被标 Key</span></div>
      </div>
    </section>

    <Toolbar>
      <template #left>
        <n-input
          v-model:value="search"
          clearable
          size="small"
          placeholder="搜索 Key ID / 原因"
          style="width:280px"
        >
          <template #prefix><Search :size="14" /></template>
        </n-input>
      </template>
    </Toolbar>

    <BatchActionBar :count="checked.length" @clear="checked = []">
      <n-popconfirm
        :positive-text="`批量清除 (${checked.length})`"
        negative-text="取消"
        @positive-click="clearChecked"
      >
        <template #trigger>
          <n-button size="small" :loading="bulkBusy">
            <template #icon><XCircle :size="13" /></template>
            批量清除标记
          </n-button>
        </template>
        将清除选中的 {{ checked.length }} 个 Key 的滥用标记。
      </n-popconfirm>
    </BatchActionBar>

    <EmptyState
      v-if="!loading && rows.length === 0"
      icon="🛡"
      title="无滥用风险"
      desc="当前所有 API Key 行为均符合安全基线"
    />

    <NDataTable
      v-else
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="(row: AbuseFlag) => row.keyId"
      :checked-row-keys="checked"
      :pagination="pagination"
      :scroll-x="1020"
      :bordered="false"
      size="small"
      class="am-table"
      @update:checked-row-keys="(keys) => (checked = keys)"
    />
  </PageContainer>
</template>

<style scoped>
.metric-strip {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}
.metric-tile {
  padding: 16px 18px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--st-border);
  border-radius: 6px;
}
.metric-tile__label {
  display: inline-flex; align-items: center; gap: 6px;
  font-size: 11px; letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter);
  margin-bottom: 8px;
}
.metric-tile__num {
  font-family: var(--st-font-mono);
  font-variant-numeric: tabular-nums;
  font-size: 24px; line-height: 1;
  color: var(--st-text-pri);
}
.metric-tile__delta { margin-top: 6px; }
.t-meta { font-size: 11px; color: var(--st-text-ter); }

:deep(.am-reason) {
  color: var(--st-text-sec);
  font-size: 13px;
}
:deep(.am-mono) {
  font-family: var(--st-font-mono);
  font-variant-numeric: tabular-nums;
  font-size: 13px;
  color: var(--st-text-pri);
}
:deep(.am-mono--warn) { color: var(--color-warning, #f5a623); }

.am-table :deep(.n-data-table-th) {
  font-size: 11px !important; font-weight: 500 !important;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter) !important;
  background: transparent !important;
  height: 32px !important; padding: 0 12px !important;
  border-bottom: 1px solid var(--st-border) !important;
}
.am-table :deep(.n-data-table-td) {
  height: 40px !important; padding: 0 12px !important;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.am-table :deep(.n-data-table-tr:hover .n-data-table-td) {
  background: rgba(255, 255, 255, 0.04) !important;
}
.am-table :deep(.n-data-table) { background: transparent !important; }
</style>
