<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NTabs, NTabPane, NDataTable, NSpin, NSwitch, NPopconfirm, NTag, useMessage, type DataTableColumns } from 'naive-ui'
import { ArrowLeft, Pencil, RefreshCw, Trash2 } from 'lucide-vue-next'
import PageContainer from '../../../components/common/PageContainer.vue'
import PageHeader from '../../../components/common/PageHeader.vue'
import MonoValue from '../../../components/common/MonoValue.vue'
import StatusBadge from '../../../components/common/StatusBadge.vue'
import EmptyState from '../../../components/common/EmptyState.vue'
import DirectChannelDrawer from '../../../components/admin/direct/DirectChannelDrawer.vue'
import KiroPoolTab from '../../../components/admin/direct/KiroPoolTab.vue'
import { listDirectChannels, patchDirectChannel, deleteDirectChannel, type DirectChannel } from '../../../api/admin/directChannels'
import { listLogs, type CallLog } from '../../../api/admin/logs'
import { useTablePagination } from '../../../composables/useTablePagination'

const KIRO_BUILTIN_ID = 'kiro:builtin'

const props = defineProps<{ id: string }>()
const router = useRouter()
const message = useMessage()

const loading = ref(false)
const togglingEnabled = ref(false)
const ch = ref<DirectChannel | null>(null)
const recentLogs = ref<CallLog[]>([])
const tab = ref<'overview' | 'pricing' | 'logs' | 'pool'>('overview')
const editShow = ref(false)
const logsPagination = useTablePagination(20)

const isKiroBuiltin = computed(() => props.id === KIRO_BUILTIN_ID || ch.value?.type === 'kiro')

async function reload() {
  loading.value = true
  try {
    const [all, logResp] = await Promise.all([
      listDirectChannels(),
      listLogs({ limit: 200 }).catch(() => ({ logs: [] })),
    ])
    if (props.id === KIRO_BUILTIN_ID) {
      // 内建 kiro 渠道：构造一条虚拟记录
      const real = all.find(c => c.type === 'kiro')
      ch.value = real || {
        id: KIRO_BUILTIN_ID,
        type: 'kiro',
        alias: 'Kiro 账号池',
        enabled: true,
        models: [],
      } as DirectChannel
    } else {
      ch.value = all.find(c => c.id === props.id) || null
    }
    recentLogs.value = (logResp.logs || []).filter(l => l.channel_id === props.id || l.channel_alias === ch.value?.alias || (isKiroBuiltin.value && l.channel_type === 'kiro'))
    if (isKiroBuiltin.value && tab.value === 'overview') tab.value = 'pool'
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function toggleEnabled(v: boolean) {
  if (!ch.value) return
  togglingEnabled.value = true
  try {
    await patchDirectChannel(ch.value.id, { enabled: v })
    ch.value.enabled = v
    message.success(v ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '切换失败')
  } finally {
    togglingEnabled.value = false
  }
}

async function doDelete() {
  if (!ch.value) return
  try {
    await deleteDirectChannel(ch.value.id)
    message.success('已删除')
    router.push({ name: 'ChannelsDirect' })
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

function formatTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}

const stats = computed(() => {
  const total = recentLogs.value.length
  const success = recentLogs.value.filter(l => !l.error && l.status !== 'error').length
  return { total, success, failed: total - success }
})

interface PriceRow { model: string; inputPerM?: number; outputPerM?: number }
const priceRows = computed<PriceRow[]>(() => {
  const rows: PriceRow[] = []
  const def = ch.value?.sellPrice?.default
  if (def && (def.inputPerM || def.outputPerM)) rows.push({ model: '(default)', inputPerM: def.inputPerM, outputPerM: def.outputPerM })
  const perModel = ch.value?.sellPrice?.models || {}
  for (const [m, v] of Object.entries(perModel)) rows.push({ model: m, inputPerM: v.inputPerM, outputPerM: v.outputPerM })
  return rows
})

const priceColumns: DataTableColumns<PriceRow> = [
  { title: 'Model', key: 'model', width: 280, ellipsis: { tooltip: true }, render: r => h('span', { class: 'mono' }, r.model) },
  { title: '售价 IN ¥/Mtok', key: 'in', width: 180, align: 'center', render: r => h('span', { class: 'mono' }, `¥${(r.inputPerM ?? 0).toFixed(4)}`) },
  { title: '售价 OUT ¥/Mtok', key: 'out', width: 180, align: 'center', render: r => h('span', { class: 'mono' }, `¥${(r.outputPerM ?? 0).toFixed(4)}`) },
]

const logsColumns: DataTableColumns<CallLog> = [
  { title: '时间', key: 'time', width: 170, align: 'center', render: r => h(MonoValue, { value: r.time || (r.timestamp ? new Date(r.timestamp * 1000).toLocaleString('zh-CN', { hour12: false }) : '-') }) },
  { title: '模型', key: 'model', width: 240, ellipsis: { tooltip: true }, render: r => h(MonoValue, { value: r.original_model || '-' }) },
  { title: 'Token', key: 'tokens', width: 110, align: 'center', render: r => h('span', { class: 'mono' }, ((r.input_tokens || 0) + (r.output_tokens || 0)).toLocaleString()) },
  {
    title: '状态',
    key: 'status',
    width: 90,
    align: 'center',
    render: r => h(StatusBadge, {
      status: r.error || r.status === 'error' ? 'error' : 'success',
      label: r.error || r.status === 'error' ? '失败' : '成功',
    }),
  },
]

onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="渠道 · 自营 · 详情"
      :kicker-dot="'#707070'"
      :title="ch?.alias || ch?.id || props.id"
      :desc="ch ? `ID ${ch.id} · 类型 ${ch.type}` : '加载中…'"
    >
      <template #actions>
        <n-button quaternary size="small" @click="router.push({ name: 'ChannelsDirect' })">
          <template #icon><ArrowLeft :size="14" /></template>
          返回列表
        </n-button>
        <n-button size="small" :loading="loading" @click="reload">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </n-button>
        <n-button v-if="ch" size="small" @click="editShow = true">
          <template #icon><Pencil :size="14" /></template>
          编辑
        </n-button>
        <n-popconfirm v-if="ch && !isKiroBuiltin" @positive-click="doDelete" positive-text="删除" negative-text="取消">
          <template #trigger>
            <n-button size="small" quaternary type="error">
              <template #icon><Trash2 :size="14" /></template>
              删除
            </n-button>
          </template>
          删除「{{ ch.alias || ch.id }}」？将软删（保留 tombstone）。
        </n-popconfirm>
      </template>
    </PageHeader>

    <n-spin v-if="loading && !ch" />
    <EmptyState v-else-if="!ch" icon="○" title="找不到该自营渠道" />

    <template v-else-if="ch">
      <section class="hero-grid">
        <div class="card">
          <span class="card__label">类型</span>
          <span class="card__value">
            <n-tag size="medium" :bordered="false" :type="ch.type === 'kiro' ? 'info' : 'success'">{{ ch.type }}</n-tag>
          </span>
          <span class="card__sub">{{ ch.type === 'kiro' ? '共享 kiro 账号池' : 'OpenAI 协议透传' }}</span>
        </div>
        <div class="card">
          <span class="card__label">模型数</span>
          <span class="card__value mono">{{ ch.models?.length || 0 }}</span>
          <span class="card__sub">{{ ch.models?.length ? '限定列表' : '透传所有上游模型' }}</span>
        </div>
        <div class="card">
          <span class="card__label">启用</span>
          <div class="card__switch">
            <n-switch size="medium" :value="ch.enabled" :loading="togglingEnabled" @update:value="toggleEnabled" />
            <span class="card__state" :class="{ on: ch.enabled }">{{ ch.enabled ? '运行中' : '已停用' }}</span>
          </div>
          <span class="card__sub">{{ ch.enabled ? '正常路由请求' : '该渠道临时禁用' }}</span>
        </div>
        <div class="card">
          <span class="card__label">最近 200 条调用</span>
          <span class="card__value mono">{{ stats.total }}</span>
          <span class="card__sub">{{ stats.success }} 成功 / {{ stats.failed }} 失败</span>
        </div>
      </section>

      <n-tabs v-model:value="tab" size="small" class="sub-tabs">
        <n-tab-pane v-if="isKiroBuiltin" name="pool" tab="账号池" />
        <n-tab-pane name="overview" tab="概览" />
        <n-tab-pane name="pricing" :tab="`定价 (${priceRows.length})`" />
        <n-tab-pane name="logs" :tab="`调用日志 (${recentLogs.length})`" />
      </n-tabs>

      <KiroPoolTab v-if="tab === 'pool'" />

      <section v-else-if="tab === 'overview'" class="overview">
        <dl class="kv">
          <div class="kv__row"><dt>Base URL</dt><dd class="mono">{{ ch.baseUrl || '-' }}</dd></div>
          <div class="kv__row"><dt>API Key</dt><dd>{{ ch.hasAPIKey ? '已配置（隐藏）' : '未配置' }}</dd></div>
          <div class="kv__row"><dt>状态</dt><dd>{{ ch.status || (ch.enabled ? '启用' : '禁用') }}</dd></div>
          <div class="kv__row"><dt>创建时间</dt><dd class="mono">{{ formatTime(ch.createdAt) }}</dd></div>
          <div class="kv__row"><dt>更新时间</dt><dd class="mono">{{ formatTime(ch.updatedAt) }}</dd></div>
          <div class="kv__row" v-if="ch.models?.length"><dt>支持模型</dt><dd class="mono small">{{ ch.models.join(', ') }}</dd></div>
        </dl>
      </section>

      <n-data-table
        v-else-if="tab === 'pricing' && priceRows.length"
        :columns="priceColumns"
        :data="priceRows"
        :row-key="r => r.model"
        :scroll-x="640"
        size="small"
        striped
      />
      <EmptyState v-else-if="tab === 'pricing'" icon="○" title="未单独设置定价" desc="沿用全局定价（定价中心）" />

      <n-data-table
        v-else-if="tab === 'logs' && recentLogs.length"
        :columns="logsColumns"
        :data="recentLogs"
        :row-key="r => r.request_id || `${r.time}-${r.original_model}`"
        :pagination="logsPagination"
        :scroll-x="610"
        size="small"
        striped
      />
      <EmptyState v-else-if="tab === 'logs'" icon="○" title="该渠道暂无调用记录" />

      <DirectChannelDrawer v-model:show="editShow" :type="ch.type" :row="ch" @saved="reload" />
    </template>
  </PageContainer>
</template>

<style scoped>
.hero-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 16px; }
.card { display: flex; flex-direction: column; gap: 6px; padding: 16px; border: 1px solid rgba(255,255,255,0.06); border-radius: 6px; background: #0a0a0a; }
.card__label { font-size: 11px; color: #707070; text-transform: uppercase; letter-spacing: 0.06em; }
.card__value { font-size: 24px; font-weight: 600; color: #ededed; }
.card__sub { font-size: 11px; color: #707070; }
.card__switch { display: flex; align-items: center; gap: 10px; margin-top: 2px; }
.card__state { font-size: 13px; color: #707070; font-weight: 500; }
.card__state.on { color: #0bd470; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; }
.mono.small { font-size: 12px; }

.sub-tabs { margin-bottom: 16px; }
.overview { padding: 16px; border: 1px solid rgba(255,255,255,0.06); border-radius: 6px; background: #0a0a0a; }
.kv { display: flex; flex-direction: column; gap: 8px; margin: 0; }
.kv__row { display: grid; grid-template-columns: 140px 1fr; font-size: 13px; align-items: start; }
.kv__row dt { color: #707070; }
.kv__row dd { color: #ededed; margin: 0; }
@media (max-width: 1080px) { .hero-grid { grid-template-columns: repeat(2, 1fr); } }
</style>
