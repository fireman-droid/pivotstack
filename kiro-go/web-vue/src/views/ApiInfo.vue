<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Copy, Terminal, Globe, ShieldCheck, Code, Check } from 'lucide-vue-next'
import { copyToClipboard } from '../utils/clipboard'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldChip from '../components/world/WorldChip.vue'

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
      bash: `curl ${base}/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: ${key}" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}],"stream":true}'`,
      powershell: `curl.exe ${base}/v1/messages -H 'Content-Type: application/json' -H 'x-api-key: ${key}' -H 'anthropic-version: 2023-06-01' -d '{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}],"stream":true}'`,
    },
    {
      label: 'OpenAI API',
      desc: 'Chat Completions 兼容接口',
      path: '/v1/chat/completions',
      bash: `curl ${base}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${key}" \\
  -d '{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"Hello!"}],"stream":true}'`,
      powershell: `curl.exe ${base}/v1/chat/completions -H 'Content-Type: application/json' -H 'Authorization: Bearer ${key}' -d '{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"Hello!"}],"stream":true}'`,
    },
    {
      label: '模型列表',
      desc: '查询所有可用模型',
      path: '/v1/models',
      bash: `curl ${base}/v1/models -H "Authorization: Bearer ${key}"`,
      powershell: `curl.exe ${base}/v1/models -H 'Authorization: Bearer ${key}'`,
    },
    {
      label: '服务状态',
      desc: '健康检查与运行指标',
      path: '/health',
      bash: `curl ${base}/health`,
      powershell: `curl.exe ${base}/health`,
    },
  ]
}

const endpoints = computed(() => buildEndpoints(apiKey.value))
const selected = computed(() => endpoints.value[selectedIndex.value])
const selectedCurl = computed(() => selected.value[shell.value])

function copy(text) {
  copyToClipboard(text)
  success('已复制到剪贴板')
}

const shellOptions = [
  { value: 'bash', label: 'Bash' },
  { value: 'powershell', label: 'PowerShell' },
]
</script>

<template>
  <div class="apiinfo-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">开发者中心</div>
        <h1 class="page-title">
          <Terminal :size="22" />
          <span>API 接入说明</span>
        </h1>
        <p class="page-sub">集成 Kiro-Stack 网关到您的应用</p>
      </div>
    </header>

    <!-- Endpoint selector -->
    <div class="endpoint-grid">
      <button
        v-for="(ep, i) in endpoints"
        :key="ep.label"
        class="ep-tile"
        :class="{ active: selectedIndex === i }"
        @click="selectedIndex = i"
      >
        <div class="ep-head">
          <span class="ep-dot" />
          <span class="ep-name">{{ ep.label }}</span>
          <Check v-if="selectedIndex === i" :size="14" class="ep-check" />
        </div>
        <div class="ep-desc">{{ ep.desc }}</div>
      </button>
    </div>

    <!-- URL bar -->
    <WorldCard padding="md" class="url-card">
      <div class="url-row" @click="copy(base + selected.path)">
        <Globe :size="14" />
        <code class="url-text">{{ base }}{{ selected.path }}</code>
        <span class="url-hint">点击复制</span>
        <Copy :size="14" class="url-copy" />
      </div>
    </WorldCard>

    <!-- Code block -->
    <WorldCard padding="none" class="code-card">
      <div class="code-head">
        <div class="code-traffic">
          <span class="dot t-r" /><span class="dot t-y" /><span class="dot t-g" />
        </div>
        <WorldSegment v-model="shell" :options="shellOptions" size="sm" />
        <WorldButton variant="ghost" size="sm" @click="copy(selectedCurl)">
          <Copy :size="13" /><span>复制</span>
        </WorldButton>
      </div>
      <pre class="code-body">{{ selectedCurl }}</pre>
    </WorldCard>

    <!-- Tips -->
    <div class="tips-row">
      <WorldCard padding="md" class="tip-card warn-tip">
        <div class="tip-content">
          <ShieldCheck :size="18" />
          <div>
            <div class="tip-title">安全提示</div>
            <p class="tip-desc">请勿在前端代码中硬编码 API Key。建议通过后端服务转发请求，或使用环境变量管理密钥。</p>
          </div>
        </div>
      </WorldCard>
      <WorldCard padding="md" class="tip-card">
        <div class="tip-content">
          <Code :size="18" />
          <div>
            <div class="tip-title">适配框架</div>
            <div class="tag-row">
              <WorldChip v-for="tag in ['Next.js', 'Python', 'Go', 'LangChain', 'Dify', 'Cursor', 'Claude Code']" :key="tag" size="sm" variant="info">
                {{ tag }}
              </WorldChip>
            </div>
          </div>
        </div>
      </WorldCard>
    </div>
  </div>
</template>

<style scoped>
.apiinfo-page { display: flex; flex-direction: column; gap: 18px; }

.page-head .title-wrap { display: flex; flex-direction: column; gap: 4px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-family: var(--world-font-display);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}
.page-sub {
  margin: 4px 0 0;
  font-size: 0.8125rem;
  color: var(--world-text-mute);
}

.endpoint-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}
.ep-tile {
  text-align: left;
  padding: 14px 16px;
  background: var(--world-glass-bg);
  border: 2px solid transparent;
  border-radius: var(--world-radius-lg);
  cursor: pointer;
  transition: all 220ms ease;
  font-family: var(--world-font-sans);
}
.ep-tile:hover {
  border-color: var(--world-glass-border);
  transform: translateY(-1px);
}
.ep-tile.active {
  border-color: var(--world-accent);
  box-shadow: 0 6px 14px -4px rgba(2, 132, 199, 0.18);
}
[data-world="daogui"] .ep-tile.active { box-shadow: 0 6px 16px -4px rgba(196, 30, 58, 0.32); }

.ep-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.ep-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: var(--world-accent);
  box-shadow: 0 0 6px var(--world-accent);
  flex-shrink: 0;
}
.ep-name {
  font-size: 0.875rem;
  font-weight: 800;
  color: var(--world-text-primary);
}
.ep-check { margin-left: auto; color: var(--world-accent); }
.ep-desc {
  font-size: 0.72rem;
  color: var(--world-text-mute);
  line-height: 1.5;
}

.url-card { transition: all 200ms; cursor: pointer; }
.url-card:hover { border-color: var(--world-accent); }
.url-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.url-text {
  flex: 1;
  font-family: var(--world-font-mono);
  font-size: 0.875rem;
  color: var(--world-accent);
  font-weight: 700;
  word-break: break-all;
}
.url-hint {
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--world-text-dim);
  white-space: nowrap;
}
.url-copy { color: var(--world-text-mute); }

.code-card { overflow: hidden; }
.code-head {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--world-divider);
  background: var(--world-overlay-light);
}
.code-traffic { display: flex; gap: 6px; }
.code-traffic .dot { width: 10px; height: 10px; border-radius: 50%; }
.t-r { background: #ef4444; }
.t-y { background: #f59e0b; }
.t-g { background: #10b981; }

.code-body {
  margin: 0;
  padding: 18px 22px;
  font-family: var(--world-font-mono);
  font-size: 0.78rem;
  color: var(--world-text-primary);
  background: var(--world-bg-card);
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-x: auto;
  line-height: 1.7;
}
[data-world="daogui"] .code-body { color: var(--world-paper); }

.tips-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
@media (max-width: 720px) { .tips-row { grid-template-columns: 1fr; } }
.tip-content {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  color: var(--world-text-mute);
}
.tip-content > svg { color: var(--world-accent); flex-shrink: 0; margin-top: 2px; }
.tip-title {
  font-size: 0.875rem;
  font-weight: 800;
  color: var(--world-text-primary);
  margin-bottom: 4px;
}
.tip-desc {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.5;
}
.warn-tip .tip-content > svg { color: var(--world-warning); }
.tag-row { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 4px; }
</style>
