<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUserAuth } from '../stores/userAuth'
import { useWorldTheme } from '../stores/worldTheme'
import WorldCard from '../components/world/WorldCard.vue'
import WorldInput from '../components/world/WorldInput.vue'
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

async function handleLogin() {
  if (!input.value.trim() || loading.value) return
  loading.value = true
  error.value = ''
  const val = input.value.trim()

  try {
    const userOk = await userAuth.login(val, true)
    if (userOk) { router.push('/user/dashboard'); return }
  } catch {}

  try {
    const adminOk = await auth.login(val)
    if (adminOk) { router.push('/'); return }
  } catch {}

  error.value = '凭证无效'
  userAuth.logout()
  loading.value = false
}
</script>

<template>
  <div class="login-shell">
    <!-- 背景层 -->
    <div class="bg-fx" aria-hidden="true">
      <div class="fx-blob fx-1" />
      <div class="fx-blob fx-2" />
      <div class="fx-grid" />
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
          Kiro<span class="brand-accent">Stack</span>
        </h1>
        <p class="brand-tagline">控制台 · Admin Portal</p>
      </header>

      <!-- 表单卡片 -->
      <WorldCard padding="lg" :hover="false" elevated class="login-card">
        <div class="card-eyebrow">
          <span class="dot" />
          <span>身份鉴权 · Authentication</span>
        </div>

        <form @submit.prevent="handleLogin" class="login-form">
          <WorldInput
            v-model="input"
            type="password"
            label="凭证 / Credential"
            placeholder="请输入访问凭证"
            :monospace="true"
            size="lg"
            :error="error || ''"
            @enter="handleLogin"
          />

          <WorldButton
            type="submit"
            variant="primary"
            size="lg"
            :loading="loading"
            :block="true"
          >
            <span>{{ loading ? '验证中' : '登录' }}</span>
            <ArrowRight v-if="!loading" :size="16" />
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

/* === 背景层 === */
.bg-fx {
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}
.fx-blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(120px);
  opacity: 0.15;
  will-change: transform;
}
.fx-1 {
  top: -10%;
  left: 10%;
  width: 50%;
  height: 50%;
  background: var(--world-accent);
  animation: blob-drift 12s ease-in-out infinite;
}
.fx-2 {
  bottom: -10%;
  right: 5%;
  width: 40%;
  height: 40%;
  background: var(--world-paper-aged, var(--world-accent-soft, #38bdf8));
  animation: blob-drift 14s ease-in-out -4s infinite;
}
.fx-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(148, 163, 184, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.05) 1px, transparent 1px);
  background-size: 40px 40px;
  opacity: 0.4;
}
[data-world="daogui"] .fx-blob { opacity: 0.18; }
[data-world="daogui"] .fx-grid { opacity: 0.06; }

@keyframes blob-drift {
  0%, 100% { transform: translate(0, 0) scale(1); }
  50%      { transform: translate(20px, -10px) scale(1.05); }
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
