<script setup lang="ts">
// Admin 视角：代理商总览。
// 基于现有 listApiKeys() 数据按 isReseller 筛选，无需额外后端 endpoint。
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  NDataTable, NInput, NSwitch, NButton,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { Search, RefreshCw, ChevronRight } from 'lucide-vue-next'
import { listApiKeys, type ApiKeyRow } from '../../api/admin/keys'
import { useTablePagination } from '../../composables/useTablePagination'

const router = useRouter()
const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const allKeys = ref<ApiKeyRow[]>([])
const search = ref('')

interface ResellerRow extends ApiKeyRow {
  childKeyCount: number
  childKeyBalance: number
  soldToChildren?: number
}

const resellers = computed<ResellerRow[]>(() => {
  const all = allKeys.value
  const childrenMap = new Map<string, ApiKeyRow[]>()
  for (const k of all) {
    if (k.parentKeyId) {
      const list = childrenMap.get(k.parentKeyId) || []
      list.push(k)
      childrenMap.set(k.parentKeyId, list)
    }
  }
  return all
    .filter(k => k.isReseller)
    .map(r => {
      const children = childrenMap.get(r.id) || []
      const childKeyBalance = children.reduce((s, c) => s + (c.totalBalance ?? (c.balance || 0) + (c.giftBalance || 0)), 0)
      return {
        ...r,
        childKeyCount: children.length,
        childKeyBalance,
        soldToChildren: (r as any).soldToChildren,
      }
    })
})

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return resellers.value
  return resellers.value.filter(r =>
    [r.note, r.id, r.key, r.keyMasked].some(v => String(v || '').toLowerCase().includes(q)),
  )
})

// metric strip
const totalResellers = computed(() => resellers.value.length)
const totalChildKeys = computed(() => resellers.value.reduce((s, r) => s + r.childKeyCount, 0))
const totalSold = computed(() => resellers.value.reduce((s, r) => s + (r.soldToChildren || 0), 0))
const avgChildren = computed(() => totalResellers.value ? totalChildKeys.value / totalResellers.value : 0)

function fmtUSD(v?: number) { return `$${Number(v || 0).toFixed(2)}` }
function relTime(ts?: number) {
  if (!ts) return '从未'
  const diff = Date.now() / 1000 - ts
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  return `${Math.floor(diff / 86400)} 天前`
}

const columns: DataTableColumns<ResellerRow> = [
  {
    title: '代理商',
    key: 'note',
    width: 240,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'rs-note' }, row.note || `代理 ${row.id.slice(0, 8)}`),
  },
  {
    title: 'Key',
    key: 'key',
    width: 160,
    render: row => h('span', { class: 'rs-mono rs-dim' }, row.keyMasked || `${row.id.slice(0, 8)}...`),
  },
  {
    title: '子 Key 数',
    key: 'children',
    width: 120,
    align: 'center',
    render: row => h('span', { class: 'rs-mono rs-children' }, `${row.childKeyCount} / ${row.maxChildKeys || '∞'}`),
  },
  {
    title: '子 Key 余额',
    key: 'childBalance',
    width: 130,
    align: 'center',
    render: row => h('span', { class: 'rs-mono' }, fmtUSD(row.childKeyBalance)),
  },
  {
    title: '累计销售',
    key: 'sold',
    width: 130,
    align: 'center',
    render: row => h('span', { class: 'rs-mono rs-sold' }, fmtUSD(row.soldToChildren)),
  },
  {
    title: '上次活跃',
    key: 'lastUsed',
    width: 120,
    align: 'center',
    render: row => h('span', { class: 'rs-dim' }, relTime(row.lastUsed)),
  },
  {
    title: '启用',
    key: 'enabled',
    width: 80,
    align: 'center',
    render: row => h(NSwitch, { size: 'small', value: row.enabled, disabled: true }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 110,
    align: 'center',
    render: row => h(NButton, {
      size: 'tiny', quaternary: true,
      onClick: () => router.push({ name: 'BillingKeyDetail', params: { id: row.id } }),
    }, { default: () => '详情', icon: () => h(ChevronRight, { size: 13 }) }),
  },
]

async function reload() {
  loading.value = true
  try {
    allKeys.value = await listApiKeys()
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(reload)
</script>

<template>
  <div class="admin-page">
    <header class="page-head">
      <div>
        <div class="page-head__crumb"><b>RESELLER</b> / 代理商总览</div>
        <div class="page-head__title">
          <div class="t-display-admin">代理商总览</div>
          <div class="page-head__sub">{{ totalResellers }} 位代理商 · 累计销售 {{ fmtUSD(totalSold) }} · {{ totalChildKeys }} 个子 Key</div>
        </div>
      </div>
      <div class="page-head__right">
        <button class="rs-btn rs-btn--ghost" :disabled="loading" @click="reload">
          <RefreshCw :size="14" :class="{ 'is-spinning': loading }" />
          刷新
        </button>
      </div>
    </header>

    <section class="metric-strip">
      <div class="metric-tile">
        <div class="metric-tile__label">代理商</div>
        <div class="metric-tile__num">{{ totalResellers }}</div>
        <div class="metric-tile__delta"><span class="t-meta">isReseller = true 的 keys</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">总子 Key</div>
        <div class="metric-tile__num">{{ totalChildKeys }}</div>
        <div class="metric-tile__delta"><span class="t-meta">所有代理商旗下</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">累计销售</div>
        <div class="metric-tile__num">${{ totalSold.toFixed(0) }}</div>
        <div class="metric-tile__delta"><span class="t-meta">SoldToChildren 合计</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">平均子 Key</div>
        <div class="metric-tile__num">{{ avgChildren.toFixed(1) }}</div>
        <div class="metric-tile__delta"><span class="t-meta">每代理商</span></div>
      </div>
    </section>

    <div class="rs-filter">
      <NInput v-model:value="search" clearable size="small" placeholder="搜索备注 / key / id" style="width:320px">
        <template #prefix><Search :size="14" /></template>
      </NInput>
    </div>

    <NDataTable
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="row => row.id"
      :pagination="pagination"
      :scroll-x="1290"
      :bordered="false"
      size="small"
      class="rs-table"
    />
  </div>
</template>

<style scoped>
.rs-btn {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--st-border);
  border-radius: 4px;
  color: var(--st-text-pri);
  font-size: 12px; font-family: inherit; cursor: pointer;
}
.rs-btn--ghost { background: transparent; }
.rs-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); }
.is-spinning { animation: rs-spin 0.8s linear infinite; }
@keyframes rs-spin { to { transform: rotate(360deg); } }

.rs-filter { margin-bottom: 16px; }

:deep(.rs-note) { color: var(--st-text-pri); font-size: 13px; font-weight: 500; }
:deep(.rs-mono) { font-family: var(--st-font-mono); font-variant-numeric: tabular-nums; font-size: 12px; }
:deep(.rs-dim) { color: var(--st-text-ter); }
:deep(.rs-children) { color: var(--st-text-pri); }
:deep(.rs-sold) { color: var(--st-success); font-weight: 500; }

.rs-table :deep(.n-data-table-th) {
  font-size: 11px !important; font-weight: 500 !important;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter) !important;
  background: transparent !important;
  height: 32px !important; padding: 0 12px !important;
  border-bottom: 1px solid var(--st-border) !important;
}
.rs-table :deep(.n-data-table-td) {
  height: 40px !important; padding: 0 12px !important;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.rs-table :deep(.n-data-table-tr:hover .n-data-table-td) { background: rgba(255, 255, 255, 0.04) !important; }
.rs-table :deep(.n-data-table) { background: transparent !important; }
</style>
