<script setup lang="ts">
// 通用 tile（无边框，靠空白和字色分层）。slot 完全自由。
</script>

<template>
  <div class="tile"><slot /></div>
</template>

<style>
.tile {
  position: relative;
  padding: 24px;
  transition: background 150ms ease;
}
.tile__head { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.tile__head--split { justify-content: space-between; align-items: flex-end; gap: 16px; }
.tile__head-right { text-align: right; }
.tile--wide { margin-bottom: 0; }
.tile--notice { padding-top: 0; }
.tile--notice .hairline { margin-bottom: 12px; }
.hairline { height: 1px; background: rgba(255,255,255,0.08); margin: 16px 0; }
@media (max-width: 768px) {
  .tile { padding: 16px; }
}
@media (max-width: 640px) {
  .tile { padding: 12px; }
}
</style>
