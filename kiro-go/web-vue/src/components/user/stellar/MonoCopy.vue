<script setup lang="ts">
import { ref } from 'vue'
import { Copy, Check } from 'lucide-vue-next'
import { useMessage } from 'naive-ui'

const props = defineProps<{
  value: string
  display?: string
  size?: 'sm' | 'lg'
}>()

const message = useMessage()
const copied = ref(false)
let resetTimer: number | undefined

async function doCopy() {
  try {
    await navigator.clipboard.writeText(props.value)
    copied.value = true
    message.success('已复制', { duration: 1500 })
    if (resetTimer) window.clearTimeout(resetTimer)
    resetTimer = window.setTimeout(() => { copied.value = false }, 1500)
  } catch {
    message.error('复制失败，请手动选中复制')
  }
}
</script>

<template>
  <div class="mono-value" :class="size === 'lg' ? 'mono-value--lg' : ''">
    <span>{{ display || value }}</span>
    <button class="copy-btn" :class="{ 'is-copied': copied }" @click="doCopy">
      <Check v-if="copied" :size="11" />
      <Copy v-else :size="11" />
    </button>
  </div>
</template>

<style>
.mono-value {
  display: inline-flex; align-items: center; gap: 8px;
  font-family: "Geist Mono", ui-monospace, monospace;
  font-variant-numeric: tabular-nums;
  font-size: 13px; color: #ededed;
  background: rgba(255,255,255,0.04);
  padding: 6px 10px;
  border-radius: 4px;
  max-width: 100%;
  transition: background 150ms ease;
}
.mono-value:hover { background: rgba(255,255,255,0.08); }
.mono-value > span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mono-value--lg { font-size: 24px; padding: 10px 14px; }
.mono-value--lg > span { font-weight: 500; letter-spacing: -0.01em; }
.copy-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 22px; height: 22px;
  color: #a1a1a1;
  border: none; background: transparent;
  border-radius: 2px;
  cursor: pointer;
  transition: color 150ms ease, background 150ms ease, transform 400ms ease;
}
.copy-btn:hover { color: #ededed; background: rgba(255,255,255,0.06); }
.copy-btn.is-copied { color: #0bd470; transform: rotate(360deg); }
</style>
