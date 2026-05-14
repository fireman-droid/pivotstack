<script setup>
import { ref, computed, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUserAuth } from '../stores/userAuth'
import { useWorldTheme } from '../stores/worldTheme'
import WorldCard from '../components/world/WorldCard.vue'
import WorldPasswordInput from '../components/world/WorldPasswordInput.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldSwitcher from '../components/WorldSwitcher.vue'
import { Shield, AlertCircle, ArrowRight } from 'lucide-vue-next'

const router = useRouter()
const auth = useAuthStore()
const userAuth = useUserAuth()
const theme = useWorldTheme()
const input = ref('')
const error = ref('')
const loading = ref(false)

// 锁定 / 失败次数提示（来自后端 admin login 限流返回）
const lockedUntil = ref(0)       // 时间戳 ms；0 表示未锁
const tickNow = ref(Date.now())
const remainingAttempts = ref(null)
const lockCountdown = computed(() => {
  if (!lockedUntil.value) return 0
  return Math.max(0, Math.ceil((lockedUntil.value - tickNow.value) / 1000))
})
let tickTimer = null
function startTick() {
  if (tickTimer) return
  tickTimer = setInterval(() => {
    tickNow.value = Date.now()
    if (lockCountdown.value <= 0) {
      lockedUntil.value = 0
      clearInterval(tickTimer)
      tickTimer = null
    }
  }, 500)
}
onUnmounted(() => { if (tickTimer) clearInterval(tickTimer) })

async function handleLogin() {
  if (!input.value.trim() || loading.value) return
  if (lockedUntil.value && Date.now() < lockedUntil.value) return
  loading.value = true
  error.value = ''
  const val = input.value.trim()

  // 先尝试用户 key（短路返回）
  try {
    const userOk = await userAuth.login(val, true)
    if (userOk) { router.push('/user/dashboard'); return }
  } catch {}

  // 再尝试 admin 登录
  let adminRes
  try {
    adminRes = await auth.login(val)
  } catch {
    adminRes = { ok: false, error: '凭证无效' }
  }
  if (adminRes && adminRes.ok) { router.push('/'); return }

  // 失败分类提示
  if (adminRes && adminRes.locked) {
    const sec = Number(adminRes.retryAfter || 600)
    lockedUntil.value = Date.now() + sec * 1000
    startTick()
    error.value = `登录已锁定，请在 ${sec} 秒后重试`
  } else if (adminRes && typeof adminRes.remainingAttempts === 'number') {
    remainingAttempts.value = adminRes.remainingAttempts
    error.value = `凭证无效（剩余 ${adminRes.remainingAttempts} 次后将被锁定）`
  } else {
    error.value = (adminRes && adminRes.error) || '凭证无效'
  }
  userAuth.logout()
  loading.value = false
}
</script>

<template>
  <div class="login-shell">
    <!-- 背景层（柔光晕 + 网格） -->
    <div class="bg-fx" aria-hidden="true">
      <div class="fx-aurora fx-a1" />
      <div class="fx-aurora fx-a2" />
      <div class="fx-aurora fx-a3" />
      <div class="fx-grid" />
      <div class="fx-vignette" />
    </div>

    <!-- 右上 主题切换 -->
    <div class="login-switcher">
      <WorldSwitcher />
    </div>

    <!-- 中央容器 -->
    <main class="login-content">
      <!-- 品牌区 -->
      <header class="brand-row">
        <div class="brand-mark">
          <!-- daogui: 心蟠/钱币 -->
          <svg v-if="theme.currentWorld === 'daogui'" class="mark-svg dg" viewBox="0 0 100 100">
            <circle cx="50" cy="50" r="46" fill="none" stroke="#b8860b" stroke-width="3" opacity="0.6" />
            <circle cx="50" cy="50" r="38" fill="none" stroke="#c41e3a" stroke-width="2" opacity="0.7" />
            <rect x="36" y="36" width="28" height="28" fill="none" stroke="#b8860b" stroke-width="3" opacity="0.65" />
            <path d="M50 14 L50 24 M86 50 L76 50 M50 86 L50 76 M14 50 L24 50" stroke="#c41e3a" stroke-width="2.5" opacity="0.6" />
          </svg>
          <!-- reality: 医疗徽章 -->
          <div v-else class="mark-medical">
            <Shield :size="32" stroke-width="2.2" />
          </div>
        </div>
        <h1 class="brand-name">
          Pivot<span class="brand-accent">Stack</span>
        </h1>
        <p class="brand-tagline">统一入口 · Sign In</p>
      </header>

      <!-- 表单卡片 -->
      <WorldCard padding="lg" :hover="false" elevated class="login-card">
        <div class="card-eyebrow">
          <span class="dot" />
          <span>身份鉴权 · Authentication</span>
        </div>

        <form @submit.prevent="handleLogin" class="login-form">
          <WorldPasswordInput
            v-model="input"
            label="凭证 / Credential"
            placeholder="请输入访问凭证"
            size="lg"
            :error="error || ''"
            @enter="handleLogin"
          />

          <WorldButton
            type="submit"
            variant="primary"
            size="lg"
            :loading="loading"
            :disabled="lockCountdown > 0"
            :block="true"
          >
            <span v-if="lockCountdown > 0">已锁定 · {{ lockCountdown }}s</span>
            <span v-else>{{ loading ? '验证中' : '登录' }}</span>
            <ArrowRight v-if="!loading && lockCountdown === 0" :size="16" />
          </WorldButton>
        </form>

        <Transition name="fade-up">
          <div v-if="error" class="error-row">
            <WorldChip variant="danger" :dot="true">
              <AlertCircle :size="13" />
              <span>{{ error }}</span>
            </WorldChip>
          </div>
        </Transition>
      </WorldCard>

      <p class="login-foot">
        © 2026 KIRO STACK · Secured Channel
      </p>
    </main>
  </div>
</template>

<style scoped>
.login-shell {
  position: relative;
  min-height: 100vh;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  overflow: hidden;
  font-family: var(--world-font-sans);
}

/* === 背景层 ===
 * 设计原则：
 *  - 用 radial-gradient 自然羽化，不再依赖 blur() 糊硬边
 *  - 三层 aurora 不同尺寸 / 速度 / 相位，构成有机层次
 *  - opacity 控制在 0.20–0.32 之间，保持"在背景里"而不是"贴在前景"
 *  - 动画用 transform + ease-in-out + 大 duration（22s+）让运动近似静止
 *  - vignette 暗角收边，消除矩形屏幕的硬边感
 */
.bg-fx {
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}
.fx-aurora {
  position: absolute;
  border-radius: 50%;
  filter: blur(40px);
  opacity: 0.45;
  will-change: transform, opacity;
  /* Light 默认：用 multiply 让色雾在白底变成淡淡水彩，不会被白底吃掉 */
  mix-blend-mode: multiply;
}
[data-theme="dark"] .fx-aurora,
.dark .fx-aurora {
  /* Dark：用 screen / lighten 让色光从黑底浮出来 */
  mix-blend-mode: lighten;
  opacity: 0.28;
}

.fx-a1 {
  top: -20%;
  left: -10%;
  width: 70%;
  height: 80%;
  background: radial-gradient(
    circle at 35% 40%,
    color-mix(in srgb, var(--world-accent) 70%, transparent) 0%,
    color-mix(in srgb, var(--world-accent) 30%, transparent) 30%,
    transparent 65%
  );
  animation: aurora-a 22s ease-in-out infinite;
}
.fx-a2 {
  bottom: -25%;
  right: -15%;
  width: 75%;
  height: 90%;
  background: radial-gradient(
    circle at 60% 55%,
    color-mix(in srgb, var(--world-paper-aged, var(--world-accent-soft, #38bdf8)) 60%, transparent) 0%,
    color-mix(in srgb, var(--world-paper-aged, var(--world-accent-soft, #38bdf8)) 25%, transparent) 35%,
    transparent 70%
  );
  animation: aurora-b 28s ease-in-out -6s infinite;
}
.fx-a3 {
  top: 30%;
  right: 20%;
  width: 38%;
  height: 38%;
  opacity: 0.18;
  background: radial-gradient(
    circle at 50% 50%,
    color-mix(in srgb, var(--world-accent) 50%, transparent) 0%,
    transparent 60%
  );
  animation: aurora-c 18s ease-in-out -9s infinite;
}

.fx-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(148, 163, 184, 0.06) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.06) 1px, transparent 1px);
  background-size: 56px 56px;
  opacity: 0.5;
  /* 中心淡出，避免抢主卡片视线 */
  mask-image: radial-gradient(ellipse 65% 55% at 50% 50%, transparent 0%, rgba(0,0,0,0.85) 70%, black 100%);
  -webkit-mask-image: radial-gradient(ellipse 65% 55% at 50% 50%, transparent 0%, rgba(0,0,0,0.85) 70%, black 100%);
}

.fx-vignette {
  position: absolute;
  inset: 0;
  background: radial-gradient(
    ellipse 80% 70% at 50% 50%,
    transparent 0%,
    transparent 55%,
    rgba(15, 23, 42, 0.08) 100%
  );
}

[data-world="daogui"] .fx-aurora { opacity: 0.22; }
[data-world="daogui"] .fx-grid   { opacity: 0.08; }

/* 三组动画：缓慢、错相、不同振幅，整体看起来像呼吸而非滑动 */
@keyframes aurora-a {
  0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
  33%      { transform: translate3d(3%, -2%, 0) scale(1.06); }
  66%      { transform: translate3d(-2%, 3%, 0) scale(0.96); }
}
@keyframes aurora-b {
  0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
  40%      { transform: translate3d(-4%, 2%, 0) scale(1.08); }
  70%      { transform: translate3d(2%, -3%, 0) scale(0.94); }
}
@keyframes aurora-c {
  0%, 100% { transform: translate3d(0, 0, 0) scale(1); opacity: 0.18; }
  50%      { transform: translate3d(2%, 4%, 0) scale(1.12); opacity: 0.10; }
}

/* 关闭动画偏好 */
@media (prefers-reduced-motion: reduce) {
  .fx-aurora { animation: none; }
}

/* === 主题切换器 === */
.login-switcher {
  position: fixed;
  top: 22px;
  right: 22px;
  z-index: 10;
}

/* === 中央内容 === */
.login-content {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 460px;
  display: flex;
  flex-direction: column;
  gap: 28px;
}

/* === 品牌 === */
.brand-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  text-align: center;
}
.brand-mark {
  position: relative;
  width: 72px;
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.mark-svg.dg {
  width: 100%;
  height: 100%;
  animation: rune-pulse 2.4s ease-in-out infinite;
  filter: drop-shadow(0 0 12px rgba(184, 134, 11, 0.4));
}
.mark-medical {
  width: 64px;
  height: 64px;
  border-radius: var(--world-radius-2xl);
  background: linear-gradient(135deg, var(--world-accent), var(--world-accent-soft, #38bdf8));
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 12px 32px -8px rgba(2, 132, 199, 0.45);
}
.brand-name {
  font-size: 2rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
.brand-accent {
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.brand-tagline {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: var(--world-text-mute);
  margin: 0;
}

/* === 卡片 === */
.login-card {
  position: relative;
}
.card-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  color: var(--world-text-mute);
  text-transform: uppercase;
  margin-bottom: 22px;
}
.card-eyebrow .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--world-accent);
  box-shadow: 0 0 6px var(--world-accent);
}
.login-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.error-row {
  margin-top: 14px;
  display: flex;
  justify-content: center;
}
.login-foot {
  text-align: center;
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.32em;
  text-transform: uppercase;
  color: var(--world-text-dim);
  margin: 0;
}

/* === 入场动画 === */
.login-content { animation: bloom-in 0.6s var(--world-ease-bounce, cubic-bezier(0.34, 1.56, 0.64, 1)); }
@keyframes bloom-in {
  0%   { opacity: 0; transform: translateY(12px) scale(0.97); }
  100% { opacity: 1; transform: translateY(0) scale(1); }
}

[data-world="daogui"] .login-content { animation: dg-bloom-in 0.7s var(--world-ease-bounce); }
@keyframes dg-bloom-in {
  0%   { opacity: 0; transform: scale(0.94); filter: blur(8px); }
  100% { opacity: 1; transform: scale(1);    filter: blur(0); }
}

.fade-up-enter-active { transition: all 320ms cubic-bezier(0.16, 1, 0.3, 1); }
.fade-up-leave-active { transition: all 220ms ease; }
.fade-up-enter-from   { opacity: 0; transform: translateY(8px); }
.fade-up-leave-to     { opacity: 0; }

/* 移动端 */
@media (max-width: 480px) {
  .login-content { gap: 22px; }
  .brand-name { font-size: 1.6rem; }
  .login-switcher { top: 12px; right: 12px; }
}

/* 自动填充修复 */
input:-webkit-autofill,
input:-webkit-autofill:hover,
input:-webkit-autofill:focus {
  -webkit-text-fill-color: var(--world-text-primary);
  -webkit-box-shadow: 0 0 0 1000px var(--world-bg-card) inset;
  transition: background-color 5000s ease-in-out 0s;
}
</style>
