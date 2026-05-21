<script setup lang="ts">
import { onMounted } from 'vue'
import AdminRail from '../components/admin/AdminRail.vue'
import AdminTree from '../components/admin/AdminTree.vue'
import Starfield from '../components/user/stellar/Starfield.vue'

onMounted(() => {
  // 把 admin 专属 vignette class 加到 starfield 的 vignette 元素上
  const v = document.querySelector('.starfield__vignette')
  if (v && !v.classList.contains('starfield__vignette--admin')) {
    v.classList.add('starfield__vignette--admin')
  }
})
</script>

<template>
  <div class="admin-shell">
    <Starfield />
    <AdminRail />
    <AdminTree />
    <main class="admin-main">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.admin-shell {
  display: flex;
  height: 100vh;
  color: #ededed;
  overflow: hidden;
  background: transparent;
}
.admin-main {
  flex: 1;
  min-width: 0;
  height: 100vh;
  overflow-y: auto;
  overflow-x: hidden;
  background: transparent;
  position: relative;
  z-index: 1; /* 在 starfield 之上 */
}
.admin-main::-webkit-scrollbar { width: 6px; }
.admin-main::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.10);
  border-radius: 3px;
}
.admin-main::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.20);
}
</style>
