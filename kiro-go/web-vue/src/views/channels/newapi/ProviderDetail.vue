<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NTabs, NTabPane, NDataTable, NTag, NSpin, NPopconfirm, useMessage, type DataTableColumns } from 'naive-ui'
import { ArrowLeft, Pencil, RefreshCw, Trash2, Plus } from 'lucide-vue-next'
import PageContainer from '../../../components/common/PageContainer.vue'
import PageHeader from '../../../components/common/PageHeader.vue'
import MonoValue from '../../../components/common/MonoValue.vue'
import EmptyState from '../../../components/common/EmptyState.vue'
import ProviderDrawer from '../../../components/admin/newapi/ProviderDrawer.vue'
import PriceCell from '../../../components/admin/newapi/PriceCell.vue'
import CreateTokenDialog from '../../../components/admin/newapi/CreateTokenDialog.vue'
import {
  getProvider, getProviderMetadata, syncProvider, deleteProvider,
  type NewAPIProvider,
} from '../../../api/admin/providers'
import { deleteNewAPIChannel } from '../../../api/admin'
import { useTablePagination } from '../../../composables/useTablePagination'
import { useNewAPIPricing } from '../../../composables/useNewAPIPricing'

// 用户决策：NewAPI 上游详情页只保留「同步 + 上游元数据」；
// channel 增删改、markup 调整都在 ChannelGroup 详情页里统一做。
// 物化渠道 tab 已移除。

const props = defineProps<{ id: string }>()
const router = useRouter()
const message = useMessage()

const loading = ref(false)
const syncing = ref(false)
const provider = ref<NewAPIProvider | null>(null)
const metadata = ref<{ groups?: any[]; models?: any[]; tokens?: any[]; updatedAt?: number }>({})
const pricing = useNewAPIPricing(metadata as any)

const tab = ref<'overview' | 'metadata'>('overview')
const editShow = ref(false)
const createTokenShow = ref(false)
const tokenPagination = useTablePagination(20)

async function reload() {
  loading.value = true
  try {
    const [p, meta] = await Promise.all([
      getProvider(props.id),
      getProviderMetadata(props.id).catch(() => ({})),
    ])
    provider.value = p
    metadata.value = meta
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function runSync() {
  syncing.value = true
  try {
    await syncProvider(props.id)
    message.success('已触发同步，10s 后自动刷新')
    setTimeout(reload, 10000)
  } catch (e: any) {
    message.error(e?.message || '同步失败')
  } finally {
    syncing.value = false
  }
}

async function doDelete() {
  try {
    await deleteProvider(props.id)
    message.success('已删除')
    router.push({ name: 'ChannelsNewAPI' })
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

// 删除 PivotStack 本地物化渠道 + 同步上游 token（保证不留 apijing 上孤儿）
async function deleteChannel(upstreamTokenID: number) {
  const channelID = `${props.id}:tok-${upstreamTokenID}`
  try {
    await deleteNewAPIChannel(channelID, { deleteUpstream: true })
    message.success('已删除渠道')
    reload()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

function formatTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}

const tokenColumns: DataTableColumns<any> = [
  { title: 'Token ID', key: 'id', width: 110, align: 'center', render: r => h(MonoValue, { value: String(r.id) }) },
  {
    title: '上游分组 / Token 名',
    key: 'name',
    width: 320,
    ellipsis: { tooltip: true },
    render: r => {
      const sameName = (r.name || '') === (r.group || '')
      return h('div', { style: 'display:flex;flex-direction:column;gap:2px;min-width:0;line-height:1.4' }, [
        h('span', { style: 'color:#ededed;font-size:13px;font-weight:500' }, r.group || '-'),
        sameName
          ? null
          : h('span', { style: 'color:#707070;font-size:11px;font-family:"Geist Mono",ui-monospace,monospace' }, `token: ${r.name}`),
      ])
    },
  },
  {
    title: '上游状态',
    key: 'status',
    width: 110,
    align: 'center',
    render: r => h(NTag, { size: 'small', bordered: false, type: r.status === 1 ? 'success' : 'default' }, () => r.status === 1 ? '可用' : `状态 ${r.status}`),
  },
  {
    title: '上游入价 $/Mtok',
    key: 'upstreamPrice',
    width: 260,
    align: 'center',
    render: r => h(PriceCell, { summary: pricing.summaryForGroup(r.group || '') }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 110,
    align: 'center',
    render: r => h(NPopconfirm, {
      onPositiveClick: () => deleteChannel(r.id),
      positiveText: '删除', negativeText: '取消',
    }, {
      default: () => `永久删除 channel「${r.name || r.id}」？会同步删除 apijing 上游 token，不可恢复。`,
      trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, {
        default: () => '删除',
        icon: () => h(Trash2, { size: 12 }),
      }),
    }),
  },
]

onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="渠道 · NEWAPI · 详情"
      :kicker-dot="'#707070'"
      :title="provider?.name || provider?.id || props.id"
      :desc="provider?.baseUrl"
    >
      <template #actions>
        <n-button quaternary size="small" @click="router.push({ name: 'ChannelsNewAPI' })">
          <template #icon><ArrowLeft :size="14" /></template>
          返回列表
        </n-button>
        <n-button size="small" :loading="syncing" @click="runSync">
          <template #icon><RefreshCw :size="14" /></template>
          手动同步
        </n-button>
        <n-button v-if="provider" size="small" @click="editShow = true">
          <template #icon><Pencil :size="14" /></template>
          编辑
        </n-button>
        <n-popconfirm v-if="provider" @positive-click="doDelete" positive-text="删除" negative-text="取消">
          <template #trigger>
            <n-button size="small" quaternary type="error">
              <template #icon><Trash2 :size="14" /></template>
              删除
            </n-button>
          </template>
          删除上游「{{ provider.name || provider.id }}」？所有物化渠道会被软删（去分组总览里恢复或清掉挂载）。
        </n-popconfirm>
      </template>
    </PageHeader>

    <n-spin v-if="loading && !provider" />
    <EmptyState v-else-if="!provider" icon="○" title="找不到该上游" />

    <template v-else-if="provider">
      <section class="hero-grid">
        <div class="card">
          <span class="card__label">上游 Token</span>
          <span class="card__value mono">{{ metadata.tokens?.length || 0 }}</span>
          <span class="card__sub">{{ metadata.groups?.length || 0 }} 个分组</span>
        </div>
        <div class="card">
          <span class="card__label">上游模型</span>
          <span class="card__value mono">{{ metadata.models?.length || 0 }}</span>
          <span class="card__sub">总计</span>
        </div>
        <div class="card">
          <span class="card__label">最近同步</span>
          <span class="card__value mono small">{{ formatTime(provider.lastSyncAt) }}</span>
          <span class="card__sub">{{ provider.lastSyncError ? '⚠ 异常' : '正常' }}</span>
        </div>
        <div class="card">
          <span class="card__label">同步间隔</span>
          <span class="card__value mono">{{ provider.syncIntervalSec ?? 600 }} s</span>
          <span class="card__sub">{{ provider.enabled ? '后台自动同步中' : '已停用' }}</span>
        </div>
      </section>

      <div v-if="provider.lastSyncError" class="error-banner">
        最近同步错误：<span class="mono">{{ provider.lastSyncError }}</span>
      </div>

      <div class="hint-banner">
        💡 在这里
        <strong>「+ 新建渠道」</strong>
        会在上游创建 token 并自动物化为 PivotStack 渠道；之后 markup / 挂载 / 改名都在
        <a class="link" @click="router.push({ name: 'ChannelsGroups' })">分组总览</a>
        处理。
        <br />
        ⚠ 表里的「上游分组」是 apijing 全局定义的（aws稳定/kiro引流福利 等），admin 无权改名；想换分组请删此 channel 后用新分组重建。
      </div>

      <div class="tabs-row">
        <n-tabs v-model:value="tab" size="small" class="sub-tabs">
          <n-tab-pane name="overview" tab="概览" />
          <n-tab-pane name="metadata" :tab="`上游元数据 (${metadata.tokens?.length || 0})`" />
        </n-tabs>
        <n-button
          v-if="tab === 'metadata'"
          size="small" type="primary"
          @click="createTokenShow = true"
          :disabled="!metadata.groups?.length"
        >
          <template #icon><Plus :size="13" /></template>
          新建渠道
        </n-button>
      </div>

      <section v-if="tab === 'overview'" class="overview-grid">
        <article class="ov-card">
          <header class="ov-card__head">
            <span class="ov-card__title">连接信息</span>
            <span class="ov-card__sub">admin 配置的上游访问参数</span>
          </header>
          <dl class="kv">
            <div class="kv__row"><dt>Provider ID</dt><dd class="mono">{{ provider.id }}</dd></div>
            <div class="kv__row"><dt>Base URL</dt><dd class="mono">{{ provider.baseUrl }}</dd></div>
            <div class="kv__row"><dt>上游用户 ID</dt><dd class="mono">{{ provider.userId ?? '-' }}</dd></div>
            <div class="kv__row"><dt>启用</dt><dd>
              <n-tag size="small" :bordered="false" :type="provider.enabled ? 'success' : 'default'">{{ provider.enabled ? '是' : '否' }}</n-tag>
            </dd></div>
          </dl>
        </article>

        <article class="ov-card">
          <header class="ov-card__head">
            <span class="ov-card__title">计价参数</span>
            <span class="ov-card__sub">把上游 quota 折算为 PivotStack 虚拟 $</span>
          </header>
          <dl class="kv">
            <div class="kv__row">
              <dt>quota / $</dt>
              <dd>
                <span class="mono">{{ (provider.quotaPerUnitDollar ?? 0).toLocaleString() }}</span>
                <span class="kv__hint">上游 quota 单位换算系数</span>
              </dd>
            </div>
            <div class="kv__row">
              <dt>¥ / 上游 $</dt>
              <dd>
                <span class="mono">{{ provider.yuanPerUpstreamDollar ?? '-' }}</span>
                <span class="kv__hint">真上游 $ 兑 ¥ 汇率</span>
              </dd>
            </div>
            <div class="kv__row">
              <dt>同步间隔</dt>
              <dd>
                <span class="mono">{{ provider.syncIntervalSec ?? 600 }} 秒</span>
                <span class="kv__hint">≈ {{ ((provider.syncIntervalSec ?? 600) / 60).toFixed(0) }} 分钟一次</span>
              </dd>
            </div>
          </dl>
        </article>
      </section>

      <n-data-table
        v-else-if="tab === 'metadata' && (metadata.tokens?.length || 0) > 0"
        :columns="tokenColumns"
        :data="metadata.tokens || []"
        :row-key="(r: any) => r.id"
        :pagination="tokenPagination"
        :scroll-x="800"
        size="small"
        striped
      />
      <EmptyState v-else-if="tab === 'metadata'" icon="○" title="还没有上游 token 缓存" desc="同步后会出现在这里" />

      <ProviderDrawer v-model:show="editShow" :row="provider" @saved="reload" />
      <CreateTokenDialog
        v-model:show="createTokenShow"
        :provider-id="props.id"
        :provider-name="provider?.name"
        :groups="metadata.groups || []"
        @created="setTimeout(reload, 1500)"
      />
    </template>
  </PageContainer>
</template>

<style scoped>
.hero-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 12px; }
.card { display: flex; flex-direction: column; gap: 6px; padding: 16px; border: 1px solid rgba(255,255,255,0.06); border-radius: 6px; background: #0a0a0a; }
.card__label { font-size: 11px; color: #707070; text-transform: uppercase; letter-spacing: 0.06em; }
.card__value { font-size: 24px; font-weight: 600; color: #ededed; }
.card__value.small { font-size: 13px; font-weight: 400; }
.card__sub { font-size: 11px; color: #707070; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; font-variant-numeric: tabular-nums; }

.error-banner {
  margin-bottom: 16px;
  padding: 10px 12px;
  background: rgba(255, 77, 77, 0.06);
  border: 1px solid rgba(255, 77, 77, 0.30);
  border-radius: 6px;
  color: #ff7a7a;
  font-size: 12px;
}
.hint-banner {
  margin-bottom: 16px;
  padding: 10px 14px;
  background: rgba(82, 168, 255, 0.06);
  border: 1px solid rgba(82, 168, 255, 0.25);
  border-radius: 6px;
  color: #a3a3a3;
  font-size: 12px;
}
.link { color: #52a8ff; cursor: pointer; }
.link:hover { text-decoration: underline; }

.sub-tabs { margin-bottom: 16px; flex: 1; }
.tabs-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
.overview-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  align-items: start;
}
.ov-card {
  padding: 18px;
  background: #0a0a0a;
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 6px;
}
.ov-card__head { display: flex; flex-direction: column; gap: 2px; margin-bottom: 14px; }
.ov-card__title { color: #ededed; font-size: 14px; font-weight: 600; }
.ov-card__sub { color: #707070; font-size: 12px; }
.kv__hint { color: #707070; font-size: 11px; margin-left: 10px; }
.kv { display: flex; flex-direction: column; gap: 8px; margin: 0; }
.kv__row { display: grid; grid-template-columns: 160px 1fr; font-size: 13px; align-items: center; }
.kv__row dt { color: #707070; }
.kv__row dd { color: #ededed; margin: 0; }
@media (max-width: 900px) {
  .overview-grid { grid-template-columns: 1fr; }
  .hero-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
