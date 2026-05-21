<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  NDataTable, NInput, NSwitch, NPopconfirm, NButton, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { Search, RefreshCw } from 'lucide-vue-next'
import CopyableText from '../../components/common/CopyableText.vue'
import { listUsers, disableUser, enableUser, type AdminUser } from '../../api/admin/users'
import { useTablePagination } from '../../composables/useTablePagination'

const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const rows = ref<AdminUser[]>([])
const search = ref('')
const togglingId = ref<string>('')

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(u =>
    [u.id, u.email, u.username, u.invitedBy].some(v => String(v || '').toLowerCase().includes(q)),
  )
})

const activeCount = computed(() => rows.value.filter(u => !u.disabled).length)
const monthAgo = Date.now() / 1000 - 30 * 86400
const newThisMonth = computed(() => rows.value.filter(u => u.createdAt >= monthAgo).length)
const totalBoundKeys = computed(() => rows.value.reduce((s, u) => s + (u.apiKeyIds?.length || 0), 0))

function fmtTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}
function relTime(ts?: number) {
  if (!ts) return '从未'
  const diff = Date.now() / 1000 - ts
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  return `${Math.floor(diff / 86400)} 天前`
}

async function reload() {
  loading.value = true
  try {
    const res = await listUsers()
    rows.value = res.users || []
  } catch (e: any) {
    message.error(e?.message || '加载用户失败')
  } finally {
    loading.value = false
  }
}

async function toggle(user: AdminUser, v: boolean) {
  togglingId.value = user.id
  try {
    if (v) await enableUser(user.id)
    else await disableUser(user.id)
    user.disabled = !v
    message.success(v ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '操作失败')
  } finally {
    togglingId.value = ''
  }
}

const columns: DataTableColumns<AdminUser> = [
  {
    title: 'User ID',
    key: 'id',
    width: 240,
    render: row => h(CopyableText, { text: row.id, mono: true, mask: false }),
  },
  {
    title: 'Email',
    key: 'email',
    width: 260,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'u-email' }, row.email),
  },
  {
    title: 'Username',
    key: 'username',
    width: 160,
    ellipsis: { tooltip: true },
    render: row => h('span', { class: 'u-dim' }, row.username || '-'),
  },
  {
    title: '绑定 Keys',
    key: 'apiKeyIds',
    width: 110,
    align: 'center',
    render: row => h('span', { class: 'u-chip u-mono' }, String(row.apiKeyIds?.length || 0)),
  },
  {
    title: '邀请人',
    key: 'invitedBy',
    width: 130,
    align: 'center',
    render: row => h('span', { class: 'u-mono u-dim' }, row.invitedBy || '-'),
  },
  {
    title: '最后登录',
    key: 'lastLoginAt',
    width: 120,
    align: 'center',
    render: row => h('span', { class: 'u-dim' }, relTime(row.lastLoginAt)),
  },
  {
    title: '注册时间',
    key: 'createdAt',
    width: 120,
    align: 'center',
    render: row => h('span', { class: 'u-dim' }, relTime(row.createdAt)),
  },
  {
    title: '状态',
    key: 'disabled',
    width: 90,
    align: 'center',
    render: row => h(NSwitch, {
      size: 'small',
      value: !row.disabled,
      loading: row.id === togglingId.value,
      onUpdateValue: (v: boolean) => toggle(row, v),
    }),
  },
]

onMounted(reload)
</script>

<template>
  <div class="admin-page">
    <header class="page-head">
      <div>
        <div class="page-head__crumb"><b>SYSTEM</b> / 用户管理</div>
        <div class="page-head__title">
          <div class="t-display-admin">用户管理</div>
          <div class="page-head__sub">User 实体（区别于 API Key）· {{ rows.length }} 个 · {{ activeCount }} 活跃</div>
        </div>
      </div>
      <div class="page-head__right">
        <button class="u-btn u-btn--ghost" :disabled="loading" @click="reload">
          <RefreshCw :size="14" :class="{ 'is-spinning': loading }" />
          刷新
        </button>
      </div>
    </header>

    <section class="metric-strip">
      <div class="metric-tile">
        <div class="metric-tile__label">总用户</div>
        <div class="metric-tile__num">{{ rows.length }}</div>
        <div class="metric-tile__delta"><span class="t-meta">{{ activeCount }} 活跃 / {{ rows.length - activeCount }} 禁用</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">本月新增</div>
        <div class="metric-tile__num">{{ newThisMonth }}</div>
        <div class="metric-tile__delta"><span class="t-meta">最近 30 天</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">绑定 Keys</div>
        <div class="metric-tile__num">{{ totalBoundKeys }}</div>
        <div class="metric-tile__delta"><span class="t-meta">全部用户合计</span></div>
      </div>
      <div class="metric-tile">
        <div class="metric-tile__label">平均 Keys/人</div>
        <div class="metric-tile__num">{{ rows.length ? (totalBoundKeys / rows.length).toFixed(1) : '0.0' }}</div>
        <div class="metric-tile__delta"><span class="t-meta">绑定密度</span></div>
      </div>
    </section>

    <div class="u-filter">
      <NInput v-model:value="search" clearable size="small" placeholder="搜索 id / email / username / 邀请人" style="width:320px">
        <template #prefix><Search :size="14" /></template>
      </NInput>
    </div>

    <NDataTable
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="row => row.id"
      :pagination="pagination"
      :scroll-x="1330"
      :bordered="false"
      size="small"
      class="u-table"
    />
  </div>
</template>

<style scoped>
.u-btn {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--st-border);
  border-radius: 4px;
  color: var(--st-text-pri);
  font-size: 12px; font-family: inherit;
  cursor: pointer;
}
.u-btn--ghost { background: transparent; }
.u-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); }
.is-spinning { animation: u-spin 0.8s linear infinite; }
@keyframes u-spin { to { transform: rotate(360deg); } }

.u-filter { margin-bottom: 16px; }

:deep(.u-email) { color: var(--st-text-pri); font-size: 13px; font-weight: 500; }
:deep(.u-dim) { color: var(--st-text-ter); font-size: 12px; }
:deep(.u-mono) { font-family: var(--st-font-mono); font-variant-numeric: tabular-nums; font-size: 12px; }
:deep(.u-chip) {
  display: inline-flex; align-items: center;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 3px;
  font-size: 11px;
  color: var(--st-text-pri);
}

.u-table :deep(.n-data-table-th) {
  font-size: 11px !important; font-weight: 500 !important;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter) !important;
  background: transparent !important;
  height: 32px !important; padding: 0 12px !important;
  border-bottom: 1px solid var(--st-border) !important;
}
.u-table :deep(.n-data-table-td) {
  height: 40px !important; padding: 0 12px !important;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.u-table :deep(.n-data-table-tr:hover .n-data-table-td) { background: rgba(255, 255, 255, 0.04) !important; }
.u-table :deep(.n-data-table) { background: transparent !important; }
</style>
