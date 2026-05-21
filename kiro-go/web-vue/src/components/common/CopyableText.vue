<script setup lang="ts">
import { ref, computed } from 'vue'
import { useMessage } from 'naive-ui'

const props = defineProps<{
  text: string
  mono?: boolean
  mask?: boolean      // 视觉上把中间部分换成 ••• ，复制仍是完整 text
  maskHead?: number   // 头保留几位（默认 4）
  maskTail?: number   // 尾保留几位（默认 4）
}>()

const message = useMessage()
const copied = ref(false)

const display = computed(() => {
  if (!props.mask || props.text.length <= (props.maskHead ?? 4) + (props.maskTail ?? 4) + 3) {
    return props.text
  }
  const h = props.maskHead ?? 4
  const t = props.maskTail ?? 4
  return `${props.text.slice(0, h)}•••${props.text.slice(-t)}`
})

async function copy() {
  try {
    await navigator.clipboard.writeText(props.text)
    copied.value = true
    message?.success?.('已复制')
    setTimeout(() => { copied.value = false }, 1200)
  } catch (e) {
    message?.error?.('复制失败')
  }
}
</script>

<template>
  <span class="copyable" :class="{ 'copyable--mono': mono }" @click="copy" :title="text">
    {{ display }}
    <span class="copyable__icon">{{ copied ? '✓' : '⧉' }}</span>
  </span>
</template>

<style scoped>
.copyable {
  display: inline-flex; align-items: center; gap: 6px;
  cursor: pointer; user-select: text;
  border-radius: 4px; padding: 1px 4px;
  transition: background-color 150ms cubic-bezier(0.4, 0, 0.2, 1);
}
.copyable:hover { background: rgba(255,255,255,0.05); }
.copyable--mono { font-family: 'Geist Mono', 'JetBrains Mono', ui-monospace, monospace; font-size: 13px; }
.copyable__icon {
  font-size: 11px; color: var(--color-text-tertiary, #707070);
  opacity: 0; transition: opacity 150ms;
}
.copyable:hover .copyable__icon { opacity: 1; }
</style>
