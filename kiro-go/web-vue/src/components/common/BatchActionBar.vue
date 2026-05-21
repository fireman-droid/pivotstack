<script setup lang="ts">
import { NButton } from 'naive-ui'
import { X } from 'lucide-vue-next'

defineProps<{ count: number }>()
const emit = defineEmits<{ (e: 'clear'): void }>()
</script>

<template>
  <Transition name="slide-down">
    <div v-if="count > 0" class="bar">
      <div class="bar__left">
        <span class="bar__count">已选 <b>{{ count }}</b> 条</span>
        <n-button size="tiny" quaternary @click="emit('clear')">
          <template #icon><X :size="13" /></template>
          取消选择
        </n-button>
      </div>
      <div class="bar__actions">
        <slot />
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 14px;
  margin-bottom: 12px;
  background: linear-gradient(180deg, rgba(82, 168, 255, 0.08), rgba(82, 168, 255, 0.03));
  border: 1px solid rgba(82, 168, 255, 0.30);
  border-radius: 6px;
}
.bar__left { display: flex; align-items: center; gap: 12px; }
.bar__count { font-size: 13px; color: #ededed; }
.bar__count b { color: #52a8ff; font-weight: 600; }
.bar__actions { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }

.slide-down-enter-active,
.slide-down-leave-active { transition: all 200ms cubic-bezier(0.16, 1, 0.3, 1); }
.slide-down-enter-from,
.slide-down-leave-to { opacity: 0; transform: translateY(-6px); }
</style>
