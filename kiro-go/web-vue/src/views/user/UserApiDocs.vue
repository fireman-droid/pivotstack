<script setup lang="ts">
import { computed, ref } from 'vue'
import { useUserAuth } from '../../stores/userAuth'
import { useMessage } from 'naive-ui'
import { Eye, EyeOff, Copy, Send, Check } from 'lucide-vue-next'
import Tile from '../../components/user/stellar/Tile.vue'
import MonoCopy from '../../components/user/stellar/MonoCopy.vue'

type DocKey = 'curl' | 'openai' | 'anthropic' | 'cursor' | 'claude' | 'cline'

const auth = useUserAuth() as any
const message = useMessage()

const apiKey = computed(() => String(auth.apiKey || 'YOUR_API_KEY'))
const baseUrl = computed(() => `${location.protocol}//${location.host}`)
const fullKey = computed(() => apiKey.value)
const showKey = ref(false)
const maskedKey = computed(() => {
  const k = apiKey.value
  if (k === 'YOUR_API_KEY' || k.length < 12) return k
  return `sk-•••••••••••••••${k.slice(-4)}`
})
const displayKey = computed(() => showKey.value ? fullKey.value : maskedKey.value)

const active = ref<DocKey>('claude')
const tabs: { key: DocKey; label: string }[] = [
  { key: 'claude', label: 'Claude Code' },
  { key: 'cursor', label: 'Cursor' },
  { key: 'cline', label: 'Cline / Roo' },
  { key: 'curl', label: 'curl' },
  { key: 'openai', label: 'OpenAI SDK' },
  { key: 'anthropic', label: 'Anthropic SDK' },
]

const snippets = computed<Record<DocKey, string>>(() => {
  const k = fullKey.value
  const u = baseUrl.value
  return {
    claude: `export ANTHROPIC_BASE_URL=${u}/v1
export ANTHROPIC_AUTH_TOKEN=${k}
claude`,
    cursor: `Base URL: ${u}/v1
API Key:  ${k}
Model:    claude-sonnet-4-5  (或其他可用模型)`,
    cline: `Provider:  OpenAI Compatible
Base URL:  ${u}/v1
API Key:   ${k}
Model ID:  claude-sonnet-4-5`,
    curl: `curl ${u}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${k}" \\
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role": "user", "content": "你好"}]
  }'`,
    openai: `import OpenAI from "openai"

const client = new OpenAI({
  apiKey: "${k}",
  baseURL: "${u}/v1",
})

const res = await client.chat.completions.create({
  model: "claude-sonnet-4-5",
  messages: [{ role: "user", content: "你好" }],
})
console.log(res.choices[0].message.content)`,
    anthropic: `import Anthropic from "@anthropic-ai/sdk"

const client = new Anthropic({
  apiKey: "${k}",
  baseURL: "${u}/v1",
})

const res = await client.messages.create({
  model: "claude-sonnet-4-5",
  max_tokens: 1024,
  messages: [{ role: "user", content: "你好" }],
})
console.log(res.content[0].text)`,
  }
})

const current = computed(() => snippets.value[active.value])

const copied = ref(false)
async function copyCode() {
  await navigator.clipboard.writeText(current.value)
  copied.value = true
  message.success('已复制示例', { duration: 1500 })
  setTimeout(() => { copied.value = false }, 1500)
}

// 测试连通
const testModel = ref('claude-sonnet-4-5')
const testPrompt = ref('你好，简单介绍一下你自己')
const testing = ref(false)
const testResult = ref<{
  shown: boolean
  ok?: boolean
  latencyMs?: number
  content?: string
  costUsd?: number
  inputTokens?: number
  outputTokens?: number
  error?: string
}>({ shown: false })

async function doTest() {
  // v7 guard: 防止用 placeholder "YOUR_API_KEY" 发请求导致诡异的 "key not found"
  if (!fullKey.value || fullKey.value === 'YOUR_API_KEY') {
    message.error('请先登录获取 API Key')
    testResult.value = { shown: true, ok: false, error: '未登录或 API Key 不可用，请先登录后再测试连通' }
    return
  }
  testing.value = true
  testResult.value = { shown: false }
  const start = performance.now()
  try {
    const res = await fetch(`${baseUrl.value}/v1/chat/completions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${fullKey.value}`,
      },
      body: JSON.stringify({
        model: testModel.value,
        messages: [{ role: 'user', content: testPrompt.value }],
        max_tokens: 256,
      }),
    })
    const elapsed = Math.round(performance.now() - start)
    if (!res.ok) {
      const text = await res.text()
      // 优先解析 JSON 错误体（PivotStack 标准错误格式），fallback 到 raw text
      let humanMsg = `HTTP ${res.status}`
      try {
        const j = JSON.parse(text)
        const m = j?.error?.message || j?.error || j?.message
        if (m) humanMsg += `: ${String(m).slice(0, 200)}`
        else humanMsg += `: ${text.slice(0, 200)}`
      } catch {
        humanMsg += `: ${text.slice(0, 200)}`
      }
      testResult.value = { shown: true, ok: false, latencyMs: elapsed, error: humanMsg }
      message.error('测试失败')
    } else {
      const data = await res.json()
      const content = data?.choices?.[0]?.message?.content || data?.content?.[0]?.text || ''
      const usage = data?.usage || {}
      testResult.value = {
        shown: true,
        ok: true,
        latencyMs: elapsed,
        content: String(content).slice(0, 600),
        inputTokens: usage.prompt_tokens || usage.input_tokens || 0,
        outputTokens: usage.completion_tokens || usage.output_tokens || 0,
      }
      message.success('测试成功')
    }
  } catch (e: any) {
    testResult.value = { shown: true, ok: false, error: String(e?.message || e) }
    message.error('请求失败')
  } finally {
    testing.value = false
  }
}

// v7.1：分组路由不再放这里。路由偏好在「我的 API Key」页面创建/编辑 key 时配置。
</script>

<template>
  <div class="api-docs stellar-scope">
    <!-- HERO -->
    <Tile class="api-hero">
      <div class="api-hero__row">
        <div class="t-label">你的 ENDPOINT</div>
        <MonoCopy :value="`${baseUrl}/v1`" size="lg" />
      </div>
      <div class="hairline"></div>
      <div class="api-hero__row">
        <div class="t-label">你的 API KEY</div>
        <div class="api-hero__key">
          <span class="t-hero-lg mono">{{ displayKey }}</span>
          <button class="btn btn--ghost btn--sm" @click="showKey = !showKey">
            <component :is="showKey ? EyeOff : Eye" :size="14" />
            {{ showKey ? '隐藏' : '显示完整' }}
          </button>
          <MonoCopy v-show="false" :value="fullKey" />
          <button
            class="btn btn--ghost btn--icon btn--sm"
            title="复制"
            @click="async () => { await navigator.clipboard.writeText(fullKey); message.success('已复制', { duration: 1500 }) }"
          >
            <Copy :size="13" />
          </button>
        </div>
      </div>
    </Tile>

    <div class="api-grid">
      <Tile>
        <div class="ctabs">
          <button
            v-for="t in tabs"
            :key="t.key"
            class="ctab"
            :class="{ 'is-active': active === t.key }"
            @click="active = t.key"
          >{{ t.label }}</button>
        </div>
        <div class="code-wrap">
          <button class="code-copy" @click="copyCode">
            <component :is="copied ? Check : Copy" :size="11" />
            {{ copied ? '已复制' : '复制' }}
          </button>
          <pre class="code-block code-block--full">{{ current }}</pre>
        </div>
      </Tile>

      <div class="grid-col">
        <Tile>
          <div class="tile__head"><span class="t-display">测试连通</span></div>
          <div class="kv">
            <span class="t-label">模型</span>
            <div class="st-input">
              <input v-model="testModel" class="mono" placeholder="claude-sonnet-4-5" />
            </div>
          </div>
          <div class="kv">
            <span class="t-label">prompt</span>
            <div class="st-input" style="height: auto; padding: 8px 12px">
              <textarea v-model="testPrompt" class="mono" rows="2" />
            </div>
          </div>
          <button class="btn btn--primary btn--block" :disabled="testing" @click="doTest">
            <Send :size="14" />{{ testing ? '发送中...' : '发送测试' }}
          </button>
          <div v-if="testResult.shown" class="test-result">
            <div class="hairline"></div>
            <div class="tok-row"><span class="t-label">延迟</span><span class="mono">{{ testResult.latencyMs }}ms</span></div>
            <div v-if="testResult.ok" class="tok-row"><span class="t-label">tokens</span><span class="mono">{{ testResult.inputTokens }} / {{ testResult.outputTokens }}</span></div>
            <pre v-if="testResult.ok" class="code-block">{{ testResult.content }}</pre>
            <pre v-else class="code-block" style="color: var(--st-error)">{{ testResult.error }}</pre>
          </div>
        </Tile>

      </div>
    </div>
  </div>
</template>

<style scoped>
.api-hero {
  background: rgba(255,255,255,0.02);
  border-radius: 8px;
  margin-bottom: 24px;
}
.api-hero__row { display: flex; flex-direction: column; gap: 8px; }
.api-hero__key { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.api-grid { display: grid; grid-template-columns: 4fr 3fr; gap: 24px; align-items: flex-start; }
@media (max-width: 1024px) {
  .api-grid { grid-template-columns: 1fr; }
}

.ctabs {
  display: flex; gap: 4px;
  border-bottom: 1px solid var(--st-border);
  margin: -24px -24px 16px;
  padding: 0 24px;
}
.ctab {
  height: 40px;
  padding: 0 12px;
  font-size: 12px;
  color: var(--st-text-sec);
  position: relative;
  border: none; background: transparent; cursor: pointer;
  font-family: inherit;
  transition: color 150ms ease;
}
.ctab:hover { color: var(--st-text-pri); }
.ctab.is-active { color: var(--st-text-pri); }
.ctab::after {
  content: ""; position: absolute; left: 12px; right: 12px; bottom: -1px;
  height: 2px; background: var(--st-success);
  transform: scaleX(0);
  transform-origin: center;
  transition: transform 200ms ease;
}
.ctab.is-active::after { transform: scaleX(1); }

.code-wrap { position: relative; }
.code-copy {
  position: absolute; top: 8px; right: 8px;
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 11px; color: var(--st-text-sec);
  padding: 4px 8px; border-radius: 2px;
  background: rgba(255,255,255,0.04);
  border: none; cursor: pointer; font-family: inherit;
  z-index: 2;
  transition: color 150ms ease, background 150ms ease;
}
.code-copy:hover { color: var(--st-text-pri); background: rgba(255,255,255,0.08); }

.kv { margin-bottom: 12px; display: flex; flex-direction: column; gap: 6px; }
.test-result { margin-top: 16px; }
.tok-row { display: flex; align-items: center; justify-content: space-between; height: 26px; font-size: 12px; color: var(--st-text-sec); }
.tok-row .mono { font-size: 12px; color: var(--st-text-pri); }

.model-table { font-size: 13px; }
.model-table__head, .model-table__row {
  display: flex; align-items: center; gap: 12px;
  height: 36px;
}
.model-table__head { color: var(--st-text-ter); font-size: 11px; letter-spacing: 0.06em; text-transform: uppercase; border-bottom: 1px solid var(--st-border); margin-bottom: 8px; }
</style>
