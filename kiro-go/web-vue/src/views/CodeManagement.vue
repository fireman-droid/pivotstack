<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Plus, Trash2, Copy, Gift, Clock, Download, Search, Filter, CheckSquare, Square, RefreshCw, X } from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const codes = ref([])
const loading = ref(true)
const generating = ref(false)
const showCreatePanel = ref(false)

// ===== 搜索 & 筛选 =====
const searchQuery = ref('')
const filterType = ref('all') // all | balance | time | days
const filterStatus = ref('all') // all | unused | used

const filteredCodes = computed(() => {
  let list = codes.value
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(c =>
      c.code.toLowerCase().includes(q) ||
      (c.note && c.note.toLowerCase().includes(q)) ||
      (c.usedBy && c.usedBy.toLowerCase().includes(q))
    )
  }
  if (filterType.value !== 'all') {
    if (filterType.value === 'time') {
      list = list.filter(c => c.type === 'time' || c.type === 'days')
    } else {
      list = list.filter(c => c.type === filterType.value)
    }
  }
  if (filterStatus.value === 'unused') list = list.filter(c => !c.usedBy)
  if (filterStatus.value === 'used') list = list.filter(c => c.usedBy)
  return list
})

// ===== 分页 =====
const pageSize = ref(20)
const currentPage = ref(1)
const totalPages = computed(() => Math.max(1, Math.ceil(filteredCodes.value.length / pageSize.value)))
const pagedCodes = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredCodes.value.slice(start, start + pageSize.value)
})
watch([searchQuery, filterType, filterStatus], () => { currentPage.value = 1 })

// ===== 批量选择 =====
const selectedCodes = ref(new Set())
const isAllSelected = computed(() =>
  pagedCodes.value.length > 0 && pagedCodes.value.every(c => selectedCodes.value.has(c.code))
)

function toggleSelectAll() {
  if (isAllSelected.value) {
    pagedCodes.value.forEach(c => selectedCodes.value.delete(c.code))
  } else {
    pagedCodes.value.forEach(c => selectedCodes.value.add(c.code))
  }
}

function toggleSelect(code) {
  if (selectedCodes.value.has(code)) {
    selectedCodes.value.delete(code)
  } else {
    selectedCodes.value.add(code)
  }
}

// ===== 创建表单 =====
const form = ref({
  type: 'balance',
  amount: 10,
  customTime: { days: 0, hours: 0, minutes: 0 },
  useCustomTime: false,
  customBalance: '',
  tier: 'free',
  count: 1,
  note: ''
})

const balancePresets = [5, 10, 50, 100, 300]
const timePresets = [
  { label: '1小时', seconds: 3600 },
  { label: '6小时', seconds: 21600 },
  { label: '1天', seconds: 86400 },
  { label: '3天', seconds: 259200 },
  { label: '7天', seconds: 604800 },
  { label: '15天', seconds: 1296000 },
  { label: '30天', seconds: 2592000 },
]

const customTimeSeconds = computed(() => {
  const t = form.value.customTime
  return (t.days || 0) * 86400 + (t.hours || 0) * 3600 + (t.minutes || 0) * 60
})

function switchType(type) {
  form.value.type = type
  form.value.useCustomTime = false
  form.value.amount = type === 'balance' ? 10 : 86400
}

// ===== API 操作 =====
async function loadCodes() {
  loading.value = true
  try {
    const res = await api('/codes')
    if (res.ok) codes.value = await res.json()
  } catch { toastError('加载失败') }
  loading.value = false
}

async function generateCodes() {
  let amount
  if (form.value.type === 'balance') {
    amount = form.value.amount === -1 ? Number(form.value.customBalance) : form.value.amount
  } else {
    amount = form.value.useCustomTime ? customTimeSeconds.value : form.value.amount
  }
  if (!amount || amount <= 0) return toastError('请设置有效的数值')

  generating.value = true
  try {
    const res = await api('/codes', {
      method: 'POST',
      body: JSON.stringify({
        type: form.value.type,
        amount,
        tier: form.value.type === 'time' ? form.value.tier : undefined,
        count: form.value.count,
        note: form.value.note
      })
    })
    if (res.ok) {
      const data = await res.json()
      success(`✅ 成功生成 ${data.count} 个激活码`)
      showCreatePanel.value = false
      loadCodes()
    }
  } catch { toastError('生成失败') }
  generating.value = false
}

async function deleteCode(code) {
  try {
    await api(`/codes/${code}`, { method: 'DELETE' })
    codes.value = codes.value.filter(c => c.code !== code)
    selectedCodes.value.delete(code)
    success('已作废')
  } catch { toastError('操作失败') }
}

async function batchDelete() {
  const selected = [...selectedCodes.value]
  const unusedSelected = selected.filter(code => {
    const c = codes.value.find(x => x.code === code)
    return c && !c.usedBy
  })
  if (!unusedSelected.length) return toastError('没有可作废的未使用激活码')
  if (!confirm(`确认批量作废 ${unusedSelected.length} 个激活码？`)) return

  let ok = 0, fail = 0
  for (const code of unusedSelected) {
    try {
      await api(`/codes/${code}`, { method: 'DELETE' })
      codes.value = codes.value.filter(c => c.code !== code)
      selectedCodes.value.delete(code)
      ok++
    } catch { fail++ }
  }
  success(`已作废 ${ok} 个${fail > 0 ? `，${fail} 个失败` : ''}`)
}

function copyCode(code) {
  navigator.clipboard?.writeText(code)
  success('已复制')
}

function copySelected() {
  const list = [...selectedCodes.value].join('\n')
  if (!list) return toastError('未选择任何激活码')
  navigator.clipboard?.writeText(list)
  success(`已复制 ${selectedCodes.value.size} 个激活码`)
}

function copyAllUnused() {
  const unused = filteredCodes.value.filter(c => !c.usedBy).map(c => c.code).join('\n')
  if (!unused) return toastError('没有未使用激活码')
  navigator.clipboard?.writeText(unused)
  success(`已复制 ${unused.split('\n').length} 个激活码`)
}

// ===== 统计 =====
const stats = computed(() => ({
  total: codes.value.length,
  unused: codes.value.filter(c => !c.usedBy).length,
  used: codes.value.filter(c => c.usedBy).length,
  balanceCodes: codes.value.filter(c => c.type === 'balance' && !c.usedBy).length,
  timeCodes: codes.value.filter(c => (c.type === 'time' || c.type === 'days') && !c.usedBy).length,
}))

// ===== 格式化 =====
function fmtDate(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleDateString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit'
  })
}

function fmtDuration(seconds) {
  if (!seconds) return '-'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const parts = []
  if (d) parts.push(`${d}天`)
  if (h) parts.push(`${h}时`)
  if (m) parts.push(`${m}分`)
  return parts.join('') || '0分'
}

function fmtAmount(c) {
  if (c.type === 'balance') return `¥${c.amount}`
  return fmtDuration(c.amount)
}

function exportCSV() {
  const rows = [['激活码','类型','面值','等级','状态','使用方','备注','创建时间']]
  filteredCodes.value.forEach(c => {
    rows.push([
      c.code, c.type === 'balance' ? '余额' : '时间',
      fmtAmount(c), c.tier || '-',
      c.usedBy ? '已使用' : '未使用', c.usedBy || '-',
      c.note || '-', fmtDate(c.createdAt)
    ])
  })
  const csv = rows.map(r => r.join(',')).join('\n')
  const a = document.createElement('a')
  a.href = 'data:text/csv;charset=utf-8,\uFEFF' + encodeURIComponent(csv)
  a.download = `activation_codes_${new Date().toISOString().slice(0,10)}.csv`
  a.click()
  success('已导出 CSV')
}

onMounted(loadCodes)
</script>

<template>
  <div class="space-y-5 max-w-[1400px] mx-auto pb-20">

    <!-- 顶部标题 + 统计 -->
    <div class="flex flex-col md:flex-row md:items-end justify-between gap-4">
      <div>
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">激活码管理</h1>
        <div class="flex items-center gap-4 mt-2 text-xs font-bold">
          <span class="text-[var(--text-secondary)]">共 <span class="text-[var(--text)]">{{ stats.total }}</span> 个</span>
          <span class="text-emerald-400">{{ stats.unused }} 可用</span>
          <span class="text-[var(--text-secondary)]/60">{{ stats.used }} 已使用</span>
          <span class="text-[var(--text-secondary)]/40">|</span>
          <span class="text-emerald-400/80">💰{{ stats.balanceCodes }}</span>
          <span class="text-sky-400/80">⏱️{{ stats.timeCodes }}</span>
        </div>
      </div>
      <div class="flex gap-2">
        <button @click="loadCodes" class="toolbar-btn" title="刷新">
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
        </button>
        <button @click="exportCSV" class="toolbar-btn" title="导出 CSV">
          <Download class="w-3.5 h-3.5" /> 导出
        </button>
        <button @click="showCreatePanel = !showCreatePanel"
          class="px-4 py-2 bg-[var(--primary)] text-white rounded-xl text-xs font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] transition-all flex items-center gap-1.5">
          <Plus class="w-4 h-4" /> 创建激活码
        </button>
      </div>
    </div>

    <!-- 创建面板（折叠） -->
    <Transition name="slide">
      <div v-if="showCreatePanel" class="modern-card p-6 space-y-5 border-l-4 border-l-[var(--primary)]">
        <div class="flex items-center justify-between">
          <div class="text-sm font-bold text-[var(--text)]">创建新激活码</div>
          <button @click="showCreatePanel = false" class="p-1 rounded-lg hover:bg-[var(--bg)] transition-colors">
            <X class="w-4 h-4 text-[var(--text-secondary)]" />
          </button>
        </div>

        <!-- 步骤式表单 -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-5">

          <!-- 1. 类型 -->
          <div class="space-y-2">
            <label class="form-label">① 类型</label>
            <div class="flex gap-2">
              <button @click="switchType('balance')"
                class="type-btn" :class="form.type === 'balance' ? 'type-btn-active-green' : ''">
                💰 余额卡
              </button>
              <button @click="switchType('time')"
                class="type-btn" :class="form.type === 'time' ? 'type-btn-active-blue' : ''">
                ⏱️ 时间卡
              </button>
            </div>
          </div>

          <!-- 2. 面值/时长 -->
          <div class="space-y-2">
            <label class="form-label">② {{ form.type === 'balance' ? '面值' : '时长' }}</label>

            <!-- 余额预设 -->
            <template v-if="form.type === 'balance'">
              <div class="flex gap-1.5 flex-wrap">
                <button v-for="v in balancePresets" :key="v" @click="form.amount = v"
                  class="preset-btn" :class="form.amount === v ? 'preset-btn-active' : ''">
                  ¥{{ v }}
                </button>
                <button @click="form.amount = -1" class="preset-btn" :class="form.amount === -1 ? 'preset-btn-active' : ''">
                  自定义
                </button>
              </div>
              <input v-if="form.amount === -1" v-model.number="form.customBalance" type="number" min="0.01" step="0.01"
                placeholder="输入金额" class="form-input" />
            </template>

            <!-- 时间预设 -->
            <template v-else>
              <div class="flex gap-1.5 flex-wrap">
                <button v-for="p in timePresets" :key="p.seconds"
                  @click="form.amount = p.seconds; form.useCustomTime = false"
                  class="preset-btn" :class="form.amount === p.seconds && !form.useCustomTime ? 'preset-btn-active' : ''">
                  {{ p.label }}
                </button>
                <button @click="form.useCustomTime = true"
                  class="preset-btn" :class="form.useCustomTime ? 'preset-btn-active' : ''">
                  自定义
                </button>
              </div>
              <div v-if="form.useCustomTime" class="flex gap-2 items-center">
                <div class="flex items-center gap-1">
                  <input v-model.number="form.customTime.days" type="number" min="0" class="form-input-mini" placeholder="0" />
                  <span class="text-[10px] text-[var(--text-secondary)]">天</span>
                </div>
                <div class="flex items-center gap-1">
                  <input v-model.number="form.customTime.hours" type="number" min="0" max="23" class="form-input-mini" placeholder="0" />
                  <span class="text-[10px] text-[var(--text-secondary)]">时</span>
                </div>
                <div class="flex items-center gap-1">
                  <input v-model.number="form.customTime.minutes" type="number" min="0" max="59" class="form-input-mini" placeholder="0" />
                  <span class="text-[10px] text-[var(--text-secondary)]">分</span>
                </div>
                <span v-if="customTimeSeconds > 0" class="text-[10px] text-emerald-400 font-bold whitespace-nowrap">= {{ fmtDuration(customTimeSeconds) }}</span>
              </div>
            </template>
          </div>

          <!-- 3. 等级（仅时间卡） -->
          <div class="space-y-2">
            <label class="form-label">③ {{ form.type === 'time' ? '等级' : '数量 & 备注' }}</label>
            <template v-if="form.type === 'time'">
              <div class="flex gap-2">
                <button @click="form.tier = 'free'" class="type-btn" :class="form.tier === 'free' ? 'type-btn-active-blue' : ''">
                  🔒 Free
                </button>
                <button @click="form.tier = 'pro'" class="type-btn" :class="form.tier === 'pro' ? 'type-btn-active-amber' : ''">
                  👑 Pro
                </button>
              </div>
              <div class="text-[10px] text-[var(--text-secondary)] mt-1">
                {{ form.tier === 'free' ? 'Free = 仅 Sonnet 4.5' : 'Pro = 全模型' }}
              </div>
            </template>
            <template v-else>
              <div class="flex gap-2">
                <input v-model.number="form.count" type="number" min="1" max="100" placeholder="数量" class="form-input w-20" />
                <input v-model="form.note" placeholder="备注（可选）" class="form-input flex-1" />
              </div>
            </template>
          </div>

          <!-- 4. 数量 & 生成 -->
          <div class="space-y-2">
            <label class="form-label">④ {{ form.type === 'time' ? '数量 & 生成' : '生成' }}</label>
            <template v-if="form.type === 'time'">
              <div class="flex gap-2">
                <input v-model.number="form.count" type="number" min="1" max="100" placeholder="数量" class="form-input w-20" />
                <input v-model="form.note" placeholder="备注" class="form-input flex-1" />
              </div>
            </template>
            <button @click="generateCodes" :disabled="generating"
              class="w-full h-10 rounded-xl bg-[var(--primary)] text-white text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.01] transition-all flex items-center justify-center gap-2 disabled:opacity-50">
              <Plus v-if="!generating" class="w-4 h-4" />
              <RefreshCw v-else class="w-4 h-4 animate-spin" />
              {{ generating ? '生成中...' : `生成 ${form.count} 个` }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 搜索 & 筛选 & 批量操作 -->
    <div class="flex flex-col md:flex-row gap-3 items-start md:items-center justify-between">
      <div class="flex gap-2 items-center flex-wrap">
        <!-- 搜索 -->
        <div class="relative">
          <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-secondary)]" />
          <input v-model="searchQuery" placeholder="搜索激活码 / 备注 / 使用方..."
            class="h-9 pl-9 pr-3 w-60 bg-[var(--card)] border border-[var(--border)] rounded-xl text-xs outline-none focus:border-[var(--primary)] transition-colors" />
        </div>
        <!-- 类型筛选 -->
        <div class="flex gap-1 items-center">
          <span class="text-[10px] text-[var(--text-secondary)] font-bold mr-1">类型</span>
          <button @click="filterType = 'all'" class="filter-btn" :class="filterType === 'all' ? 'filter-btn-active' : ''">全部</button>
          <button @click="filterType = 'balance'" class="filter-btn" :class="filterType === 'balance' ? 'filter-btn-active' : ''">💰 余额</button>
          <button @click="filterType = 'time'" class="filter-btn" :class="filterType === 'time' ? 'filter-btn-active' : ''">⏱️ 时间</button>
        </div>
        <span class="text-[var(--border)] mx-1">|</span>
        <!-- 状态筛选 -->
        <div class="flex gap-1 items-center">
          <span class="text-[10px] text-[var(--text-secondary)] font-bold mr-1">状态</span>
          <button @click="filterStatus = 'all'" class="filter-btn" :class="filterStatus === 'all' ? 'filter-btn-active' : ''">全部</button>
          <button @click="filterStatus = 'unused'" class="filter-btn" :class="filterStatus === 'unused' ? 'filter-btn-active' : ''">可用</button>
          <button @click="filterStatus = 'used'" class="filter-btn" :class="filterStatus === 'used' ? 'filter-btn-active' : ''">已使用</button>
        </div>
      </div>

      <!-- 批量操作 -->
      <div v-if="selectedCodes.size > 0" class="flex gap-2 items-center">
        <span class="text-xs font-bold text-[var(--primary)]">已选 {{ selectedCodes.size }} 项</span>
        <button @click="copySelected" class="toolbar-btn">
          <Copy class="w-3.5 h-3.5" /> 复制
        </button>
        <button @click="batchDelete" class="toolbar-btn text-rose-400 hover:bg-rose-500/10 border-rose-500/20">
          <Trash2 class="w-3.5 h-3.5" /> 批量作废
        </button>
        <button @click="selectedCodes.clear()" class="toolbar-btn">
          <X class="w-3.5 h-3.5" /> 取消
        </button>
      </div>
      <div v-else class="flex gap-2">
        <button @click="copyAllUnused" class="toolbar-btn">
          <Copy class="w-3.5 h-3.5" /> 复制全部未使用
        </button>
      </div>
    </div>

    <!-- 列表 -->
    <div class="modern-card overflow-hidden">
      <!-- 表头 -->
      <div class="code-table-row" style="font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; color: var(--text-secondary); border-bottom: 1px solid var(--border); padding: 10px 20px;">
        <div style="cursor: pointer; display: flex; align-items: center; justify-content: center;" @click="toggleSelectAll">
          <CheckSquare v-if="isAllSelected" class="w-4 h-4" style="color: var(--primary)" />
          <Square v-else class="w-4 h-4" />
        </div>
        <span>激活码</span>
        <span>类型 / 面值</span>
        <span>状态</span>
        <span>创建时间</span>
        <span>备注</span>
        <span style="text-align: right">操作</span>
      </div>

      <!-- 数据行 -->
      <div v-for="c in pagedCodes" :key="c.code"
        class="code-table-row"
        :style="{ opacity: c.usedBy ? 0.5 : 1, background: selectedCodes.has(c.code) ? 'rgba(var(--primary-rgb, 196,30,58), 0.04)' : 'transparent', padding: '12px 20px', borderBottom: '1px solid rgba(128,128,128,0.1)', cursor: 'pointer' }"
        @click="toggleSelect(c.code)">

        <div style="display: flex; align-items: center; justify-content: center;">
          <CheckSquare v-if="selectedCodes.has(c.code)" class="w-4 h-4" style="color: var(--primary)" />
          <Square v-else class="w-4 h-4" style="color: var(--text-secondary)" />
        </div>

        <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
          <span style="font-family: monospace; font-weight: 700; font-size: 13px; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">{{ c.code }}</span>
          <button @click.stop="copyCode(c.code)" style="padding: 4px; border: none; background: none; cursor: pointer; border-radius: 4px; display: flex; flex-shrink: 0;">
            <Copy class="w-3.5 h-3.5" style="color: var(--text-secondary)" />
          </button>
        </div>

        <div style="display: flex; align-items: center; gap: 8px;">
          <Gift v-if="c.type === 'balance'" class="w-4 h-4" style="color: #059669; flex-shrink: 0;" />
          <Clock v-else class="w-4 h-4" style="color: #0284c7; flex-shrink: 0;" />
          <span style="font-size: 13px; font-weight: 600; color: var(--text);">{{ fmtAmount(c) }}</span>
          <span v-if="(c.type === 'days' || c.type === 'time') && c.tier"
            :style="'padding: 2px 6px; border-radius: 4px; font-size: 10px; font-weight: 700; text-transform: uppercase;' + (c.tier === 'pro' ? 'background:rgba(217,119,6,0.15);color:#b45309' : 'background:rgba(2,132,199,0.12);color:#0369a1')">
            {{ c.tier }}
          </span>
        </div>

        <span>
          <span v-if="c.usedBy" style="padding: 3px 8px; border-radius: 4px; font-size: 12px; background: rgba(107,114,128,0.12); color: #6b7280;">已使用</span>
          <span v-else style="padding: 3px 8px; border-radius: 4px; font-size: 12px; font-weight: 700; background: rgba(5,150,105,0.12); color: #059669;">可用</span>
        </span>

        <span style="font-size: 12px; color: var(--text-secondary);">{{ fmtDate(c.createdAt) }}</span>

        <span style="font-size: 12px; color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ c.note || '-' }}</span>

        <div style="display: flex; justify-content: flex-end;">
          <button v-if="!c.usedBy" @click.stop="deleteCode(c.code)" style="padding: 6px; border: none; background: none; cursor: pointer; border-radius: 6px;" title="作废">
            <Trash2 class="w-4 h-4" style="color: #e11d48" />
          </button>
        </div>
      </div>

      <!-- 空状态 -->
      <div v-if="!pagedCodes.length && !loading" class="text-center py-16">
        <div class="text-3xl mb-3">📭</div>
        <div class="text-sm font-bold text-[var(--text-secondary)]">
          {{ searchQuery || filterType !== 'all' || filterStatus !== 'all' ? '没有匹配的激活码' : '暂无激活码' }}
        </div>
        <div class="text-xs text-[var(--text-secondary)]/50 mt-1">
          {{ searchQuery ? '尝试修改搜索条件' : '点击上方「创建激活码」开始' }}
        </div>
      </div>

      <!-- 加载中 -->
      <div v-if="loading" class="text-center py-12 text-sm text-[var(--text-secondary)]">加载中...</div>
    </div>

    <!-- 分页 -->
    <div v-if="totalPages > 1" class="flex items-center justify-between">
      <span class="text-xs text-[var(--text-secondary)]">
        显示 {{ (currentPage - 1) * pageSize + 1 }}-{{ Math.min(currentPage * pageSize, filteredCodes.length) }} / 共 {{ filteredCodes.length }} 条
      </span>
      <div class="flex gap-1">
        <button @click="currentPage = Math.max(1, currentPage - 1)" :disabled="currentPage === 1"
          class="page-btn">‹</button>
        <template v-for="p in totalPages" :key="p">
          <button v-if="p === 1 || p === totalPages || Math.abs(p - currentPage) <= 1"
            @click="currentPage = p" class="page-btn" :class="p === currentPage ? 'page-btn-active' : ''">
            {{ p }}
          </button>
          <span v-else-if="Math.abs(p - currentPage) === 2" class="text-[var(--text-secondary)] text-xs px-1">…</span>
        </template>
        <button @click="currentPage = Math.min(totalPages, currentPage + 1)" :disabled="currentPage === totalPages"
          class="page-btn">›</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 表格行布局 */
.code-table-row {
  display: grid;
  grid-template-columns: 36px 2fr 1.5fr 80px 140px 1fr 50px;
  gap: 16px;
  align-items: center;
}

/* 工具栏按钮 */
.toolbar-btn {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.5rem 0.75rem;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--text-secondary);
  transition: all 0.2s;
  cursor: pointer;
}
.toolbar-btn:hover { border-color: var(--primary); }

/* 表单标签 */
.form-label {
  display: block;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--text-secondary);
}

/* 表单输入 */
.form-input {
  height: 2.25rem;
  padding: 0 0.75rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  font-size: 0.75rem;
  outline: none;
  color: var(--text);
  transition: border-color 0.2s;
}
.form-input:focus { border-color: var(--primary); }

.form-input-mini {
  width: 3.5rem;
  height: 2.25rem;
  padding: 0 0.5rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  font-size: 0.75rem;
  outline: none;
  text-align: center;
  color: var(--text);
  transition: border-color 0.2s;
}
.form-input-mini:focus { border-color: var(--primary); }

/* 类型按钮 */
.type-btn {
  flex: 1;
  padding: 0.625rem 0.75rem;
  border-radius: 0.75rem;
  font-size: 0.75rem;
  font-weight: 700;
  transition: all 0.2s;
  text-align: center;
  background: var(--bg);
  color: var(--text-secondary);
  border: 1px solid transparent;
  cursor: pointer;
}
.type-btn:hover { border-color: var(--border); }
.type-btn-active-green {
  background: rgba(16, 185, 129, 0.15) !important;
  color: #34d399 !important;
  border-color: rgba(16, 185, 129, 0.3) !important;
}
.type-btn-active-blue {
  background: rgba(14, 165, 233, 0.15) !important;
  color: #38bdf8 !important;
  border-color: rgba(14, 165, 233, 0.3) !important;
}
.type-btn-active-amber {
  background: rgba(245, 158, 11, 0.15) !important;
  color: #fbbf24 !important;
  border-color: rgba(245, 158, 11, 0.3) !important;
}

/* 预设按钮 */
.preset-btn {
  padding: 0.375rem 0.75rem;
  border-radius: 0.5rem;
  font-size: 0.75rem;
  font-weight: 700;
  transition: all 0.2s;
  background: var(--bg);
  color: var(--text-secondary);
  border: none;
  cursor: pointer;
}
.preset-btn:hover { color: var(--text); }
.preset-btn-active {
  background: rgba(196, 30, 58, 0.15) !important;
  color: var(--primary) !important;
  box-shadow: 0 0 0 1px rgba(196, 30, 58, 0.3);
}

/* 筛选按钮 */
.filter-btn {
  padding: 0.375rem 0.625rem;
  border-radius: 0.5rem;
  font-size: 11px;
  font-weight: 700;
  transition: all 0.2s;
  color: var(--text-secondary);
  background: none;
  border: none;
  cursor: pointer;
}
.filter-btn:hover { color: var(--text); }
.filter-btn-active {
  background: rgba(196, 30, 58, 0.1) !important;
  color: var(--primary) !important;
}

/* 分页按钮 */
.page-btn {
  width: 2rem;
  height: 2rem;
  border-radius: 0.5rem;
  font-size: 0.75rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
  background: none;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
}
.page-btn:hover { background: var(--bg); }
.page-btn:disabled { opacity: 0.3; cursor: default; }
.page-btn-active {
  background: rgba(196, 30, 58, 0.15) !important;
  color: var(--primary) !important;
}

/* 过渡动画 */
.slide-enter-active, .slide-leave-active { transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1); }
.slide-enter-from { opacity: 0; max-height: 0; transform: translateY(-10px); }
.slide-enter-to { opacity: 1; max-height: 500px; }
.slide-leave-from { opacity: 1; max-height: 500px; }
.slide-leave-to { opacity: 0; max-height: 0; transform: translateY(-10px); }
</style>
