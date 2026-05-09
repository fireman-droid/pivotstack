<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { copyToClipboard } from '../utils/clipboard'
import {
  Plus, Trash2, Copy, Eye, EyeOff, Key,
  Pencil, Search, X, Save, Clock, Wallet,
  ChevronDown
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldModal from '../components/world/WorldModal.vue'

const { success, error: toastErr } = useToast()

const keys = ref([])
const loading = ref(false)
const showCreate = ref(false)
const showKeyId = ref(null)
const expandedId = ref(null)
const searchQuery = ref('')
const editingId = ref(null)
const editForm = reactive({
  note: '', balance: 0, giftBalance: 0, expiresAt: 0,
  // 代理设置
  isReseller: false, maxChildKeys: 0, resellerDiscountPercent: 100, parentKeyId: '',
})
const form = ref({ note: '' })

const CNY_PER_USD = 0.05  // 1$ face value = ¥0.05

async function loadKeys() {
  loading.value = true
  try {
    const res = await api('/apikeys')
    if (res.ok) keys.value = await res.json()
  } catch { toastErr('加载失败') }
  loading.value = false
}

async function createKey() {
  try {
    const res = await api('/apikeys', { method: 'POST', body: JSON.stringify({ note: form.value.note }) })
    if (res.ok) {
      const newKey = await res.json()
      keys.value.unshift(newKey)
      showCreate.value = false
      showKeyId.value = newKey.id
      form.value = { note: '' }
      success('API Key 已创建')
    }
  } catch { toastErr('创建失败') }
}

async function toggleKey(k) {
  try {
    await api(`/apikeys/${k.id}`, { method: 'PUT', body: JSON.stringify({ enabled: !k.enabled }) })
    k.enabled = !k.enabled
    success(k.enabled ? '已启用' : '已禁用')
  } catch { toastErr('操作失败') }
}

async function deleteKey(k) {
  if (!confirm(`确认删除 Key "${k.note || k.id.slice(0, 8)}"？此操作不可撤销。`)) return
  try {
    await api(`/apikeys/${k.id}`, { method: 'DELETE' })
    keys.value = keys.value.filter(x => x.id !== k.id)
    success('已删除')
  } catch { toastErr('删除失败') }
}

function startEdit(k) {
  editingId.value = k.id
  editForm.note = k.note || ''
  editForm.balance     = ((k.balance || 0) * CNY_PER_USD).toFixed(2)
  editForm.giftBalance = ((k.giftBalance || 0) * CNY_PER_USD).toFixed(2)
  editForm.expiresAt = k.expiresAt || 0
  // 代理设置
  editForm.isReseller = !!k.isReseller
  editForm.maxChildKeys = k.maxChildKeys || 0
  // 后端存的是小数（0.5），UI 显示为百分比（50）；空 / 0 视作 100%（无折扣）
  const d = Number(k.resellerDiscount || 0)
  editForm.resellerDiscountPercent = d > 0 && d < 1 ? Math.round(d * 100) : 100
  editForm.parentKeyId = k.parentKeyId || ''
}
function cancelEdit() { editingId.value = null }

async function saveEdit(k) {
  try {
    // 子 key 不允许开代理；UI 已 disable 但后端再防一手
    const isChild = !!k.parentKeyId
    const isReseller = !isChild && editForm.isReseller
    const pct = Math.max(1, Math.min(100, Number(editForm.resellerDiscountPercent) || 100))
    const body = {
      note: editForm.note,
      balance: Number(editForm.balance) / CNY_PER_USD,
      giftBalance: Number(editForm.giftBalance) / CNY_PER_USD,
      expiresAt: Number(editForm.expiresAt),
      isReseller: isReseller,
      maxChildKeys: Math.max(0, Number(editForm.maxChildKeys) || 0),
      resellerDiscount: pct >= 100 ? 0 : pct / 100,  // 100% 存 0（视作无折扣）
    }
    const res = await api(`/apikeys/${k.id}`, { method: 'PUT', body: JSON.stringify(body) })
    if (res.ok) {
      k.note = editForm.note
      k.balance = body.balance
      k.giftBalance = body.giftBalance
      k.expiresAt = body.expiresAt
      k.isReseller = body.isReseller
      k.maxChildKeys = body.maxChildKeys
      k.resellerDiscount = body.resellerDiscount
      editingId.value = null
      success('已保存')
    }
  } catch { toastErr('保存失败') }
}

function toggleExpand(k) {
  expandedId.value = expandedId.value === k.id ? null : k.id
}
function copyText(text) { copyToClipboard(text); success('已复制') }
function maskKey(k) { if (!k) return ''; return k.slice(0, 7) + '••••••••' + k.slice(-4) }
function formatDate(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
function timeRemaining(expiresAt) {
  if (!expiresAt) return { text: '永不过期', variant: 'success' }
  const diff = expiresAt - Date.now() / 1000
  if (diff <= 0) return { text: '已过期', variant: 'danger' }
  const days = Math.floor(diff / 86400)
  const hours = Math.floor((diff % 86400) / 3600)
  const mins = Math.max(1, Math.ceil((diff % 3600) / 60))
  let text = ''
  if (days > 0) text += `${days}天`
  if (hours > 0) text += `${hours}小时`
  if (days === 0 && mins > 0) text += `${mins}分钟`
  const variant = days < 1 ? 'danger' : days < 3 ? 'warning' : 'success'
  return { text: text || '1分钟', variant }
}
function expiresAtDisplay(ts) { if (!ts) return '未设置'; return new Date(ts * 1000).toLocaleString('zh-CN') }
function addTime(amount) {
  const now = Math.floor(Date.now() / 1000)
  const base = editForm.expiresAt > now ? editForm.expiresAt : now
  editForm.expiresAt = Math.max(now, base + amount)
}

// 清空到期时间需二次确认，避免误点把已生效的天卡蒸发
function clearExpiresAt() {
  const now = Math.floor(Date.now() / 1000)
  const remaining = (editForm.expiresAt || 0) - now
  if (remaining > 0) {
    const days = Math.floor(remaining / 86400)
    const hours = Math.floor((remaining % 86400) / 3600)
    const human = days > 0 ? `${days}天${hours}小时` : `${hours}小时`
    if (!confirm(`⚠️ 当前账户还剩 ${human} 有效期。\n\n确认要清空到期时间（设为永不过期）吗？\n\n这通常会让用户失去已购买的天卡时间。`)) {
      return
    }
  }
  editForm.expiresAt = 0
}

const filteredKeys = computed(() => {
  if (!searchQuery.value) return keys.value
  const q = searchQuery.value.toLowerCase()
  return keys.value.filter(k =>
    k.note?.toLowerCase().includes(q) ||
    k.key?.toLowerCase().includes(q) ||
    k.id?.toLowerCase().includes(q)
  )
})

onMounted(loadKeys)
</script>

<template>
  <div class="apikeys-page">
    <!-- Header -->
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">API 密钥管理</div>
        <h1 class="page-title">Key 管理</h1>
      </div>
      <div class="head-actions">
        <WorldChip variant="info" size="sm">
          共 {{ keys.length }} 个 · {{ keys.filter(k => k.enabled).length }} 活跃
        </WorldChip>
        <WorldButton variant="primary" size="md" @click="showCreate = true">
          <Plus :size="14" /><span>创建 Key</span>
        </WorldButton>
      </div>
    </header>

    <!-- Search -->
    <WorldCard padding="md" class="search-card">
      <div class="search-wrap">
        <Search :size="14" class="search-icon" />
        <input
          v-model="searchQuery"
          class="search-input"
          placeholder="搜索备注、Key 或 ID"
          spellcheck="false"
        />
        <button v-if="searchQuery" @click="searchQuery = ''" class="clear-btn"><X :size="12" /></button>
      </div>
    </WorldCard>

    <!-- Key list -->
    <div class="keys-list">
      <WorldCard
        v-for="k in filteredKeys"
        :key="k.id"
        padding="none"
        class="key-card"
        :class="{ 'is-disabled': !k.enabled }"
      >
        <!-- Main row -->
        <div class="key-main" @click="toggleExpand(k)">
          <div class="key-icon"><Key :size="18" /></div>
          <div class="key-info">
            <div class="key-name">{{ k.note || (k.id || '').slice(0, 8) }}</div>
            <div class="key-meta">
              <code class="key-display">{{ showKeyId === k.id ? k.key : maskKey(k.key) }}</code>
              <button class="micro-btn" @click.stop="showKeyId = showKeyId === k.id ? null : k.id" :title="showKeyId === k.id ? '隐藏' : '显示'">
                <Eye v-if="showKeyId !== k.id" :size="12" />
                <EyeOff v-else :size="12" />
              </button>
              <button class="micro-btn" @click.stop="copyText(k.key)" title="复制">
                <Copy :size="12" />
              </button>
            </div>
          </div>
          <div class="key-quick">
            <WorldChip
              v-if="k.expiresAt"
              :variant="timeRemaining(k.expiresAt).variant"
              size="sm"
            >
              <Clock :size="11" />
              {{ timeRemaining(k.expiresAt).text }}
            </WorldChip>
            <WorldChip
              v-if="k.balance !== undefined && k.balance !== null"
              :variant="(k.balance || 0) < 1 ? 'danger' : 'success'"
              size="sm"
            >
              <Wallet :size="11" />
              ${{ (k.balance || 0).toFixed(2) }}
            </WorldChip>
            <WorldChip v-if="k.isReseller" variant="info" size="sm">代理</WorldChip>
            <WorldChip v-if="k.parentKeyId" variant="neutral" size="sm">子 Key</WorldChip>
            <WorldChip :variant="k.enabled ? 'success' : 'neutral'" size="sm" :dot="true">
              {{ k.enabled ? '启用' : '禁用' }}
            </WorldChip>
            <ChevronDown :size="14" class="key-expand-icon" :class="{ rotated: expandedId === k.id }" />
          </div>
        </div>

        <!-- Expanded -->
        <Transition name="expand">
          <div v-if="expandedId === k.id" class="key-expanded">
            <!-- Edit mode -->
            <div v-if="editingId === k.id" class="edit-grid">
              <WorldInput v-model="editForm.note" label="备注" />
              <WorldInput v-model.number="editForm.balance"     type="number" label="付费余额（¥）" />
              <WorldInput v-model.number="editForm.giftBalance" type="number" label="赠送余额（¥）" />
              <div class="time-block">
                <label class="time-label">到期时间</label>
                <div class="time-display">{{ expiresAtDisplay(editForm.expiresAt) }}</div>
                <div class="time-quick">
                  <button @click="addTime(86400)">+1 天</button>
                  <button @click="addTime(7 * 86400)">+7 天</button>
                  <button @click="addTime(30 * 86400)">+30 天</button>
                  <button @click="clearExpiresAt">永不过期</button>
                </div>
              </div>

              <!-- 代理设置（高级，仅非子 key 可开） -->
              <div v-if="!editForm.parentKeyId" class="reseller-block">
                <details>
                  <summary>代理设置（高级）</summary>
                  <label class="reseller-toggle">
                    <input type="checkbox" v-model="editForm.isReseller" />
                    <span>开通代理（让此 Key 能创建子 Key 分发给其他人）</span>
                  </label>
                  <div v-if="editForm.isReseller" class="reseller-fields">
                    <WorldInput v-model.number="editForm.maxChildKeys" type="number" label="子 Key 上限（0=无限）" />
                    <WorldInput v-model.number="editForm.resellerDiscountPercent" type="number" label="充值折扣率（%，100=无折扣）" />
                    <p class="reseller-hint">
                      💡 折扣 50% 时 reseller 充 ¥100 = 余额 ¥200（用于赚差价）。100% = 跟普通用户一样无折扣。
                    </p>
                  </div>
                </details>
              </div>
              <div v-else class="reseller-block child-note">
                ⓘ 此 Key 是子 Key（属于 reseller <code>{{ editForm.parentKeyId.slice(0, 8) }}</code>），不能再开代理。
              </div>

              <div class="edit-actions">
                <WorldButton variant="ghost" size="sm" @click="cancelEdit">
                  <X :size="13" /><span>取消</span>
                </WorldButton>
                <WorldButton variant="primary" size="sm" @click="saveEdit(k)">
                  <Save :size="13" /><span>保存</span>
                </WorldButton>
              </div>
            </div>
            <!-- View mode -->
            <div v-else class="info-grid">
              <div class="info-cell"><span class="info-label">Key ID</span><span class="info-val mono">{{ k.id }}</span></div>
              <div class="info-cell"><span class="info-label">创建时间</span><span class="info-val">{{ formatDate(k.createdAt) }}</span></div>
              <div class="info-cell"><span class="info-label">最后使用</span><span class="info-val">{{ k.lastUsed ? formatDate(k.lastUsed) : '从未使用' }}</span></div>
              <div class="info-cell"><span class="info-label">总请求</span><span class="info-val">{{ (k.requests || 0).toLocaleString() }}</span></div>
              <div class="info-cell"><span class="info-label">消耗 Credit</span><span class="info-val">{{ (k.credits || 0).toFixed(4) }}</span></div>
              <div class="info-cell"><span class="info-label">套餐</span><span class="info-val">{{ k.plan || '—' }}</span></div>
              <div class="info-cell"><span class="info-label">付费余额</span><span class="info-val">${{ (k.balance || 0).toFixed(4) }}</span></div>
              <div class="info-cell"><span class="info-label">赠送余额</span><span class="info-val">${{ (k.giftBalance || 0).toFixed(4) }}</span></div>
              <div class="info-cell"><span class="info-label">累计充值</span><span class="info-val">${{ (k.totalRecharged || 0).toFixed(2) }}</span></div>
              <div class="info-cell"><span class="info-label">累计赠送</span><span class="info-val">${{ (k.totalGifted || 0).toFixed(2) }}</span></div>
              <div class="info-cell"><span class="info-label">到期时间</span><span class="info-val">{{ k.expiresAt ? formatDate(k.expiresAt) : '永不过期' }}</span></div>

              <div v-if="k.isReseller" class="info-cell">
                <span class="info-label">代理身份</span>
                <span class="info-val">代理（折扣 {{ k.resellerDiscount > 0 && k.resellerDiscount < 1 ? Math.round(k.resellerDiscount * 100) : 100 }}% · 上限 {{ k.maxChildKeys || '∞' }}）</span>
              </div>
              <div v-if="k.parentKeyId" class="info-cell">
                <span class="info-label">所属代理</span>
                <span class="info-val mono">{{ k.parentKeyId.slice(0, 8) }}</span>
              </div>
              <div v-if="k.isReseller" class="info-cell">
                <span class="info-label">累计已售</span>
                <span class="info-val">${{ (k.soldToChildren || 0).toFixed(2) }}</span>
              </div>

              <div class="actions-row">
                <WorldButton variant="secondary" size="sm" @click="toggleKey(k)">
                  <span>{{ k.enabled ? '禁用' : '启用' }}</span>
                </WorldButton>
                <WorldButton variant="secondary" size="sm" @click="startEdit(k)">
                  <Pencil :size="13" /><span>编辑</span>
                </WorldButton>
                <WorldButton variant="danger" size="sm" @click="deleteKey(k)">
                  <Trash2 :size="13" /><span>删除</span>
                </WorldButton>
              </div>
            </div>
          </div>
        </Transition>
      </WorldCard>

      <WorldCard v-if="!loading && !filteredKeys.length" padding="lg">
        <div class="empty-row">
          <Key :size="32" />
          <span>{{ searchQuery ? '没有匹配的 Key' : '暂无 Key' }}</span>
        </div>
      </WorldCard>
    </div>

    <!-- Create modal -->
    <WorldModal v-model="showCreate" title="创建 API Key" size="md">
      <div class="create-body">
        <p class="hint">创建后需通过兑换激活码来充值余额或时间，才能开始使用。</p>
        <WorldInput
          v-model="form.note"
          label="备注（用户名 / 用途说明）"
          placeholder="user-001"
        />
      </div>
      <template #footer>
        <WorldButton variant="ghost" @click="showCreate = false">取消</WorldButton>
        <WorldButton variant="primary" @click="createKey">确认创建</WorldButton>
      </template>
    </WorldModal>
  </div>
</template>

<style scoped>
.apikeys-page { display: flex; flex-direction: column; gap: 14px; }

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
.head-actions { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }

.search-card { padding: 10px 14px; }
.search-wrap {
  position: relative;
  display: flex;
  align-items: center;
}
.search-icon {
  position: absolute;
  left: 12px;
  color: var(--world-text-mute);
}
.search-input {
  flex: 1;
  height: 34px;
  padding: 0 32px 0 36px;
  background: transparent;
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  color: var(--world-text-primary);
  font-size: 0.8125rem;
  outline: none;
  font-family: var(--world-font-sans);
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

.keys-list { display: flex; flex-direction: column; gap: 8px; }

.key-card { transition: all 220ms ease; }
.key-card.is-disabled { opacity: 0.6; }

.key-main {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  cursor: pointer;
  transition: background 200ms;
}
.key-main:hover {
  background: var(--world-overlay-light);
}

.key-icon {
  width: 38px; height: 38px;
  border-radius: var(--world-radius-md);
  background: var(--world-overlay-light);
  color: var(--world-accent);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.key-info { flex: 1; min-width: 0; }
.key-name {
  font-size: 0.875rem;
  font-weight: 800;
  color: var(--world-text-primary);
  margin-bottom: 4px;
}
.key-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--world-font-mono);
  font-size: 0.7rem;
  color: var(--world-text-mute);
}
.key-display {
  font-family: var(--world-font-mono);
  font-size: 0.7rem;
}
.micro-btn {
  width: 22px; height: 22px;
  border-radius: var(--world-radius-sm);
  background: transparent;
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 200ms ease;
}
.micro-btn:hover { color: var(--world-accent); border-color: var(--world-accent); }

.key-quick {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.key-expand-icon {
  color: var(--world-text-mute);
  transition: transform 240ms ease;
}
.key-expand-icon.rotated { transform: rotate(180deg); }

.key-expanded {
  border-top: 1px solid var(--world-divider);
  padding: 18px;
  background: var(--world-overlay-light);
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 14px;
}
.info-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.info-label {
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.info-val {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--world-text-primary);
}
.info-val.mono { font-family: var(--world-font-mono); font-size: 0.78rem; word-break: break-all; }
.actions-row {
  grid-column: 1 / -1;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  padding-top: 8px;
  border-top: 1px solid var(--world-divider);
}

.edit-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}
.edit-grid .time-block { grid-column: 1 / -1; }
.edit-grid .edit-actions { grid-column: 1 / -1; display: flex; gap: 10px; justify-content: flex-end; }

.time-label {
  display: block;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--world-text-mute);
  margin-bottom: 6px;
}
.time-display {
  font-family: var(--world-font-mono);
  font-size: 0.85rem;
  color: var(--world-text-primary);
  padding: 8px 12px;
  background: var(--world-bg-card);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  margin-bottom: 8px;
}
.time-quick {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.time-quick button {
  padding: 4px 10px;
  font-size: 0.72rem;
  font-weight: 700;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-sm);
  color: var(--world-text-mute);
  cursor: pointer;
  transition: all 200ms ease;
}
.time-quick button:hover { color: var(--world-accent); border-color: var(--world-accent); }

.empty-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 36px;
  color: var(--world-text-mute);
  font-size: 0.875rem;
}

.create-body { display: flex; flex-direction: column; gap: 12px; }
.hint {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
  line-height: 1.5;
}

.expand-enter-active, .expand-leave-active { transition: all 320ms ease; max-height: 600px; overflow: hidden; }
.expand-enter-from, .expand-leave-to { max-height: 0; opacity: 0; }

/* 代理设置 */
.reseller-block {
  grid-column: 1 / -1;
  padding: 10px 12px;
  background: var(--world-bg-card);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
}
.reseller-block details summary {
  cursor: pointer;
  font-size: 0.78rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  color: var(--world-text-mute);
  text-transform: uppercase;
  list-style: none;
  user-select: none;
  display: flex;
  align-items: center;
  gap: 6px;
}
.reseller-block details summary::before {
  content: "▶";
  font-size: 0.6rem;
  transition: transform 160ms ease;
}
.reseller-block details[open] summary::before { transform: rotate(90deg); }
.reseller-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 12px 0 8px;
  font-size: 0.85rem;
  color: var(--world-text-primary);
  cursor: pointer;
}
.reseller-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-top: 8px;
}
.reseller-hint {
  grid-column: 1 / -1;
  margin: 0;
  padding: 8px 10px;
  background: var(--world-overlay-light);
  border-radius: var(--world-radius-sm);
  font-size: 0.75rem;
  color: var(--world-text-mute);
  line-height: 1.5;
}
.reseller-block.child-note {
  font-size: 0.8125rem;
  color: var(--world-text-mute);
  padding: 12px;
}
.reseller-block.child-note code {
  font-family: var(--world-font-mono);
  background: var(--world-overlay-light);
  padding: 2px 6px;
  border-radius: 4px;
}
</style>
