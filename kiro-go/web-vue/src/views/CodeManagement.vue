<script setup>
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Plus, Trash2, Copy, Gift, Clock, Download } from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const codes = ref([])
const loading = ref(true)
const generating = ref(false)

const form = ref({
  type: 'balance',
  amount: 10,
  customAmount: '',
  tier: 'free',
  count: 1,
  note: ''
})

const amountPresets = {
  balance: [5, 10, 50, 100, 300],
  days: [1, 3, 7, 15, 30]
}

async function loadCodes() {
  try {
    const res = await api('/codes')
    if (res.ok) codes.value = await res.json()
  } catch { toastError('加载失败') }
  loading.value = false
}

async function generateCodes() {
  generating.value = true
  try {
    const res = await api('/codes', {
      method: 'POST',
      body: JSON.stringify({
        type: form.value.type,
        amount: form.value.amount === -1 ? Number(form.value.customAmount) : form.value.amount,
        tier: form.value.type === 'days' ? form.value.tier : undefined,
        count: form.value.count,
        note: form.value.note
      })
    })
    if (res.ok) {
      const data = await res.json()
      success(`生成 ${data.count} 个激活码`)
      loadCodes()
    }
  } catch { toastError('生成失败') }
  generating.value = false
}

async function deleteCode(code) {
  if (!confirm(`确认作废激活码 ${code}？`)) return
  try {
    await api(`/codes/${code}`, { method: 'DELETE' })
    codes.value = codes.value.filter(c => c.code !== code)
    success('已作废')
  } catch { toastError('操作失败') }
}

function copyCode(code) {
  navigator.clipboard?.writeText(code)
  success('已复制')
}

function copyAllUnused() {
  const unused = codes.value.filter(c => !c.usedBy).map(c => c.code).join('\n')
  if (!unused) return toastError('没有可复制的未使用激活码')
  navigator.clipboard?.writeText(unused)
  success(`已复制 ${unused.split('\n').length} 个激活码`)
}

const stats = computed(() => ({
  total: codes.value.length,
  unused: codes.value.filter(c => !c.usedBy).length,
  used: codes.value.filter(c => c.usedBy).length,
}))

function fmtDate(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function exportCSV() {
  const rows = [['激活码','类型','面值','状态','使用方','创建时间']]
  codes.value.forEach(c => rows.push([c.code, c.type, c.amount, c.usedBy ? '已使用' : '未使用', c.usedBy || '-', fmtDate(c.createdAt)]))
  const csv = rows.map(r => r.join(',')).join('\n')
  const a = document.createElement('a')
  a.href = 'data:text/csv;charset=utf-8,\uFEFF' + encodeURIComponent(csv)
  a.download = 'activation_codes.csv'
  a.click()
}

onMounted(loadCodes)
</script>

<template>
  <div class="space-y-6 max-w-[1400px] mx-auto pb-20">
    <div class="space-y-1">
      <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">激活码管理</h1>
      <p class="text-sm text-[var(--text-secondary)]">
        共 {{ stats.total }} 个 · {{ stats.unused }} 未使用 · {{ stats.used }} 已使用
      </p>
    </div>

    <!-- Generate Form -->
    <div class="modern-card p-5 space-y-4">
      <div class="text-[11px] font-bold uppercase tracking-widest text-[var(--text-secondary)]">批量生成</div>
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4 items-end">
        <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">类型</label>
          <div class="flex gap-2">
            <button @click="form.type = 'balance'; form.amount = 10"
              class="flex-1 px-3 py-2 rounded-lg text-xs font-bold transition-all text-center"
              :class="form.type === 'balance' ? 'bg-emerald-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)]'">
              💰 余额
            </button>
            <button @click="form.type = 'days'; form.amount = 7"
              class="flex-1 px-3 py-2 rounded-lg text-xs font-bold transition-all text-center"
              :class="form.type === 'days' ? 'bg-sky-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)]'">
              📅 天数
            </button>
          </div>
        </div>
          <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">{{ form.type === 'balance' ? '面值 (¥)' : '天数' }}</label>
          <div class="flex gap-1 flex-wrap">
            <button v-for="v in amountPresets[form.type]" :key="v" @click="form.amount = v"
              class="px-2.5 py-1 rounded-lg text-xs font-bold transition-all"
              :class="form.amount === v ? 'bg-[var(--primary)] text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)]'">
              {{ form.type === 'balance' ? '¥' : '' }}{{ v }}{{ form.type === 'days' ? '天' : '' }}
            </button>
            <button @click="form.amount = -1"
              class="px-2.5 py-1 rounded-lg text-xs font-bold transition-all"
              :class="form.amount === -1 ? 'bg-[var(--primary)] text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)]'">
              自定义
            </button>
          </div>
          <input v-if="form.amount === -1" v-model.number="form.customAmount" type="number" min="1" step="1"
            :placeholder="form.type === 'balance' ? '输入金额 (¥)' : '输入天数'"
            class="w-full h-9 px-3 mt-1 bg-[var(--bg)] border border-[var(--border)] rounded-lg text-xs outline-none focus:border-[var(--primary)]" />
        </div>
        <!-- Tier (only for days) -->
        <div v-if="form.type === 'days'" class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">等级</label>
          <div class="flex gap-2">
            <button @click="form.tier = 'free'"
              class="flex-1 px-3 py-2 rounded-lg text-xs font-bold transition-all text-center"
              :class="form.tier === 'free' ? 'bg-sky-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)]'">
              🔒 Free（仅 4.5）
            </button>
            <button @click="form.tier = 'pro'"
              class="flex-1 px-3 py-2 rounded-lg text-xs font-bold transition-all text-center"
              :class="form.tier === 'pro' ? 'bg-amber-500 text-white' : 'bg-[var(--bg)] text-[var(--text-secondary)]'">
              👑 Pro（全模型）
            </button>
          </div>
        </div>
        <div class="space-y-1">
          <label class="text-xs text-[var(--text-secondary)]">数量 / 备注</label>
          <div class="flex gap-2">
            <input v-model.number="form.count" type="number" min="1" max="100"
              class="w-16 h-9 px-3 bg-[var(--bg)] border border-[var(--border)] rounded-lg text-xs outline-none focus:border-[var(--primary)]" />
            <input v-model="form.note" placeholder="备注"
              class="flex-1 h-9 px-3 bg-[var(--bg)] border border-[var(--border)] rounded-lg text-xs outline-none focus:border-[var(--primary)]" />
          </div>
        </div>
        <button @click="generateCodes" :disabled="generating"
          class="flex items-center justify-center gap-2 px-5 py-2.5 bg-[var(--primary)] text-white rounded-xl text-sm font-bold shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] transition-all">
          <Plus class="w-4 h-4" />
          {{ generating ? '生成中...' : '生成' }}
        </button>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex gap-2">
      <button @click="copyAllUnused" class="px-4 py-2 bg-[var(--card)] border border-[var(--border)] rounded-xl text-xs font-bold hover:border-[var(--primary)] transition-all">
        📋 复制所有未使用码
      </button>
      <button @click="exportCSV" class="flex items-center gap-1.5 px-4 py-2 bg-[var(--card)] border border-[var(--border)] rounded-xl text-xs font-bold hover:border-[var(--primary)] transition-all">
        <Download class="w-3.5 h-3.5" /> 导出 CSV
      </button>
    </div>

    <!-- Code List -->
    <div class="modern-card overflow-hidden">
      <div class="grid grid-cols-[2fr_1fr_1fr_1fr_1fr_auto] gap-3 px-5 py-3 text-[10px] font-bold uppercase tracking-widest text-[var(--text-secondary)] border-b border-[var(--border)]">
        <span>激活码</span>
        <span>类型</span>
        <span>面值</span>
        <span>状态</span>
        <span>创建时间</span>
        <span class="w-16">操作</span>
      </div>

      <div v-for="c in codes" :key="c.code"
        class="grid grid-cols-[2fr_1fr_1fr_1fr_1fr_auto] gap-3 px-5 py-3 items-center text-sm border-b border-[var(--border)]/50 hover:bg-[var(--bg)]/50 transition-colors"
        :class="{ 'opacity-50': c.usedBy }">
        <div class="flex items-center gap-2">
          <span class="font-mono font-bold text-[var(--primary)] text-xs">{{ c.code }}</span>
          <button @click="copyCode(c.code)" class="p-1 hover:bg-[var(--bg)] rounded">
            <Copy class="w-3 h-3 text-[var(--text-secondary)]" />
          </button>
        </div>
        <div class="flex items-center gap-1">
          <Gift v-if="c.type === 'balance'" class="w-3 h-3 text-emerald-500" />
          <Clock v-else class="w-3 h-3 text-sky-500" />
          <span class="text-xs">{{ c.type === 'balance' ? '余额' : '天数' }}</span>
          <span v-if="c.type === 'days' && c.tier" class="ml-1 px-1.5 py-0.5 rounded text-[9px] font-bold uppercase"
            :class="c.tier === 'pro' ? 'bg-amber-500/10 text-amber-500' : 'bg-sky-500/10 text-sky-500'">
            {{ c.tier }}
          </span>
        </div>
        <span class="text-xs font-bold">
          {{ c.type === 'balance' ? '¥' + c.amount : c.amount + '天' }}
        </span>
        <span class="text-xs">
          <span v-if="c.usedBy" class="px-2 py-0.5 rounded bg-[var(--text-secondary)]/10 text-[var(--text-secondary)]">已使用</span>
          <span v-else class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-500 font-bold">可用</span>
        </span>
        <span class="text-[11px] text-[var(--text-secondary)]">{{ fmtDate(c.createdAt) }}</span>
        <div class="w-16 flex justify-end">
          <button v-if="!c.usedBy" @click="deleteCode(c.code)" class="p-1.5 rounded-lg hover:bg-rose-500/10">
            <Trash2 class="w-3.5 h-3.5 text-rose-500" />
          </button>
        </div>
      </div>

      <div v-if="!codes.length" class="text-center py-12 text-sm text-[var(--text-secondary)]">
        暂无激活码
      </div>
    </div>
  </div>
</template>
