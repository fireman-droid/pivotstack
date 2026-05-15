<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import WorldInput from './WorldInput.vue'
import { Eye, EyeOff, Lock } from 'lucide-vue-next'

defineProps({
  modelValue: { type: String, default: '' },
  label: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  error: { type: String, default: '' },
  hint: { type: String, default: '' },
  size: { type: String, default: 'md' },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'enter'])

const showPassword = ref(false)
const capsLockActive = ref(false)

function checkCapsLock(e) {
  if (e && typeof e.getModifierState === 'function') {
    capsLockActive.value = e.getModifierState('CapsLock')
  }
}

onMounted(() => {
  window.addEventListener('keydown', checkCapsLock)
  window.addEventListener('keyup', checkCapsLock)
})
onUnmounted(() => {
  window.removeEventListener('keydown', checkCapsLock)
  window.removeEventListener('keyup', checkCapsLock)
})
</script>

<template>
  <div class="world-password-field">
    <WorldInput
      :modelValue="modelValue"
      @update:modelValue="emit('update:modelValue', $event)"
      :type="showPassword ? 'text' : 'password'"
      :label="label"
      :placeholder="placeholder"
      :error="error"
      :hint="capsLockActive ? '大写锁定已开启' : hint"
      :size="size"
      :disabled="disabled"
      :monospace="true"
      @enter="emit('enter')"
    >
      <template #prefix>
        <Lock :size="14" />
      </template>
      <template #append>
        <button
          type="button"
          class="eye-btn"
          tabindex="-1"
          :aria-label="showPassword ? '隐藏密码' : '显示密码'"
          :title="showPassword ? '隐藏密码' : '显示密码'"
          @click="showPassword = !showPassword"
        >
          <Eye v-if="!showPassword" :size="16" />
          <EyeOff v-else :size="16" />
        </button>
      </template>
    </WorldInput>
  </div>
</template>

<style scoped>
.world-password-field {
  width: 100%;
}
.eye-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 28px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 6px;
  color: var(--world-text-mute, #94a3b8);
  cursor: pointer;
  transition: color 220ms, background 220ms;
}
.eye-btn:hover {
  color: var(--world-accent, #3b82f6);
  background: var(--world-bg-soft, rgba(148, 163, 184, 0.08));
}
.eye-btn:focus-visible {
  outline: 2px solid var(--world-accent, #3b82f6);
  outline-offset: 2px;
}
</style>
