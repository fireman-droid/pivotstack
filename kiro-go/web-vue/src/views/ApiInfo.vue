<script setup>
import { useToast } from '../composables/useToast'
import { 
  Copy, 
  Terminal, 
  Globe, 
  ShieldCheck, 
  ChevronRight,
  Code,
  Zap,
  Info
} from 'lucide-vue-next'

const { success } = useToast()
const base = location.origin

const endpoints = [
  { label: 'Claude API', url: base + '/v1/messages', color: 'bg-indigo-500' },
  { label: 'OpenAI API', url: base + '/v1/chat/completions', color: 'bg-emerald-500' },
  { label: 'Model Discovery', url: base + '/v1/models', color: 'bg-amber-500' },
  { label: 'Metrics Stats', url: base + '/v1/status', color: 'bg-rose-500' },
]

function copy(text) {
  navigator.clipboard.writeText(text)
  success('已复制到剪贴板')
}

const curlExample = `curl ${base}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -d '{
    "model": "claude-3-5-sonnet",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'`
</script>

<template>
  <div class="space-y-8 max-w-[1200px] mx-auto pb-20">
    <!-- Header -->
    <div class="space-y-1">
      <h1 class="text-3xl font-black tracking-tighter text-[var(--text)]">开发者中心</h1>
      <p class="text-sm text-[var(--text-secondary)] font-medium flex items-center gap-2">
        <Terminal class="w-3.5 h-3.5 text-primary" />
        集成 Kiro-Stack 高速网关到您的应用程序
      </p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
      <!-- Endpoints List -->
      <div class="lg:col-span-7 space-y-6">
        <div class="flex items-center gap-2 px-2">
          <Globe class="w-5 h-5 text-primary" />
          <h2 class="font-bold text-sm uppercase tracking-widest text-[var(--text-secondary)]">API 终端地址</h2>
        </div>

        <div class="grid grid-cols-1 gap-4">
          <div v-for="ep in endpoints" :key="ep.label" class="modern-card p-6 group hover:translate-y-[-2px] transition-all">
            <div class="flex justify-between items-center mb-4">
              <span class="text-sm font-bold text-[var(--text)]">{{ ep.label }}</span>
              <div class="flex gap-2">
                <span class="px-2 py-0.5 rounded-full bg-primary/5 text-primary text-[9px] font-black uppercase tracking-wider border border-primary/10">Stable</span>
                <span class="px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-[var(--text-secondary)] text-[9px] font-black uppercase tracking-wider border border-[var(--border)]">v1</span>
              </div>
            </div>
            
            <div class="flex items-center gap-3 p-4 bg-[var(--bg)] rounded-2xl border border-[var(--border)] group/box relative cursor-pointer transition-colors hover:border-primary/50" @click="copy(ep.url)">
              <div :class="ep.color" class="w-2 h-2 rounded-full shrink-0 shadow-lg"></div>
              <code class="text-xs font-mono text-primary flex-1 truncate">{{ ep.url }}</code>
              <Copy class="w-4 h-4 text-[var(--text-secondary)] opacity-0 group-hover/box:opacity-100 transition-opacity" />
            </div>
          </div>
        </div>
      </div>

      <!-- Quick Integration -->
      <div class="lg:col-span-5 space-y-6">
        <div class="flex items-center gap-2 px-2">
          <Code class="w-5 h-5 text-amber-500" />
          <h2 class="font-bold text-sm uppercase tracking-widest text-[var(--text-secondary)]">快速接入示例</h2>
        </div>

        <div class="modern-card p-6 bg-slate-900 shadow-2xl overflow-hidden relative group">
          <div class="flex justify-between items-center mb-4">
            <div class="flex items-center gap-2">
              <div class="w-2.5 h-2.5 rounded-full bg-rose-500"></div>
              <div class="w-2.5 h-2.5 rounded-full bg-amber-500"></div>
              <div class="w-2.5 h-2.5 rounded-full bg-emerald-500"></div>
            </div>
            <button @click="copy(curlExample)" class="p-2 text-slate-400 hover:text-white transition-colors">
              <Copy class="w-4 h-4" />
            </button>
          </div>
          <pre class="text-[11px] font-mono text-indigo-300 leading-relaxed overflow-x-auto custom-scrollbar pt-2">{{ curlExample }}</pre>
          <div class="absolute -right-10 -top-10 w-40 h-40 bg-primary/5 rounded-full blur-3xl -z-10 group-hover:bg-primary/10 transition-colors"></div>
        </div>

        <!-- Security Notice -->
        <div class="modern-card p-6 bg-amber-500/5 border-amber-500/20">
          <div class="flex items-start gap-4">
            <div class="w-10 h-10 rounded-xl bg-amber-500/10 flex items-center justify-center shrink-0">
              <ShieldCheck class="w-5 h-5 text-amber-500" />
            </div>
            <div class="space-y-1">
              <h4 class="text-sm font-bold text-amber-500">安全性警示</h4>
              <p class="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                管理 API（如 /status）必须在请求头中携带有效的管理令牌。请勿在客户端 JavaScript 中硬编码此令牌。
              </p>
            </div>
          </div>
        </div>

        <!-- Tech Stack -->
        <div class="modern-card p-5 space-y-3">
          <div class="text-[10px] font-black uppercase tracking-widest text-[var(--text-secondary)] opacity-50 mb-1">适配框架</div>
          <div class="flex flex-wrap gap-2">
             <span v-for="tag in ['Next.js', 'Python', 'Go', 'LangChain', 'Dify']" :key="tag" class="px-3 py-1 bg-[var(--bg)] border border-[var(--border)] rounded-lg text-[10px] font-bold">{{ tag }}</span>
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
