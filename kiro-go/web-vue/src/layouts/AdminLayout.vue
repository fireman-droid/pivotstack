<script setup lang="ts">
import AdminSidebar from '../components/admin/AdminSidebar.vue'
import AdminTopbar from '../components/admin/AdminTopbar.vue'
</script>

<template>
  <div class="admin-layout">
    <AdminSidebar />
    <div class="admin-layout__main">
      <AdminTopbar />
      <main class="admin-layout__content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin-layout {
  display: flex;
  height: 100vh;
  background: #000000;
  color: #ededed;
  overflow: hidden;
}
.admin-layout__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow-y: auto;   /* 主区独立滚动；sidebar 静止；topbar 内部 sticky */
  overflow-x: hidden;
}
.admin-layout__content {
  flex: 1;
}

/* 主区滚动条 */
.admin-layout__main::-webkit-scrollbar { width: 6px; }
.admin-layout__main::-webkit-scrollbar-track { background: transparent; }
.admin-layout__main::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.10);
  border-radius: 3px;
}
.admin-layout__main::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.20);
}
</style>
