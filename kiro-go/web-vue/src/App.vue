<script setup>
import { RouterView, useRoute } from 'vue-router'
import { onMounted, computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import AppLayout from './components/AppLayout.vue'
import Toast from './components/ui/Toast.vue'
import WorldTransition from './components/WorldTransition.vue'
import { useWorldTheme } from './stores/worldTheme'

const route = useRoute()
const router = useRouter()
const theme = useWorldTheme()
const ready = ref(false)

onMounted(async () => {
  document.documentElement.setAttribute('data-world', theme.currentWorld)
  await router.isReady()
  ready.value = true
})

const needsAdminLayout = computed(() => {
  const path = route.path
  return path !== '/login' && !path.startsWith('/user')
})
</script>

<template>
  <div class="abyss-layout min-h-screen">
    <a href="#main-content" class="skip-to-content">跳转到主内容</a>

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

    <div v-if="theme.currentWorld === 'daogui'" class="fixed inset-0 pointer-events-none z-0 overflow-hidden">
      <div class="absolute top-[-20%] right-[-10%] w-[60%] h-[60%] rounded-full bg-[#c41e3a] opacity-[0.04] blur-[120px] animate-blood-mist"></div>
      <div class="absolute bottom-[-15%] left-[-10%] w-[50%] h-[50%] rounded-full bg-[#4a1a4a] opacity-[0.06] blur-[100px] animate-blood-mist" style="animation-delay: -5s;"></div>
    </div>

    <WorldTransition :currentWorld="theme.currentWorld" />

    <!-- 等 Router 就绪后再渲染，防止闪烁 -->
    <template v-if="ready">
      <AppLayout v-if="needsAdminLayout">
        <RouterView />
      </AppLayout>
      <RouterView v-else />
    </template>

    <Toast />
  </div>
</template>
