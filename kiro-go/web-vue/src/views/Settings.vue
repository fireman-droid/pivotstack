<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { 
  Key, 
  Settings, 
  Wand2, 
  ShieldAlert, 
  Lock, 
  RefreshCw, 
  Cpu, 
  Route,
  Save,
  Fingerprint,
  ChevronRight,
  Info
} from 'lucide-vue-next'

const { success, error } = useToast()

const requireApiKey = ref(false)
const apiKey = ref('')
const thinkingSuffix = ref('-thinking')
const openaiFormat = ref('reasoning_content')
const claudeFormat = ref('thinking')
const preferredEndpoint = ref('auto')
const newPassword = ref('')
const loading = ref({ api: false, thinking: false, endpoint: false, pwd: false })

onMounted(async () => {
  try {
    const [settingsRes, thinkingRes, endpointRes] = await Promise.all([
      api('/settings'), api('/thinking'), api('/endpoint')
    ])
    if (settingsRes.ok) {
      const d = await settingsRes.json()
      requireApiKey.value = d.requireApiKey
      apiKey.value = d.apiKey || ''
    }
    if (thinkingRes.ok) {
      const d = await thinkingRes.json()
      thinkingSuffix.value = d.suffix || '-thinking'
      openaiFormat.value = d.openaiFormat || 'reasoning_content'
      claudeFormat.value = d.claudeFormat || 'thinking'
    }
    if (endpointRes.ok) {
      const d = await endpointRes.json()
      preferredEndpoint.value = d.preferredEndpoint || 'auto'
    }
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

async function changePassword() {
  if (!newPassword.value) return error('请输入新密码')
  loading.value.pwd = true
  const res = await api('/settings', { method: 'POST', body: JSON.stringify({ password: newPassword.value }) })
  if (res.ok) {
    const { useAuthStore } = await import('../stores/auth')
    const auth = useAuthStore()
    auth.password = newPassword.value
    localStorage.setItem('admin_password', newPassword.value)
    newPassword.value = ''
    success('密码已修改')
  } else error('修改失败')
  loading.value.pwd = false
}

async function resetStats() {
  if (!confirm('确定彻底重置全局统计？此操作将清空所有累积数据且不可恢复。')) return
  await api('/stats/reset', { method: 'POST' })
  success('统计已重置')
  setTimeout(() => location.reload(), 1000)
}
</script>

<template>
  <div class="max-w-[1600px] mx-auto space-y-10 pb-20">
    <!-- Centered Header -->
    <div class="text-center space-y-2 py-4">
      <h1 class="text-3xl font-black tracking-tighter text-[var(--text)]">系统参数设定</h1>
      <p class="text-sm text-[var(--text)]-secondary font-medium">配置网关核心行为、安全性及模型输出协议</p>
    </div>

    <!-- API Security Card -->
    <section class="modern-card overflow-hidden shadow-sm">
      <div class="px-8 py-5 border-b border-[var(--border)] bg-[var(--bg)]/50 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-indigo-500/10 text-indigo-500"><Key class="w-4 h-4" /></div>
        <h2 class="text-sm font-black uppercase tracking-widest text-[var(--text)]">API 鉴权协议</h2>
      </div>
      <div class="p-8 space-y-8">
        <label class="flex items-center gap-3 cursor-pointer group w-fit">
          <div class="relative flex items-center">
            <input type="checkbox" v-model="requireApiKey" class="peer h-5 w-5 cursor-pointer appearance-none rounded-md border border-[var(--border)] bg-[var(--bg)] checked:bg-[var(--primary)] transition-all" />
            <svg class="absolute h-3.5 w-3.5 pointer-events-none opacity-0 peer-checked:opacity-100 text-white left-0.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="4"><polyline points="20 6 9 17 4 12"></polyline></svg>
          </div>
          <span class="text-sm font-bold text-[var(--text)]-secondary group-hover:text-[var(--text)] transition-colors">启用全局 API Key 强制验证</span>
        </label>

        <div class="space-y-3">
          <div class="flex justify-between items-end pl-1">
            <span class="text-[11px] font-black uppercase tracking-widest text-[var(--text)]-secondary opacity-60">系统 API 密钥</span>
            <button @click="generateApiKey" class="text-[10px] font-black text-[var(--primary)] hover:underline uppercase tracking-tighter">重新生成密钥</button>
          </div>
          <div class="relative group">
            <Fingerprint class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500 group-focus-within:text-[var(--primary)] transition-colors" />
            <input v-model="apiKey" placeholder="留空则允许匿名访问" class="w-full h-14 pl-12 pr-4 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-sm font-mono outline-none focus:border-[var(--primary)]/50 focus:ring-4 focus:ring-primary/5 transition-all" />
          </div>
        </div>

        <button @click="saveApiSettings" :disabled="loading.api" class="flex items-center gap-2 px-6 py-3 bg-[var(--primary)] text-white rounded-xl font-black text-xs shadow-lg shadow-[var(--primary)]/20 hover:scale-[1.02] active:scale-95 transition-all">
          <Save class="w-4 h-4" /> 保存鉴权配置
        </button>
      </div>
    </section>

    <!-- Thinking Mode Card -->
    <section class="modern-card overflow-hidden shadow-sm">
      <div class="px-8 py-5 border-b border-[var(--border)] bg-[var(--bg)]/50 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-emerald-500/10 text-emerald-500"><Wand2 class="w-4 h-4" /></div>
        <h2 class="text-sm font-black uppercase tracking-widest text-[var(--text)]">Thinking 思考模式配置</h2>
      </div>
      <div class="p-8 space-y-8">
        <div class="space-y-3">
          <span class="text-[11px] font-black uppercase tracking-widest text-[var(--text)]-secondary opacity-60 pl-1">触发后缀 / Trigger Suffix</span>
          <input v-model="thinkingSuffix" placeholder="-thinking" class="w-full h-14 px-5 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-sm font-bold outline-none focus:border-emerald-500/50 focus:ring-4 focus:ring-emerald-500/5 transition-all" />
          <p class="text-[10px] text-[var(--text)]-secondary font-medium italic opacity-60 ml-1">在请求模型名后添加此后缀将强制激活思考路径映射。</p>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div class="space-y-3">
            <span class="text-[11px] font-black uppercase tracking-widest text-[var(--text)]-secondary opacity-60 pl-1">OpenAI 协议响应格式</span>
            <select v-model="openaiFormat" class="w-full h-14 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-xs font-bold outline-none cursor-pointer hover:border-emerald-500 transition-colors">
              <option value="reasoning_content">reasoning_content (标准)</option>
              <option value="thinking">&lt;thinking&gt; 标签 (Claude 风格)</option>
              <option value="think">&lt;think&gt; 标签 (OpenAI 风格)</option>
            </select>
          </div>
          <div class="space-y-3">
            <span class="text-[11px] font-black uppercase tracking-widest text-[var(--text)]-secondary opacity-60 pl-1">Claude 协议响应格式</span>
            <select v-model="claudeFormat" class="w-full h-14 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-2xl text-xs font-bold outline-none cursor-pointer hover:border-emerald-500 transition-colors">
              <option value="thinking">&lt;thinking&gt; 标签 (Claude 风格)</option>
              <option value="think">&lt;think&gt; 标签 (OpenAI 风格)</option>
              <option value="reasoning_content">直接明文输出 (不带标签)</option>
            </select>
          </div>
        </div>

        <button @click="saveThinking" :disabled="loading.thinking" class="flex items-center gap-2 px-6 py-3 bg-emerald-600 text-white rounded-xl font-black text-xs shadow-lg shadow-emerald-600/20 hover:scale-[1.02] active:scale-95 transition-all">
          <Save class="w-4 h-4" /> 应用模型映射规则
        </button>
      </div>
    </section>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
      <!-- Routing Card -->
      <section class="modern-card overflow-hidden shadow-sm">
        <div class="px-6 py-4 border-b border-[var(--border)] bg-[var(--bg)]/50 flex items-center gap-3">
          <Route class="w-4 h-4 text-amber-500" />
          <h2 class="text-[11px] font-black uppercase tracking-widest text-[var(--text)]">端点智能路由</h2>
        </div>
        <div class="p-6 space-y-6">
          <div class="space-y-3">
            <span class="text-[10px] font-black uppercase tracking-widest text-[var(--text)]-secondary opacity-60">首选连接节点</span>
            <select v-model="preferredEndpoint" class="w-full h-12 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-xs font-bold outline-none">
              <option value="auto">自动智能负载 (推荐)</option>
              <option value="codewhisperer">Amazon CodeWhisperer Node</option>
              <option value="amazonq">Amazon Q Business Node</option>
            </select>
          </div>
          <button @click="saveEndpoint" :disabled="loading.endpoint" class="w-full py-3 bg-amber-500 text-white rounded-xl font-black text-xs hover:bg-amber-600 transition-all">保存路由</button>
        </div>
      </section>

      <!-- Password Card -->
      <section class="modern-card overflow-hidden shadow-sm">
        <div class="px-6 py-4 border-b border-[var(--border)] bg-[var(--bg)]/50 flex items-center gap-3">
          <Lock class="w-4 h-4 text-[var(--primary)]" />
          <h2 class="text-[11px] font-black uppercase tracking-widest text-[var(--text)]">安全凭证管理</h2>
        </div>
        <div class="p-6 space-y-6">
          <div class="space-y-3">
            <span class="text-[10px] font-black uppercase tracking-widest text-[var(--text)]-secondary opacity-60">修改管理令牌</span>
            <input v-model="newPassword" type="password" placeholder="输入新访问密码" class="w-full h-12 px-4 bg-[var(--bg)] border border-[var(--border)] rounded-xl text-xs font-black outline-none focus:border-[var(--primary)]" />
          </div>
          <button @click="changePassword" :disabled="loading.pwd" class="w-full py-3 bg-slate-900 dark:bg-[var(--primary)] text-white rounded-xl font-black text-xs hover:opacity-90 transition-all">确认重置密码</button>
        </div>
      </section>
    </div>

    <!-- Danger Zone Optimized -->
    <section class="p-8 rounded-[32px] border-2 border-dashed border-rose-500/20 bg-rose-500/[0.02] space-y-6">
      <div class="flex items-center gap-3">
        <div class="p-2 rounded-lg bg-rose-500/10 text-rose-500 animate-pulse"><ShieldAlert class="w-5 h-5" /></div>
        <h2 class="text-sm font-black uppercase tracking-[0.2em] text-rose-500">危险操作区</h2>
      </div>
      
      <div class="flex flex-col md:flex-row items-center justify-between p-6 bg-white dark:bg-slate-900 border border-rose-500/10 rounded-2xl gap-6 shadow-sm">
        <div class="flex items-start gap-4">
          <div class="p-3 rounded-full bg-rose-50 dark:bg-rose-900/20 text-rose-500"><Cpu class="w-5 h-5" /></div>
          <div>
            <h4 class="text-sm font-black text-[var(--text)]">重置全局统计流水</h4>
            <p class="text-[11px] text-[var(--text)]-secondary mt-1 font-medium">警告：这将清空所有历史请求数、Token 消耗及成本统计。此操作不可逆。</p>
          </div>
        </div>
        <button @click="resetStats" class="px-6 py-3 bg-rose-500 text-white rounded-xl font-black text-xs hover:bg-rose-600 transition-all shadow-xl shadow-rose-500/20 active:scale-95 whitespace-nowrap">
          立即执行重置
        </button>
      </div>
    </section>

    <!-- Version Footer -->
    <div class="flex flex-col items-center gap-2 pt-10">
       <div class="flex items-center gap-2 px-3 py-1 rounded-full bg-border/30 text-[9px] font-black text-[var(--text)]-secondary uppercase tracking-widest">
          引擎版本 v1.0.3
       </div>
       <p class="text-[9px] font-bold text-slate-500 uppercase tracking-[0.3em]">由 Kiro-Stack 核心团队构建</p>
    </div>
  </div>
</template>

<style scoped>
/* 原生选择器美化 */
select {
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke='currentColor'%3E%3Cpath stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M19 9l-7 7-7-7'%3E%3C/path%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 1rem center;
  background-size: 1em;
}
</style>
