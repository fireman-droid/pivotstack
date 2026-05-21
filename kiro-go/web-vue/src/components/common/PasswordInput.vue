<script setup lang="ts">
// 密码输入组件：实时弱口令提示。前端不做硬校验（最终由后端 config.ValidateStrongPassword 决定），
// 但给用户视觉反馈避免提交后才被 reject。
import { computed, ref } from 'vue'
import { NInput } from 'naive-ui'
import { Eye, EyeOff } from 'lucide-vue-next'

const props = defineProps<{
  modelValue: string
  placeholder?: string
  minLen?: number
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
}>()

const show = ref(false)

// 后端 weakPasswordDict 的前缀；前端只挑出最常见几个做即时反馈，权威判断仍在后端。
const commonPasswords = new Set([
  '12345678', '123456789', 'password', 'password1', 'password123',
  'qwerty', 'qwerty123', 'admin123', 'iloveyou', 'letmein',
])

const strength = computed(() => {
  const v = props.modelValue
  if (!v) return { score: 0, hint: '', color: '#4d4d4d' }

  if (v.length < (props.minLen ?? 8)) return { score: 1, hint: '太短', color: '#ff4d4d' }

  const cats =
    (/[a-z]/.test(v) ? 1 : 0) +
    (/[A-Z]/.test(v) ? 1 : 0) +
    (/[0-9]/.test(v) ? 1 : 0) +
    (/[^a-zA-Z0-9]/.test(v) ? 1 : 0)

  if (cats < 2) return { score: 2, hint: '需要至少 2 类字符', color: '#f5a623' }
  if (commonPasswords.has(v.toLowerCase())) return { score: 1, hint: '太常见', color: '#ff4d4d' }
  if (cats < 3) return { score: 3, hint: '中等', color: '#52a8ff' }
  return { score: 4, hint: '强', color: '#0bd470' }
})
</script>

<template>
  <div class="pwd-input">
    <NInput
      :value="modelValue"
      :type="show ? 'text' : 'password'"
      :placeholder="placeholder || '请输入密码'"
      @update:value="(v) => emit('update:modelValue', v)"
    >
      <template #suffix>
        <button type="button" class="pwd-eye" :title="show ? '隐藏' : '显示'" @click="show = !show">
          <Eye v-if="!show" :size="14" />
          <EyeOff v-else :size="14" />
        </button>
      </template>
    </NInput>

    <div v-if="modelValue" class="pwd-meta">
      <div class="pwd-bar">
        <div
          class="pwd-bar-fill"
          :style="{ width: strength.score * 25 + '%', backgroundColor: strength.color }"
        />
      </div>
      <div class="pwd-hint" :style="{ color: strength.color }">{{ strength.hint }}</div>
    </div>
  </div>
</template>

<style scoped>
.pwd-input { width: 100%; position: relative; }
.pwd-eye {
  background: transparent;
  border: none;
  color: #707070;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
  border-radius: 3px;
  transition: color 150ms ease, background 150ms ease;
}
.pwd-eye:hover { color: #ededed; background: rgba(255, 255, 255, 0.06); }

.pwd-meta { margin-top: 6px; }
.pwd-bar {
  height: 2px;
  width: 100%;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 1px;
  overflow: hidden;
}
.pwd-bar-fill { height: 100%; transition: width 0.3s ease, background-color 0.3s ease; }
.pwd-hint {
  font-size: 11px;
  margin-top: 4px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
</style>
