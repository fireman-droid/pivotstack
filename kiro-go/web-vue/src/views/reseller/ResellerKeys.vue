<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { userApi } from '../../api/user'
import { useToast } from '../../composables/useToast'
import { copyToClipboard } from '../../utils/clipboard'
import {
  Plus, KeyRound, Wallet, Pencil, Trash2, Copy, Eye, EyeOff,
  Check, ChevronDown, X, Search,
} from 'lucide-vue-next'
import WorldCard from '../../components/world/WorldCard.vue'
import WorldButton from '../../components/world/WorldButton.vue'
import WorldChip from '../../components/world/WorldChip.vue'
import WorldInput from '../../components/world/WorldInput.vue'
import WorldModal from '../../components/world/WorldModal.vue'
import WorldLoader from '../../components/world/WorldLoader.vue'

const { success, error: toastErr } = useToast()

const summary = ref(null)
const keys = ref([])
const loading = ref(true)
const expandedId = ref(null)
const showKeyId = ref(null)
const searchQuery = ref('')

// 创建 modal
const showCreate = ref(false)
const createForm = reactive({ note: '', initialCNY: 0 })
const newCreatedKey = ref(null) // 创建后展示完整 key 一次

// 转账 modal
const showTransfer = ref(false)
const transferTarget = ref(null)
const transferForm = reactive({ amountCNY: 0 })

// 删除确认 modal
const showDelete = ref(false)
const deleteTarget = ref(null)

const CNY_PER_USD = 0.05  // 1$ face = ¥0.05

async function load() {
  try {
    const [s, k] = await Promise.all([
      userApi('/reseller/summary'),
      userApi('/reseller/keys'),
    ])
    summary.value = s
    keys.value = k.keys || []
  } catch (e) {
    toastErr('加载失败：' + e.message)
  }
  loading.value = false
}

async function createChildKey() {
  try {
    const initialUSD = (Number(createForm.initialCNY) || 0) / CNY_PER_USD
    const data = await userApi('/reseller/keys', {
      method: 'POST',
      body: { note: createForm.note, initialBalanceUSD: initialUSD },
    })
    newCreatedKey.value = data
    success('子 Key 已创建')
    createForm.note = ''
    createForm.initialCNY = 0
    await load()
  } catch (e) {
    toastErr('创建失败：' + e.message)
  }
}

function closeCreate() {
  showCreate.value = false
  newCreatedKey.value = null
}

function openTransfer(k) {
  transferTarget.value = k
  transferForm.amountCNY = 0
  showTransfer.value = true
}

async function submitTransfer() {
  try {
    const amountUSD = (Number(transferForm.amountCNY) || 0) / CNY_PER_USD
    if (amountUSD <= 0) {
      toastErr('请输入大于 0 的金额')
      return
    }
    await userApi('/reseller/transfer', {
      method: 'POST',
      body: { toKeyId: transferTarget.value.id, amountUSD },
    })
    success(`已转账 ¥${Number(transferForm.amountCNY).toFixed(2)}`)
    showTransfer.value = false
    await load()
  } catch (e) {
    toastErr('转账失败：' + e.message)
  }
}

async function toggleEnabled(k) {
  try {
    await userApi(`/reseller/keys/${k.id}`, {
      method: 'PATCH',
      body: { enabled: !k.enabled },
    })
    success(k.enabled ? '已禁用' : '已启用')
    await load()
  } catch (e) {
    toastErr('操作失败：' + e.message)
  }
}

function openDelete(k) {
  deleteTarget.value = k
  showDelete.value = true
}

async function confirmDelete() {
  try {
    const data = await userApi(`/reseller/keys/${deleteTarget.value.id}`, {
      method: 'DELETE',
    })
    const cny = (data.refundedUSD || 0) * CNY_PER_USD
    success(`已删除${cny > 0 ? `，退还 ¥${cny.toFixed(2)}` : ''}`)
    showDelete.value = false
    deleteTarget.value = null
    await load()
  } catch (e) {
    toastErr('删除失败：' + e.message)
  }
}

function copyText(t) { copyToClipboard(t); success('已复制') }
function toggleExpand(k) { expandedId.value = expandedId.value === k.id ? null : k.id }
function maskKey(k) { if (!k) return ''; return k.slice(0, 7) + '••••••••' + k.slice(-4) }
function formatDate(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN', { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' })
}

const filtered = computed(() => {
  if (!searchQuery.value) return keys.value
  const q = searchQuery.value.toLowerCase()
  return keys.value.filter(k =>
    k.note?.toLowerCase().includes(q) ||
    k.keyFull?.toLowerCase().includes(q) ||
    k.id?.toLowerCase().includes(q)
  )
})

const canCreate = computed(() => {
  if (!summary.value) return false
  const max = summary.value.maxChildKeys || 0
  if (max === 0) return true
  return (summary.value.childCount || 0) < max
})

const refundCNYPreview = computed(() => {
  if (!deleteTarget.value) return 0
  return (deleteTarget.value.totalBalance || 0) * CNY_PER_USD
})

onMounted(load)
</script>

<template>
  <div v-if="!loading" class="reseller-keys-page">
    <!-- 头部：余额 + 创建按钮 -->
    <header class="head-row">
      <div class="head-info">
        <WorldChip variant="info" size="md">
          <Wallet :size="13" />
          可用余额 ¥{{ ((summary?.totalBalance || 0) * 0.05).toFixed(2) }} (${{ (summary?.totalBalance || 0).toFixed(2) }})
        </WorldChip>
        <WorldChip variant="neutral" size="md">
          子 Key {{ summary?.childCount || 0 }} {{ summary?.maxChildKeys ? `/ ${summary.maxChildKeys}` : '' }}
        </WorldChip>
      </div>
      <WorldButton
        variant="primary"
        size="md"
        :disabled="!canCreate"
        @click="showCreate = true; newCreatedKey = null"
      >
        <Plus :size="14" />
        <span>创建子 Key</span>
      </WorldButton>
    </header>

    <!-- 搜索 -->
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

    <!-- 子 key 列表 -->
    <div class="keys-list">
      <WorldCard
        v-for="k in filtered"
        :key="k.id"
        padding="none"
        class="key-card"
        :class="{ 'is-disabled': !k.enabled }"
      >
        <div class="key-main" @click="toggleExpand(k)">
          <div class="key-icon"><KeyRound :size="18" /></div>
          <div class="key-info">
            <div class="key-name">{{ k.note || k.id.slice(0, 8) }}</div>
            <div class="key-meta">
              <code class="key-display">{{ showKeyId === k.id ? k.keyFull : maskKey(k.keyFull) }}</code>
              <button class="micro-btn" @click.stop="showKeyId = showKeyId === k.id ? null : k.id" :title="showKeyId === k.id ? '隐藏' : '显示'">
                <Eye v-if="showKeyId !== k.id" :size="12" />
                <EyeOff v-else :size="12" />
              </button>
              <button class="micro-btn" @click.stop="copyText(k.keyFull)" title="复制">
                <Copy :size="12" />
              </button>
            </div>
          </div>
          <div class="key-quick">
            <WorldChip
              :variant="(k.totalBalance || 0) < 1 ? 'danger' : 'success'"
              size="sm"
            >
              <Wallet :size="11" />
              ¥{{ ((k.totalBalance || 0) * 0.05).toFixed(2) }}
            </WorldChip>
            <WorldChip variant="neutral" size="sm">
              {{ k.recentCalls7d || 0 }} 次/7天
            </WorldChip>
            <WorldChip :variant="k.enabled ? 'success' : 'neutral'" size="sm" :dot="true">
              {{ k.enabled ? '启用' : '禁用' }}
            </WorldChip>
            <ChevronDown :size="14" class="key-expand-icon" :class="{ rotated: expandedId === k.id }" />
          </div>
        </div>

        <Transition name="expand">
          <div v-if="expandedId === k.id" class="key-expanded">
            <div class="info-grid">
              <div class="info-cell"><span class="info-label">Key ID</span><span class="info-val mono">{{ k.id }}</span></div>
              <div class="info-cell"><span class="info-label">创建时间</span><span class="info-val">{{ formatDate(k.createdAt) }}</span></div>
              <div class="info-cell"><span class="info-label">最后使用</span><span class="info-val">{{ k.lastUsed ? formatDate(k.lastUsed) : '从未使用' }}</span></div>
              <div class="info-cell"><span class="info-label">总请求</span><span class="info-val">{{ (k.requests || 0).toLocaleString() }}</span></div>
              <div class="info-cell"><span class="info-label">消耗 Credits</span><span class="info-val">{{ (k.credits || 0).toFixed(4) }}</span></div>
              <div class="info-cell"><span class="info-label">付费余额</span><span class="info-val">${{ (k.balance || 0).toFixed(4) }}</span></div>
              <div class="info-cell"><span class="info-label">赠送余额</span><span class="info-val">${{ (k.giftBalance || 0).toFixed(4) }}</span></div>
              <div class="info-cell"><span class="info-label">累计充值</span><span class="info-val">${{ (k.totalRecharged || 0).toFixed(2) }}</span></div>

              <div class="actions-row">
                <WorldButton variant="primary" size="sm" @click="openTransfer(k)">
                  <Wallet :size="13" /><span>充值</span>
                </WorldButton>
                <WorldButton variant="secondary" size="sm" @click="toggleEnabled(k)">
                  <span>{{ k.enabled ? '禁用' : '启用' }}</span>
                </WorldButton>
                <WorldButton variant="danger" size="sm" @click="openDelete(k)">
                  <Trash2 :size="13" /><span>删除</span>
                </WorldButton>
              </div>
            </div>
          </div>
        </Transition>
      </WorldCard>

      <WorldCard v-if="!filtered.length" padding="lg">
        <div class="empty-row">
          <KeyRound :size="32" />
          <span>{{ searchQuery ? '没有匹配的子 Key' : '还没有创建子 Key' }}</span>
        </div>
      </WorldCard>
    </div>

    <!-- 创建 modal -->
    <WorldModal v-model="showCreate" :title="newCreatedKey ? '创建成功' : '创建子 Key'" size="md">
      <div v-if="!newCreatedKey" class="create-body">
        <p class="hint">创建后可立即划账，余额将从你的代理账户扣除。</p>
        <WorldInput
          v-model="createForm.note"
          label="备注（用户名 / 用途）"
          placeholder="user-001"
        />
        <WorldInput
          v-model.number="createForm.initialCNY"
          type="number"
          label="初始充值（¥，可留 0 之后再充）"
          placeholder="0"
        />
        <p v-if="(createForm.initialCNY * 1) > (summary?.totalBalance || 0) * 0.05" class="warn-hint">
          ⚠️ 初始充值超过你的余额（¥{{ ((summary?.totalBalance || 0) * 0.05).toFixed(2) }}）
        </p>
      </div>
      <div v-else class="create-body">
        <p class="hint success">✅ 子 Key 已创建。请立即复制 Key 给真实用户：</p>
        <div class="key-result">
          <code>{{ newCreatedKey.key }}</code>
          <button class="copy-result-btn" @click="copyText(newCreatedKey.key)">
            <Copy :size="14" />
          </button>
        </div>
        <p class="hint">用户用此 Key 登录 <code>/user/login</code>，完全跟普通用户一样。</p>
      </div>
      <template #footer>
        <template v-if="!newCreatedKey">
          <WorldButton variant="ghost" @click="showCreate = false">取消</WorldButton>
          <WorldButton variant="primary" @click="createChildKey">确认创建</WorldButton>
        </template>
        <template v-else>
          <WorldButton variant="primary" @click="closeCreate">完成</WorldButton>
        </template>
      </template>
    </WorldModal>

    <!-- 转账 modal -->
    <WorldModal v-model="showTransfer" title="给子 Key 充值" size="md">
      <div class="create-body" v-if="transferTarget">
        <p class="hint">
          目标：<strong>{{ transferTarget.note || transferTarget.id.slice(0, 8) }}</strong>
          （当前余额 ¥{{ ((transferTarget.totalBalance || 0) * 0.05).toFixed(2) }}）
        </p>
        <WorldInput
          v-model.number="transferForm.amountCNY"
          type="number"
          label="充值金额（¥）"
          placeholder="50"
        />
        <p v-if="(transferForm.amountCNY * 1) > (summary?.totalBalance || 0) * 0.05" class="warn-hint">
          ⚠️ 超过你的余额（¥{{ ((summary?.totalBalance || 0) * 0.05).toFixed(2) }}）
        </p>
      </div>
      <template #footer>
        <WorldButton variant="ghost" @click="showTransfer = false">取消</WorldButton>
        <WorldButton variant="primary" @click="submitTransfer">确认充值</WorldButton>
      </template>
    </WorldModal>

    <!-- 删除确认 modal -->
    <WorldModal v-model="showDelete" title="确认删除子 Key" size="md">
      <div class="create-body" v-if="deleteTarget">
        <p class="hint">
          将删除子 Key <strong>{{ deleteTarget.note || deleteTarget.id.slice(0, 8) }}</strong>。
        </p>
        <p v-if="refundCNYPreview > 0" class="hint refund">
          💰 删除后，余额 <strong>¥{{ refundCNYPreview.toFixed(2) }}</strong> 将自动退回到你的代理余额。
        </p>
        <p class="warn-hint">
          ⚠️ 此操作不可撤销，子 Key 立即失效，所有调用记录将保留。
        </p>
      </div>
      <template #footer>
        <WorldButton variant="ghost" @click="showDelete = false">取消</WorldButton>
        <WorldButton variant="danger" @click="confirmDelete">确认删除</WorldButton>
      </template>
    </WorldModal>
  </div>

  <div v-else class="loading-wrap">
    <WorldLoader :size="48" label="载入数据中" />
  </div>
</template>

<style scoped>
.reseller-keys-page { display: flex; flex-direction: column; gap: 14px; }

.head-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.head-info { display: flex; gap: 8px; flex-wrap: wrap; }

.search-card { padding: 10px 14px; }
.search-wrap { position: relative; display: flex; align-items: center; }
.search-icon { position: absolute; left: 12px; color: var(--world-text-mute); }
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
.key-card.is-disabled { opacity: 0.6; }

.key-main {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  cursor: pointer;
  transition: background 200ms;
}
.key-main:hover { background: var(--world-overlay-light); }

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
.key-display { font-family: var(--world-font-mono); font-size: 0.7rem; }
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

.key-quick { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
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
.info-cell { display: flex; flex-direction: column; gap: 3px; }
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

.create-body { display: flex; flex-direction: column; gap: 12px; }
.hint {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
  line-height: 1.5;
}
.hint.success { color: var(--world-success); font-weight: 700; }
.hint.refund { color: var(--world-success); }
.warn-hint {
  margin: 0;
  padding: 8px 10px;
  background: rgba(245, 158, 11, 0.10);
  border-radius: var(--world-radius-sm);
  font-size: 0.78rem;
  color: var(--world-warning);
  line-height: 1.5;
}

.key-result {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  font-family: var(--world-font-mono);
}
.key-result code { flex: 1; font-size: 0.78rem; word-break: break-all; }
.copy-result-btn {
  width: 28px; height: 28px;
  border-radius: var(--world-radius-sm);
  background: transparent;
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-mute);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.copy-result-btn:hover { color: var(--world-accent); border-color: var(--world-accent); }

.empty-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 36px;
  color: var(--world-text-mute);
  font-size: 0.875rem;
}

.expand-enter-active, .expand-leave-active {
  transition: all 320ms ease;
  max-height: 600px;
  overflow: hidden;
}
.expand-enter-from, .expand-leave-to { max-height: 0; opacity: 0; }

.loading-wrap {
  min-height: 50vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
