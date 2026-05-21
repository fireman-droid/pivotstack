<script setup>
// 最小 Switch shim：legacy view (Accounts/Settings/Stealth) 还在用。
// v6 view 全部用 naive-ui NSwitch，不应再 import 此文件。
const props = defineProps({ modelValue: { type: Boolean, default: false } })
const emit = defineEmits(['update:modelValue'])
function toggle() { emit('update:modelValue', !props.modelValue) }
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    class="switch"
    :class="{ 'switch--on': modelValue }"
    @click="toggle"
  >
    <span class="switch__thumb" />
  </button>
</template>

<style scoped>
.switch {
  position: relative;
  width: 34px;
  height: 18px;
  border-radius: 999px;
  border: 1px solid rgba(255,255,255,0.12);
  background: rgba(255,255,255,0.06);
  cursor: pointer;
  padding: 0;
  transition: background 150ms;
}
.switch--on { background: #0bd470; border-color: #0bd470; }
.switch__thumb {
  position: absolute;
  top: 1px; left: 1px;
  width: 14px; height: 14px;
  border-radius: 999px;
  background: #ededed;
  transition: transform 150ms;
}
.switch--on .switch__thumb { transform: translateX(16px); background: #000; }
</style>
