<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { Search } from 'lucide-vue-next'
import { adminSidebar } from '../../design/sidebar'

const route = useRoute()
const search = ref('')

// 面包屑：根据当前 route.path 找匹配的 sidebar item label。
const currentLabel = computed(() => {
  const path = route.path
  let best = ''
  let bestLabel = ''
  for (const item of adminSidebar) {
    const base = item.to.split('?')[0]
    if (path === base || path.startsWith(base + '/')) {
      if (base.length > best.length) {
        best = base
        bestLabel = item.label
      }
    }
  }
  return bestLabel || '控制台'
})
</script>

<template>
  <header class="topbar">
    <!-- 左：面包屑 -->
    <nav class="crumbs" aria-label="面包屑">
      <RouterLink to="/overview" class="crumbs__home">控制台</RouterLink>
      <span class="crumbs__sep" aria-hidden="true">/</span>
      <span class="crumbs__current">{{ currentLabel }}</span>
    </nav>

    <!-- 右：搜索 + 状态 -->
    <div class="actions">
      <label class="search">
        <Search :size="14" class="search__icon" aria-hidden="true" />
        <input
          v-model="search"
          type="text"
          class="search__input"
          placeholder="搜索 Key / 渠道 / 模型"
          aria-label="搜索"
        />
        <kbd class="search__hint">⌘K</kbd>
      </label>
      <span class="status">
        <span class="status__dot" />
        <span class="status__text">在线</span>
      </span>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: 56px;
  padding: 0 24px;
  background: #000000;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  position: sticky;
  top: 0;
  z-index: 100;
  font-size: 13px;
}

/* ===== 面包屑 ===== */
.crumbs {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #707070;
}
.crumbs__home {
  color: #707070;
  text-decoration: none;
  transition: color 150ms;
}
.crumbs__home:hover { color: #a1a1a1; }
.crumbs__sep {
  color: #4d4d4d;
}
.crumbs__current {
  color: #ededed;
  font-weight: 500;
}

/* ===== 右侧 actions ===== */
.actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 搜索框 */
.search {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  width: 280px;
  transition: border-color 150ms, background 150ms;
}
.search:focus-within {
  border-color: rgba(255, 255, 255, 0.20);
  background: rgba(255, 255, 255, 0.06);
}
.search__icon {
  color: #707070;
  flex-shrink: 0;
}
.search__input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: #ededed;
  font-size: 13px;
  min-width: 0;
}
.search__input::placeholder {
  color: #4d4d4d;
}
.search__hint {
  font-family: "Geist Mono", ui-monospace, monospace;
  font-size: 11px;
  color: #707070;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 3px;
  padding: 1px 5px;
  font-weight: 500;
}

/* 在线状态 */
.status {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  background: rgba(11, 212, 112, 0.08);
  border: 1px solid rgba(11, 212, 112, 0.18);
  border-radius: 6px;
  font-size: 12px;
  color: #0bd470;
}
.status__dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: #0bd470;
  box-shadow: 0 0 6px rgba(11, 212, 112, 0.6);
  animation: pulse 2.4s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

@media (prefers-reduced-motion: reduce) {
  .status__dot { animation: none; }
}

@media (max-width: 768px) {
  .search { width: 180px; }
}
</style>
