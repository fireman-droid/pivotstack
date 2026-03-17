<script setup>
import { ref, computed, onMounted } from 'vue'
import { userApi } from '../../api/user'
import { FileX, CheckCircle2, XCircle, Clock, Database, Coins, Timer, ChevronLeft, ChevronRight } from 'lucide-vue-next'

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

function prevPage() {
  if (page.value > 1) { page.value--; loadLogs() }
}
function nextPage() {
  if (hasMore.value) { page.value++; loadLogs() }
}
function gotoPage(p) {
  page.value = p
  loadLogs()
}

onMounted(loadLogs)

function fmtTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}

function creditToUSD(credits, model) {
  if (!credits) return '0.00'
  const m = (model || '').toLowerCase()
  const isProPool = m.includes('opus') || (m.includes('sonnet') && (m.includes('4.6') || m.includes('4-6')))
  const pricePerCredit = isProPool ? 0.20 : 0.04
  return (credits * pricePerCredit).toFixed(4)
}
</script>

<template>
  <div class="logs-page">
    <div class="page-header">
      <div class="title-section">
        <h3>请求日志</h3>
        <span class="count-badge">{{ total }}</span>
      </div>
      <!-- Pagination Top -->
      <div v-if="total > limit" class="pagination">
        <button @click="prevPage" :disabled="page <= 1" class="pg-btn">
          <ChevronLeft :size="16" />
        </button>
        <template v-for="p in totalPages" :key="p">
          <button v-if="p === 1 || p === totalPages || (p >= page - 1 && p <= page + 1)"
            @click="gotoPage(p)" :class="['pg-btn', { active: p === page }]">{{ p }}</button>
          <span v-else-if="p === page - 2 || p === page + 2" class="pg-dots">…</span>
        </template>
        <button @click="nextPage" :disabled="!hasMore" class="pg-btn">
          <ChevronRight :size="16" />
        </button>
        <span class="pg-info">{{ (page - 1) * limit + 1 }}-{{ Math.min(page * limit, total) }} / {{ total }}</span>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div v-for="i in 4" :key="i" class="skeleton-card glass shimmer"></div>
    </div>

    <div v-else-if="logs.length === 0" class="empty-state">
      <div class="empty-icon-wrapper glass">
        <FileX :size="32" color="#475569" />
      </div>
      <h4>没有查询到日志</h4>
      <p>您的所有 API 请求记录都会在此实时更新，目前空空如也。</p>
    </div>

    <div v-else class="log-list">
      <div
        v-for="log in logs"
        :key="log.request_id"
        :class="['log-card', 'glass', log.status === 'error' ? 'status-error' : 'status-success']"
      >
        <div class="log-main">
          <div class="log-info">
            <div class="model-name">
              {{ log.actual_model || log.original_model }}
            </div>
          </div>
          <div class="log-time">
            <Clock :size="13" style="margin-right:5px" />
            {{ log.time || fmtTime(log.timestamp * 1000) }}
          </div>
        </div>

        <div class="log-meta">
          <div class="meta-item">
            <Database :size="13" />
            <span>{{ (((log.input_tokens || 0) + (log.output_tokens || 0)) / 1000).toFixed(1) }}K Tokens</span>
          </div>
          <div class="meta-item">
            <Coins :size="13" />
            <span>${{ log.cost_usd ? log.cost_usd.toFixed(4) : creditToUSD(log.credits, log.actual_model || log.original_model) }}</span>
          </div>
          <div class="meta-item" v-if="log.duration_ms">
            <Timer :size="13" />
            <span>{{ log.duration_ms }}ms</span>
          </div>
          <div class="status-tag" :class="log.status === 'error' ? 'err' : 'ok'">
            <component :is="log.status === 'error' ? XCircle : CheckCircle2" :size="11" style="margin-right:4px" />
            {{ log.status === 'error' ? 'ERROR' : (log.stop_reason || 'SUCCESS') }}
          </div>
        </div>

        <div v-if="log.error" class="error-detail">
          {{ log.error }}
        </div>
      </div>
    </div>

    <!-- Pagination Bottom -->
    <div v-if="total > limit && !loading" class="pagination bottom">
      <button @click="prevPage" :disabled="page <= 1" class="pg-btn">
        <ChevronLeft :size="16" />
      </button>
      <span class="pg-info">第 {{ page }} / {{ totalPages }} 页</span>
      <button @click="nextPage" :disabled="!hasMore" class="pg-btn">
        <ChevronRight :size="16" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  padding: 1rem 0;
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.page-header {
  margin-bottom: 2.5rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
}

.pagination {
  display: flex;
  align-items: center;
  gap: 0.375rem;
}
.pagination.bottom {
  justify-content: center;
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}
.pg-btn {
  all: unset;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 32px;
  border-radius: 8px;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #94a3b8;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  transition: all 150ms ease;
}
.pg-btn:hover:not(:disabled) {
  background: rgba(99, 102, 241, 0.15);
  color: #a5b4fc;
  border-color: rgba(99, 102, 241, 0.3);
}
.pg-btn.active {
  background: rgba(99, 102, 241, 0.2);
  color: #c7d2fe;
  border-color: rgba(99, 102, 241, 0.4);
}
.pg-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}
.pg-dots {
  color: #475569;
  font-size: 0.75rem;
  padding: 0 0.25rem;
}
.pg-info {
  color: #64748b;
  font-size: 0.75rem;
  margin-left: 0.5rem;
  white-space: nowrap;
}

.title-section {
  display: flex;
  align-items: flex-end;
  gap: 1rem;
}

h3 {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.75rem;
  font-weight: 700;
  color: #f8fafc;
  margin: 0;
}

.count-badge {
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
  padding: 0.25rem 0.75rem;
  border-radius: 20px;
  font-size: 0.8125rem;
  font-weight: 600;
  border: 1px solid rgba(99, 102, 241, 0.15);
  margin-bottom: 2px;
}

.glass {
  background: rgba(255, 255, 255, 0.03);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.log-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-height: calc(100vh - 200px);
  overflow-y: auto;
  padding-right: 4px;
}

.log-list::-webkit-scrollbar {
  width: 6px;
}
.log-list::-webkit-scrollbar-track {
  background: transparent;
}
.log-list::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.1);
  border-radius: 3px;
}
.log-list::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.2);
}

.log-card {
  border-radius: 14px;
  padding: 1.25rem 1.5rem;
  transition: all 200ms ease;
  position: relative;
  overflow: hidden;
  flex-shrink: 0;
}

.log-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.1);
}

.status-success { border-left: 4px solid #10b981; }
.status-error { border-left: 4px solid #ef4444; }

.log-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.log-info { flex: 1; }

.model-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: #a855f7;
  font-weight: 600;
  font-size: 1rem;
}

.log-time {
  display: flex;
  align-items: center;
  color: #64748b;
  font-size: 0.8125rem;
}

.log-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 1.5rem;
  align-items: center;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: #94a3b8;
  font-size: 0.8125rem;
}

.status-tag {
  margin-left: auto;
  display: flex;
  align-items: center;
  padding: 0.25rem 0.625rem;
  border-radius: 4px;
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.status-tag.ok {
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
}

.status-tag.err {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.error-detail {
  margin-top: 1rem;
  padding: 0.75rem;
  background: rgba(239, 68, 68, 0.08);
  border-radius: 6px;
  color: #fca5a5;
  font-size: 0.75rem;
  font-family: ui-monospace, monospace;
  border-left: 2px solid #ef4444;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem 2rem;
  text-align: center;
}

.empty-icon-wrapper {
  width: 72px;
  height: 72px;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 1.5rem;
}

.empty-state h4 {
  color: #f8fafc;
  font-size: 1.125rem;
  font-weight: 600;
  margin: 0 0 0.5rem;
}

.empty-state p {
  color: #64748b;
  font-size: 0.875rem;
  max-width: 260px;
  margin: 0;
  line-height: 1.5;
}

.loading-state {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.skeleton-card {
  height: 88px;
  border-radius: 14px;
}

.shimmer {
  position: relative;
  overflow: hidden;
}

.shimmer::after {
  position: absolute;
  top: 0; right: 0; bottom: 0; left: 0;
  transform: translateX(-100%);
  background-image: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0,
    rgba(255, 255, 255, 0.03) 20%,
    rgba(255, 255, 255, 0.06) 60%,
    rgba(255, 255, 255, 0)
  );
  animation: shimmer 2s infinite;
  content: '';
}

@keyframes shimmer {
  100% { transform: translateX(100%); }
}
</style>
