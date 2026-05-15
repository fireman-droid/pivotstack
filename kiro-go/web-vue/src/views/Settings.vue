<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/admin'
import { useAuthStore } from '../stores/auth'
import { useToast } from '../composables/useToast'
import { usePasswordStrength } from '../composables/usePasswordStrength'
import {
  Key, Wand2, ShieldAlert, Lock, Cpu, Route, Save, Zap, Check, X, Copy, Shield,
  RefreshCw, FileX, DollarSign, XCircle, Trophy, Gift
} from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldPasswordInput from '../components/world/WorldPasswordInput.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldSelect from '../components/world/WorldSelect.vue'
import WorldModal from '../components/world/WorldModal.vue'
import Switch from '../components/ui/Switch.vue'

const openaiFormatOptions = [
  { value: 'reasoning_content', label: 'reasoning_content（标准）' },
  { value: 'thinking',          label: '<thinking> 标签（Claude 风格）' },
  { value: 'think',             label: '<think> 标签（OpenAI 风格）' },
]
const claudeFormatOptions = [
  { value: 'thinking',          label: '<thinking> 标签（Claude 风格）' },
  { value: 'think',             label: '<think> 标签（OpenAI 风格）' },
  { value: 'reasoning_content', label: '直接明文输出（不带标签）' },
]
const endpointOptions = [
  { value: 'auto',          label: '自动智能负载（推荐）' },
  { value: 'codewhisperer', label: 'Amazon CodeWhisperer Node' },
  { value: 'amazonq',       label: 'Amazon Q Business Node' },
]

const { success, error } = useToast()
const router = useRouter()

const requireApiKey = ref(false)
const apiKey = ref('')
const thinkingSuffix = ref('-thinking')
const openaiFormat = ref('reasoning_content')
const claudeFormat = ref('thinking')
const preferredEndpoint = ref('auto')
// === 修改密码表单（v2：旧密码 + 新密码 + 确认 + 强度 + 生成器） ===
const pwdForm = ref({ oldPassword: '', newPassword: '', confirmPassword: '' })
const pwdErrors = ref({ old: '', confirm: '' })
const pwdCopied = ref(false)
const { strength, rules, strengthText, strengthColor, canSubmit: pwdStrengthOK } =
  usePasswordStrength(computed(() => pwdForm.value.newPassword))
const canSubmitPwd = computed(() =>
  pwdForm.value.oldPassword.length > 0 &&
  pwdStrengthOK.value &&
  pwdForm.value.newPassword === pwdForm.value.confirmPassword
)
const maxConcurrentPerKey = ref(20)
const maxInFlightPerAccountFree = ref(50)
const maxInFlightPerAccountPro = ref(50)
const timedKeyRPM = ref(10) // 天卡防共享速率（仅天卡有效期内的 key 受此约束）
const loading = ref({ api: false, thinking: false, endpoint: false, pwd: false, concurrency: false, leaderboard: false })

// Abuse monitor
const flagged = ref([])
const abuseLoading = ref(false)

const tab = ref('general')
const tabOptions = [
  { value: 'general',     label: '常规' },
  { value: 'thinking',    label: '思考模式' },
  { value: 'routing',     label: '路由 / 并发' },
  { value: 'leaderboard', label: '排行榜' },
  { value: 'abuse',       label: '滥用监控' },
  { value: 'danger',      label: '危险操作' },
]

// === 排行榜配置 ===
const leaderboardEnabled = ref(false)
const leaderboardFakeUsers = ref(0)

async function loadLeaderboardConfig() {
  try {
    const res = await api('/leaderboard/config')
    if (res.ok) {
      const d = await res.json()
      leaderboardEnabled.value = !!d.enabled
      leaderboardFakeUsers.value = d.fakeUsers || 0
    }
  } catch {}
}
async function saveLeaderboardConfig() {
  loading.value.leaderboard = true
  const res = await api('/leaderboard/config', {
    method: 'PUT',
    body: JSON.stringify({
      enabled: leaderboardEnabled.value,
      fakeUsers: Number(leaderboardFakeUsers.value) || 0,
    }),
  })
  res.ok ? success('排行榜配置已保存') : error('保存失败')
  loading.value.leaderboard = false
}

// === 一键清赠金 ===
const clearGiftOpen = ref(false)
const clearGiftConfirm = ref('')
const clearGiftLoading = ref(false)
async function doClearAllGift() {
  if (clearGiftConfirm.value.trim() !== '清除') return
  clearGiftLoading.value = true
  try {
    const res = await api('/apikeys/clear-gift', {
      method: 'POST',
      body: JSON.stringify({ confirm: true }),
    })
    if (res.ok) {
      const d = await res.json()
      success(`已清除 ${d.cleared} 个 Key 的赠金，总计 $${(d.totalGiftCleared || 0).toFixed(2)}`)
      clearGiftOpen.value = false
      clearGiftConfirm.value = ''
    } else {
      error('清除失败')
    }
  } catch {
    error('清除失败')
  }
  clearGiftLoading.value = false
}

onMounted(async () => {
  try {
    const [s, t, e] = await Promise.all([api('/settings'), api('/thinking'), api('/endpoint')])
    if (s.ok) {
      const d = await s.json()
      requireApiKey.value = d.requireApiKey
      apiKey.value = d.apiKey || ''
    }
    if (t.ok) {
      const d = await t.json()
      thinkingSuffix.value = d.suffix || '-thinking'
      openaiFormat.value = d.openaiFormat || 'reasoning_content'
      claudeFormat.value = d.claudeFormat || 'thinking'
    }
    if (e.ok) {
      preferredEndpoint.value = (await e.json()).preferredEndpoint || 'auto'
    }
    const c = await api('/concurrency')
    if (c.ok) {
      const d = await c.json()
      maxConcurrentPerKey.value = d.maxConcurrentPerKey || 20
      maxInFlightPerAccountFree.value = d.maxInFlightPerAccountFree || d.maxInFlightPerAccount || 50
      maxInFlightPerAccountPro.value = d.maxInFlightPerAccountPro || d.maxInFlightPerAccount || 50
      // timedKeyRPM：0 = 走老兜底；前端 ref 默认 10 但若服务返回了具体值就用服务的（包括 0）
      if (typeof d.timedKeyRPM === 'number') timedKeyRPM.value = d.timedKeyRPM
    }
    loadFlagged()
    loadLeaderboardConfig()
  } catch {}
})

function generateApiKey() {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
  let key = 'sk-'
  for (let i = 0; i < 32; i++) key += chars.charAt(Math.floor(Math.random() * chars.length))
  apiKey.value = key
}

async function saveApiSettings() {
  loading.value.api = true
  if (requireApiKey.value && !apiKey.value.trim()) generateApiKey()
  const res = await api('/settings', { method: 'POST', body: JSON.stringify({ requireApiKey: requireApiKey.value, apiKey: apiKey.value }) })
  res.ok ? success('API 设置已保存') : error('保存失败')
  loading.value.api = false
}

async function saveThinking() {
  loading.value.thinking = true
  const res = await api('/thinking', { method: 'POST', body: JSON.stringify({ suffix: thinkingSuffix.value, openaiFormat: openaiFormat.value, claudeFormat: claudeFormat.value }) })
  res.ok ? success('Thinking 设置已保存') : error('保存失败')
  loading.value.thinking = false
}

async function saveEndpoint() {
  loading.value.endpoint = true
  const res = await api('/endpoint', { method: 'POST', body: JSON.stringify({ preferredEndpoint: preferredEndpoint.value }) })
  res.ok ? success('端点设置已保存') : error('保存失败')
  loading.value.endpoint = false
}

async function saveConcurrency() {
  loading.value.concurrency = true
  const res = await api('/concurrency', { method: 'POST', body: JSON.stringify({
    maxConcurrentPerKey: maxConcurrentPerKey.value,
    maxInFlightPerAccountFree: maxInFlightPerAccountFree.value,
    maxInFlightPerAccountPro: maxInFlightPerAccountPro.value,
    timedKeyRPM: Number(timedKeyRPM.value) || 0,
  }) })
  res.ok ? success('并发设置已保存') : error('保存失败')
  loading.value.concurrency = false
}

function generateStrongPassword() {
  // 20 位密码字符集（排除易混的 0/O/1/l/I）
  const charset = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%^&*'
  const len = 20
  const arr = new Uint32Array(len)
  crypto.getRandomValues(arr)
  const out = Array.from(arr, x => charset[x % charset.length]).join('')
  pwdForm.value.newPassword = out
  pwdForm.value.confirmPassword = out
  success('已生成 20 位强密码，建议立即复制保存')
}

async function copyNewPassword() {
  if (!pwdForm.value.newPassword) return
  try {
    await navigator.clipboard.writeText(pwdForm.value.newPassword)
    pwdCopied.value = true
    success('已复制，2 秒后将清空剪贴板')
    setTimeout(() => {
      pwdCopied.value = false
      navigator.clipboard.writeText('').catch(() => {})
    }, 2000)
  } catch {
    error('复制失败，请手动复制')
  }
}

async function changePassword() {
  pwdErrors.value = { old: '', confirm: '' }
  if (!pwdForm.value.oldPassword) {
    pwdErrors.value.old = '请输入当前密码'
    return
  }
  if (pwdForm.value.newPassword !== pwdForm.value.confirmPassword) {
    pwdErrors.value.confirm = '两次输入的新密码不一致'
    return
  }
  if (!pwdStrengthOK.value) {
    return error('新密码强度不足（长度需 ≥ 12 且达到「中」强度以上）')
  }
  loading.value.pwd = true
  try {
    await api('/password', { method: 'POST', body: JSON.stringify({
      oldPassword: pwdForm.value.oldPassword,
      newPassword: pwdForm.value.newPassword,
      confirmPassword: pwdForm.value.confirmPassword,
    }) })
    success('密码已修改，所有设备需要重新登录')
    pwdForm.value = { oldPassword: '', newPassword: '', confirmPassword: '' }
    const auth = useAuthStore()
    auth.clearLocal()
    setTimeout(() => router.push('/login'), 1200)
  } catch (e) {
    const msg = e && e.message ? e.message : '修改失败'
    if (msg.includes('invalid old')) {
      pwdErrors.value.old = '当前密码不正确'
    } else {
      error(msg)
    }
  } finally {
    loading.value.pwd = false
  }
}

async function resetStats() {
  if (!confirm('确定彻底重置全局统计？此操作将清空所有累积数据且不可恢复。')) return
  await api('/stats/reset', { method: 'POST' })
  success('统计已重置')
  setTimeout(() => location.reload(), 1000)
}

async function clearLogs() {
  if (!confirm('确定清空所有调用日志？\n\n• API Key 和用户余额不受影响\n• 此操作不可恢复')) return
  const res = await api('/logs', { method: 'DELETE' })
  res.ok ? success('调用日志已清空') : error('清空日志失败')
}

async function resetPricing() {
  if (!confirm('确定重置采购记录？\n\n• 将清空所有 PRO/FREE 号采购记录\n• 售价配置不受影响\n• 此操作不可恢复')) return
  const get = await api('/pricing')
  if (!get.ok) { error('获取配置失败'); return }
  const pricing = await get.json()
  pricing.proCostEntries = []
  pricing.freeCostEntries = []
  const res = await api('/pricing', { method: 'PUT', body: JSON.stringify(pricing) })
  res.ok ? success('采购记录已清空') : error('重置失败')
}

async function loadFlagged() {
  abuseLoading.value = true
  try {
    const res = await api('/abuse')
    if (res.ok) flagged.value = await res.json()
  } catch {}
  abuseLoading.value = false
}
async function clearFlag(keyId) {
  try {
    await api(`/abuse/${keyId}/clear`, { method: 'POST' })
    flagged.value = flagged.value.filter(f => f.keyId !== keyId)
    success('已清除标记')
  } catch { error('清除失败') }
}
</script>

<template>
  <div class="settings-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">系统配置</div>
        <h1 class="page-title">系统设置</h1>
      </div>
      <WorldSegment v-model="tab" :options="tabOptions" />
    </header>

    <!-- 常规：API 鉴权 + 密码 -->
    <template v-if="tab === 'general'">
      <WorldCard padding="lg">
        <header class="section-head">
          <h3><Key :size="16" /><span>API 鉴权</span></h3>
        </header>
        <div class="row-block">
          <div class="row">
            <div class="row-text">
              <div class="row-title">启用全局 API Key 强制验证</div>
              <div class="row-hint">未提供 API Key 的请求将被拒绝</div>
            </div>
            <Switch v-model="requireApiKey" />
          </div>
          <WorldInput
            v-model="apiKey"
            label="全局 API Key"
            :monospace="true"
            placeholder="sk-..."
            hint="留空则会自动生成"
          />
          <div class="row-actions">
            <WorldButton variant="secondary" size="sm" @click="generateApiKey">生成新 Key</WorldButton>
            <WorldButton variant="primary" size="md" :loading="loading.api" @click="saveApiSettings">
              <Save :size="14" /><span>保存</span>
            </WorldButton>
          </div>
        </div>
      </WorldCard>

      <WorldCard padding="lg">
        <header class="section-head">
          <h3><Lock :size="16" /><span>修改管理密码</span></h3>
          <p class="section-hint">改密成功后会强制所有设备重新登录</p>
        </header>

        <div class="pwd-form">
          <WorldPasswordInput
            v-model="pwdForm.oldPassword"
            label="当前密码"
            placeholder="请输入当前密码"
            :error="pwdErrors.old"
          />

          <WorldPasswordInput
            v-model="pwdForm.newPassword"
            label="新密码"
            placeholder="至少 12 位，建议混合大小写 / 数字 / 特殊符号"
          />

          <div class="pwd-strength" aria-live="polite">
            <div class="strength-bars">
              <div
                v-for="i in 5"
                :key="i"
                class="seg"
                :class="{ active: strength >= i }"
                :style="strength >= i ? { background: strengthColor } : {}"
              />
            </div>
            <span class="strength-label" :style="{ color: strengthColor }">{{ strengthText }}</span>
          </div>

          <ul class="pwd-rules">
            <li v-for="rule in rules" :key="rule.label" :class="{ ok: rule.met }">
              <Check v-if="rule.met" :size="12" /><X v-else :size="12" />
              <span>{{ rule.label }}</span>
            </li>
          </ul>

          <WorldPasswordInput
            v-model="pwdForm.confirmPassword"
            label="确认新密码"
            placeholder="再次输入新密码"
            :error="pwdErrors.confirm"
          />

          <div class="pwd-actions">
            <WorldButton variant="secondary" size="sm" @click="generateStrongPassword">
              <Shield :size="14" /><span>生成 20 位强密码</span>
            </WorldButton>
            <WorldButton
              variant="secondary"
              size="sm"
              :disabled="!pwdForm.newPassword"
              @click="copyNewPassword"
            >
              <Check v-if="pwdCopied" :size="14" />
              <Copy v-else :size="14" />
              <span>{{ pwdCopied ? '已复制' : '复制新密码' }}</span>
            </WorldButton>
            <WorldButton
              variant="primary"
              :loading="loading.pwd"
              :disabled="!canSubmitPwd"
              @click="changePassword"
            >
              <Save :size="14" /><span>确认重置密码</span>
            </WorldButton>
          </div>
        </div>
      </WorldCard>
    </template>

    <!-- 思考模式 -->
    <template v-if="tab === 'thinking'">
      <WorldCard padding="lg">
        <header class="section-head">
          <h3><Wand2 :size="16" /><span>Thinking 思考模式</span></h3>
        </header>
        <div class="row-block">
          <WorldInput
            v-model="thinkingSuffix"
            label="触发后缀"
            placeholder="-thinking"
            :monospace="true"
            hint="在请求模型名后添加此后缀将强制激活思考路径"
          />
          <div class="cfg-grid">
            <div class="select-cell">
              <label class="select-label">OpenAI 协议响应格式</label>
              <WorldSelect v-model="openaiFormat" :options="openaiFormatOptions" size="md" />
            </div>
            <div class="select-cell">
              <label class="select-label">Claude 协议响应格式</label>
              <WorldSelect v-model="claudeFormat" :options="claudeFormatOptions" size="md" />
            </div>
          </div>
          <div class="row-actions">
            <WorldButton variant="primary" :loading="loading.thinking" @click="saveThinking">
              <Save :size="14" /><span>保存配置</span>
            </WorldButton>
          </div>
        </div>
      </WorldCard>
    </template>

    <!-- 路由 / 并发 -->
    <template v-if="tab === 'routing'">
      <WorldCard padding="lg">
        <header class="section-head">
          <h3><Route :size="16" /><span>端点路由</span></h3>
        </header>
        <div class="row-block">
          <div class="select-cell">
            <label class="select-label">首选连接节点</label>
            <WorldSelect v-model="preferredEndpoint" :options="endpointOptions" size="md" />
          </div>
          <div class="row-actions">
            <WorldButton variant="primary" :loading="loading.endpoint" @click="saveEndpoint">
              <Save :size="14" /><span>保存路由</span>
            </WorldButton>
          </div>
        </div>
      </WorldCard>

      <WorldCard padding="lg">
        <header class="section-head">
          <h3><Zap :size="16" /><span>并发控制</span></h3>
        </header>
        <div class="row-block">
          <WorldInput v-model.number="maxConcurrentPerKey" type="number"
                      label="单 Key 最大并发流"
                      hint="每个 API Key 同时允许的最大并发请求数（默认 20）" />
          <WorldInput v-model.number="maxInFlightPerAccountFree" type="number"
                      label="FREE 账号最大并发"
                      hint="免费号池中每个账号同时处理的最大请求数（默认 50）" />
          <WorldInput v-model.number="maxInFlightPerAccountPro" type="number"
                      label="PRO 账号最大并发"
                      hint="付费号池中每个账号同时处理的最大请求数（默认 50）" />
          <WorldInput v-model.number="timedKeyRPM" type="number"
                      label="天卡 / 时间卡 速率上限（次/分钟）"
                      hint="只对 plan=timed/hybrid 且未过期的 key 生效。默认 10。设值越低越能劝退分发（N 人共享一张卡时人均配额变低）。0 = 不限速（走老兜底 200/min）。credit 用户与已过期 key 不受影响。" />
          <div class="row-actions">
            <WorldButton variant="primary" :loading="loading.concurrency" @click="saveConcurrency">
              <Save :size="14" /><span>保存并发配置</span>
            </WorldButton>
          </div>
        </div>
      </WorldCard>
    </template>

    <!-- 排行榜配置 -->
    <template v-if="tab === 'leaderboard'">
      <WorldCard padding="lg">
        <header class="section-head">
          <h3><Trophy :size="16" /><span>排行榜</span></h3>
        </header>
        <div class="row-block">
          <div class="row">
            <div class="row-text">
              <div class="row-title">向用户展示排行榜</div>
              <div class="row-hint">
                关闭后用户访问 /user/leaderboard 会返回 404；管理员视图始终可用。
              </div>
            </div>
            <Switch v-model="leaderboardEnabled" />
          </div>
          <WorldInput
            v-model.number="leaderboardFakeUsers"
            type="number"
            label="虚拟用户数量"
            placeholder="0"
            hint="混入用户视图的虚拟条目数（0-30），用于活跃度引导。每个 UTC 日稳定一份名单。"
          />
          <div class="row-actions">
            <WorldButton variant="primary" :loading="loading.leaderboard" @click="saveLeaderboardConfig">
              <Save :size="14" /><span>保存配置</span>
            </WorldButton>
          </div>
        </div>
      </WorldCard>
    </template>

    <!-- 滥用监控 -->
    <template v-if="tab === 'abuse'">
      <WorldCard padding="lg">
        <header class="section-head">
          <h3><ShieldAlert :size="16" /><span>滥用监控</span></h3>
          <WorldButton variant="secondary" size="sm" @click="loadFlagged">
            <RefreshCw :size="13" /><span>刷新</span>
          </WorldButton>
        </header>
        <p class="section-hint">被标记的异常 API Key（IP多样性过高、并发异常等）</p>

        <div v-if="abuseLoading" class="empty-row">载入中…</div>
        <div v-else-if="!flagged.length" class="empty-row">
          <ShieldAlert :size="32" />
          <span>当前没有被标记的 Key</span>
        </div>
        <div v-else class="flagged-list">
          <div v-for="item in flagged" :key="item.keyId" class="flagged-item">
            <div class="flagged-head">
              <div class="flagged-info">
                <ShieldAlert :size="16" class="warn-icon" />
                <div>
                  <div class="flagged-id">{{ item.keyId }}</div>
                  <div class="flagged-reason">{{ item.reason || '异常行为' }}</div>
                </div>
              </div>
              <WorldButton variant="secondary" size="sm" @click="clearFlag(item.keyId)">
                <XCircle :size="13" /><span>清除标记</span>
              </WorldButton>
            </div>
            <div class="flagged-grid">
              <div class="fg-cell"><span>活跃流</span><strong>{{ item.activeStreams || 0 }}</strong></div>
              <div class="fg-cell"><span>IP 数</span><strong :class="{ warn: (item.distinctIPs || 0) > 10 }">{{ item.distinctIPs || 0 }}</strong></div>
              <div class="fg-cell"><span>近期请求</span><strong>{{ item.recentRequests || 0 }}</strong></div>
              <div class="fg-cell"><span>标记时间</span><strong class="time">{{ item.flaggedAt ? new Date(item.flaggedAt).toLocaleString('zh-CN') : '-' }}</strong></div>
            </div>
          </div>
        </div>
      </WorldCard>
    </template>

    <!-- 危险操作 -->
    <template v-if="tab === 'danger'">
      <WorldCard padding="lg" class="danger-card">
        <header class="section-head">
          <h3 class="danger-title"><ShieldAlert :size="16" /><span>危险操作区</span></h3>
        </header>
        <div class="danger-list">
          <div class="danger-item">
            <div class="d-info">
              <FileX :size="20" class="d-icon warn" />
              <div>
                <div class="d-title">清空调用日志</div>
                <div class="d-desc">清除所有历史 API 调用记录。API Key 和用户余额不受影响。</div>
              </div>
            </div>
            <WorldButton variant="danger" size="sm" @click="clearLogs">清空日志</WorldButton>
          </div>
          <div class="danger-item">
            <div class="d-info">
              <DollarSign :size="20" class="d-icon warn" />
              <div>
                <div class="d-title">重置采购记录</div>
                <div class="d-desc">清空所有 PRO/FREE 号采购成本记录。售价配置不受影响。</div>
              </div>
            </div>
            <WorldButton variant="danger" size="sm" @click="resetPricing">重置采购记录</WorldButton>
          </div>
          <div class="danger-item">
            <div class="d-info">
              <Cpu :size="20" class="d-icon" />
              <div>
                <div class="d-title">重置全局统计</div>
                <div class="d-desc">清空所有历史请求数、Token 消耗及成本统计。此操作不可逆。</div>
              </div>
            </div>
            <WorldButton variant="danger" size="sm" @click="resetStats">立即执行重置</WorldButton>
          </div>
          <div class="danger-item">
            <div class="d-info">
              <Gift :size="20" class="d-icon" />
              <div>
                <div class="d-title">一键清除所有赠金</div>
                <div class="d-desc">
                  将所有 API Key 的 <strong>GiftBalance</strong> 归零（仅清赠金，<strong>不动</strong>付费余额与累计赠送记录）。
                  执行前需要二次确认。此操作不可逆。
                </div>
              </div>
            </div>
            <WorldButton variant="danger" size="sm" @click="clearGiftOpen = true">一键清赠金</WorldButton>
          </div>
        </div>
      </WorldCard>
    </template>

    <!-- 一键清赠金确认 Modal -->
    <WorldModal v-model="clearGiftOpen" title="确认清除所有赠金" size="sm">
      <div class="modal-body">
        <p class="modal-warn">
          此操作会把所有 API Key 的赠金（GiftBalance）置为 0。
          已扣费的付费余额（Balance）与历史累计赠送（TotalGifted）<strong>不受影响</strong>。
        </p>
        <p class="modal-warn">操作不可逆。请输入「<strong>清除</strong>」以确认：</p>
        <WorldInput
          v-model="clearGiftConfirm"
          placeholder="输入：清除"
          :monospace="false"
        />
      </div>
      <template #footer>
        <WorldButton variant="secondary" @click="clearGiftOpen = false; clearGiftConfirm = ''">取消</WorldButton>
        <WorldButton
          variant="danger"
          :loading="clearGiftLoading"
          :disabled="clearGiftConfirm.trim() !== '清除'"
          @click="doClearAllGift"
        >
          确认执行
        </WorldButton>
      </template>
    </WorldModal>
  </div>
</template>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 16px; }

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
  margin: 0;
  color: var(--world-text-primary);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.section-head h3 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 0.95rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
.section-hint {
  margin: 0 0 14px;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
}

.row-block { display: flex; flex-direction: column; gap: 14px; }
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.row-text .row-title { font-size: 0.875rem; font-weight: 700; color: var(--world-text-primary); margin-bottom: 2px; }
.row-text .row-hint { font-size: 0.75rem; color: var(--world-text-mute); }
.row-actions { display: flex; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }

.cfg-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
@media (max-width: 720px) { .cfg-grid { grid-template-columns: 1fr; } }

.select-label {
  display: block;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--world-text-mute);
  margin-bottom: 6px;
}
.select-cell { display: flex; flex-direction: column; }
.select-cell .world-select { width: 100%; display: flex; }
.select-cell :deep(.ws-trigger) { width: 100%; }

.empty-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 40px 20px;
  color: var(--world-text-mute);
  font-size: 0.875rem;
}

.flagged-list { display: flex; flex-direction: column; gap: 10px; }
.flagged-item {
  padding: 14px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
}
.flagged-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.flagged-info { display: flex; align-items: center; gap: 12px; }
.flagged-info .warn-icon { color: var(--world-warning); }
.flagged-id {
  font-family: var(--world-font-mono);
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--world-text-primary);
}
.flagged-reason {
  font-size: 0.7rem;
  color: var(--world-text-mute);
  margin-top: 2px;
}
.flagged-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}
@media (max-width: 720px) { .flagged-grid { grid-template-columns: repeat(2, 1fr); } }
.fg-cell {
  padding: 8px 10px;
  background: var(--world-bg-card);
  border-radius: var(--world-radius-sm);
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.fg-cell span {
  font-size: 0.6rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--world-text-mute);
}
.fg-cell strong {
  font-size: 0.85rem;
  font-weight: 800;
  color: var(--world-text-primary);
}
.fg-cell strong.warn { color: var(--world-warning); }
.fg-cell strong.time { font-size: 0.72rem; font-family: var(--world-font-mono); }

/* Danger zone */
.danger-card { border-color: rgba(239, 68, 68, 0.30); }
.danger-title { color: var(--world-error); }
.danger-list { display: flex; flex-direction: column; gap: 12px; }
.danger-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
  background: rgba(239, 68, 68, 0.04);
  border: 1px solid rgba(239, 68, 68, 0.20);
  border-radius: var(--world-radius-md);
}
.d-info { display: flex; align-items: flex-start; gap: 12px; flex: 1; }
.d-icon { color: var(--world-error); flex-shrink: 0; margin-top: 2px; }
.d-icon.warn { color: var(--world-warning); }
.d-title { font-size: 0.875rem; font-weight: 800; color: var(--world-text-primary); }
.d-desc { font-size: 0.75rem; color: var(--world-text-mute); margin-top: 2px; line-height: 1.5; }
@media (max-width: 720px) {
  .danger-item { flex-direction: column; align-items: stretch; }
}

/* Modal body */
.modal-body { display: flex; flex-direction: column; gap: 12px; }
.modal-warn {
  margin: 0;
  font-size: 0.875rem;
  color: var(--world-text-primary);
  line-height: 1.6;
}
.modal-warn strong { color: var(--world-error); }

/* === 修改密码块 === */
.section-hint {
  margin: 4px 0 0;
  font-size: 0.72rem;
  color: var(--world-text-mute, #94a3b8);
}
.pwd-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.pwd-strength {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: -6px;
}
.strength-bars {
  display: flex;
  gap: 4px;
  flex: 1;
}
.strength-bars .seg {
  height: 4px;
  flex: 1;
  background: var(--world-divider, rgba(148, 163, 184, 0.2));
  border-radius: 2px;
  transition: background 220ms;
}
.strength-label {
  font-size: 0.75rem;
  font-weight: 700;
  min-width: 36px;
  text-align: right;
}
.pwd-rules {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 16px;
}
.pwd-rules li {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.72rem;
  color: var(--world-text-mute, #94a3b8);
}
.pwd-rules li.ok {
  color: var(--world-success, #10b981);
  font-weight: 600;
}
.pwd-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}
@media (max-width: 520px) {
  .pwd-rules { grid-template-columns: 1fr; }
  .pwd-actions { justify-content: stretch; }
  .pwd-actions > * { flex: 1 1 100%; }
}
</style>
