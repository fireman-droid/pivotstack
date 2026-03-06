<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { 
  Copy, 
  Terminal, 
  Globe, 
  ShieldCheck, 
  Code,
  Check
} from 'lucide-vue-next'

const { success } = useToast()
const base = location.origin

const selectedIndex = ref(0)
const shell = ref('bash')
const apiKey = ref('YOUR_API_KEY')

onMounted(async () => {
  try {
    const res = await api('/settings')
    if (res.ok) {
      const data = await res.json()
      if (data.apiKey) apiKey.value = data.apiKey
    }
  } catch {}
})

function buildEndpoints(key) {
  return [
    {
      label: 'Claude API',
      desc: 'Anthropic Messages 兼容接口',
      path: '/v1/messages',
      color: 'bg-indigo-500',
      bash: `curl ${base}/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: ${key}" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}],"stream":true}'`,
      powershell: `curl.exe ${base}/v1/messages -H 'Content-Type: application/json' -H 'x-api-key: ${key}' -H 'anthropic-version: 2023-06-01' -d '{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}],"stream":true}'`
    },
    {
      label: 'OpenAI API',
      desc: 'Chat Completions 兼容接口',
      path: '/v1/chat/completions',
      color: 'bg-emerald-500',
      bash: `curl ${base}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${key}" \\
  -d '{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"Hello!"}],"stream":true}'`,
      powershell: `curl.exe ${base}/v1/chat/completions -H 'Content-Type: application/json' -H 'Authorization: Bearer ${key}' -d '{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"Hello!"}],"stream":true}'`
    },
    {
      label: '模型列表',
      desc: '查询所有可用模型',
      path: '/v1/models',
      color: 'bg-amber-500',
      bash: `curl ${base}/v1/models \\
  -H "Authorization: Bearer ${key}"`,
      powershell: `curl.exe ${base}/v1/models -H 'Authorization: Bearer ${key}'`
    },
    {
      label: '服务状态',
      desc: '健康检查与运行指标',
      path: '/health',
      color: 'bg-rose-500',
      bash: `curl ${base}/health`,
      powershell: `curl.exe ${base}/health`
    },
  ]
}

const endpoints = computed(() => buildEndpoints(apiKey.value))
const selected = computed(() => endpoints.value[selectedIndex.value])
const selectedCurl = computed(() => selected.value[shell.value])

function copy(text) {
  navigator.clipboard.writeText(text)
  success('已复制到剪贴板')
}
</script>

<template>
  <div class="space-y-6 max-w-[1600px] mx-auto pb-10">
    <div class="space-y-1">
      <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">开发者中心</h1>
      <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
        <Terminal class="w-3.5 h-3.5 text-primary" />
        集成 Kiro-Stack 网关到您的应用
      </p>
    </div>

    <!-- Endpoint Tabs -->
    <div class="flex items-center gap-2 px-1">
      <Globe class="w-4 h-4 text-primary" />
      <span class="font-bold text-xs uppercase tracking-widest text-[var(--text-secondary)]">选择接口</span>
    </div>
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <button
        v-for="(ep, i) in endpoints" :key="ep.label"
        @click="selectedIndex = i"
        class="modern-card p-4 text-left transition-all duration-200 border-2"
        :class="selectedIndex === i
          ? 'border-primary shadow-md shadow-primary/5 scale-[1.02]'
          : 'border-transparent hover:border-[var(--border)] hover:translate-y-[-1px]'"
      >
        <div class="flex items-center gap-2.5 mb-2">
          <div :class="ep.color" class="w-2 h-2 rounded-full shrink-0 shadow-lg"></div>
          <span class="text-sm font-bold">{{ ep.label }}</span>
          <Check v-if="selectedIndex === i" class="w-3.5 h-3.5 text-primary ml-auto" />
        </div>
        <p class="text-[10px] text-[var(--text-secondary)] leading-snug">{{ ep.desc }}</p>
      </button>
    </div>

    <!-- URL + Code Example -->
    <div class="space-y-4">
      <!-- Endpoint URL -->
      <div class="flex items-center gap-3 p-4 modern-card cursor-pointer group transition-colors hover:border-primary/30" @click="copy(base + selected.path)">
        <div :class="selected.color" class="w-2.5 h-2.5 rounded-full shrink-0 shadow-lg"></div>
        <code class="text-sm font-mono text-primary flex-1 truncate">{{ base }}{{ selected.path }}</code>
        <span class="text-[10px] text-[var(--text-secondary)] font-bold uppercase tracking-wider opacity-0 group-hover:opacity-100 transition-opacity mr-1">点击复制</span>
        <Copy class="w-4 h-4 text-[var(--text-secondary)] opacity-50 group-hover:opacity-100 transition-opacity" />
      </div>

      <!-- Code Block -->
      <div class="modern-card bg-slate-900 overflow-hidden">
        <div class="flex justify-between items-center px-5 py-3 border-b border-white/5">
          <div class="flex items-center gap-3">
            <div class="flex items-center gap-1.5">
              <div class="w-2.5 h-2.5 rounded-full bg-rose-500"></div>
              <div class="w-2.5 h-2.5 rounded-full bg-amber-500"></div>
              <div class="w-2.5 h-2.5 rounded-full bg-emerald-500"></div>
            </div>
            <div class="flex items-center bg-white/5 rounded-md p-0.5">
              <button
                v-for="s in ['bash', 'powershell']" :key="s"
                @click="shell = s"
                class="px-2.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider transition-all"
                :class="shell === s ? 'bg-primary text-white' : 'text-slate-500 hover:text-slate-300'"
              >{{ s === 'bash' ? 'Bash' : 'PowerShell' }}</button>
            </div>
          </div>
          <button @click="copy(selectedCurl)" class="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-slate-400 hover:text-white hover:bg-white/5 transition-colors text-[11px] font-medium">
            <Copy class="w-3.5 h-3.5" />
            复制
          </button>
        </div>
        <pre class="px-5 py-4 text-[11px] font-mono text-indigo-300 leading-relaxed overflow-x-auto custom-scrollbar">{{ selectedCurl }}</pre>
      </div>
    </div>

    <!-- Bottom Info Row -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div class="modern-card p-5 bg-amber-500/5 border-amber-500/10 flex items-start gap-3">
        <ShieldCheck class="w-5 h-5 text-amber-500 shrink-0 mt-0.5" />
        <div>
          <h4 class="text-xs font-bold text-amber-500 mb-1">安全提示</h4>
          <p class="text-[11px] text-[var(--text-secondary)] leading-relaxed">
            请勿在前端代码中硬编码 API Key。建议通过后端服务转发请求，或使用环境变量管理密钥。
          </p>
        </div>
      </div>
      <div class="modern-card p-5 flex items-start gap-3">
        <Code class="w-5 h-5 text-primary shrink-0 mt-0.5" />
        <div>
          <h4 class="text-xs font-bold text-[var(--text)] mb-1">适配框架</h4>
          <div class="flex flex-wrap gap-1.5 mt-1.5">
            <span v-for="tag in ['Next.js', 'Python', 'Go', 'LangChain', 'Dify', 'Cursor', 'Claude Code']" :key="tag"
              class="px-2 py-0.5 bg-[var(--bg)] border border-[var(--border)] rounded text-[10px] font-bold">{{ tag }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar { height: 4px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 10px; }
pre { white-space: pre-wrap; word-wrap: break-word; }
</style>
