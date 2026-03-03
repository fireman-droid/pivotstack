<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ShieldCheck, ArrowRight, Lock, Loader2 } from 'lucide-vue-next'

const router = useRouter()
const auth = useAuthStore()
const pwd = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  if (!pwd.value || loading.value) return
  loading.value = true
  error.value = ''
  
  const ok = await auth.login(pwd.value)
  
  if (ok) {
    router.push('/')
  } else {
    loading.value = false
    error.value = '鉴权失败：管理凭证无效'
  }
}
</script>

<template>
  <div class="min-h-screen w-full flex flex-col items-center justify-center p-8 bg-[#030712] relative overflow-hidden">
    <!-- Subtle Background Decor -->
    <div class="absolute inset-0 z-0">
      <div class="absolute top-[-10%] left-[-10%] w-[50%] h-[50%] bg-indigo-600/10 blur-[150px] rounded-full"></div>
      <div class="absolute bottom-[-10%] right-[-10%] w-[50%] h-[50%] bg-blue-600/10 blur-[150px] rounded-full"></div>
    </div>

    <!-- Main Container with Vertical Stretch -->
    <div class="w-full max-w-[500px] relative z-10 flex flex-col items-stretch">
      
      <!-- Brand Header -->
      <div class="text-center mb-16 flex flex-col items-center gap-6">
        <div class="inline-flex items-center justify-center w-24 h-24 rounded-[32px] bg-indigo-600 shadow-2xl shadow-indigo-600/30">
          <ShieldCheck class="w-12 h-12 text-white" />
        </div>
        <div class="space-y-2">
          <h1 class="text-5xl font-black tracking-tighter text-white">
            Kiro<span class="text-indigo-500">Stack</span>
          </h1>
          <p class="text-slate-500 font-bold uppercase tracking-[0.2em] text-xs">High Performance Proxy Gateway</p>
        </div>
      </div>

      <!-- Main Login Card -->
      <div class="bg-slate-900/40 backdrop-blur-3xl border border-white/[0.08] rounded-[48px] px-10 py-16 shadow-2xl flex flex-col gap-12">
        
        <!-- Welcome Text -->
        <div class="space-y-2">
          <h2 class="text-2xl font-bold text-white tracking-tight">身份鉴权</h2>
          <p class="text-slate-500 text-xs font-bold uppercase tracking-widest">Administrator Access Required</p>
        </div>

        <!-- Form with Large Gaps -->
        <form @submit.prevent="handleLogin" class="flex flex-col gap-10">
          
          <!-- Input Group -->
          <div class="flex flex-col gap-4">
            <label class="block text-[11px] font-black uppercase tracking-[0.3em] text-slate-500 ml-1">管理密钥 / SECURITY TOKEN</label>
            <div class="relative group">
              <Lock class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-600 group-focus-within:text-indigo-500 transition-colors" />
              <input 
                v-model="pwd" 
                type="password" 
                placeholder="请输入管理访问令牌..." 
                required
                class="w-full h-18 pl-14 pr-6 bg-white/[0.03] border border-white/10 rounded-2xl text-white outline-none focus:border-indigo-500/50 focus:ring-4 focus:ring-indigo-500/10 transition-all text-lg font-medium placeholder:text-slate-700"
              />
            </div>
          </div>

          <!-- Action Button -->
          <button 
            type="submit" 
            :disabled="loading"
            class="w-full h-18 rounded-2xl bg-indigo-600 hover:bg-indigo-500 text-white font-black text-lg shadow-xl shadow-indigo-600/20 transition-all active:scale-[0.98] flex items-center justify-center gap-4 disabled:opacity-50"
          >
            <template v-if="!loading">
              <span>进入控制台</span>
              <ArrowRight class="w-5 h-5" />
            </template>
            <template v-else>
              <Loader2 class="w-6 h-6 animate-spin" />
              <span>验证鉴权中...</span>
            </template>
          </button>
        </form>

        <!-- Error Alert -->
        <Transition name="fade-slide">
          <div v-if="error" class="p-5 rounded-2xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-sm font-bold text-center">
            {{ error }}
          </div>
        </Transition>
      </div>

      <!-- Footer Info -->
      <div class="mt-16 text-center text-[10px] font-bold uppercase tracking-[0.5em] text-slate-700">
        &copy; 2026 KIRO ENGINEERING · SECURED PROTOCOL
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 确保高度撑满 */
.h-18 { height: 4.5rem; }

.fade-slide-enter-active, .fade-slide-leave-active {
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}
.fade-slide-enter-from {
  opacity: 0;
  transform: translateY(12px);
}
.fade-slide-leave-to {
  opacity: 0;
}

/* 移除浏览器默认外框 */
input:-webkit-autofill,
input:-webkit-autofill:hover, 
input:-webkit-autofill:focus {
  -webkit-text-fill-color: white;
  -webkit-box-shadow: 0 0 0px 1000px #0f172a inset;
  transition: background-color 5000s ease-in-out 0s;
}
</style>

<style scoped>
.slide-up-enter-active, .slide-up-leave-active {
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}
.slide-up-enter-from {
  opacity: 0;
  transform: translateY(10px);
}
.slide-up-leave-to {
  opacity: 0;
}
</style>
