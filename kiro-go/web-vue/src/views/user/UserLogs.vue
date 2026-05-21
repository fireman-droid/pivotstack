<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { userApi } from '../../api/user'
import { useMessage, NSelect, NDatePicker } from 'naive-ui'
import { Search, Download, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import LogStatRibbon from '../../components/user/logs/LogStatRibbon.vue'
import LogDrawer, { type LogDetail } from '../../components/user/logs/LogDrawer.vue'
import StatusDot from '../../components/user/stellar/StatusDot.vue'
import { fmtCost } from '../../utils/format'

const message = useMessage()

interface UserLog extends LogDetail {}

const logs = ref<UserLog[]>([])
const loading = ref(false)
// mode: today=当天 / date=指定某天 / all=全部历史
const mode = ref<'today' | 'date' | 'all'>('today')
const selectedDate = ref<string>(todayStr()) // YYYY-MM-DD
const page = ref(1)
const limit = ref(50)
const total = ref(0)
const search = ref('')
const onlyError = ref(false)
const autoRefresh = ref(false)
let autoTimer: number | undefined

const selected = ref<UserLog | null>(null)

function todayStr(): string {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function buildLogsQS(): string {
  const qs = new URLSearchParams()
  qs.set('page', String(page.value))
  qs.set('limit', String(limit.value))
  if (mode.value === 'today') {
    qs.set('date', todayStr())
  } else if (mode.value === 'date') {
    qs.set('date', selectedDate.value || todayStr())
  } else {
    qs.set('date', 'all')
  }
  return qs.toString()
}

async function fetchLogs() {
  loading.value = true
  try {
    const resp = await userApi(`/logs?${buildLogsQS()}`)
    logs.value = resp.logs || []
    total.value = resp.total || 0
  } catch (e: any) {
    message.error(e?.message || '加载日志失败')
  } finally {
    loading.value = false
  }
}

function setMode(m: 'today' | 'date' | 'all') {
  if (mode.value === m && m !== 'date') return
  mode.value = m
  page.value = 1
  fetchLogs()
}

function onDateChange(v: string) {
  selectedDate.value = v
  mode.value = 'date'
  page.value = 1
  fetchLogs()
}

// NDatePicker 用 timestamp(ms) 不是 string，做一层桥接到 selectedDate(YYYY-MM-DD)。
const dateTs = computed<number | null>(() => {
  if (!selectedDate.value) return null
  const [y, m, d] = selectedDate.value.split('-').map(Number)
  if (!y || !m || !d) return null
  return new Date(y, m - 1, d).getTime()
})
function onDateTsChange(ts: number | null) {
  if (ts == null) {
    selectedDate.value = todayStr()
    mode.value = 'today'
  } else {
    const dt = new Date(ts)
    const pad = (n: number) => String(n).padStart(2, '0')
    selectedDate.value = `${dt.getFullYear()}-${pad(dt.getMonth() + 1)}-${pad(dt.getDate())}`
    mode.value = 'date'
  }
  page.value = 1
  fetchLogs()
}
function disableFuture(ts: number) {
  return ts > Date.now()
}

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))
function goPage(p: number) {
  if (p < 1 || p > totalPages.value || p === page.value) return
  page.value = p
  fetchLogs()
}

const pageSizeOptions = [
  { label: '20 / 页', value: 20 },
  { label: '50 / 页', value: 50 },
  { label: '100 / 页', value: 100 },
  { label: '200 / 页', value: 200 },
  { label: '500 / 页', value: 500 },
]
function onLimitChange(v: number) {
  limit.value = v
  page.value = 1
  fetchLogs()
}

function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) {
    autoTimer = window.setInterval(fetchLogs, 15_000)
  } else if (autoTimer) {
    window.clearInterval(autoTimer)
    autoTimer = undefined
  }
}

const filteredLogs = computed(() => {
  const q = search.value.trim().toLowerCase()
  return logs.value.filter(l => {
    if (onlyError.value && !(l.error || l.status === 'error')) return false
    if (!q) return true
    return (l.request_id || '').toLowerCase().includes(q) ||
      (l.original_model || '').toLowerCase().includes(q) ||
      (l.actual_model || '').toLowerCase().includes(q) ||
      (l.channel_alias || '').toLowerCase().includes(q) ||
      (l.channel_id || '').toLowerCase().includes(q)
  })
})

// stat ribbon 数据：今天 24 小时桶
function buildHourBuckets() {
  const calls = new Array(24).fill(0)
  const latencyAcc = new Array(24).fill(0)
  const latencyCnt = new Array(24).fill(0)
  const errors = new Array(24).fill(0)
  for (const log of logs.value) {
    const t = log.timestamp ? new Date(log.timestamp * 1000) : (log.time ? new Date(log.time) : null)
    if (!t) continue
    const h = t.getHours()
    calls[h] += 1
    if (typeof log.duration_ms === 'number' && log.duration_ms > 0) {
      latencyAcc[h] += log.duration_ms
      latencyCnt[h] += 1
    }
    if (log.error || log.status === 'error') errors[h] += 1
  }
  const latency = latencyAcc.map((a, i) => latencyCnt[i] ? a / latencyCnt[i] : 0)
  return { calls, latency, errors }
}

const ribbon = computed(() => {
  const b = buildHourBuckets()
  const total = b.calls.reduce((s, v) => s + v, 0)
  const latencyAll = logs.value.filter(l => l.duration_ms).map(l => l.duration_ms || 0)
  const avgLat = latencyAll.length ? latencyAll.reduce((s, v) => s + v, 0) / latencyAll.length : 0
  const errCount = logs.value.filter(l => l.error || l.status === 'error').length
  const errPct = total ? (errCount / total) * 100 : 0
  return {
    todayCount: total,
    avgLatencyMs: avgLat,
    errorRatePct: errPct,
    callsPerHour: b.calls,
    latencyPerHour: b.latency,
    errorsPerHour: b.errors,
  }
})

function fmtTime(l: UserLog) {
  const t = l.timestamp ? new Date(l.timestamp * 1000) : (l.time ? new Date(l.time) : null)
  if (!t) return '-'
  return `${String(t.getHours()).padStart(2, '0')}:${String(t.getMinutes()).padStart(2, '0')}:${String(t.getSeconds()).padStart(2, '0')}`
}
function fmtNum(n?: number) { return (n || 0).toLocaleString('en-US') }
function dollar(n?: number) { return fmtCost(n) }
function statusLabel(l: UserLog): string {
  if (l.error || l.status === 'error') return 'ERR'
  if (typeof l.status === 'number') return String(l.status)
  if (typeof l.status === 'string' && l.status) return l.status
  return 'OK'
}
function latencyClass(ms?: number) {
  const v = ms || 0
  if (v && v < 150) return 'num--green'
  if (v && v < 280) return ''
  if (v) return 'num--warn'
  return ''
}
function statusOf(l: UserLog): 'ok' | 'warn' | 'err' {
  if (l.error || l.status === 'error') return 'err'
  if ((l.duration_ms || 0) > 800) return 'warn'
  return 'ok'
}
function shortRid(r?: string) { return (r || '').slice(0, 8) }

function exportCsv() {
  if (!filteredLogs.value.length) {
    message.warning('当前没有可导出的日志')
    return
  }
  const headers = ['time', 'request_id', 'model', 'channel', 'in', 'out', 'total', 'cost_usd', 'duration_ms', 'status']
  const rows = filteredLogs.value.map(l => [
    fmtTime(l),
    l.request_id || '',
    l.original_model || l.actual_model || '',
    l.channel_alias || l.channel_id || '',
    l.input_tokens || 0,
    l.output_tokens || 0,
    (l.input_tokens || 0) + (l.output_tokens || 0),
    l.cost_usd || 0,
    l.duration_ms || 0,
    l.error ? 'error' : 'ok',
  ])
  const csv = [headers.join(','), ...rows.map(r => r.join(','))].join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `logs-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
  message.success('已导出 CSV')
}

function openDrawer(l: UserLog) { selected.value = l }
function closeDrawer() { selected.value = null }

// 给抽屉的 baseline：同模型最近 N 次延迟
const drawerBaseline = computed(() => {
  if (!selected.value) return []
  const m = selected.value.original_model || selected.value.actual_model
  return logs.value
    .filter(l => (l.original_model || l.actual_model) === m && typeof l.duration_ms === 'number' && l.duration_ms > 0)
    .map(l => l.duration_ms as number)
    .slice(0, 50)
})

onMounted(fetchLogs)
</script>

<template>
  <div class="logs stellar-scope">
    <LogStatRibbon
      :today-count="ribbon.todayCount"
      :avg-latency-ms="ribbon.avgLatencyMs"
      :error-rate-pct="ribbon.errorRatePct"
      :calls-per-hour="ribbon.callsPerHour"
      :latency-per-hour="ribbon.latencyPerHour"
      :errors-per-hour="ribbon.errorsPerHour"
    />

    <!-- Filter ribbon -->
    <div class="filter-ribbon">
      <div class="filter-ribbon__left">
        <button class="st-select" :class="{ 'is-active': mode === 'today' }" @click="setMode('today')">
          <span>今天</span>
        </button>
        <n-date-picker
          type="date"
          size="small"
          :value="dateTs"
          :is-date-disabled="disableFuture"
          :class="{ 'is-active': mode === 'date' }"
          class="st-date"
          format="yyyy-MM-dd"
          placement="bottom"
          clearable
          placeholder="选择日期"
          @update:value="onDateTsChange"
        />
        <button class="st-select" :class="{ 'is-active': mode === 'all' }" @click="setMode('all')">
          <span>全部历史</span>
        </button>
        <button class="st-select" :class="{ 'is-active': onlyError }" @click="onlyError = !onlyError">
          <span>{{ onlyError ? '仅错误' : '全部状态' }}</span>
        </button>
        <div class="st-input st-input--search">
          <Search :size="13" />
          <input v-model="search" class="mono" placeholder="搜索 request_id / 模型 / 渠道" />
        </div>
      </div>
      <div class="filter-ribbon__right">
        <button class="btn btn--secondary btn--sm" @click="exportCsv">
          <Download :size="13" />导出 CSV
        </button>
        <div class="st-switch">
          <span class="t-label">自动刷新</span>
          <button
            class="st-switch__track"
            :class="{ 'is-on': autoRefresh }"
            @click="toggleAutoRefresh"
          ><span class="st-switch__thumb"></span></button>
        </div>
      </div>
    </div>

    <!-- Table + Drawer -->
    <div class="logs-layout">
      <div class="tile tile--logs">
        <div class="table">
          <div class="table__head">
            <div style="width:88px">时间</div>
            <div style="width:100px">request_id</div>
            <div style="flex:1;min-width:150px">模型</div>
            <div style="width:100px">上游</div>
            <div style="width:64px;text-align:right">in</div>
            <div style="width:64px;text-align:right">out</div>
            <div style="width:80px;text-align:right">total</div>
            <div style="width:72px;text-align:right">花费</div>
            <div style="width:80px;text-align:right">延迟</div>
            <div style="width:40px;text-align:center">状态</div>
          </div>
          <div class="table__body">
            <div
              v-for="l in filteredLogs"
              :key="l.request_id || `${l.timestamp}-${l.original_model}`"
              class="table__row"
              :class="{ 'is-selected': selected?.request_id === l.request_id }"
              @click="openDrawer(l)"
            >
              <div class="time" style="width:88px">{{ fmtTime(l) }}</div>
              <div style="width:100px" class="mono">{{ shortRid(l.request_id) }}</div>
              <div style="flex:1;min-width:150px">{{ l.original_model || l.actual_model || '-' }}</div>
              <div style="width:100px"><span class="chip chip--mono">{{ l.channel_alias || l.channel_id || '-' }}</span></div>
              <div class="num" style="width:64px;text-align:right">{{ fmtNum(l.input_tokens) }}</div>
              <div class="num" style="width:64px;text-align:right">{{ fmtNum(l.output_tokens) }}</div>
              <div class="num num--strong" style="width:80px;text-align:right">{{ fmtNum((l.input_tokens || 0) + (l.output_tokens || 0)) }}</div>
              <div class="num num--green" style="width:72px;text-align:right">{{ dollar(l.cost_usd) }}</div>
              <div class="num" :class="latencyClass(l.duration_ms)" style="width:80px;text-align:right">{{ l.duration_ms || 0 }}ms</div>
              <div class="status-cell" :class="`status--${statusOf(l)}`">
                <StatusDot :status="statusOf(l)" />
                <span class="status-text">{{ statusLabel(l) }}</span>
              </div>
            </div>
            <div v-if="!loading && !filteredLogs.length" class="t-label tertiary" style="padding: 24px 12px">
              {{ search.trim() || onlyError ? '无符合条件的日志 · 调整筛选条件后再试' : '还没有调用记录' }}
            </div>
          </div>
          <!-- pagination footer -->
          <div v-if="total > 0" class="pager">
            <div class="pager__info">
              共 <b>{{ total.toLocaleString('en-US') }}</b> 条
              <span v-if="filteredLogs.length !== logs.length" class="pager__filtered">（当前页过滤后 {{ filteredLogs.length }}）</span>
              · 第 {{ page }} / {{ totalPages }} 页
            </div>
            <div class="pager__ctrl">
              <button class="st-select" :disabled="page <= 1" @click="goPage(page - 1)" title="上一页">
                <ChevronLeft :size="13" />
              </button>
              <input
                type="number"
                class="pager__jump mono"
                :value="page"
                :min="1"
                :max="totalPages"
                @change="goPage(Number(($event.target as HTMLInputElement).value))"
              />
              <button class="st-select" :disabled="page >= totalPages" @click="goPage(page + 1)" title="下一页">
                <ChevronRight :size="13" />
              </button>
              <n-select
                :value="limit"
                :options="pageSizeOptions"
                size="small"
                class="pager__limit"
                :consistent-menu-width="false"
                @update:value="onLimitChange"
              />
            </div>
          </div>
        </div>
      </div>

      <LogDrawer
        :log="selected"
        :baseline="drawerBaseline"
        @close="closeDrawer"
      />
    </div>
  </div>
</template>

<style scoped>
.filter-ribbon {
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px;
  padding: 0 8px;
  margin-bottom: 16px;
  flex-wrap: wrap;
  min-height: 56px;
}
.filter-ribbon__left, .filter-ribbon__right { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.st-select.is-active { background: rgba(11,212,112,0.08); color: var(--st-success); box-shadow: inset 0 0 0 1px rgba(11,212,112,0.3); }
.st-input--search { flex: 1; min-width: 200px; max-width: 280px; gap: 8px; height: 32px; }
.st-input--search input { font-size: 12px; }

.logs-layout {
  display: grid;
  grid-template-columns: 1fr 420px;
  gap: 24px;
  align-items: flex-start;
}
.tile--logs { padding: 16px 0; min-height: 600px; background: rgba(255,255,255,0.02); border-radius: 8px; }
.tile--logs .table__head { padding-left: 24px; padding-right: 24px; }
.tile--logs .table__row { padding-left: 24px; padding-right: 24px; }
.tile--logs .table__row.is-selected { background: rgba(11,212,112,0.06); }

@media (max-width: 1280px) {
  .logs-layout { grid-template-columns: 1fr 380px; }
}
@media (max-width: 1024px) {
  .logs-layout { grid-template-columns: 1fr; }
}
@media (max-width: 768px) {
  .tile--logs .table__head, .tile--logs .table__row { padding-left: 12px; padding-right: 12px; }
  .logs-layout { overflow-x: auto; }
}

/* date picker — 跟其他 st-select 视觉同步 */
.st-date {
  height: 28px;
  padding: 0 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 4px;
  color: #ededed;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
  font-size: 12px;
  cursor: pointer;
  transition: border-color 0.12s, background 0.12s;
}
.st-date::-webkit-calendar-picker-indicator { filter: invert(0.7); cursor: pointer; }
.st-date:hover { border-color: rgba(255, 255, 255, 0.20); background: rgba(255, 255, 255, 0.05); }
.st-date.is-active {
  border-color: rgba(82, 168, 255, 0.50);
  background: rgba(82, 168, 255, 0.08);
  color: #52a8ff;
}

/* pagination footer */
.pager {
  display: flex; align-items: center; justify-content: space-between;
  gap: 16px; flex-wrap: wrap;
  padding: 12px 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.04);
  background: rgba(255, 255, 255, 0.01);
}
.pager__info { color: #a3a3a3; font-size: 12px; }
.pager__info b { color: #ededed; font-variant-numeric: tabular-nums; }
.pager__filtered { color: #707070; }
.pager__ctrl { display: flex; align-items: center; gap: 6px; }
.pager__ctrl .st-select { padding: 0 10px; min-width: 32px; height: 28px; display: inline-flex; align-items: center; justify-content: center; }
.pager__ctrl .st-select:disabled { opacity: 0.4; cursor: not-allowed; }
.pager__jump {
  width: 56px; height: 28px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 4px;
  color: #ededed; text-align: center;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  -moz-appearance: textfield;
}
.pager__jump::-webkit-outer-spin-button,
.pager__jump::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.pager__jump:focus { outline: none; border-color: rgba(82, 168, 255, 0.50); }
.pager__limit { width: 96px; }

/* 状态列：圆点 + 文本（200 / ERR / OK）— 修复仅显示圆点没数字的 user/admin 不一致 */
.status-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  justify-content: center;
  width: 64px;
  font-variant-numeric: tabular-nums;
  font-family: var(--st-font-mono, 'Geist Mono', monospace);
  font-size: 11px;
  font-weight: 600;
}
.status-cell .status-text { letter-spacing: 0.04em; }
.status--ok  .status-text { color: var(--st-success, #0bd470); }
.status--warn .status-text { color: var(--st-warning, #f5a623); }
.status--err .status-text { color: var(--st-error, #ff4d4d); }
</style>
