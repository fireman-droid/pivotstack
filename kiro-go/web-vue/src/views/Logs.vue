<script setup>
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import {
  RotateCw, Trash2, Search, History, ChevronLeft, ChevronRight,
  Radio, X, CheckCircle2, XCircle, Cpu, Key, Clock
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldSelect from '../components/world/WorldSelect.vue'
import WorldTimeline from '../components/world/WorldTimeline.vue'

const { success, error: toastErr } = useToast()
const route = useRoute()
const router = useRouter()
const logs = ref([])
const loading = ref(false)
const searchQuery = ref('')
const expandedIndex = ref(-1)
const statusFilter = ref(route.query.status || 'all')
const keyFilter = ref(route.query.key || 'all')
const apiKeys = ref([])
const sseConnected = ref(false)
const currentPage = ref(parseInt(route.query.page) || 1)
const totalLogs = ref(0)
const pageSize = ref(50)
let eventSource = null
let reconnectTimer = null
const seenIds = new Set()

watch([statusFilter, keyFilter, currentPage], ([s, k, p]) => {
  const q = {}
  if (s !== 'all') q.status = s
  if (k !== 'all') q.key = k
  if (p > 1) q.page = p
  router.replace({ query: q }).catch(() => {})
})

function connectSSE() {
  const password = localStorage.getItem('admin_password') || ''
  const url = `${location.origin}/admin/api/sse/logs?password=${encodeURIComponent(password)}`
  eventSource = new EventSource(url)
  eventSource.addEventListener('log', (e) => {
    try {
      const entry = JSON.parse(e.data)
      if (statusFilter.value === 'error' && !entry.error && entry.status !== 'error') return
      if (statusFilter.value === 'success' && (entry.error || entry.status === 'error')) return
      if (keyFilter.value !== 'all' && entry.api_key_id !== keyFilter.value) return
      if (searchQuery.value) {
        const q = searchQuery.value.toLowerCase()
        const matches =
          entry.actual_model?.toLowerCase().includes(q) ||
          entry.original_model?.toLowerCase().includes(q) ||
          entry.account?.toLowerCase().includes(q) ||
          entry.error?.toLowerCase().includes(q) ||
          entry.request_id?.toLowerCase().includes(q) ||
          entry.stop_reason?.toLowerCase().includes(q)
        if (!matches) return
      }
      const id = entry.request_id || `${entry.time}-${entry.actual_model}-${entry.account}`
      if (seenIds.has(id)) return
      seenIds.add(id)
      totalLogs.value++
      if (currentPage.value !== 1) return
      logs.value.unshift(entry)
      if (logs.value.length > pageSize.value) {
        const evicted = logs.value.pop()
        seenIds.delete(evicted.request_id || `${evicted.time}-${evicted.actual_model}-${evicted.account}`)
      }
    } catch {}
  })
  eventSource.onopen = () => { sseConnected.value = true }
  eventSource.onerror = () => {
    sseConnected.value = false
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      if (eventSource) { eventSource.close(); connectSSE() }
    }, 3000)
  }
}

async function loadApiKeys() {
  try {
    const res = await api('/apikeys')
    if (res.ok) apiKeys.value = await res.json()
  } catch {}
}

async function loadLogs(page = 1) {
  loading.value = true
  try {
    const params = new URLSearchParams({ page, limit: pageSize.value })
    if (statusFilter.value !== 'all') params.append('status', statusFilter.value)
    if (keyFilter.value !== 'all') params.append('key', keyFilter.value)
    if (searchQuery.value) params.append('search', searchQuery.value)
    const res = await api(`/logs?${params.toString()}`)
    if (res.ok) {
      const d = await res.json()
      logs.value = d.logs || []
      seenIds.clear()
      logs.value.forEach(l => {
        seenIds.add(l.request_id || `${l.time}-${l.actual_model}-${l.account}`)
      })
      totalLogs.value = d.total || 0
      currentPage.value = d.page || 1
    }
  } catch { toastErr('无法加载日志') }
  loading.value = false
}

const totalPages = computed(() => Math.max(1, Math.ceil(totalLogs.value / pageSize.value)))

function goToPage(page) {
  if (page < 1 || page > totalPages.value) return
  expandedIndex.value = -1
  loadLogs(page)
}

async function clearLogs() {
  if (!confirm('确认清空所有历史调用日志？此操作不可撤销。')) return
  try {
    await api('/logs', { method: 'DELETE' })
    logs.value = []
    seenIds.clear()
    totalLogs.value = 0
    success('日志已清空')
  } catch { toastErr('清空失败') }
}

function formatDuration(ms) {
  if (!ms && ms !== 0) return '-'
  if (ms < 1000) return ms + 'ms'
  return (ms / 1000).toFixed(1) + 's'
}

function formatLogTime(t) {
  if (!t) return '-'
  // log.time 可能是 ISO 字符串或 unix 秒
  let d
  if (typeof t === 'string') {
    d = new Date(t)
  } else if (typeof t === 'number') {
    d = new Date(t < 1e12 ? t * 1000 : t)
  } else {
    return '-'
  }
  if (isNaN(d.getTime())) return '-'
  // 同一天内显示 时:分:秒，跨天显示 月-日 时:分
  const now = new Date()
  const sameDay = d.getFullYear() === now.getFullYear()
    && d.getMonth() === now.getMonth()
    && d.getDate() === now.getDate()
  if (sameDay) {
    return d.toLocaleTimeString('zh-CN', { hour12: false })
  }
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${d.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' })}`
}

const apiKeyMap = computed(() => new Map(apiKeys.value.map(k => [
  k.id,
  k.note || (k.key || '').slice(0, 10) + '...'
])))

function getApiKeyDisplay(keyId) {
  if (!keyId) return '未关联'
  return apiKeyMap.value.get(keyId) || keyId
}

let searchTimeout = null
watch(searchQuery, () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => goToPage(1), 500)
})
watch([statusFilter, keyFilter], () => goToPage(1))

onMounted(async () => {
  await Promise.all([loadLogs(), loadApiKeys()])
  connectSSE()
})
onUnmounted(() => {
  if (eventSource) { eventSource.close(); eventSource = null }
  if (reconnectTimer) clearTimeout(reconnectTimer)
  if (searchTimeout) clearTimeout(searchTimeout)
})

const statusOpts = [
  { value: 'all',     label: '全部' },
  { value: 'success', label: '成功' },
  { value: 'error',   label: '错误' },
]

const keyOptions = computed(() => [
  { value: 'all', label: '全部 API Key' },
  ...apiKeys.value.map(k => ({
    value: k.id,
    label: k.note || k.id.slice(0, 8),
    hint: k.note ? k.id.slice(0, 6) : null,
  })),
])

const items = computed(() => logs.value.map((log, i) => ({
  id: log.request_id || `${log.time}-${i}`,
  time: formatLogTime(log.time),
  status: (log.error || log.status === 'error') ? 'danger' : 'success',
  raw: log,
})))
</script>

<template>
  <div class="logs-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow"><History :size="11" /> 调用记录</div>
        <h1 class="page-title">使用日志</h1>
      </div>
      <div class="head-actions">
        <WorldChip v-if="sseConnected" variant="success" :dot="true" :pulse="true">
          <Radio :size="11" /> 实时
        </WorldChip>
        <WorldChip v-else variant="warning" :dot="true">离线</WorldChip>
        <WorldButton variant="secondary" size="sm" @click="loadLogs(currentPage)">
          <RotateCw :size="13" /><span>刷新</span>
        </WorldButton>
        <WorldButton variant="danger" size="sm" @click="clearLogs">
          <Trash2 :size="13" /><span>清空</span>
        </WorldButton>
      </div>
    </header>

    <!-- Filters -->
    <WorldCard padding="md">
      <div class="filter-row">
        <div class="search-wrap">
          <Search :size="14" class="search-icon" />
          <input
            v-model="searchQuery"
            class="search-input"
            placeholder="搜索模型、账号、错误信息或 request_id"
          />
          <button v-if="searchQuery" @click="searchQuery = ''" class="clear-btn"><X :size="12" /></button>
        </div>
        <WorldSegment v-model="statusFilter" :options="statusOpts" size="sm" />
        <WorldSelect
          v-model="keyFilter"
          :options="keyOptions"
          size="sm"
          searchable
          placeholder="筛选 API Key"
          align="end"
        />
      </div>
    </WorldCard>

    <!-- Timeline -->
    <WorldCard padding="md">
      <WorldTimeline :items="items" empty-text="暂无日志">
        <template #title="{ item }">
          <span class="model-row">
            <code class="orig-model">{{ item.raw.original_model || '—' }}</code>
            <span v-if="item.raw.actual_model && item.raw.actual_model !== item.raw.original_model" class="arrow">→</span>
            <code v-if="item.raw.actual_model && item.raw.actual_model !== item.raw.original_model" class="actual-model">
              {{ item.raw.actual_model }}
            </code>
          </span>
        </template>
        <template #body="{ item }">
          <div class="log-meta-row">
            <span v-if="item.raw.api_key_id" class="meta-cell"><Key :size="11" />{{ getApiKeyDisplay(item.raw.api_key_id) }}</span>
            <span v-if="item.raw.account" class="meta-cell"><Cpu :size="11" />{{ item.raw.account }}</span>
            <span class="meta-cell"><Clock :size="11" />{{ formatDuration(item.raw.duration_ms) }}</span>
            <span class="meta-cell">{{ ((item.raw.input_tokens || 0) + (item.raw.output_tokens || 0)).toLocaleString() }} tok</span>
            <!-- 掺水前/后 + 金额 -->
            <span
              v-if="item.raw.upstream_credits && item.raw.upstream_credits !== item.raw.credits"
              class="meta-cell credits-pre"
              title="上游真实消耗（掺水前）"
            >
              <span class="cr-label">掺水前</span>{{ item.raw.upstream_credits.toFixed(4) }} cr
            </span>
            <span
              v-if="item.raw.credits"
              class="meta-cell credits-post"
              :title="item.raw.upstream_credits && item.raw.upstream_credits !== item.raw.credits ? '计费 credits（掺水后放大到 originalModel 口径）' : '计费 credits'"
            >
              <span v-if="item.raw.upstream_credits && item.raw.upstream_credits !== item.raw.credits" class="cr-label">掺水后</span>
              {{ item.raw.credits.toFixed(4) }} cr
            </span>
            <span v-if="item.raw.cost_usd" class="meta-cell credits-cost" title="实际扣费金额">
              ${{ item.raw.cost_usd.toFixed(4) }}
            </span>
            <WorldChip
              :variant="item.raw.error || item.raw.status === 'error' ? 'danger' : 'success'"
              size="sm" :dot="true"
            >
              <component :is="item.raw.error || item.raw.status === 'error' ? XCircle : CheckCircle2" :size="11" />
              {{ item.raw.error || item.raw.status === 'error' ? 'ERROR' : (item.raw.stop_reason || 'OK') }}
            </WorldChip>
          </div>
          <div v-if="item.raw.error" class="err-detail">{{ item.raw.error }}</div>
        </template>
      </WorldTimeline>
    </WorldCard>

    <!-- Pagination -->
    <div v-if="totalLogs > pageSize" class="pagination">
      <WorldButton variant="ghost" size="sm" :disabled="currentPage <= 1" @click="goToPage(currentPage - 1)">
        <ChevronLeft :size="14" /><span>上一页</span>
      </WorldButton>
      <span class="page-info">
        第 {{ currentPage }} / {{ totalPages }} 页 · 共 {{ totalLogs }} 条
      </span>
      <WorldButton variant="ghost" size="sm" :disabled="currentPage >= totalPages" @click="goToPage(currentPage + 1)">
        <span>下一页</span><ChevronRight :size="14" />
      </WorldButton>
    </div>
  </div>
</template>

<style scoped>
.logs-page { display: flex; flex-direction: column; gap: 14px; }

.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.5rem;
  font-weight: 800;
  margin: 0;
  color: var(--world-text-primary);
}
.head-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.search-wrap { position: relative; display: flex; align-items: center; flex: 1; min-width: 240px; }
.search-icon { position: absolute; left: 12px; color: var(--world-text-mute); }
.search-input {
  flex: 1;
  height: 34px;
  padding: 0 32px 0 36px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  color: var(--world-text-primary);
  font-size: 0.8125rem;
  font-family: var(--world-font-sans);
  outline: none;
  transition: border-color 200ms;
}
.search-input:focus { border-color: var(--world-accent); }
.clear-btn {
  position: absolute;
  right: 8px;
  width: 22px; height: 22px;
  border-radius: 50%;
  background: transparent;
  border: none;
  color: var(--world-text-mute);
  cursor: pointer;
}

.model-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.orig-model {
  font-family: var(--world-font-mono);
  font-size: 0.85rem;
  color: var(--world-text-primary);
  font-weight: 700;
}
.arrow { color: var(--world-text-dim); font-size: 0.75rem; }
.actual-model {
  font-family: var(--world-font-mono);
  font-size: 0.78rem;
  color: var(--world-warning);
  font-weight: 700;
  padding: 1px 6px;
  background: rgba(245, 158, 11, 0.10);
  border: 1px solid rgba(245, 158, 11, 0.25);
  border-radius: var(--world-radius-sm);
}
[data-world="daogui"] .orig-model { color: var(--world-paper-aged); }

.log-meta-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.meta-cell {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.7rem;
  font-family: var(--world-font-mono);
  color: var(--world-text-mute);
}
.cr-label {
  font-size: 0.6rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  padding: 1px 5px;
  border-radius: var(--world-radius-sm);
  margin-right: 2px;
}
.credits-pre .cr-label {
  background: rgba(148, 163, 184, 0.18);
  color: var(--world-text-secondary);
}
.credits-pre {
  text-decoration: line-through;
  text-decoration-color: rgba(148, 163, 184, 0.5);
  opacity: 0.85;
}
.credits-post .cr-label {
  background: rgba(245, 158, 11, 0.18);
  color: var(--world-warning);
}
.credits-post {
  color: var(--world-warning);
  font-weight: 700;
}
.credits-cost {
  color: var(--world-success);
  font-weight: 700;
}
[data-world="daogui"] .credits-post { color: #f3c66e; }
[data-world="daogui"] .credits-cost { color: #95b5a8; }
.err-detail {
  margin-top: 6px;
  padding: 8px 10px;
  background: rgba(239, 68, 68, 0.08);
  border-left: 2px solid var(--world-error);
  border-radius: var(--world-radius-sm);
  font-size: 0.72rem;
  color: var(--world-error);
  font-family: var(--world-font-mono);
  word-break: break-all;
}
[data-world="daogui"] .err-detail { background: rgba(196, 30, 58, 0.10); color: #f5707f; }

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 8px 0;
}
.page-info {
  font-size: 0.78rem;
  color: var(--world-text-mute);
  font-family: var(--world-font-mono);
}
</style>
