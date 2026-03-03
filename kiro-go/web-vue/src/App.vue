<script setup>
import { RouterView, useRoute, useRouter } from 'vue-router'
import { watchEffect } from 'vue'
import AppLayout from './components/AppLayout.vue'
import Toast from './components/ui/Toast.vue'
import { useAuthStore } from './stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

// 严格路由鉴权增强：当检测到未授权且访问受限路由时，强制拦截
watchEffect(() => {
  if (route.meta.auth && !auth.password) {
    router.replace('/login')
  }
})
</script>

<template>
  <!-- 只有在登录页或已通过鉴权的情况下才渲染 -->
  <template v-if="route.path === '/login' || auth.password">
    <AppLayout v-if="route.path !== '/login'">
      <RouterView />
    </AppLayout>
    <RouterView v-else />
  </template>
  
  <!-- 未鉴权且不在登录页时展示加载占位或留白，防止内容泄露 -->
  <div v-else class="min-h-screen bg-[#030712] flex items-center justify-center">
    <div class="w-12 h-12 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin"></div>
  </div>

  <Toast />
</template>
