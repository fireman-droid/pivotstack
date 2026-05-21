<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUserAuth } from '../stores/userAuth'
import { Eye, EyeOff, RotateCw } from 'lucide-vue-next'
import logoUrl from '../assets/pivotstack-logo.png'

const router = useRouter()
const auth = useAuthStore()
const userAuth = useUserAuth() as any

const tab = ref<'key' | 'user'>('user')
const userMode = ref<'login' | 'register'>('login')
const showKey = ref(false)
const showPwd = ref(false)

const keyInput = ref('')
const userInput = ref({ username: '', password: '' })
const registerInput = ref({ email: '', password: '', activationCode: '' })

const error = ref('')
const loading = ref(false)
const cardVisible = ref(true)

const lockedUntil = ref(0)
const tickNow = ref(Date.now())
const lockCountdown = computed(() =>
  lockedUntil.value ? Math.max(0, Math.ceil((lockedUntil.value - tickNow.value) / 1000)) : 0,
)
let tickTimer: ReturnType<typeof setInterval> | null = null
function startTick() {
  if (tickTimer) return
  tickTimer = setInterval(() => {
    tickNow.value = Date.now()
    if (lockCountdown.value <= 0) {
      lockedUntil.value = 0
      if (tickTimer) { clearInterval(tickTimer); tickTimer = null }
    }
  }, 500)
}

/* ───────── Warp Tunnel canvas ───────── */
const canvasRef = ref<HTMLCanvasElement | null>(null)
type Star = { a: number; r: number; speed: number; maxR: number }
let stars: Star[] = []
let raf = 0
let phase: 'warp' | 'idle' = 'warp'
let phaseStart = 0
const TOTAL_STARS = 480

function fit() {
  const c = canvasRef.value
  if (!c) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  const w = window.innerWidth, h = window.innerHeight
  c.width = w * dpr
  c.height = h * dpr
  c.style.width = w + 'px'
  c.style.height = h + 'px'
  const ctx = c.getContext('2d')
  if (!ctx) return
  ctx.setTransform(1, 0, 0, 1, 0, 0)
  ctx.scale(dpr, dpr)
}

function spawnStars() {
  stars = []
  const c = canvasRef.value
  if (!c) return
  const w = window.innerWidth, h = window.innerHeight
  const maxR = Math.max(w, h) * 0.75
  for (let i = 0; i < TOTAL_STARS; i++) {
    stars.push({
      a: Math.random() * Math.PI * 2,
      r: Math.random() * maxR + 5,
      speed: 0.6 + Math.random() * 2.4,
      maxR,
    })
  }
}

function loop(now: number) {
  const c = canvasRef.value
  if (!c) return
  const ctx = c.getContext('2d')
  if (!ctx) return
  const w = window.innerWidth, h = window.innerHeight
  const cx = w / 2, cy = h / 2
  ctx.clearRect(0, 0, w, h)
  const elapsed = (now - phaseStart) / 1000
  let mult = 1
  if (phase === 'warp') {
    mult = elapsed < 1.5 ? (3 + elapsed * 6) : Math.max(0.35, 1.8 - (elapsed - 1.5) * 2.9)
  } else {
    mult = 0.65
  }
  for (const s of stars) {
    s.r += s.speed * mult
    if (s.r > s.maxR) {
      s.r = 5 + Math.random() * 30
      s.a = Math.random() * Math.PI * 2
      s.speed = 0.6 + Math.random() * 2.4
    }
    const distFactor = 0.4 + (s.r / s.maxR) * 1.6
    const streakLen = s.speed * mult * 6 * distFactor
    const x = cx + Math.cos(s.a) * s.r
    const y = cy + Math.sin(s.a) * s.r
    const tx = cx + Math.cos(s.a) * Math.max(0, s.r - streakLen)
    const ty = cy + Math.sin(s.a) * Math.max(0, s.r - streakLen)
    const alpha = Math.min(1, (s.r / s.maxR) * 1.2)
    const intensity = phase === 'warp' ? alpha : alpha * 0.85
    ctx.strokeStyle = `rgba(255,255,255,${intensity})`
    ctx.lineWidth = phase === 'warp' ? (1.0 + (s.r / s.maxR) * 1.2) : (0.8 + (s.r / s.maxR) * 0.5)
    ctx.lineCap = 'round'
    ctx.beginPath()
    ctx.moveTo(tx, ty)
    ctx.lineTo(x, y)
    ctx.stroke()
  }
  if (phase === 'warp' && elapsed > 2.0) {
    phase = 'idle'
  }
  raf = requestAnimationFrame(loop)
}

function startWarp() {
  spawnStars()
  phase = 'warp'
  phaseStart = performance.now()
  cancelAnimationFrame(raf)
  raf = requestAnimationFrame(loop)
}

onMounted(async () => {
  await nextTick()
  fit()
  window.addEventListener('resize', fit)
  startWarp()
})
onUnmounted(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', fit)
  if (tickTimer) clearInterval(tickTimer)
})

/* ───────── Login flows ───────── */
async function handleKeyLogin() {
  const val = keyInput.value.trim()
  if (!val || loading.value) return
  if (lockedUntil.value && Date.now() < lockedUntil.value) return
  loading.value = true
  error.value = ''
  try {
    const userOk = await userAuth.login(val, true)
    if (userOk) { router.push('/user/dashboard'); return }
  } catch { /* fall through */ }

  let adminRes: any
  try { adminRes = await auth.login(val) } catch { adminRes = { ok: false, error: '凭证无效' } }
  if (adminRes?.ok) { router.push('/'); return }

  if (adminRes?.locked) {
    const sec = Number(adminRes.retryAfter || 600)
    lockedUntil.value = Date.now() + sec * 1000
    startTick()
    error.value = `登录已锁定，请在 ${sec} 秒后重试`
  } else if (typeof adminRes?.remainingAttempts === 'number') {
    error.value = `凭证无效（剩余 ${adminRes.remainingAttempts} 次后将被锁定）`
  } else {
    error.value = adminRes?.error || '凭证无效'
  }
  userAuth.logout()
  loading.value = false
}

async function handleUserLogin() {
  if (loading.value) return
  const u = userInput.value.username.trim()
  const p = userInput.value.password
  if (!u || !p) { error.value = '请填写邮箱和密码'; return }
  loading.value = true
  error.value = ''
  try {
    const res = await fetch('/user/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: u, password: p }),
    })
    if (res.ok) {
      const data = await res.json().catch(() => ({}))
      const key = data?.apiKey
      if (!key) throw new Error('登录响应无 apiKey')
      const ok = await userAuth.login(key, true)
      if (ok) { router.push('/user/dashboard'); return }
      throw new Error('登录后获取用户信息失败')
    }
    let msg = `HTTP ${res.status}`
    try { const d = await res.json(); if (d?.error) msg = d.error } catch {}
    error.value = msg
  } catch (e: any) {
    error.value = e?.message || '登录失败'
  } finally {
    loading.value = false
  }
}

async function handleRegister() {
  if (loading.value) return
  const email = registerInput.value.email.trim()
  const pwd = registerInput.value.password
  const code = registerInput.value.activationCode.trim()
  if (!email || !pwd) { error.value = '请填写邮箱和密码'; return }
  if (pwd.length < 8) { error.value = '密码至少 8 位'; return }
  loading.value = true
  error.value = ''
  try {
    const res = await fetch('/user/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password: pwd, activationCode: code }),
    })
    if (res.ok) {
      const data = await res.json().catch(() => ({}))
      const key = data?.apiKey
      if (!key) throw new Error('注册响应无 apiKey')
      const ok = await userAuth.login(key, true)
      if (ok) { router.push('/user/dashboard'); return }
      throw new Error('注册成功但登录失败')
    }
    let msg = `HTTP ${res.status}`
    try { const d = await res.json(); if (d?.error) msg = d.error } catch {}
    error.value = msg
  } catch (e: any) {
    error.value = e?.message || '注册失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login">
    <canvas ref="canvasRef" class="login__canvas" />

    <div class="login__wrap" :class="{ 'is-visible': cardVisible }">
      <svg class="login__frame" viewBox="0 0 420 540" preserveAspectRatio="none" aria-hidden="true">
        <rect class="login__frame-rect" x="1" y="1" width="418" height="538" rx="8" />
      </svg>

      <div class="login__card">
        <div class="login__brand">
          <img class="login__logo" :src="logoUrl" alt="PivotStack" />
          <span class="login__name">PivotStack</span>
        </div>

        <div class="login__tabs">
          <button class="login__tab" :class="{ 'is-active': tab === 'user' }" @click="tab = 'user'; error = ''">用户名 / 邮箱</button>
          <button class="login__tab" :class="{ 'is-active': tab === 'key' }" @click="tab = 'key'; error = ''">旧用户登录</button>
        </div>

        <form v-if="tab === 'key'" class="login__form" @submit.prevent="handleKeyLogin">
          <label class="login__label">ACCESS TOKEN</label>
          <div class="login__field">
            <input v-model="keyInput" :type="showKey ? 'text' : 'password'" class="login__input mono" placeholder="sk-..." autocomplete="off" @keyup.enter="handleKeyLogin" />
            <button type="button" class="login__eye" @click="showKey = !showKey" :aria-label="showKey ? '隐藏' : '显示'">
              <Eye v-if="!showKey" :size="14" />
              <EyeOff v-else :size="14" />
            </button>
          </div>
          <p class="login__hint">来自老用户?继续用 API key 即可登录</p>

          <Transition name="fade-up">
            <div v-if="error" class="login__error">{{ error }}</div>
          </Transition>

          <button type="submit" class="login__submit" :disabled="loading || lockCountdown > 0 || !keyInput.trim()">
            <span v-if="lockCountdown > 0">已锁定 · {{ lockCountdown }}s</span>
            <span v-else>{{ loading ? '验证中…' : '登录' }}</span>
          </button>
          <button type="button" class="login__switch" @click="tab = 'user'; error = ''">用户名密码登录 →</button>
        </form>

        <form v-else-if="userMode === 'login'" class="login__form" @submit.prevent="handleUserLogin">
          <label class="login__label">邮箱</label>
          <div class="login__field">
            <input v-model="userInput.username" type="text" class="login__input" placeholder="用户名 或 your@email.com" autocomplete="username" />
          </div>
          <label class="login__label">密码</label>
          <div class="login__field">
            <input v-model="userInput.password" :type="showPwd ? 'text' : 'password'" class="login__input" placeholder="••••••••" autocomplete="current-password" @keyup.enter="handleUserLogin" />
            <button type="button" class="login__eye" @click="showPwd = !showPwd" :aria-label="showPwd ? '隐藏' : '显示'">
              <Eye v-if="!showPwd" :size="14" />
              <EyeOff v-else :size="14" />
            </button>
          </div>

          <Transition name="fade-up">
            <div v-if="error" class="login__error">{{ error }}</div>
          </Transition>

          <button type="submit" class="login__submit" :disabled="loading || !userInput.username.trim() || !userInput.password">
            {{ loading ? '验证中…' : '登录' }}
          </button>
          <div class="login__row">
            <button type="button" class="login__switch login__switch--inline" @click="tab = 'key'; error = ''">← API Key</button>
            <button type="button" class="login__switch login__switch--inline" @click="userMode = 'register'; error = ''">立即注册 →</button>
          </div>
        </form>

        <form v-else class="login__form" @submit.prevent="handleRegister">
          <label class="login__label">邮箱</label>
          <div class="login__field">
            <input v-model="registerInput.email" type="email" class="login__input" placeholder="your@email.com" autocomplete="email" />
          </div>
          <label class="login__label">密码 <span class="login__label-hint">≥ 8 位</span></label>
          <div class="login__field">
            <input v-model="registerInput.password" :type="showPwd ? 'text' : 'password'" class="login__input" placeholder="设置一个安全的密码" autocomplete="new-password" />
            <button type="button" class="login__eye" @click="showPwd = !showPwd" :aria-label="showPwd ? '隐藏' : '显示'">
              <Eye v-if="!showPwd" :size="14" />
              <EyeOff v-else :size="14" />
            </button>
          </div>
          <label class="login__label">激活码</label>
          <div class="login__field">
            <input v-model="registerInput.activationCode" type="text" class="login__input mono" placeholder="KIRO-XXXX-XXXX-XXXX" autocomplete="off" />
          </div>

          <Transition name="fade-up">
            <div v-if="error" class="login__error">{{ error }}</div>
          </Transition>

          <button type="submit" class="login__submit" :disabled="loading || !registerInput.email.trim() || !registerInput.password">
            {{ loading ? '注册中…' : '注册并登录' }}
          </button>
          <button type="button" class="login__switch" @click="userMode = 'login'; error = ''">← 已有账号？返回登录</button>
        </form>

        <div class="login__divider" />
        <div class="login__foot">v6.0 · 激活码 · 没账号联系 admin</div>
      </div>
    </div>

    <button class="login__replay" @click="startWarp" title="重放动画">
      <RotateCw :size="12" />
      <span>重放 warp</span>
    </button>
  </main>
</template>

<style scoped>
.login {
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  background: #000;
  color: #ededed;
  font-family: "Geist Sans", Inter, "PingFang SC", "Microsoft YaHei", sans-serif;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.login__canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  z-index: 0;
  pointer-events: none;
}

.login__wrap {
  position: relative;
  width: 420px;
  height: 540px;
  opacity: 0;
  transform: scale(0.94);
  transition: opacity 360ms cubic-bezier(0.16, 1, 0.3, 1), transform 360ms cubic-bezier(0.16, 1, 0.3, 1);
  z-index: 5;
}
.login__wrap.is-visible { opacity: 1; transform: scale(1); }

/* ─── 绿色 L 角描边 frame ─── */
.login__frame {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 2;
}
.login__frame-rect {
  fill: none;
  stroke: #0bd470;
  stroke-width: 1.5;
  stroke-dasharray: 200 1716;
  stroke-dashoffset: 0;
  filter: drop-shadow(0 0 6px rgba(11, 212, 112, 0.6));
  animation: lf-walk 4s linear infinite;
}
@keyframes lf-walk {
  to { stroke-dashoffset: -1916; }
}

/* ─── 卡片本体 ─── */
.login__card {
  position: absolute;
  top: 16px;
  left: 16px;
  width: calc(100% - 32px);
  height: calc(100% - 32px);
  padding: 28px 32px;
  background: rgba(8, 8, 8, 0.85);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  z-index: 3;
}

.login__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 28px;
}
.login__logo {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: #ededed;
  object-fit: contain;
  flex-shrink: 0;
  display: block;
}
.login__name {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: #ededed;
}

.login__tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  margin-bottom: 24px;
}
.login__tab {
  background: transparent;
  border: none;
  color: #707070;
  font-size: 14px;
  font-weight: 500;
  padding: 8px 0;
  margin-right: 20px;
  cursor: pointer;
  position: relative;
  transition: color 160ms ease;
  font-family: inherit;
}
.login__tab:hover { color: #ededed; }
.login__tab.is-active { color: #ededed; font-weight: 600; }
.login__tab.is-active::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 2px;
  background: #0bd470;
}

.login__form {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.login__label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.10em;
  color: #707070;
  text-transform: uppercase;
  margin-bottom: 8px;
}
.login__label + .login__field { margin-bottom: 14px; }

.login__field {
  position: relative;
  display: flex;
  align-items: center;
}
.login__input {
  width: 100%;
  height: 38px;
  padding: 0 38px 0 12px;
  background: #050505;
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 4px;
  color: #ededed;
  font-size: 13px;
  font-family: inherit;
  outline: none;
  transition: border-color 160ms ease, background 160ms ease;
}
.login__input.mono {
  font-family: "Geist Mono", "JetBrains Mono", ui-monospace, monospace;
  letter-spacing: 0.02em;
}
.login__input::placeholder { color: #4d4d4d; }
.login__input:focus {
  border-color: rgba(11, 212, 112, 0.45);
  background: #070707;
}
.login__eye {
  position: absolute;
  right: 6px;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: #707070;
  cursor: pointer;
  border-radius: 3px;
  transition: color 160ms ease, background 160ms ease;
}
.login__eye:hover { color: #ededed; background: rgba(255, 255, 255, 0.04); }

.login__hint {
  margin: 8px 0 0;
  font-size: 11px;
  color: #707070;
}

.login__error {
  margin-top: 12px;
  padding: 6px 10px;
  background: rgba(255, 77, 77, 0.06);
  border: 1px solid rgba(255, 77, 77, 0.30);
  border-radius: 4px;
  color: #ff7a7a;
  font-size: 12px;
}

.login__submit {
  margin-top: 16px;
  width: 100%;
  height: 40px;
  background: #ededed;
  color: #000;
  border: none;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  transition: background 160ms ease, opacity 160ms ease, transform 80ms ease;
}
.login__submit:hover:not(:disabled) { background: #ffffff; }
.login__submit:active:not(:disabled) { transform: translateY(1px); }
.login__submit:disabled { opacity: 0.5; cursor: not-allowed; }

.login__switch {
  margin-top: 12px;
  width: 100%;
  height: 34px;
  background: transparent;
  color: #a1a1a1;
  border: none;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: color 160ms ease;
}
.login__switch:hover { color: #ededed; }
.login__switch--inline { width: auto; height: auto; padding: 4px 6px; }
.login__row {
  display: flex;
  justify-content: space-between;
  margin-top: 8px;
}
.login__label-hint {
  font-weight: 400;
  font-size: 10px;
  letter-spacing: 0;
  text-transform: none;
  color: #4d4d4d;
  margin-left: 6px;
}

.login__divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.08);
  margin: 12px 0 12px;
}
.login__foot {
  text-align: center;
  font-size: 11px;
  letter-spacing: 0.06em;
  color: #4d4d4d;
}

/* ─── 重放按钮 ─── */
.login__replay {
  position: absolute;
  bottom: 24px;
  right: 24px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 12px;
  background: rgba(255, 255, 255, 0.06);
  color: #a1a1a1;
  border: none;
  border-radius: 4px;
  font-size: 11px;
  font-family: inherit;
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease;
  z-index: 6;
}
.login__replay:hover { background: rgba(255, 255, 255, 0.10); color: #ededed; }

/* ─── fade transition for error ─── */
.fade-up-enter-active { transition: all 200ms cubic-bezier(0.16, 1, 0.3, 1); }
.fade-up-leave-active { transition: all 160ms ease; }
.fade-up-enter-from { opacity: 0; transform: translateY(4px); }
.fade-up-leave-to { opacity: 0; }

/* ─── responsive ─── */
@media (max-width: 480px) {
  .login__wrap { width: 92vw; height: 540px; }
}
</style>
