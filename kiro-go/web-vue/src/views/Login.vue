<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUserAuth } from '../stores/userAuth'

const router = useRouter()
const auth = useAuthStore()
const userAuth = useUserAuth()
const input = ref('')
const error = ref('')
const loading = ref(false)

const isApiKey = computed(() => input.value.trim().startsWith('sk-'))
const inputHint = computed(() => {
  if (!input.value.trim()) return ''
  return isApiKey.value ? '🔑 将以用户身份登录' : '🛡️ 将以管理员身份登录'
})

async function handleLogin() {
  if (!input.value.trim() || loading.value) return
  loading.value = true
  error.value = ''

  try {
    if (isApiKey.value) {
      // User login with API Key
      const ok = await userAuth.login(input.value.trim(), true)
      if (ok) {
        router.push('/user/dashboard')
      } else {
        error.value = userAuth.error || 'API Key 无效或已禁用'
      }
    } else {
      // Admin login with password
      const ok = await auth.login(input.value.trim())
      if (ok) {
        router.push('/')
      } else {
        error.value = '管理凭证无效'
      }
    }
  } catch (e) {
    error.value = e.message || '连接失败，请检查服务是否运行'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen w-full flex flex-col items-center justify-center p-8 relative overflow-hidden">
    <!-- 血雾渐变背景 -->
    <div class="absolute inset-0 z-0">
      <div class="absolute top-[-15%] left-[20%] w-[50%] h-[50%] bg-[var(--primary)] opacity-[0.06] blur-[150px] rounded-full animate-blood-mist"></div>
      <div class="absolute bottom-[-10%] right-[10%] w-[40%] h-[40%] bg-text-secondary opacity-[0.08] blur-[120px] rounded-full animate-blood-mist" style="animation-delay: -3s;"></div>
    </div>

    <div class="w-full max-w-[460px] relative z-10 flex flex-col items-stretch">
      
      <!-- 符文 Logo -->
      <div class="text-center mb-14 flex flex-col items-center gap-5">
        <div class="relative">
          <svg class="w-20 h-20 animate-rune-pulse" viewBox="0 0 100 100">
            <circle cx="50" cy="50" r="45" fill="none" stroke="#b8860b" stroke-width="5" opacity="0.7" />
            <circle cx="50" cy="50" r="42" fill="none" stroke="#b8860b" stroke-width="1" opacity="0.3" />
            <rect x="36" y="36" width="28" height="28" fill="none" stroke="#b8860b" stroke-width="4" opacity="0.7" />
            <path d="M50 12 L50 24 M88 50 L76 50 M50 88 L50 76 M12 50 L24 50" stroke="#b8860b" stroke-width="3" opacity="0.4" />
          </svg>
          <div class="absolute inset-0 rounded-full bg-[var(--world-accent-alt)] opacity-[0.08] blur-xl animate-rune-pulse"></div>
        </div>

        <div class="space-y-2">
          <h1 class="text-4xl font-black tracking-tighter text-[var(--text)]">
            Kiro<span class="text-[var(--primary)]">Stack</span>
          </h1>
          <p class="text-[var(--text)]-secondary font-bold uppercase tracking-[0.25em] text-[10px]">High Performance Proxy Gateway</p>
        </div>
      </div>

      <!-- 统一登录卡 -->
      <div class="bg-[var(--card)]/80 backdrop-blur-2xl border border-[var(--border)] rounded-3xl px-10 py-14 shadow-2xl flex flex-col gap-8 relative overflow-hidden">
        <div class="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-[#b8860b]/40 to-transparent"></div>
        <div class="absolute bottom-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-[#c41e3a]/30 to-transparent"></div>

        <div class="space-y-2">
          <h2 class="text-xl font-bold text-[var(--text)] tracking-tight">统 一 登 录</h2>
          <p class="text-[var(--text)]-secondary text-[10px] font-bold uppercase tracking-[0.2em]">API Key 或 管理密码</p>
        </div>

        <form @submit.prevent="handleLogin" class="flex flex-col gap-6">
          <div class="flex flex-col gap-3">
            <label class="block text-[10px] font-black uppercase tracking-[0.3em] text-[var(--world-accent-alt)] ml-1">凭证 / CREDENTIAL</label>
            <div class="relative group">
              <span class="absolute left-4 top-1/2 -translate-y-1/2 text-[var(--world-accent-alt)]/40 text-lg select-none">{{ isApiKey ? '🔑' : '卍' }}</span>
              <input 
                v-model="input" 
                type="password" 
                placeholder="输入 API Key (sk-...) 或管理密码" 
                required
                autofocus
                class="w-full h-16 pl-12 pr-6 bg-[var(--bg)]/60 border border-[var(--border)] rounded-xl text-[var(--text)] outline-none focus:border-[var(--primary)]/50 focus:shadow-[0_0_20px_rgba(196,30,58,0.15)] transition-all text-base font-medium placeholder:text-[var(--text)]-secondary"
              />
            </div>
            <!-- 类型提示 -->
            <Transition name="fade-slide">
              <p v-if="inputHint" class="text-[11px] font-bold ml-1 transition-all"
                :class="isApiKey ? 'text-emerald-400' : 'text-amber-400'">
                {{ inputHint }}
              </p>
            </Transition>
          </div>

          <button 
            type="submit" 
            :disabled="loading"
            class="w-full h-16 rounded-xl text-white font-black text-base shadow-xl transition-all active:scale-[0.98] flex items-center justify-center gap-3 disabled:opacity-50 relative overflow-hidden"
            :class="isApiKey 
              ? 'bg-emerald-600 hover:bg-emerald-500 shadow-emerald-600/20' 
              : 'bg-[var(--primary)] hover:bg-[#d42444] shadow-[var(--primary)]/20 blood-glow-hover'"
          >
            <template v-if="!loading">
              <span>{{ isApiKey ? '进 入 用 户 面 板' : '进 入 管 理 控 制 台' }}</span>
              <span class="text-lg opacity-60">{{ isApiKey ? '→' : '☯' }}</span>
            </template>
            <template v-else>
              <svg class="w-6 h-6 animate-coin-spin" viewBox="0 0 100 100">
                <circle cx="50" cy="50" r="40" fill="none" stroke="currentColor" stroke-width="6" opacity="0.6" />
                <rect x="38" y="38" width="24" height="24" fill="none" stroke="currentColor" stroke-width="4" opacity="0.6" />
              </svg>
              <span>验 证 中...</span>
            </template>
          </button>
        </form>

        <Transition name="fade-slide">
          <div v-if="error" class="p-4 rounded-xl bg-[var(--primary)]/10 border border-[var(--primary)]/25 text-[var(--primary)] text-sm font-bold text-center flex items-center justify-center gap-2">
            <span class="text-lg">☠</span>
            {{ error }}
          </div>
        </Transition>

        <!-- 提示 -->
        <div class="text-center text-[10px] text-[var(--text)]-secondary opacity-50 space-y-1">
          <p>输入以 <code class="text-emerald-400/70 font-mono">sk-</code> 开头的 API Key 进入用户面板</p>
          <p>输入管理密码进入管理控制台</p>
        </div>
      </div>

      <div class="mt-12 text-center text-[9px] font-bold uppercase tracking-[0.5em] text-[var(--text)]-secondary">
        &copy; 2026 KIRO ENGINEERING · SECURED PROTOCOL
      </div>
    </div>
  </div>
</template>

<style scoped>
.fade-slide-enter-active, .fade-slide-leave-active {
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}
.fade-slide-enter-from { opacity: 0; transform: translateY(12px); }
.fade-slide-leave-to { opacity: 0; }

input:-webkit-autofill,
input:-webkit-autofill:hover, 
input:-webkit-autofill:focus {
  -webkit-text-fill-color: #e5e5e5;
  -webkit-box-shadow: 0 0 0px 1000px #0a0a0a inset;
  transition: background-color 5000s ease-in-out 0s;
}
</style>
