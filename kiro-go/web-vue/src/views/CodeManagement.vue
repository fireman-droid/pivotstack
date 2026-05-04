<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Plus, Trash2, Copy, Search, X, Gift, Clock, ChevronLeft, ChevronRight, Sparkles } from 'lucide-vue-next'
import { copyToClipboard } from '../utils/clipboard'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldStat from '../components/world/WorldStat.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldModal from '../components/world/WorldModal.vue'
import WorldInput from '../components/world/WorldInput.vue'

const { success, error: toastErr } = useToast()
const codes = ref([])
const loading = ref(true)
const generating = ref(false)
const showCreate = ref(false)

const searchQuery = ref('')
const filterType = ref('all')
const filterStatus = ref('all')

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

const pageSize = ref(20)
const currentPage = ref(1)
const totalPages = computed(() => Math.max(1, Math.ceil(filteredCodes.value.length / pageSize.value)))
const pagedCodes = computed(() => {
  const s = (currentPage.value - 1) * pageSize.value
  return filteredCodes.value.slice(s, s + pageSize.value)
})
watch([searchQuery, filterType, filterStatus], () => { currentPage.value = 1 })

const selectedCodes = ref(new Set())
const isAllSelected = computed(() =>
  pagedCodes.value.length > 0 && pagedCodes.value.every(c => selectedCodes.value.has(c.code))
)
function toggleSelectAll() {
  if (isAllSelected.value) pagedCodes.value.forEach(c => selectedCodes.value.delete(c.code))
  else pagedCodes.value.forEach(c => selectedCodes.value.add(c.code))
}
function toggleSelect(code) {
  if (selectedCodes.value.has(code)) selectedCodes.value.delete(code)
  else selectedCodes.value.add(code)
}

const form = ref({
  type: 'balance',
  amount: 10,
  customBalance: '',
  customTime: { days: 0, hours: 0, minutes: 0 },
  useCustomTime: false,
  tier: 'free',
  count: 1,
  note: '',
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

async function loadCodes() {
  loading.value = true
  try {
    const res = await api('/codes')
    if (res.ok) codes.value = await res.json()
  } catch { toastErr('加载失败') }
  loading.value = false
}

async function generateCodes() {
  let amount
  if (form.value.type === 'balance') {
    amount = form.value.amount === -1 ? Number(form.value.customBalance) : form.value.amount
  } else {
    amount = form.value.useCustomTime ? customTimeSeconds.value : form.value.amount
  }
  if (!amount || amount <= 0) return toastErr('请设置有效的数值')

  generating.value = true
  try {
    const res = await api('/codes', {
      method: 'POST',
      body: JSON.stringify({
        type: form.value.type,
        amount,
        tier: form.value.type === 'time' ? form.value.tier : undefined,
        count: form.value.count,
        note: form.value.note,
      }),
    })
    if (res.ok) {
      const data = await res.json()
      success(`✅ 成功生成 ${data.count} 个激活码`)
      showCreate.value = false
      loadCodes()
    }
  } catch { toastErr('生成失败') }
  generating.value = false
}

async function deleteCode(code) {
  try {
    await api(`/codes/${code}`, { method: 'DELETE' })
    codes.value = codes.value.filter(c => c.code !== code)
    selectedCodes.value.delete(code)
    success('已作废')
  } catch { toastErr('操作失败') }
}

async function batchDelete() {
  const sel = [...selectedCodes.value]
  const unused = sel.filter(code => {
    const c = codes.value.find(x => x.code === code)
    return c && !c.usedBy
  })
  if (!unused.length) return toastErr('没有可作废的未使用激活码')
  if (!confirm(`确认批量作废 ${unused.length} 个激活码？`)) return
  let ok = 0, fail = 0
  for (const code of unused) {
    try {
      await api(`/codes/${code}`, { method: 'DELETE' })
      codes.value = codes.value.filter(c => c.code !== code)
      selectedCodes.value.delete(code)
      ok++
    } catch { fail++ }
  }
  success(`已作废 ${ok} 个${fail > 0 ? `，${fail} 个失败` : ''}`)
}

async function cleanupUsedCodes() {
  if (!confirm('确认彻底清除所有已作废和已使用的激活码记录？此操作不可恢复。')) return
  try {
    const res = await api('/codes/cleanup', { method: 'POST' })
    if (res.ok) {
      const data = await res.json()
      success(`清理成功，共清除了 ${data.cleaned} 条无效记录`)
      loadCodes()
    }
  } catch { toastErr('清理失败') }
}

function copyCode(code) { copyToClipboard(code); success('已复制') }
function copySelected() {
  const list = [...selectedCodes.value].join('\n')
  if (!list) return toastErr('未选择任何激活码')
  copyToClipboard(list)
  success(`已复制 ${selectedCodes.value.size} 个激活码`)
}
function copyAllUnused() {
  const unused = filteredCodes.value.filter(c => !c.usedBy).map(c => c.code).join('\n')
  if (!unused) return toastErr('没有未使用激活码')
  copyToClipboard(unused)
  success(`已复制 ${unused.split('\n').length} 个激活码`)
}

const stats = computed(() => ({
  total: codes.value.length,
  unused: codes.value.filter(c => !c.usedBy).length,
  used: codes.value.filter(c => c.usedBy).length,
  balanceCodes: codes.value.filter(c => c.type === 'balance' && !c.usedBy).length,
  timeCodes: codes.value.filter(c => (c.type === 'time' || c.type === 'days') && !c.usedBy).length,
}))

function fmtAmount(c) {
  if (c.type === 'balance') return `+$${(c.amount || 0).toFixed(2)}`
  const sec = c.amount || 0
  if (sec >= 86400) return `+${Math.floor(sec / 86400)}天`
  if (sec >= 3600) return `+${Math.floor(sec / 3600)}小时`
  return `+${Math.floor(sec / 60)}分钟`
}

function fmtTime(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

const typeOpts = [
  { value: 'all', label: '全部' },
  { value: 'balance', label: '余额' },
  { value: 'time', label: '时长' },
]
const statusOpts = [
  { value: 'all', label: '全部' },
  { value: 'unused', label: '未使用' },
  { value: 'used', label: '已使用' },
]

onMounted(loadCodes)
</script>

<template>
  <div class="codes-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow"><Gift :size="11" /> 激活码</div>
        <h1 class="page-title">激活码管理</h1>
      </div>
      <div class="head-actions">
        <WorldButton variant="secondary" size="md" @click="loadCodes">
          <span>刷新</span>
        </WorldButton>
        <WorldButton variant="primary" size="md" @click="showCreate = true">
          <Plus :size="14" /><span>生成激活码</span>
        </WorldButton>
      </div>
    </header>

    <!-- Stats -->
    <div class="stats-row">
      <WorldStat label="总数" :value="stats.total" />
      <WorldStat label="未使用" :value="stats.unused" variant="success" :icon="Sparkles" />
      <WorldStat label="已使用" :value="stats.used" variant="info" />
      <WorldStat label="未使用余额码" :value="stats.balanceCodes" variant="info" />
    </div>

    <!-- Filters -->
    <WorldCard padding="md">
      <div class="filter-row">
        <div class="search-wrap">
          <Search :size="14" class="search-icon" />
          <input
            v-model="searchQuery"
            class="search-input"
            placeholder="搜索激活码、备注、使用者"
          />
          <button v-if="searchQuery" @click="searchQuery = ''" class="clear-btn"><X :size="12" /></button>
        </div>
        <WorldSegment v-model="filterType" :options="typeOpts" size="sm" />
        <WorldSegment v-model="filterStatus" :options="statusOpts" size="sm" />
        <div class="filter-actions">
          <WorldButton v-if="selectedCodes.size > 0" variant="primary" size="sm" @click="copySelected">
            <Copy :size="13" /><span>复制选中 ({{ selectedCodes.size }})</span>
          </WorldButton>
          <WorldButton variant="secondary" size="sm" @click="copyAllUnused">
            <Copy :size="13" /><span>复制未使用</span>
          </WorldButton>
          <WorldButton v-if="selectedCodes.size > 0" variant="danger" size="sm" @click="batchDelete">
            <Trash2 :size="13" /><span>批量作废</span>
          </WorldButton>
          <WorldButton variant="secondary" size="sm" @click="cleanupUsedCodes">
            <Trash2 :size="13" /><span>清理记录</span>
          </WorldButton>
        </div>
      </div>
    </WorldCard>

    <!-- List -->
    <WorldCard padding="none">
      <div v-if="loading" class="empty-row">载入中…</div>
      <div v-else-if="!pagedCodes.length" class="empty-row">暂无激活码</div>
      <div v-else>
        <div class="th-row">
          <div class="th-cell select-cell">
            <input type="checkbox" :checked="isAllSelected" @change="toggleSelectAll" class="cb" />
          </div>
          <div class="th-cell">激活码</div>
          <div class="th-cell">类型</div>
          <div class="th-cell">数值</div>
          <div class="th-cell">备注</div>
          <div class="th-cell">使用者</div>
          <div class="th-cell">创建时间</div>
          <div class="th-cell action-cell">操作</div>
        </div>
        <div v-for="c in pagedCodes" :key="c.code" :class="['tr-row', { used: c.usedBy }]">
          <div class="td-cell select-cell">
            <input type="checkbox" :checked="selectedCodes.has(c.code)" @change="toggleSelect(c.code)" class="cb" />
          </div>
          <div class="td-cell mono">{{ c.code }}</div>
          <div class="td-cell">
            <WorldChip :variant="c.type === 'balance' ? 'info' : 'warning'" size="sm">
              <component :is="c.type === 'balance' ? Gift : Clock" :size="11" />
              {{ c.type === 'balance' ? '余额' : '时长' }}
            </WorldChip>
          </div>
          <div class="td-cell mono">{{ fmtAmount(c) }}</div>
          <div class="td-cell">{{ c.note || '—' }}</div>
          <div class="td-cell mono">{{ c.usedBy ? c.usedBy.slice(0, 12) + '…' : '—' }}</div>
          <div class="td-cell mono dim">{{ fmtTime(c.createdAt * 1000) }}</div>
          <div class="td-cell action-cell">
            <button class="ic-btn" @click="copyCode(c.code)" title="复制"><Copy :size="13" /></button>
            <button v-if="!c.usedBy" class="ic-btn danger" @click="deleteCode(c.code)" title="作废"><Trash2 :size="13" /></button>
          </div>
        </div>
      </div>
    </WorldCard>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="pagination">
      <WorldButton variant="ghost" size="sm" :disabled="currentPage <= 1" @click="currentPage--">
        <ChevronLeft :size="14" /><span>上一页</span>
      </WorldButton>
      <span class="page-info">第 {{ currentPage }} / {{ totalPages }} 页</span>
      <WorldButton variant="ghost" size="sm" :disabled="currentPage >= totalPages" @click="currentPage++">
        <span>下一页</span><ChevronRight :size="14" />
      </WorldButton>
    </div>

    <!-- Generate modal -->
    <WorldModal v-model="showCreate" title="生成激活码" size="lg">
      <div class="gen-body">
        <!-- Type switcher -->
        <div class="gen-section">
          <label class="gen-label">类型</label>
          <WorldSegment
            :modelValue="form.type"
            @update:modelValue="switchType"
            :options="[{ value: 'balance', label: '余额激活码' }, { value: 'time', label: '时长激活码' }]"
          />
        </div>

        <!-- Balance preset -->
        <div v-if="form.type === 'balance'" class="gen-section">
          <label class="gen-label">面额（$）</label>
          <div class="preset-grid">
            <button
              v-for="amt in balancePresets" :key="amt"
              :class="['preset-btn', { active: form.amount === amt }]"
              @click="form.amount = amt"
            >
              ${{ amt }}
            </button>
            <button :class="['preset-btn', { active: form.amount === -1 }]" @click="form.amount = -1">自定义</button>
          </div>
          <WorldInput
            v-if="form.amount === -1"
            v-model.number="form.customBalance"
            type="number"
            label="自定义金额（$）"
            placeholder="例如 25.5"
          />
        </div>

        <!-- Time preset -->
        <div v-if="form.type === 'time'" class="gen-section">
          <label class="gen-label">时长</label>
          <div class="preset-grid">
            <button
              v-for="t in timePresets" :key="t.label"
              :class="['preset-btn', { active: form.amount === t.seconds && !form.useCustomTime }]"
              @click="form.amount = t.seconds; form.useCustomTime = false"
            >
              {{ t.label }}
            </button>
            <button :class="['preset-btn', { active: form.useCustomTime }]" @click="form.useCustomTime = true">自定义</button>
          </div>
          <div v-if="form.useCustomTime" class="custom-time">
            <WorldInput v-model.number="form.customTime.days" type="number" label="天" />
            <WorldInput v-model.number="form.customTime.hours" type="number" label="小时" />
            <WorldInput v-model.number="form.customTime.minutes" type="number" label="分钟" />
          </div>
        </div>

        <!-- Common fields -->
        <div class="gen-section dual">
          <WorldInput v-model.number="form.count" type="number" label="生成数量" />
          <WorldInput v-model="form.note" label="备注（可选）" placeholder="批次说明" />
        </div>
      </div>
      <template #footer>
        <WorldButton variant="ghost" @click="showCreate = false">取消</WorldButton>
        <WorldButton variant="primary" :loading="generating" @click="generateCodes">生成</WorldButton>
      </template>
    </WorldModal>
  </div>
</template>

<style scoped>
.codes-page { display: flex; flex-direction: column; gap: 14px; }

.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 4px;
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
  margin: 0;
  color: var(--world-text-primary);
}
.head-actions { display: flex; gap: 8px; flex-wrap: wrap; }

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 720px) { .stats-row { grid-template-columns: repeat(2, 1fr); } }

.filter-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.search-wrap { position: relative; display: flex; align-items: center; flex: 1; min-width: 200px; }
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
.filter-actions { display: flex; gap: 8px; flex-wrap: wrap; }

/* Table */
.th-row, .tr-row {
  display: grid;
  grid-template-columns: 32px 1.6fr 1fr 1fr 1.2fr 1.2fr 1.4fr 90px;
  gap: 8px;
  padding: 10px 14px;
  align-items: center;
  font-size: 0.78rem;
}
.th-row {
  background: var(--world-overlay-light);
  border-bottom: 1px solid var(--world-divider);
  font-weight: 800;
  color: var(--world-text-mute);
  text-transform: uppercase;
  font-size: 0.65rem;
  letter-spacing: 0.06em;
}
.tr-row {
  border-bottom: 1px solid var(--world-divider);
  transition: background 220ms;
}
.tr-row:hover { background: var(--world-overlay-light); }
.tr-row.used { opacity: 0.6; }
.tr-row:last-child { border-bottom: none; }
.td-cell.mono { font-family: var(--world-font-mono); font-size: 0.8125rem; word-break: break-all; }
.td-cell.dim { color: var(--world-text-dim); }
.select-cell, .action-cell { display: flex; gap: 4px; align-items: center; }
.cb { accent-color: var(--world-accent); cursor: pointer; }

.ic-btn {
  width: 26px; height: 26px;
  border-radius: var(--world-radius-sm);
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.ic-btn:hover { color: var(--world-accent); border-color: var(--world-accent); }
.ic-btn.danger:hover { color: white; background: var(--world-error); border-color: var(--world-error); }

.empty-row {
  text-align: center;
  padding: 50px 20px;
  color: var(--world-text-mute);
  font-size: 0.875rem;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}
.page-info { font-size: 0.78rem; color: var(--world-text-mute); font-family: var(--world-font-mono); }

/* Generate body */
.gen-body { display: flex; flex-direction: column; gap: 16px; }
.gen-section { display: flex; flex-direction: column; gap: 10px; }
.gen-section.dual { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.gen-label {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.preset-grid {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.preset-btn {
  padding: 6px 12px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-sm);
  color: var(--world-text-mute);
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 200ms ease;
  font-family: var(--world-font-mono);
}
.preset-btn:hover { color: var(--world-text-primary); border-color: var(--world-accent); }
.preset-btn.active {
  background: var(--world-accent);
  border-color: var(--world-accent);
  color: white;
}
.custom-time {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

@media (max-width: 800px) {
  .th-row, .tr-row { grid-template-columns: 32px 1fr 0.8fr 0.8fr 1fr 80px; }
  .th-row .td-cell:nth-child(5),
  .th-row .td-cell:nth-child(6),
  .th-row .td-cell:nth-child(7),
  .tr-row .td-cell:nth-child(5),
  .tr-row .td-cell:nth-child(6),
  .tr-row .td-cell:nth-child(7) { display: none; }
}
</style>
