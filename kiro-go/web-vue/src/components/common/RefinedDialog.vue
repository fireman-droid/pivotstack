<script setup lang="ts">
// PivotStack 统一弹窗壳。
// 设计原则：
//   - 圆角 10px，深色背景 #0a0a0a，边框 1px 微发光
//   - 顶部条：icon + 标题 + 副标题 + 右上 close
//   - 内容区滚动，padding 20px 24px
//   - 底部按钮区 sticky，顶部分隔线
//   - 按钮 hover 用 rgba 灰，禁纯白
// 用法：<RefinedDialog v-model:show :title :subtitle> ...form... <template #footer>...</> </>

import { computed, type Component } from 'vue'
import { NModal, NButton } from 'naive-ui'
import { X } from 'lucide-vue-next'

const props = defineProps<{
  show: boolean
  title: string
  subtitle?: string
  // 顶部 icon 组件（lucide-vue-next）；不传则不显示
  icon?: Component
  // dialog 宽度，默认 560px
  width?: number | string
  maskClosable?: boolean
  closable?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
}>()

const widthStyle = computed(() => {
  const w = props.width ?? 560
  return typeof w === 'number' ? `${w}px` : w
})

function close() {
  if (props.closable !== false) emit('update:show', false)
}
</script>

<template>
  <n-modal
    :show="show"
    @update:show="(v: boolean) => emit('update:show', v)"
    :mask-closable="maskClosable ?? true"
    :transform-origin="'center'"
  >
    <div class="rd" :style="{ width: widthStyle }">
      <header class="rd__head">
        <div class="rd__head-left">
          <component :is="icon" v-if="icon" :size="16" class="rd__icon" />
          <div class="rd__titles">
            <h2 class="rd__title">{{ title }}</h2>
            <p v-if="subtitle" class="rd__subtitle">{{ subtitle }}</p>
          </div>
        </div>
        <button v-if="closable !== false" class="rd__close" @click="close" aria-label="关闭">
          <X :size="16" />
        </button>
      </header>
      <div class="rd__body">
        <slot />
      </div>
      <footer v-if="$slots.footer" class="rd__foot">
        <slot name="footer" />
      </footer>
    </div>
  </n-modal>
</template>

<style scoped>
.rd {
  background: #0a0a0a;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  box-shadow:
    0 18px 48px rgba(0, 0, 0, 0.45),
    0 0 0 1px rgba(255, 255, 255, 0.02) inset;
  display: flex;
  flex-direction: column;
  max-height: 90vh;
  overflow: hidden;
  font-family: "Geist", system-ui, -apple-system, sans-serif;
}
.rd__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 22px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.rd__head-left { display: flex; align-items: flex-start; gap: 10px; flex: 1; min-width: 0; }
.rd__icon { color: #a3a3a3; margin-top: 2px; flex-shrink: 0; }
.rd__titles { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.rd__title { margin: 0; color: #ededed; font-size: 15px; font-weight: 500; letter-spacing: -0.01em; }
.rd__subtitle { margin: 0; color: #707070; font-size: 12px; line-height: 1.4; }
.rd__close {
  background: transparent;
  border: none;
  color: #707070;
  width: 26px; height: 26px;
  border-radius: 5px;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
  flex-shrink: 0;
}
.rd__close:hover { background: rgba(255, 255, 255, 0.06); color: #ededed; }
.rd__body {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}
.rd__foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 22px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(0, 0, 0, 0.3);
}
</style>
