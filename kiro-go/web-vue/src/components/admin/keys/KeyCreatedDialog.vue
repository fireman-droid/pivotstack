<script setup lang="ts">
import { computed } from 'vue'
import { NModal, NButton, NSpace, useMessage } from 'naive-ui'
import { Copy, Check, AlertTriangle } from 'lucide-vue-next'
import type { ApiKeyRow } from '../../../api/admin/keys'
import { ref } from 'vue'

const props = defineProps<{ show: boolean; row: ApiKeyRow | null }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const message = useMessage()
const copied = ref(false)

const fullKey = computed(() => props.row?.key || '')

async function copyKey() {
  if (!fullKey.value) return
  await navigator.clipboard.writeText(fullKey.value)
  copied.value = true
  message.success('已复制到剪贴板')
  setTimeout(() => { copied.value = false }, 1500)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :style="{ width: '520px' }"
    :mask-closable="false"
    :closable="false"
    title="API Key 已创建"
  >
    <div class="warn">
      <AlertTriangle :size="16" />
      <span>这是唯一一次能看到完整 Key 的机会，请立即复制保存。关闭后只能看到掩码。</span>
    </div>

    <section class="key-box">
      <div class="key-box__label">完整 Key（仅本次显示）</div>
      <div class="key-box__value">{{ fullKey }}</div>
    </section>

    <dl class="meta">
      <div class="meta__row">
        <dt>备注</dt>
        <dd>{{ row?.note || '-' }}</dd>
      </div>
      <div class="meta__row">
        <dt>Key ID</dt>
        <dd class="mono">{{ row?.id }}</dd>
      </div>
    </dl>

    <template #footer>
      <n-space justify="end" :size="8">
        <n-button quaternary @click="emit('update:show', false)">我已保存</n-button>
        <n-button type="primary" @click="copyKey">
          <template #icon>
            <Check v-if="copied" :size="14" />
            <Copy v-else :size="14" />
          </template>
          {{ copied ? '已复制' : '复制 Key' }}
        </n-button>
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
  margin-bottom: 16px;
  line-height: 1.5;
}
.warn :first-child { flex-shrink: 0; margin-top: 1px; }

.key-box {
  background: #0a0a0a;
  border: 1px solid rgba(255,255,255,0.10);
  border-radius: 6px;
  padding: 14px 16px;
  margin-bottom: 16px;
}
.key-box__label { font-size: 11px; color: #707070; text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 8px; }
.key-box__value {
  font-family: "Geist Mono", ui-monospace, monospace;
  font-size: 13px;
  color: #ededed;
  word-break: break-all;
  user-select: all;
  line-height: 1.5;
}

.meta { margin: 0; display: flex; flex-direction: column; gap: 6px; }
.meta__row { display: grid; grid-template-columns: 80px 1fr; font-size: 13px; }
.meta__row dt { color: #707070; }
.meta__row dd { margin: 0; color: #ededed; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; }
</style>
