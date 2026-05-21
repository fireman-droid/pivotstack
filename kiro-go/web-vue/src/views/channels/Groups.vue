<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { NDataTable, NInput, NButton, NSpace, NTag, NSwitch, NPopconfirm, useMessage, NEmpty, type DataTableColumns } from 'naive-ui'
import { Plus, Layers, Trash2, Settings as SettingsIcon, RefreshCw } from 'lucide-vue-next'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import Toolbar from '../../components/common/Toolbar.vue'
import StatusBadge from '../../components/common/StatusBadge.vue'
import EmptyState from '../../components/common/EmptyState.vue'
import GroupCreateDialog from '../../components/admin/groups/GroupCreateDialog.vue'
import {
  listChannelGroups, deleteChannelGroup, updateChannelGroup,
  type ChannelGroupView,
} from '../../api/admin/groups'
import { useTablePagination } from '../../composables/useTablePagination'

const router = useRouter()
const message = useMessage()
const pagination = useTablePagination(20)
const loading = ref(false)
const rows = ref<ChannelGroupView[]>([])
const search = ref('')
const expandedRowKeys = ref<string[]>([])

const createDialogShow = ref(false)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(g =>
    g.id.toLowerCase().includes(q) ||
    g.name.toLowerCase().includes(q) ||
    (g.description || '').toLowerCase().includes(q) ||
    g.channels.some(c => c.alias.toLowerCase().includes(q))
  )
})

async function reload() {
  loading.value = true
  try {
    rows.value = await listChannelGroups()
  } catch (e: any) {
    message.error(e?.message || '加载失败')
    rows.value = []
  } finally {
    loading.value = false
  }
}

function openDetail(id: string) {
  router.push({ name: 'ChannelsGroupDetail', params: { id } })
}

async function toggleEnabled(g: ChannelGroupView, val: boolean) {
  try {
    await updateChannelGroup(g.id, { enabled: val })
    g.enabled = val
    message.success(val ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '切换失败')
  }
}

async function removeGroup(id: string) {
  try {
    await deleteChannelGroup(id)
    message.success('已删除')
    await reload()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

const columns: DataTableColumns<ChannelGroupView> = [
  {
    type: 'expand',
    expandable: row => row.channels.length > 0,
    renderExpand: row => h('div', { class: 'expand' }, [
      h('div', { class: 'expand__head' }, `已挂载 ${row.channels.length} 条渠道（启用 ${row.enabledChannelCount}）`),
      h('div', { class: 'expand__rows' }, row.channels.map(c =>
        h('div', { class: 'expand__row' }, [
          h(NTag, { size: 'small', bordered: false, type: c.sourceType === 'newapi' ? 'info' : 'success' }, () => c.sourceType === 'newapi' ? 'NewAPI' : '直连'),
          h('span', { class: 'expand__alias' }, c.alias),
          h('span', { class: 'expand__detail mono' }, c.sourceDetail || '-'),
          h('span', { class: 'expand__billing mono' }, c.billing || '-'),
          h(StatusBadge, { status: c.enabled ? 'enabled' : 'disabled' }),
          row.defaultRuntimeChannelId === c.runtimeId
            ? h(NTag, { size: 'tiny', type: 'warning', bordered: false }, () => '默认')
            : null,
        ])
      )),
    ]),
  },
  {
    title: '分组',
    key: 'name',
    width: 220,
    render: row => h('div', { class: 'name' }, [
      h('div', { class: 'name__title' }, row.name),
      h('div', { class: 'name__id mono' }, row.id),
    ]),
  },
  {
    title: '描述',
    key: 'description',
    width: 480,
    ellipsis: { tooltip: true },
    render: row => row.description || h('span', { class: 'dim' }, '-'),
  },
  {
    title: '成员',
    key: 'channels',
    width: 110,
    align: 'center',
    render: row => h('div', { class: 'count' }, [
      h('span', { class: 'mono' }, `${row.enabledChannelCount}/${row.channelCount}`),
      h('span', { class: 'count__hint' }, '启用/总'),
    ]),
  },
  {
    title: '启用',
    key: 'enabled',
    width: 70,
    align: 'center',
    render: row => h(NSwitch, {
      size: 'small',
      value: row.enabled,
      onUpdateValue: (v: boolean) => toggleEnabled(row, v),
    }),
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    align: 'center',
    render: row => h(StatusBadge, { status: row.enabled ? 'enabled' : 'disabled' }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 180,
    align: 'center',
    render: row => h(NSpace, { size: 6, justify: 'center' }, () => [
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => openDetail(row.id) }, { default: () => '管理', icon: () => h(SettingsIcon, { size: 12 }) }),
      h(NPopconfirm, {
        onPositiveClick: () => removeGroup(row.id),
        positiveText: '删除',
        negativeText: '取消',
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { default: () => '删除', icon: () => h(Trash2, { size: 12 }) }),
        default: () => `删除分组「${row.name}」？user 上指向它的偏好会被一起清除。`,
      }),
    ]),
  },
]

onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="渠道"
      :kicker-dot="'#0bd470'"
      title="分组总览"
      desc="admin 自由建分组并挂载多条渠道；user 按分组挑具体渠道"
    >
      <template #actions>
        <n-button size="small" @click="reload" :loading="loading">
          <template #icon><RefreshCw :size="13" /></template>
          刷新
        </n-button>
        <n-button size="small" type="primary" @click="createDialogShow = true">
          <template #icon><Plus :size="14" /></template>
          新建分组
        </n-button>
      </template>
    </PageHeader>
    <Toolbar>
      <template #left>
        <n-input v-model:value="search" placeholder="搜索分组 / 描述 / 渠道" size="small" clearable style="width: 280px" />
      </template>
    </Toolbar>
    <n-data-table
      v-if="filtered.length || loading"
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="(row: ChannelGroupView) => row.id"
      :pagination="pagination"
      :expanded-row-keys="expandedRowKeys"
      @update:expanded-row-keys="(k: string[]) => expandedRowKeys = k"
      :scroll-x="1240"
      size="small"
      striped
    />
    <EmptyState v-else icon="○" title="还没有分组" desc="先点「+ 新建分组」起一个，再去挂载具体渠道">
      <template #cta>
        <n-button size="small" type="primary" @click="createDialogShow = true">
          <Plus :size="14" />
          新建分组
        </n-button>
      </template>
    </EmptyState>
    <GroupCreateDialog v-model:show="createDialogShow" @created="(id: string) => openDetail(id)" />
  </PageContainer>
</template>

<style scoped>
.dim { color: #707070; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; font-variant-numeric: tabular-nums; }
.name { display: flex; flex-direction: column; gap: 2px; line-height: 1.3; }
.name__title { color: #ededed; font-size: 13px; font-weight: 500; }
.name__id { color: #707070; font-size: 11px; }
.count { display: flex; flex-direction: column; align-items: flex-end; line-height: 1.3; }
.count__hint { color: #707070; font-size: 10px; }
.expand { padding: 8px 16px; background: #0a0a0a; }
.expand__head { color: #707070; font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 8px; }
.expand__rows { display: flex; flex-direction: column; gap: 6px; }
.expand__row { display: grid; grid-template-columns: 60px 1fr 1.5fr 1fr 80px 50px; gap: 8px; align-items: center; padding: 6px 0; border-bottom: 1px dashed rgba(255,255,255,0.04); font-size: 12px; }
.expand__row:last-child { border-bottom: none; }
.expand__alias { color: #ededed; }
.expand__detail { color: #707070; font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.expand__billing { color: #a3a3a3; font-size: 11px; }
</style>
