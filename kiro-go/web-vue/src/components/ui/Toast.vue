<script setup>
import { useToast } from '../../composables/useToast'
const { toasts } = useToast()
</script>

<template>
  <div class="fixed top-4 right-4 z-50 flex flex-col gap-2">
    <TransitionGroup name="toast">
      <div v-for="t in toasts" :key="t.id"
        class="px-4 py-3 rounded-lg shadow-lg text-sm font-medium max-w-sm"
        :class="{
          'bg-green-500 text-white': t.type === 'success',
          'bg-red-500 text-white': t.type === 'error',
          'bg-violet-600 text-white': t.type === 'info',
        }">
        {{ t.message }}
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active { transition: all .3s ease; }
.toast-leave-active { transition: all .3s ease; }
.toast-enter-from { opacity: 0; transform: translateX(30px); }
.toast-leave-to { opacity: 0; transform: translateX(30px); }
</style>
