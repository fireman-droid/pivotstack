<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  NTabs, NTabPane, NDataTable, NTag, NSwitch, NSpin, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { ArrowLeft, Pencil, RefreshCw } from 'lucide-vue-next'
import CopyableText from '../../components/common/CopyableText.vue'
import MonoValue from '../../components/common/MonoValue.vue'
import StatusBadge from '../../components/common/StatusBadge.vue'
import KeyFormDrawer from '../../components/admin/keys/KeyFormDrawer.vue'
import KeyInsights from '../../components/admin/keys/KeyInsights.vue'
import {
  listApiKeys, updateApiKey,
  getApiKeyLogs, getApiKeyRecharges,
  type ApiKeyRow, type ApiKeyLogEntry, type RechargeRecord,
} from '../../api/admin/keys'
import { useTablePagination } from '../../composables/useTablePagination'
import { fmtCost, planLabel } from '../../utils/format'

const props = defineProps<{ id: string }>()
const router = useRouter()
const message = useMessage()

const loading = ref(false)
const togglingEnabled = ref(false)
const key = ref<ApiKeyRow | null>(null)
const logs = ref<ApiKeyLogEntry[]>([])
const recharges = ref<RechargeRecord[]>([])
const tab = ref<'overview' | 'logs' | 'recharges'>('overview')
const editShow = ref(false)
const logsPagination = useTablePagination(20)
const rechargesPagination = useTablePagination(20)

async function reload() {
  loading.value = true
  try {
    const [allKeys, logList, rechargeResp] = await Promise.all([
      listApiKeys(),
      getApiKeyLogs(props.id),
      getApiKeyRecharges(props.id, 0, 100).catch(() => ({ records: [], total: 0 })),
    ])
    key.value = allKeys.find(k => k.id === props.id) || null
    logs.value = logList
    recharges.value = rechargeResp.records || []
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const stats = computed(() => {
  const totalReq = logs.value.length
  const success = logs.value.filter(l => !l.error && l.status !== 'error').length
  const failed = totalReq - success
  const totalTokens = logs.value.reduce((s, l) => s + (l.input_tokens || 0) + (l.output_tokens || 0), 0)
  // token/newapi 模式 paid_credits/gifted_credits 都是 0；优先用 charged_usd（v2+ 实际扣费总额），
  // 旧日志 fallback cost_usd（仅 paid），再不济用 credits（legacy）。
  const totalCostUsd = logs.value.reduce(
    (s, l) => s + (((l as any).charged_usd as number | undefined) ?? l.cost_usd ?? ((l.paid_credits || 0) + (l.gifted_credits || 0))),
    0,
  )
  return { totalReq, success, failed, totalTokens, totalCostUsd }
})

/**
 * 自动用户标签（基于现有字段实时算）：
 *  - VIP：累计充值 + 赠送 ≥ $50（≈ ¥360）
 *  - 回头客：充值记录数 ≥ 2
 *  - 沉睡：lastUsed > 7 天前
 *  - 活跃：lastUsed < 24 小时
 *  - 新人：createdAt < 30 天且累计请求 < 50
 */
const autoTags = computed(() => {
  const k = key.value; if (!k) return [] as Array<{ label: string; level: 'ok' | 'warn' | 'idle' | 'info' }>
  const tags: Array<{ label: string; level: 'ok' | 'warn' | 'idle' | 'info' }> = []
  const now = Date.now() / 1000
  const totalRecharged = (k.totalBalance ?? 0) + ((k as any).totalRecharged ?? 0) + ((k as any).totalGifted ?? 0)
  if (totalRecharged >= 50) tags.push({ label: 'VIP', level: 'ok' })
  if (recharges.value.length >= 2) tags.push({ label: '回头客', level: 'info' })
  if (k.lastUsed && now - k.lastUsed < 86400) tags.push({ label: '活跃', level: 'ok' })
  else if (k.lastUsed && now - k.lastUsed > 7 * 86400) tags.push({ label: '沉睡', level: 'idle' })
  if (k.createdAt && now - k.createdAt < 30 * 86400 && (k.requests ?? 0) < 50) {
    tags.push({ label: '新人', level: 'info' })
  }
  return tags
})

async function toggleEnabled(v: boolean) {
  if (!key.value) return
  togglingEnabled.value = true
  try {
    await updateApiKey(key.value.id, { enabled: v })
    key.value.enabled = v
    message.success(v ? '已启用' : '已禁用')
  } catch (e: any) {
    message.error(e?.message || '切换失败')
  } finally {
    togglingEnabled.value = false
  }
}

function fmtTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}
function fmtDuration(ms?: number | null) {
  if (ms == null || ms === 0) return '-'
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)} s`
  const total = Math.round(s)
  return `${Math.floor(total / 60)}m ${total - Math.floor(total / 60) * 60}s`
}

const logsColumns: DataTableColumns<ApiKeyLogEntry> = [
  { title: '时间', key: 'time', width: 170, align: 'center', render: row => h(MonoValue, { value: row.time || fmtTime(row.timestamp) }) },
  { title: '模型', key: 'model', width: 240, ellipsis: { tooltip: true }, render: row => h(MonoValue, { value: row.original_model || row.actual_model || '-' }) },
  { title: '渠道', key: 'channel', width: 200, ellipsis: { tooltip: true }, render: row => row.channel_alias || row.channel_id || '-' },
  { title: 'Token', key: 'tokens', width: 110, align: 'center', render: row => h('span', { class: 'kd-mono' }, ((row.input_tokens || 0) + (row.output_tokens || 0)).toLocaleString()) },
  { title: '耗时', key: 'dur', width: 90, align: 'center', render: row => h('span', { class: 'kd-mono' }, fmtDuration(row.duration_ms)) },
  {
    title: '状态', key: 'status', width: 90, align: 'center',
    render: row => h(StatusBadge, {
      status: row.error || row.status === 'error' ? 'error' : 'success',
      label: row.error || row.status === 'error' ? '失败' : '成功',
    }),
  },
]

const rechargeColumns: DataTableColumns<RechargeRecord> = [
  { title: '时间', key: 'time', width: 170, align: 'center', render: row => h(MonoValue, { value: row.time || fmtTime(row.timestamp) }) },
  {
    title: '类型', key: 'type', width: 130, align: 'center',
    render: row => h(NTag, { size: 'small', bordered: false }, () => row.type || '-'),
  },
  { title: '金额 ¥', key: 'amountCny', width: 110, align: 'center', render: row => h('span', { class: 'kd-mono' }, row.amountCny != null ? `¥${row.amountCny.toFixed(2)}` : '-') },
  { title: '余额变化', key: 'balanceDelta', width: 240, align: 'center', render: row => h('span', { class: 'kd-mono' }, `¥${(row.balanceBefore ?? 0).toFixed(2)} → ¥${(row.balanceAfter ?? 0).toFixed(2)}`) },
  { title: '操作员', key: 'operator', width: 150, align: 'center', ellipsis: { tooltip: true }, render: row => row.operator || '-' },
  { title: '备注', key: 'note', width: 240, ellipsis: { tooltip: true }, render: row => row.note || '-' },
]

onMounted(reload)
</script>

<template>
  <div class="admin-page">
    <NSpin v-if="loading && !key" />
    <div v-else-if="!key" class="kd-empty">
      <span>找不到该 Key，可能已被删除或 ID 错误。</span>
      <button class="kd-btn" @click="router.push({ name: 'BillingKeys' })">返回列表</button>
    </div>

    <template v-else-if="key">
      <!-- page-head -->
      <header class="page-head">
        <div>
          <div class="page-head__crumb"><b>BILLING</b> / API Keys / 详情</div>
          <div class="page-head__title">
            <div class="t-display-admin">{{ key.note || `Key ${key.id.slice(0, 8)}` }}</div>
            <div class="page-head__sub">
              ID <span class="mono">{{ key.id }}</span>
              <span v-for="t in autoTags" :key="t.label" class="kd-tag" :class="`kd-tag--${t.level}`">{{ t.label }}</span>
            </div>
          </div>
        </div>
        <div class="page-head__right">
          <button class="kd-btn kd-btn--ghost" @click="router.push({ name: 'BillingKeys' })">
            <ArrowLeft :size="14" />
            返回
          </button>
          <button class="kd-btn kd-btn--ghost" :disabled="loading" @click="reload">
            <RefreshCw :size="14" :class="{ 'is-spinning': loading }" />
            刷新
          </button>
          <button class="kd-btn kd-btn--primary" @click="editShow = true">
            <Pencil :size="14" />
            编辑
          </button>
        </div>
      </header>

      <!-- hero 4 卡 -->
      <section class="metric-strip">
        <div class="metric-tile">
          <div class="metric-tile__label">余额 ¥</div>
          <div class="metric-tile__num kd-accent">¥{{ ((key.balance ?? 0) + (key.giftBalance ?? 0)).toFixed(2) }}</div>
          <div class="metric-tile__delta">
            <span class="t-meta">付费 ¥{{ (key.balance ?? 0).toFixed(2) }} · 赠送 ¥{{ (key.giftBalance ?? 0).toFixed(2) }}</span>
          </div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">套餐</div>
          <div class="metric-tile__num" style="font-size:18px">{{ key.plan ? planLabel(key.plan) : (key.tier || '余额卡') }}</div>
          <div class="metric-tile__delta">
            <span class="t-meta">{{ key.expiresAt ? `到期 ${fmtTime(key.expiresAt)}` : '永久' }}</span>
          </div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">启用</div>
          <div class="metric-tile__num" style="font-size:14px;padding-top:4px">
            <NSwitch size="medium" :value="key.enabled" :loading="togglingEnabled" @update:value="toggleEnabled" />
          </div>
          <div class="metric-tile__delta">
            <span class="t-meta">{{ key.enabled ? '正常受理请求' : '已停用' }}</span>
          </div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">累计请求</div>
          <div class="metric-tile__num">{{ stats.totalReq.toLocaleString() }}</div>
          <div class="metric-tile__delta">
            <span class="t-meta">{{ stats.success }} 成功 / {{ stats.failed }} 失败</span>
          </div>
        </div>
      </section>

      <!-- key string -->
      <section class="kd-keycard">
        <span class="kd-keycard__label">KEY 字符串（掩码展示，点击可复制完整值）</span>
        <CopyableText :text="key.key || key.keyMasked || key.id" :mono="true" :mask="true" />
      </section>

      <!-- tabs -->
      <NTabs v-model:value="tab" size="small" class="kd-tabs">
        <NTabPane name="overview" tab="概览" />
        <NTabPane name="logs" :tab="`调用日志 (${logs.length})`" />
        <NTabPane name="recharges" :tab="`充值流水 (${recharges.length})`" />
      </NTabs>

      <!-- overview -->
      <template v-if="tab === 'overview'">
        <section class="kd-overview">
          <dl class="kd-kv">
            <div class="kd-kv-row"><dt>累计 Token</dt><dd class="mono">{{ stats.totalTokens.toLocaleString() }}</dd></div>
            <div class="kd-kv-row"><dt>累计扣费</dt><dd class="mono">{{ fmtCost(stats.totalCostUsd) }}</dd></div>
            <div class="kd-kv-row"><dt>代理商</dt><dd>{{ key.isReseller ? `开启 · 子 Key 上限 ${key.maxChildKeys ?? 0}` : '未开启' }}</dd></div>
            <div class="kd-kv-row"><dt>父 Key</dt><dd>{{ key.parentKeyId || '-' }}</dd></div>
            <div class="kd-kv-row"><dt>创建时间</dt><dd class="mono">{{ fmtTime(key.createdAt) }}</dd></div>
            <div class="kd-kv-row"><dt>最近调用</dt><dd class="mono">{{ fmtTime(key.lastUsed) }}</dd></div>
          </dl>
        </section>
        <KeyInsights :logs="logs" />
      </template>

      <NDataTable
        v-else-if="tab === 'logs' && logs.length"
        :columns="logsColumns"
        :data="logs"
        :row-key="row => row.request_id || `${row.time}-${row.original_model}`"
        :pagination="logsPagination"
        :scroll-x="900"
        :bordered="false"
        size="small"
        class="kd-table"
      />
      <div v-else-if="tab === 'logs'" class="kd-empty-tab">暂无调用记录</div>

      <NDataTable
        v-else-if="tab === 'recharges' && recharges.length"
        :columns="rechargeColumns"
        :data="recharges"
        :row-key="row => `${row.timestamp}-${row.type}`"
        :pagination="rechargesPagination"
        :scroll-x="1080"
        :bordered="false"
        size="small"
        class="kd-table"
      />
      <div v-else-if="tab === 'recharges'" class="kd-empty-tab">暂无充值流水</div>

      <KeyFormDrawer v-model:show="editShow" :row="key" @updated="reload" />
    </template>
  </div>
</template>

<style scoped>
.mono { font-family: var(--st-font-mono); font-variant-numeric: tabular-nums; }
.kd-mono { font-family: var(--st-font-mono); font-variant-numeric: tabular-nums; font-size: 12px; color: var(--st-text-pri); }
.kd-accent { color: var(--st-success); }

.kd-btn {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--st-border);
  border-radius: 4px;
  color: var(--st-text-pri);
  font-size: 12px; font-family: inherit;
  cursor: pointer;
  transition: background 150ms ease, border-color 150ms ease;
}
.kd-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); border-color: var(--st-border-strong); }
.kd-btn--ghost { background: transparent; }
.kd-btn--primary { background: var(--st-primary); color: var(--st-text-inv); border-color: transparent; }
.kd-btn--primary:hover { background: var(--st-primary-hover); }
.is-spinning { animation: kd-spin 0.8s linear infinite; }
@keyframes kd-spin { to { transform: rotate(360deg); } }

.kd-tag {
  display: inline-flex; align-items: center;
  margin-left: 8px;
  padding: 1px 6px;
  border-radius: 2px;
  font-size: 10px; font-weight: 700;
  letter-spacing: 0.08em;
}
.kd-tag--ok { background: rgba(11, 212, 112, 0.10); color: var(--st-success); }
.kd-tag--info { background: rgba(82, 168, 255, 0.10); color: var(--st-info); }
.kd-tag--warn { background: rgba(245, 166, 35, 0.10); color: var(--st-warning); }
.kd-tag--idle { background: rgba(255, 255, 255, 0.06); color: var(--st-text-ter); }

.kd-keycard {
  display: flex; flex-direction: column; gap: 8px;
  padding: 14px 16px;
  background: var(--st-bg-surface);
  border: 1px solid var(--st-border);
  border-radius: 6px;
  margin-bottom: 16px;
}
.kd-keycard__label {
  font-size: 11px; font-weight: 500;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter);
}

.kd-tabs { margin-bottom: 12px; }

.kd-overview {
  padding: 14px 16px;
  background: var(--st-bg-surface);
  border: 1px solid var(--st-border);
  border-radius: 6px;
}
.kd-kv { display: flex; flex-direction: column; gap: 8px; margin: 0; }
.kd-kv-row {
  display: grid;
  grid-template-columns: 140px 1fr;
  font-size: 13px;
  align-items: center;
}
.kd-kv-row dt { color: var(--st-text-ter); font-weight: normal; }
.kd-kv-row dd { color: var(--st-text-pri); margin: 0; }

.kd-empty,
.kd-empty-tab {
  padding: 32px;
  text-align: center;
  color: var(--st-text-ter);
  font-size: 13px;
  background: var(--st-bg-surface);
  border: 1px solid var(--st-border);
  border-radius: 6px;
}
.kd-empty { display: flex; flex-direction: column; align-items: center; gap: 12px; }

.kd-table :deep(.n-data-table-th) {
  font-size: 11px !important; font-weight: 500 !important;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter) !important;
  background: transparent !important;
  height: 32px !important; padding: 0 12px !important;
  border-bottom: 1px solid var(--st-border) !important;
}
.kd-table :deep(.n-data-table-td) {
  height: 36px !important; padding: 0 12px !important;
  font-size: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04) !important;
  background: transparent !important;
}
.kd-table :deep(.n-data-table-tr:hover .n-data-table-td) { background: rgba(255, 255, 255, 0.04) !important; }
.kd-table :deep(.n-data-table) { background: transparent !important; }
</style>
