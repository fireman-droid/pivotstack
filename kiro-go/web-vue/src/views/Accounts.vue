<script setup>
import { onMounted, computed, ref, watch } from 'vue'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../composables/useToast'
import { api } from '../api/admin'
import {
  Plus, Search, RotateCw, Users, Crown, ShieldAlert, CheckCircle2,
  PackageSearch, LayoutGrid, List, X, Trash2, ChevronLeft, ChevronRight
} from 'lucide-vue-next'
import AccountCard from '../components/AccountCard.vue'
import BatchBar from '../components/BatchBar.vue'
import WorldCard from '../components/world/WorldCard.vue'
import WorldStat from '../components/world/WorldStat.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldModal from '../components/world/WorldModal.vue'

const store = useAccountsStore()
const { success, error } = useToast()
const isRefreshing = ref(false)
const viewMode = ref('grid')
const currentPage = ref(1)
const pageSize = 50

const showAddDialog = ref(false)
const isAdding = ref(false)
const jsonText = ref('')
const importStatus = ref({ phase: 'idle', total: 0, imported: 0, failed: 0, elapsed: 0, message: '' })
let elapsedTimer = null

const poolStats = ref({
  freePool: { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 },
  proPool:  { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 },
})

async function loadPoolStats() {
  try {
    const res = await api('/status')
    if (res.ok) {
      const d = await res.json()
      const def = { total: 0, available: 0, usageLimit: 0, usageCurrent: 0, trialLimit: 0, trialCurrent: 0 }
      poolStats.value = {
        freePool: { ...def, ...d.freePool },
        proPool:  { ...def, ...d.proPool },
      }
    }
  } catch {}
}

const stats = computed(() => {
  const all = store.accounts.length
  const freeAccs = store.accounts.filter(a => !a.subscriptionType || a.subscriptionType === 'FREE')
  const proAccs  = store.accounts.filter(a => /pro|power/i.test(a.subscriptionType || ''))
  const banned   = store.accounts.filter(a => a.banStatus && a.banStatus !== 'ACTIVE').length
  return {
    all,
    freeCount: freeAccs.length,
    proCount:  proAccs.length,
    banned,
    freeAvailable: poolStats.value.freePool.available,
    proAvailable:  poolStats.value.proPool.available,
  }
})

onMounted(() => {
  store.load()
  loadPoolStats()
})

const totalPages = computed(() => Math.max(1, Math.ceil(store.filtered.length / pageSize)))
const paginatedAccounts = computed(() => {
  const s = (currentPage.value - 1) * pageSize
  return store.filtered.slice(s, s + pageSize)
})

watch(() => [store.filterKeyword, store.filterStatus, store.filterTier], () => { currentPage.value = 1 })

async function refreshAll() {
  isRefreshing.value = true
  try {
    await store.load()
    await loadPoolStats()
    success('已刷新账号列表')
  } finally {
    isRefreshing.value = false
  }
}

const isDeletingBanned = ref(false)
async function deleteBanned() {
  const banned = store.accounts.filter(a => a.banStatus && a.banStatus !== 'ACTIVE')
  if (!banned.length) { error('没有封禁账号'); return }
  if (!confirm(`确定删除 ${banned.length} 个封禁/限制账号？此操作不可恢复！`)) return
  isDeletingBanned.value = true
  try {
    const res = await api('/accounts/batch', {
      method: 'POST',
      body: JSON.stringify({ ids: banned.map(a => a.id), action: 'delete' }),
    })
    const d = await res.json()
    if (d.success) {
      success(`已删除 ${d.deleted} 个封禁账号`)
      store.filterStatus = 'all'
      await store.load()
    } else error(d.error || '删除失败')
  } catch (e) { error('删除失败: ' + e.message) }
  isDeletingBanned.value = false
}

function resetAddDialog() {
  jsonText.value = ''
  importStatus.value = { phase: 'idle', total: 0, imported: 0, failed: 0, elapsed: 0, message: '' }
  if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
}

async function submitImport() {
  if (!jsonText.value.trim()) { error('请粘贴账号 JSON 数据'); return }
  let parsed
  try { parsed = JSON.parse(jsonText.value.trim()) } catch { error('JSON 格式错误，请检查内容'); return }

  const items = Array.isArray(parsed) ? parsed : [parsed]
  if (!items.length) { error('JSON 数组为空'); return }

  const accounts = items.map(item => {
    let authMethod = item.authMethod || ''
    if (!authMethod) {
      const provider = (item.provider || '').toLowerCase()
      if (provider === 'google' || provider === 'github') authMethod = 'social'
      else if (item.clientId || item.clientID) authMethod = 'idc'
      else authMethod = 'social'
    }
    return {
      accessToken:  item.accessToken  || item.access_token  || '',
      refreshToken: item.refreshToken || item.refresh_token || '',
      clientId:     item.clientId     || item.clientID     || item.client_id || '',
      clientSecret: item.clientSecret || item.client_secret || '',
      authMethod,
      provider: item.provider || '',
      region:   item.region   || 'us-east-1',
      email:    item.email    || '',
      userId:   item.userId   || item.user_id || '',
      profileArn: item.profileArn || '',
      machineId:  item.machineId  || '',
      usageData:  item.usageData  || null,
    }
  }).filter(a => a.refreshToken || a.accessToken)

  if (!accounts.length) { error('没有有效的账号（缺少 token）'); return }

  isAdding.value = true
  importStatus.value = { phase: 'importing', total: accounts.length, done: 0, imported: 0, failed: 0, elapsed: 0, message: '' }

  const startMs = Date.now()
  elapsedTimer = setInterval(() => {
    importStatus.value.elapsed = ((Date.now() - startMs) / 1000).toFixed(1)
  }, 200)

  try {
    // batch-import 需要流式响应，无法用 api() wrapper 包（wrapper 在 !res.ok 时直接 throw 吃掉 body）
    // 所以这里手写 fetch，但鉴权改成 cookie + CSRF（跟 api wrapper 同一套口径）
    const auth = (await import('../stores/auth')).useAuthStore()
    const res = await fetch('/admin/api/auth/credentials/batch', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': auth.csrfToken,
      },
      body: JSON.stringify({ accounts, concurrency: 20 }),
    })
    if (res.status === 401) auth.clearLocal()
    if (!res.ok) throw new Error(`HTTP ${res.status}`)

    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop()
      let currentEvent = ''
      for (const line of lines) {
        if (line.startsWith('event: ')) currentEvent = line.slice(7).trim()
        else if (line.startsWith('data: ')) {
          try {
            const data = JSON.parse(line.slice(6))
            if (currentEvent === 'progress') {
              importStatus.value.done = data.done || 0
              importStatus.value.imported = data.ok || 0
              importStatus.value.failed = data.fail || 0
            } else if (currentEvent === 'done') {
              clearInterval(elapsedTimer); elapsedTimer = null
              importStatus.value = {
                phase: 'done',
                total: accounts.length,
                done: accounts.length,
                imported: data.imported || 0,
                failed: data.failed || 0,
                elapsed: data.elapsed_sec?.toFixed(1) || importStatus.value.elapsed,
                message: data.message || '',
              }
              setTimeout(() => store.load(), 1500)
            }
          } catch {}
          currentEvent = ''
        }
      }
    }
    if (importStatus.value.phase === 'importing') {
      clearInterval(elapsedTimer); elapsedTimer = null
      importStatus.value.phase = 'done'
      importStatus.value.message = `导入完成: ${importStatus.value.imported} 成功, ${importStatus.value.failed} 失败`
      setTimeout(() => store.load(), 1500)
    }
  } catch (e) {
    clearInterval(elapsedTimer); elapsedTimer = null
    importStatus.value = { ...importStatus.value, phase: 'error', message: '请求失败: ' + e.message }
  }
  isAdding.value = false
}

const tierOptions = [
  { value: 'all',  label: '全部' },
  { value: 'free', label: 'FREE' },
  { value: 'pro',  label: 'PRO' },
]
const statusOptions = [
  { value: 'all',     label: '全部' },
  { value: 'enabled', label: '启用' },
  { value: 'disabled',label: '禁用' },
  { value: 'banned',  label: '封禁' },
]
const viewOptions = [
  { value: 'grid', label: '网格', icon: LayoutGrid },
  { value: 'list', label: '列表', icon: List },
]
</script>

<template>
  <div class="accounts-page">
    <!-- Header -->
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">账号资产</div>
        <h1 class="page-title">账号管理</h1>
      </div>
      <div class="head-actions">
        <WorldButton variant="secondary" size="md" @click="refreshAll" :loading="isRefreshing">
          <RotateCw :size="14" /><span>刷新</span>
        </WorldButton>
        <WorldButton variant="primary" size="md" @click="showAddDialog = true">
          <Plus :size="14" /><span>添加账号</span>
        </WorldButton>
      </div>
    </header>

    <!-- Stats -->
    <div class="stats-row">
      <WorldStat label="账号总数" :value="stats.all" :icon="Users" />
      <WorldStat label="FREE 账号" :value="stats.freeCount" :hint="`${stats.freeAvailable} 可用`" :icon="Users" variant="success" />
      <WorldStat label="PRO 账号"  :value="stats.proCount"  :hint="`${stats.proAvailable} 可用`" :icon="Crown" variant="info" />
      <WorldStat label="封禁数"    :value="stats.banned"   :icon="ShieldAlert" :variant="stats.banned ? 'danger' : 'success'" />
    </div>

    <!-- Filter bar -->
    <WorldCard padding="md" class="filter-card">
      <div class="filter-row">
        <div class="search-wrap">
          <Search :size="14" class="search-icon" />
          <input
            v-model="store.filterKeyword"
            type="text"
            placeholder="搜索 email / id"
            class="search-input"
          />
          <button v-if="store.filterKeyword" @click="store.filterKeyword = ''" class="clear-btn"><X :size="12" /></button>
        </div>
        <WorldSegment v-model="store.filterTier"   :options="tierOptions"   size="sm" />
        <WorldSegment v-model="store.filterStatus" :options="statusOptions" size="sm" />
        <WorldSegment v-model="viewMode"           :options="viewOptions"   size="sm" />
        <WorldButton
          v-if="stats.banned > 0"
          variant="danger" size="sm"
          @click="deleteBanned" :loading="isDeletingBanned"
        >
          <Trash2 :size="13" /><span>清理封禁</span>
        </WorldButton>
      </div>
    </WorldCard>

    <!-- Batch action bar (when items selected) -->
    <BatchBar />

    <!-- Account grid -->
    <div v-if="paginatedAccounts.length === 0" class="empty-row">
      <WorldCard padding="lg">
        <div class="empty-content">
          <PackageSearch :size="40" />
          <h4>暂无账号</h4>
          <p>请添加账号或调整筛选条件</p>
        </div>
      </WorldCard>
    </div>
    <div v-else :class="['account-grid', `view-${viewMode}`]">
      <AccountCard v-for="acc in paginatedAccounts" :key="acc.id" :account="acc" />
    </div>

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

    <!-- Add modal -->
    <WorldModal
      v-model="showAddDialog"
      title="批量导入账号"
      size="lg"
      @close="resetAddDialog"
    >
      <div class="import-body">
        <p class="import-hint">粘贴账号 JSON 数据，支持单条或数组格式</p>
        <textarea
          v-model="jsonText"
          class="json-input"
          rows="10"
          placeholder='[{ "refreshToken": "...", "clientId": "...", "clientSecret": "...", ... }]'
          spellcheck="false"
        />

        <div v-if="importStatus.phase !== 'idle'" class="import-status" :class="`phase-${importStatus.phase}`">
          <div class="status-row" v-if="importStatus.phase === 'importing'">
            <span>导入中… {{ importStatus.done }} / {{ importStatus.total }}（{{ importStatus.elapsed }}s）</span>
          </div>
          <div class="status-row" v-else-if="importStatus.phase === 'done'">
            <CheckCircle2 :size="14" />
            <span>{{ importStatus.message || `导入完成：${importStatus.imported} 成功 / ${importStatus.failed} 失败 / ${importStatus.elapsed}s` }}</span>
          </div>
          <div class="status-row" v-else-if="importStatus.phase === 'error'">
            <X :size="14" />
            <span>{{ importStatus.message }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <WorldButton variant="ghost" @click="showAddDialog = false">取消</WorldButton>
        <WorldButton variant="primary" :loading="isAdding" @click="submitImport">开始导入</WorldButton>
      </template>
    </WorldModal>
  </div>
</template>

<style scoped>
.accounts-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

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
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}
.head-actions { display: flex; gap: 8px; flex-wrap: wrap; }

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 920px) { .stats-row { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 480px) { .stats-row { grid-template-columns: 1fr; } }

.filter-card { padding: 12px 16px; }
.filter-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.search-wrap {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 200px;
}
.search-icon {
  position: absolute;
  left: 12px;
  color: var(--world-text-mute);
  pointer-events: none;
}
.search-input {
  width: 100%;
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
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.clear-btn:hover { color: var(--world-error); }

.account-grid.view-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 14px;
}
.account-grid.view-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.empty-content {
  text-align: center;
  padding: 24px;
  color: var(--world-text-mute);
}
.empty-content h4 { margin: 12px 0 4px; font-size: 1rem; font-weight: 800; color: var(--world-text-primary); }
.empty-content p { margin: 0; font-size: 0.8125rem; }

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

/* Import modal */
.import-body { display: flex; flex-direction: column; gap: 12px; }
.import-hint { margin: 0; font-size: 0.8125rem; color: var(--world-text-mute); }
.json-input {
  width: 100%;
  padding: 12px 14px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  color: var(--world-text-primary);
  font-family: var(--world-font-mono);
  font-size: 0.8125rem;
  resize: vertical;
  outline: none;
  transition: border-color 200ms;
  min-height: 200px;
}
.json-input:focus { border-color: var(--world-accent); }

.import-status {
  padding: 10px 12px;
  border-radius: var(--world-radius-md);
  font-size: 0.8125rem;
}
.phase-importing { background: rgba(2, 132, 199, 0.10); color: var(--world-info); border: 1px solid rgba(2, 132, 199, 0.25); }
.phase-done { background: rgba(16, 185, 129, 0.10); color: var(--world-success); border: 1px solid rgba(16, 185, 129, 0.25); }
.phase-error { background: rgba(239, 68, 68, 0.10); color: var(--world-error); border: 1px solid rgba(239, 68, 68, 0.25); }
.status-row { display: flex; align-items: center; gap: 8px; }
</style>
