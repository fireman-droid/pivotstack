<script setup lang="ts">
import { computed } from 'vue'
import { NDrawer, NDrawerContent, NButton, NTag, useMessage } from 'naive-ui'
import { X } from 'lucide-vue-next'
import StatusBadge from '../../common/StatusBadge.vue'
import CopyableText from '../../common/CopyableText.vue'
import type { CallLog } from '../../../api/admin/logs'

const props = defineProps<{ log: CallLog | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const message = useMessage()

const show = computed(() => !!props.log)

function formatTime(row: CallLog) {
  if (row.time) return row.time
  return row.timestamp
    ? new Date(row.timestamp * 1000).toLocaleString('zh-CN', { hour12: false })
    : '-'
}

function formatDuration(ms?: number | null) {
  if (ms == null || ms === 0) return '-'
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)} s`
  const total = Math.round(s)
  return `${Math.floor(total / 60)}m ${total - Math.floor(total / 60) * 60}s`
}

const detail = computed(() => {
  const r = props.log
  if (!r) return null
  const inputT = r.input_tokens || 0
  const outT = r.output_tokens || 0
  return {
    raw: r,
    timeText: formatTime(r),
    requestId: r.request_id || '-',
    statusKey: r.error || r.status === 'error' ? 'error' : 'success',
    apiType: (r as any).api_type || '-',
    keyName: (r as any).api_key_note || (r as any).api_key_id?.slice(0, 10) || '-',
    keyId: r.api_key_id || '-',
    account: (r as any).account || '-',
    channel: r.channel_alias || r.channel_id || '-',
    channelType: r.channel_type || '-',
    originalModel: r.original_model || '-',
    actualModel: r.actual_model || '-',
    priceModel: (r as any).price_model || '-',
    inputTokens: inputT,
    outputTokens: outT,
    totalTokens: (r as any).total_tokens || inputT + outT,
    paidCredits: (r as any).paid_credits ?? 0,
    giftedCredits: (r as any).gifted_credits ?? 0,
    totalCredits: ((r as any).paid_credits ?? 0) + ((r as any).gifted_credits ?? 0),
    costUSD: r.cost_usd ?? 0,
    billingMode: (r as any).billing_mode || '-',
    billingStatus: (r as any).billing_status || '-',
    duration: formatDuration(r.duration_ms),
    stopReason: (r as any).stop_reason || '-',
    stream: !!(r as any).stream,
    error: r.error || '',
  }
})

function billingModeLabel(v: string) {
  return { token: 'Token 计费', subscription: '订阅覆盖', legacy_credits: 'Legacy Credits' }[v] || v
}
function billingStatusLabel(v: string) {
  return { paid: '已扣款', free: '免费（订阅覆盖）', estimated: '估算待对账' }[v] || v
}
function statusBadgeType(v: string): 'success' | 'warning' | 'default' {
  return v === 'paid' ? 'success' : v === 'estimated' ? 'warning' : 'default'
}

function copyJSON() {
  if (!props.log) return
  navigator.clipboard.writeText(JSON.stringify(props.log, null, 2))
  message.success('原始 JSON 已复制')
}

function onUpdateShow(v: boolean) {
  if (!v) emit('close')
}
</script>

<template>
  <NDrawer :show="show" :width="560" placement="right" @update:show="onUpdateShow">
    <NDrawerContent v-if="detail" closable>
      <template #header>
        <div class="cld__head">
          <span class="cld__head-title">调用详情</span>
        </div>
      </template>

      <!-- 顶部 hero -->
      <div class="cld__hero">
        <div class="cld__hero-top">
          <StatusBadge
            :status="detail.statusKey === 'success' ? 'success' : 'error'"
            :label="detail.statusKey === 'success' ? '调用成功' : '调用失败'"
          />
          <NTag size="small" :bordered="false" :type="detail.stream ? 'info' : 'default'">
            {{ detail.stream ? '流式' : '非流式' }}
          </NTag>
          <NTag size="small" :bordered="false">{{ detail.apiType }}</NTag>
        </div>
        <div class="cld__hero-time">{{ detail.timeText }}</div>
        <CopyableText :text="detail.requestId" :mono="true" class="cld__hero-rid" />
      </div>

      <!-- 错误优先 -->
      <section v-if="detail.error" class="cld__section cld__section--error">
        <header class="cld__section-title">错误</header>
        <pre class="cld__error-text">{{ detail.error }}</pre>
      </section>

      <!-- 模型 & 渠道 -->
      <section class="cld__section">
        <header class="cld__section-title">模型 & 渠道</header>
        <dl class="cld__dl">
          <div class="cld__row">
            <dt>请求模型</dt>
            <dd><span class="mono">{{ detail.originalModel }}</span></dd>
          </div>
          <div v-if="detail.actualModel !== detail.originalModel" class="cld__row">
            <dt>上游实际</dt>
            <dd><span class="mono">{{ detail.actualModel }}</span></dd>
          </div>
          <div class="cld__row">
            <dt>渠道</dt>
            <dd>{{ detail.channel }} <span class="cld__dim">· {{ detail.channelType }}</span></dd>
          </div>
          <div class="cld__row">
            <dt>上游账号</dt>
            <dd><span class="mono">{{ detail.account }}</span></dd>
          </div>
          <div v-if="detail.priceModel !== '-' && detail.priceModel !== detail.originalModel" class="cld__row">
            <dt>计价模型</dt>
            <dd><span class="mono">{{ detail.priceModel }}</span></dd>
          </div>
        </dl>
      </section>

      <!-- Key 信息 -->
      <section class="cld__section">
        <header class="cld__section-title">销售 Key</header>
        <dl class="cld__dl">
          <div class="cld__row">
            <dt>Key 名称</dt>
            <dd>{{ detail.keyName }}</dd>
          </div>
          <div class="cld__row">
            <dt>Key ID</dt>
            <dd><CopyableText :text="detail.keyId" :mono="true" /></dd>
          </div>
        </dl>
      </section>

      <!-- Token & 计费 -->
      <section class="cld__section">
        <header class="cld__section-title">Token & 计费</header>
        <div class="cld__metrics">
          <div class="cld__metric">
            <span class="cld__metric-label">输入</span>
            <span class="cld__metric-val mono">{{ detail.inputTokens.toLocaleString() }}</span>
          </div>
          <div class="cld__metric">
            <span class="cld__metric-label">输出</span>
            <span class="cld__metric-val mono">{{ detail.outputTokens.toLocaleString() }}</span>
          </div>
          <div class="cld__metric">
            <span class="cld__metric-label">合计</span>
            <span class="cld__metric-val mono">{{ detail.totalTokens.toLocaleString() }}</span>
          </div>
        </div>
        <dl class="cld__dl cld__dl--tight">
          <div class="cld__row">
            <dt>付费 Credits</dt>
            <dd><span class="mono">{{ detail.paidCredits.toFixed(4) }}</span></dd>
          </div>
          <div class="cld__row">
            <dt>赠送 Credits</dt>
            <dd><span class="mono">{{ detail.giftedCredits.toFixed(4) }}</span></dd>
          </div>
          <div class="cld__row cld__row--strong">
            <dt>合计扣费</dt>
            <dd><span class="mono cld__accent">{{ detail.totalCredits.toFixed(4) }}</span></dd>
          </div>
          <div class="cld__row">
            <dt>上游成本</dt>
            <dd><span class="mono">${{ detail.costUSD.toFixed(6) }}</span></dd>
          </div>
          <div class="cld__row">
            <dt>计费模式</dt>
            <dd>
              <NTag size="small" :bordered="false">{{ billingModeLabel(detail.billingMode) }}</NTag>
              <NTag size="small" :bordered="false" :type="statusBadgeType(detail.billingStatus)" style="margin-left:6px">
                {{ billingStatusLabel(detail.billingStatus) }}
              </NTag>
            </dd>
          </div>
        </dl>
      </section>

      <!-- 性能 -->
      <section class="cld__section">
        <header class="cld__section-title">性能</header>
        <dl class="cld__dl">
          <div class="cld__row">
            <dt>耗时</dt>
            <dd><span class="mono">{{ detail.duration }}</span></dd>
          </div>
          <div class="cld__row">
            <dt>停止原因</dt>
            <dd><span class="mono">{{ detail.stopReason }}</span></dd>
          </div>
        </dl>
      </section>

      <template #footer>
        <div class="cld__foot">
          <NButton size="small" quaternary @click="copyJSON">复制原始 JSON</NButton>
        </div>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.mono { font-family: var(--st-font-mono); color: var(--st-text-pri); font-variant-numeric: tabular-nums; }
.cld__head-title { font-size: 14px; font-weight: 600; color: var(--st-text-pri); }
.cld__dim { color: var(--st-text-ter); }
.cld__accent { color: var(--st-success); }

.cld__hero {
  padding: 0 0 16px;
  border-bottom: 1px solid var(--st-border);
  margin-bottom: 16px;
}
.cld__hero-top { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.cld__hero-time {
  font-family: var(--st-font-mono);
  font-size: 13px; color: var(--st-text-sec);
  margin-bottom: 4px;
}
.cld__hero-rid :deep(.copyable) { font-size: 11px; color: var(--st-text-ter); }

.cld__section {
  padding: 14px 16px;
  border: 1px solid var(--st-border);
  border-radius: 6px;
  background: var(--st-bg-surface);
  margin-bottom: 12px;
}
.cld__section--error {
  border-color: rgba(255, 77, 77, 0.30);
  background: rgba(255, 77, 77, 0.05);
}
.cld__section-title {
  font-size: 11px; font-weight: 500;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--st-text-ter);
  margin-bottom: 10px;
}

.cld__dl { display: flex; flex-direction: column; gap: 8px; margin: 0; }
.cld__dl--tight { margin-top: 12px; gap: 6px; }
.cld__row {
  display: grid;
  grid-template-columns: 100px 1fr;
  align-items: center;
  font-size: 13px;
  min-height: 22px;
}
.cld__row dt { color: var(--st-text-ter); font-weight: normal; }
.cld__row dd { color: var(--st-text-pri); margin: 0; }
.cld__row--strong dt { color: var(--st-text-pri); font-weight: 500; }

.cld__metrics {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1px;
  background: var(--st-border);
  border-radius: 4px;
  overflow: hidden;
}
.cld__metric {
  padding: 10px 14px;
  background: #0f0f0f;
  display: flex; flex-direction: column; gap: 4px;
}
.cld__metric-label {
  font-size: 11px;
  color: var(--st-text-ter);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.cld__metric-val { font-size: 16px; font-weight: 500; }

.cld__error-text {
  margin: 0;
  padding: 12px;
  background: rgba(0, 0, 0, 0.4);
  border-radius: 4px;
  color: #ff7a7a;
  font-family: var(--st-font-mono);
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 220px;
  overflow: auto;
}

.cld__foot { display: flex; justify-content: flex-end; gap: 8px; }
</style>
