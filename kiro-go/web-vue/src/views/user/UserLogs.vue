<script setup>
import { ref, computed, onMounted } from 'vue'
import { userApi } from '../../api/user'
import { FileX, Clock, Database, Coins, Timer, ChevronLeft, ChevronRight, CheckCircle2, XCircle } from 'lucide-vue-next'
import WorldCard from '../../components/world/WorldCard.vue'
import WorldChip from '../../components/world/WorldChip.vue'
import WorldTimeline from '../../components/world/WorldTimeline.vue'
import WorldButton from '../../components/world/WorldButton.vue'

const logs = ref([])
const loading = ref(true)
const page = ref(1)
const limit = ref(50)
const total = ref(0)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))
const hasMore = computed(() => page.value < totalPages.value)

async function loadLogs() {
  loading.value = true
  try {
    const data = await userApi(`/logs?page=${page.value}&limit=${limit.value}`)
    logs.value = data.logs || []
    total.value = data.total || 0
  } catch {}
  loading.value = false
}

function prevPage() { if (page.value > 1) { page.value--; loadLogs() } }
function nextPage() { if (hasMore.value) { page.value++; loadLogs() } }
function gotoPage(p) { page.value = p; loadLogs() }

onMounted(loadLogs)

function fmtTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}

// 反穿帮硬约束：必须保留 actual_model || original_model fallback
function modelOf(log) {
  return log.actual_model || log.original_model || '-'
}

function creditToUSD(credits, model) {
  if (!credits) return '0.00'
  const m = (model || '').toLowerCase()
  const isProPool = m.includes('opus') || (m.includes('sonnet') && (m.includes('4.6') || m.includes('4-6')))
  const pricePerCredit = isProPool ? 0.20 : 0.04
  return (credits * pricePerCredit).toFixed(4)
}

const items = computed(() => logs.value.map(log => ({
  id: log.request_id,
  status: log.status === 'error' ? 'danger' : 'success',
  raw: log,
})))
</script>

<template>
  <div class="logs-page">
    <header class="page-head">
      <div class="title-row">
        <div class="title-wrap">
          <div class="eyebrow">用户中心</div>
          <h1 class="page-title">请求日志</h1>
        </div>
        <WorldChip variant="info" :dot="true">{{ total }} 条</WorldChip>
      </div>
      <!-- 顶部分页 -->
      <div v-if="total > limit" class="pagination">
        <WorldButton variant="ghost" size="sm" :disabled="page <= 1" @click="prevPage">
          <ChevronLeft :size="14" />
        </WorldButton>
        <template v-for="p in totalPages" :key="p">
          <button v-if="p === 1 || p === totalPages || (p >= page - 1 && p <= page + 1)"
            @click="gotoPage(p)" :class="['pg-num', { active: p === page }]">{{ p }}</button>
          <span v-else-if="p === page - 2 || p === page + 2" class="pg-dots">…</span>
        </template>
        <WorldButton variant="ghost" size="sm" :disabled="!hasMore" @click="nextPage">
          <ChevronRight :size="14" />
        </WorldButton>
        <span class="pg-info">{{ (page - 1) * limit + 1 }}-{{ Math.min(page * limit, total) }} / {{ total }}</span>
      </div>
    </header>

    <WorldCard padding="md" v-if="loading">
      <div class="loading-state">
        <div v-for="i in 4" :key="i" class="skeleton-line" />
      </div>
    </WorldCard>

    <WorldCard padding="lg" v-else-if="!logs.length">
      <div class="empty-state">
        <div class="empty-icon"><FileX :size="32" /></div>
        <h4>没有查询到日志</h4>
        <p>您的所有 API 请求记录都会在此实时更新</p>
      </div>
    </WorldCard>

    <WorldCard v-else padding="md">
      <WorldTimeline :items="items" empty-text="没有查询到日志">
        <template #title="{ item }">
          <span class="model-name">{{ modelOf(item.raw) }}</span>
        </template>
        <template #body="{ item }">
          <div class="log-meta">
            <span class="meta-item"><Database :size="12" />
              {{ (((item.raw.input_tokens || 0) + (item.raw.output_tokens || 0)) / 1000).toFixed(1) }}K Tokens
            </span>
            <span class="meta-item"><Coins :size="12" />
              ${{ item.raw.cost_usd ? item.raw.cost_usd.toFixed(4) : creditToUSD(item.raw.credits, modelOf(item.raw)) }}
            </span>
            <span class="meta-item" v-if="item.raw.duration_ms">
              <Timer :size="12" />{{ item.raw.duration_ms }}ms
            </span>
            <WorldChip
              :variant="item.raw.status === 'error' ? 'danger' : 'success'"
              :dot="true"
              size="sm"
            >
              <component :is="item.raw.status === 'error' ? XCircle : CheckCircle2" :size="11" />
              {{ item.raw.status === 'error' ? 'ERROR' : (item.raw.stop_reason || 'SUCCESS') }}
            </WorldChip>
          </div>
          <div v-if="item.raw.error" class="error-detail">{{ item.raw.error }}</div>
        </template>
      </WorldTimeline>
    </WorldCard>

    <!-- 底部分页 -->
    <div v-if="total > limit && !loading" class="pagination bottom">
      <WorldButton variant="secondary" size="sm" :disabled="page <= 1" @click="prevPage">
        <ChevronLeft :size="14" /><span>上一页</span>
      </WorldButton>
      <span class="pg-info">第 {{ page }} / {{ totalPages }} 页</span>
      <WorldButton variant="secondary" size="sm" :disabled="!hasMore" @click="nextPage">
        <span>下一页</span><ChevronRight :size="14" />
      </WorldButton>
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-head {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.title-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}

.pagination {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.pagination.bottom { justify-content: center; padding: 6px 0; }
.pg-num {
  min-width: 30px;
  height: 30px;
  padding: 0 10px;
  border-radius: var(--world-radius-md);
  background: transparent;
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 200ms ease;
}
.pg-num:hover { border-color: var(--world-accent); color: var(--world-text-primary); }
.pg-num.active {
  background: var(--world-accent);
  border-color: var(--world-accent);
  color: #fff;
}
.pg-dots { color: var(--world-text-dim); padding: 0 4px; }
.pg-info {
  font-size: 0.75rem;
  color: var(--world-text-mute);
  margin-left: 8px;
  font-family: var(--world-font-mono);
}

/* loading skeleton */
.loading-state { display: flex; flex-direction: column; gap: 14px; padding: 4px 0; }
.skeleton-line {
  height: 60px;
  border-radius: var(--world-radius-md);
  background: linear-gradient(
    90deg,
    var(--world-overlay-light) 0%,
    var(--world-overlay-medium) 50%,
    var(--world-overlay-light) 100%
  );
  background-size: 200% 100%;
  animation: shimmer 1.4s linear infinite;
}
@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* empty */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 24px 12px;
  gap: 12px;
}
.empty-icon {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--world-overlay-light);
  color: var(--world-text-mute);
}
.empty-state h4 {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
  color: var(--world-text-primary);
}
.empty-state p {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
}

/* log entries */
.model-name {
  font-family: var(--world-font-mono);
  font-size: 0.875rem;
  font-weight: 700;
  color: var(--world-text-primary);
  letter-spacing: 0.02em;
}
[data-world="daogui"] .model-name {
  color: var(--world-paper-aged);
}
.log-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 4px;
}
.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.75rem;
  color: var(--world-text-mute);
  font-family: var(--world-font-mono);
}
.error-detail {
  margin-top: 8px;
  padding: 8px 10px;
  background: rgba(239, 68, 68, 0.08);
  border-left: 2px solid var(--world-error);
  border-radius: var(--world-radius-sm);
  font-size: 0.75rem;
  color: var(--world-error);
  font-family: var(--world-font-mono);
  word-break: break-all;
}
[data-world="daogui"] .error-detail {
  background: rgba(196, 30, 58, 0.10);
  color: #f5707f;
}
</style>
