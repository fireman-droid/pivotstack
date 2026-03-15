<script setup>
import { RouterView, useRoute, useRouter } from 'vue-router'
import { watchEffect, onMounted } from 'vue'
import AppLayout from './components/AppLayout.vue'
import Toast from './components/ui/Toast.vue'
import CopperCoinLoader from './components/ui/CopperCoinLoader.vue'
import WorldTransition from './components/WorldTransition.vue'
import { useAuthStore } from './stores/auth'
import { useWorldTheme } from './stores/worldTheme'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const theme = useWorldTheme()

// 初始化 data-world 属性
onMounted(() => {
  document.documentElement.setAttribute('data-world', theme.currentWorld)
})

watchEffect(() => {
  if (route.meta.auth && !auth.password) {
    router.replace('/login')
  }
})
</script>

<template>
  <div class="abyss-layout min-h-screen">
    <!-- 跳过导航链接（可访问性） -->
    <a href="#main-content" class="skip-to-content">
      跳转到主内容
    </a>

    <!-- SVG 滤镜定义（Gooey 边界溶解） -->
    <svg style="position: absolute; width: 0; height: 0;">
      <defs>
        <filter id="gooey-filter">
          <feGaussianBlur in="SourceGraphic" stdDeviation="10" result="blur" />
          <feColorMatrix in="blur" mode="matrix" values="
            1 0 0 0 0
            0 1 0 0 0
            0 0 1 0 0
            0 0 0 18 -7
          " result="gooey" />
          <feComposite in="SourceGraphic" in2="gooey" operator="atop" />
        </filter>
      </defs>
    </svg>

    <!-- 血雾背景层（仅 daogui 模式） -->
    <div v-if="theme.currentWorld === 'daogui'" class="fixed inset-0 pointer-events-none z-0 overflow-hidden">
      <div class="absolute top-[-20%] right-[-10%] w-[60%] h-[60%] rounded-full bg-[#c41e3a] opacity-[0.04] blur-[120px] animate-blood-mist"></div>
      <div class="absolute bottom-[-15%] left-[-10%] w-[50%] h-[50%] rounded-full bg-[#4a1a4a] opacity-[0.06] blur-[100px] animate-blood-mist" style="animation-delay: -5s;"></div>
    </div>

    <!-- 世界过渡动画 -->
    <WorldTransition :currentWorld="theme.currentWorld" />

    <!-- 用户端 (不需要管理权限) -->
    <template v-if="route.path.startsWith('/user')">
      <RouterView />
    </template>

    <!-- 已授权 (管理端) -->
    <template v-else-if="route.path === '/login' || auth.password">
      <AppLayout v-if="route.path !== '/login'">
        <RouterView />
      </AppLayout>
      <RouterView v-else />
    </template>

    <!-- 未授权：铜钱加载 -->
    <div v-else class="min-h-screen flex items-center justify-center relative z-10">
      <CopperCoinLoader />
    </div>

    <Toast />
  </div>
</template>
