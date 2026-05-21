<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton, NDataTable, NPopconfirm, NTag, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { Plus, RefreshCw, Pencil, Trash2, Power, PowerOff, BookOpen } from 'lucide-vue-next'
import UserKeyCreateDrawer from '../../components/user/keys/UserKeyCreateDrawer.vue'
import UserKeyModelsDrawer from '../../components/user/keys/UserKeyModelsDrawer.vue'
import CopyableText from '../../components/common/CopyableText.vue'
import { listUserKeys, patchUserKey, deleteUserKey } from '../../api/user'

interface UserKey {
  id: string
  key?: string  // raw API key — user 自己的 key 可重复查看复制
  note?: string
  enabled: boolean
  plan?: string
  // v8: balance/giftBalance 不再是 per-key 字段（钱包在 user 上），保留以防后端兼容返回
  balance?: number
  giftBalance?: number
  createdAt: number
  requests: number
  errors: number
  tokens: number
  credits: number
  isDefault?: boolean
}

const message = useMessage()
const rows = ref<UserKey[]>([])
const loading = ref(false)
const drawerShow = ref(false)
const editKey = ref<UserKey | null>(null) // null = 新建模式；非空 = 编辑该 key
const modelsDrawerShow = ref(false)
const modelsForKey = ref<UserKey | null>(null)
const needsBindAccount = ref(false)

const summary = computed(() => {
  return {
    total: rows.value.length,
    enabled: rows.value.filter(k => k.enabled).length,
    totalCredits: rows.value.reduce((s, k) => s + (k.credits || 0), 0),
  }
})

async function reload(showSpinner = false) {
  if (showSpinner) loading.value = true
  needsBindAccount.value = false
  let data: UserKey[] | null = null
  let err: any = null
  try {
    data = await listUserKeys()
  } catch (e: any) {
    err = e
  }
  loading.value = false
  if (data) {
    rows.value = data
    return
  }
  if (err) {
    const msg = String(err?.message || err || '')
    if (/bind|forbid|未绑/i.test(msg)) {
      needsBindAccount.value = true
      rows.value = []
    } else {
      message.error(msg || '加载失败')
    }
  }
}
function onManualRefresh() { reload(true) }

async function toggleEnabled(row: UserKey) {
  try {
    const patched = await patchUserKey(row.id, { enabled: !row.enabled })
    Object.assign(row, patched)
    message.success(row.enabled ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '更新失败')
  }
}

function openEdit(row: UserKey) {
  editKey.value = row
  drawerShow.value = true
}
function openCreate() {
  editKey.value = null
  drawerShow.value = true
}
function openModels(row: UserKey) {
  modelsForKey.value = row
  modelsDrawerShow.value = true
}

async function doDelete(row: UserKey) {
  try {
    await deleteUserKey(row.id)
    rows.value = rows.value.filter(r => r.id !== row.id)
    message.success('已删除')
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

function fmtTime(ts?: number) {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false })
}


const columns: DataTableColumns<UserKey> = [
  {
    title: '名称', key: 'note', minWidth: 240,
    render: (row) => h('div', { class: 'note-cell' }, [
      h('span', { class: 'note-cell__name' }, row.note || '（未命名）'),
      row.isDefault ? h(NTag, { size: 'tiny', type: 'success', bordered: false }, () => '默认') : null,
    ]),
  },
  {
    title: 'API Key', key: 'key', width: 240, align: 'center',
    render: (row) => h(CopyableText, {
      text: row.key || row.id,
      mono: true,
      mask: true,
      maskHead: 6,
      maskTail: 6,
    }),
  },
  {
    title: '状态', key: 'enabled', width: 100, align: 'center',
    render: (row) => h(NTag, {
      size: 'small', bordered: false,
      type: row.enabled ? 'success' : 'default',
    }, () => row.enabled ? '启用' : '禁用'),
  },
  {
    title: '调用 / 错误', key: 'requests', width: 130, align: 'center',
    render: (row) => h('span', { class: 'cell-mono' }, `${row.requests || 0} / ${row.errors || 0}`),
  },
  {
    title: '消耗', key: 'credits', width: 110, align: 'center',
    render: (row) => h('span', { class: 'cell-mono cell-mono--dim' }, `$${(row.credits || 0).toFixed(4)}`),
  },
  {
    title: '创建时间', key: 'createdAt', width: 170, align: 'center',
    render: (row) => h('span', { class: 'cell-mono cell-mono--dim' }, fmtTime(row.createdAt)),
  },
  {
    title: '操作', key: 'actions', width: 170, align: 'center',
    render: (row) => h('div', { class: 'cell-actions' }, [
      h(NButton, { quaternary: true, size: 'tiny', title: '查看可用模型与单价', onClick: () => openModels(row) },
        { default: () => h(BookOpen, { size: 14 }) }),
      h(NButton, { quaternary: true, size: 'tiny', title: '编辑名称与路由', onClick: () => openEdit(row) },
        { default: () => h(Pencil, { size: 14 }) }),
      h(NButton, {
        quaternary: true, size: 'tiny',
        title: row.enabled ? '禁用此 Key' : '启用此 Key',
        onClick: () => toggleEnabled(row),
      }, { default: () => h(row.enabled ? Power : PowerOff, { size: 14 }) }),
      h(NPopconfirm, {
        onPositiveClick: () => doDelete(row),
        positiveText: '删除', negativeText: '取消',
        positiveButtonProps: { type: 'error', size: 'small' },
        negativeButtonProps: { quaternary: true, size: 'small' },
      }, {
        default: () => '确认删除此 Key？此操作不可恢复',
        trigger: () => h(NButton, { quaternary: true, size: 'tiny', title: '删除', type: 'error' },
          { default: () => h(Trash2, { size: 14 }) }),
      }),
    ]),
  },
]

onMounted(reload)
</script>

<template>
  <div class="page">
    <header class="page__head">
      <div>
        <div class="page__crumb"><b>USER</b> / API Keys</div>
        <h1 class="page__title">我的 API Key</h1>
        <div class="page__sub">
          共 {{ summary.total }} 把 · {{ summary.enabled }} 启用 · 累计消耗 ${{ summary.totalCredits.toFixed(2) }}
        </div>
      </div>
      <div class="page__actions">
        <NButton size="small" :loading="loading" @click="onManualRefresh">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </NButton>
        <NButton type="primary" size="small" @click="openCreate">
          <template #icon><Plus :size="14" /></template>
          新建 Key
        </NButton>
      </div>
    </header>

    <div v-show="needsBindAccount" class="bind-hint">
      <div class="bind-hint__title">👋 你还没有账号</div>
      <div class="bind-hint__body">
        当前用 <code>API Key</code> 登录的旧用户，只有一把 key。<br>
        升级账号（邮箱 + 密码）后即可创建多把 key，每把 key 单独配置渠道、过期时间和速率限制。
      </div>
      <div class="bind-hint__hint">系统首次打开会弹"升级账号"窗口；如果之前跳过了，回到 <a href="/user/dashboard">概览</a> 再刷新即可重新弹出。</div>
    </div>

    <NDataTable
      v-show="!needsBindAccount"
      :columns="columns"
      :data="rows"
      :loading="loading"
      :scroll-x="1180"
      :bordered="false"
      :single-line="false"
      size="small"
      :row-key="(row: UserKey) => row.id"
      class="ntable"
    />

    <UserKeyCreateDrawer
      v-model:show="drawerShow"
      :edit-key="editKey"
      @created="reload"
      @updated="reload"
    />
    <UserKeyModelsDrawer
      v-model:show="modelsDrawerShow"
      :api-key="modelsForKey"
    />
  </div>
</template>

<style scoped>
.page { padding: 24px 32px; background: #000; min-height: 100vh; color: #ededed; }
.page__head {
  display: flex; justify-content: space-between; align-items: flex-end;
  margin-bottom: 24px; padding-bottom: 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.page__crumb { font-size: 11px; letter-spacing: 0.06em; color: #707070; text-transform: uppercase; }
.page__crumb b { color: #ededed; font-weight: 600; }
.page__title { font-size: 18px; font-weight: 600; letter-spacing: -0.01em; margin: 4px 0; color: #ededed; }
.page__sub { font-size: 12px; color: #707070; }
.page__actions { display: flex; gap: 8px; }

:deep(.note-cell) { display: inline-flex; align-items: center; gap: 8px; }
:deep(.note-cell__name) { color: #ededed; font-size: 13px; }
:deep(.cell-mono) {
  font-family: "Geist Mono", ui-monospace, monospace;
  font-size: 12px; font-variant-numeric: tabular-nums;
}
:deep(.cell-mono--dim) { color: #707070; }
:deep(.cell-actions) { display: inline-flex; gap: 4px; }

.bind-hint {
  padding: 32px 32px;
  background: rgba(82, 168, 255, 0.04);
  border: 1px solid rgba(82, 168, 255, 0.20);
  border-radius: 6px;
  max-width: 720px;
  margin: 16px 0;
}
.bind-hint__title { color: #ededed; font-size: 16px; font-weight: 600; margin-bottom: 12px; }
.bind-hint__body { color: #a1a1a1; font-size: 13px; line-height: 1.7; }
.bind-hint__body code {
  padding: 1px 6px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 3px;
  font-family: "Geist Mono", monospace;
  font-size: 11px;
}
.bind-hint__hint { color: #707070; font-size: 12px; line-height: 1.6; margin-top: 14px; }
.bind-hint__hint a { color: #52a8ff; text-decoration: none; }
.bind-hint__hint a:hover { text-decoration: underline; }
</style>
