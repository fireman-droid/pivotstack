<script setup lang="ts">
import { computed, ref } from 'vue'
import { NModal, NButton, NSpace, useMessage } from 'naive-ui'
import { Copy, Check, Download, AlertTriangle } from 'lucide-vue-next'

const props = defineProps<{
  show: boolean
  codes: string[]
  meta: { type: string; amount: number; tier?: string; salePriceCNY?: number } | null
}>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const message = useMessage()
const copied = ref(false)

const csv = computed(() => props.codes.join('\n'))
const summaryText = computed(() => {
  const m = props.meta
  if (!m) return ''
  if (m.type === 'balance') return `余额型 · 单张 ¥${m.amount.toFixed(2)} · ${props.codes.length} 张`
  const unit = m.type === 'days' ? '天' : '秒'
  const sale = m.salePriceCNY ? ` · 售价 ¥${m.salePriceCNY.toFixed(2)}/张` : ''
  return `${m.tier?.toUpperCase()} ${unit}卡 · 单张 ${m.amount} ${unit} · ${props.codes.length} 张${sale}`
})

async function copyAll() {
  await navigator.clipboard.writeText(csv.value)
  copied.value = true
  message.success(`已复制 ${props.codes.length} 张激活码`)
  setTimeout(() => { copied.value = false }, 1500)
}

function downloadCsv() {
  const blob = new Blob([csv.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  const ts = new Date().toISOString().slice(0, 16).replace(/[:T]/g, '-')
  a.href = url
  a.download = `codes-${props.meta?.type || 'batch'}-${ts}.txt`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :style="{ width: '560px' }"
    :mask-closable="false"
    :closable="false"
    title="激活码已生成"
  >
    <div class="warn">
      <AlertTriangle :size="16" />
      <span>请立即复制或下载保存，关闭后激活码会回到列表，但批量获取就只能逐条点开。</span>
    </div>

    <div class="meta">{{ summaryText }}</div>

    <textarea readonly class="codes" :value="csv" />

    <template #footer>
      <n-space justify="space-between" :size="8">
        <n-button quaternary @click="emit('update:show', false)">关闭</n-button>
        <n-space :size="8">
          <n-button quaternary @click="downloadCsv">
            <template #icon><Download :size="14" /></template>
            下载 .txt
          </n-button>
          <n-button type="primary" @click="copyAll">
            <template #icon>
              <Check v-if="copied" :size="14" />
              <Copy v-else :size="14" />
            </template>
            {{ copied ? '已复制' : '复制全部' }}
          </n-button>
        </n-space>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.warn {
  display: flex; align-items: flex-start; gap: 8px;
  padding: 10px 12px;
  background: rgba(245, 166, 35, 0.08);
  border: 1px solid rgba(245, 166, 35, 0.30);
  border-radius: 6px;
  color: #f5a623;
  font-size: 12px;
  margin-bottom: 14px;
  line-height: 1.5;
}
.warn :first-child { flex-shrink: 0; margin-top: 1px; }

.meta {
  color: #a1a1a1; font-size: 12px;
  margin-bottom: 10px;
}

.codes {
  width: 100%;
  box-sizing: border-box;
  height: 240px;
  padding: 12px 14px;
  background: #0a0a0a;
  border: 1px solid rgba(255,255,255,0.10);
  border-radius: 6px;
  color: #ededed;
  font-family: "Geist Mono", ui-monospace, monospace;
  font-size: 13px;
  line-height: 1.6;
  resize: none;
  outline: none;
}
.codes:focus { border-color: rgba(255,255,255,0.20); }
</style>
