<script setup lang="ts">
// PivotStack 统一右侧 Drawer 壳。视觉与 RefinedDialog 同源，但右侧滑入。
// 设计原则：
//   - 背景 #0a0a0a，左侧 1px border + 微阴影
//   - 顶部条：icon + 标题 + 副标题 + 右上 close（与 RefinedDialog 一致）
//   - 内容区滚动，padding 20px 24px
//   - 底部按钮区 sticky，顶部分隔线
//   - 按钮 hover 用 rgba 灰，禁纯白
// 用法：
//   <RefinedDrawer v-model:show :title :subtitle :icon="Pencil" :width="520" :loading="submitting">
//     <RefinedField label="备注" required>
//       <n-input v-model:value="..." />
//     </RefinedField>
//     <template #footer>
//       <n-button quaternary @click="close">取消</n-button>
//       <n-button type="primary" :loading="submitting" @click="submit">保存</n-button>
//     </template>
//   </RefinedDrawer>

import { computed, type Component } from 'vue'
import { NDrawer } from 'naive-ui'
import { X } from 'lucide-vue-next'

const props = defineProps<{
  show: boolean
  title: string
  subtitle?: string
  /** 顶部 icon 组件（lucide-vue-next）；不传则不显示 */
  icon?: Component
  /** drawer 宽度，默认 520px */
  width?: number | string
  /** loading 时禁用 close（防止表单提交途中被关闭） */
  loading?: boolean
  /** 点 mask 是否能关闭（默认 true，符合用户预期；表单 drawer 可显式传 false 防止误关丢输入） */
  maskClosable?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
}>()

const widthValue = computed(() => props.width ?? 520)

function close() {
  if (props.loading) return
  emit('update:show', false)
}
</script>

<template>
  <n-drawer
    :show="show"
    :width="widthValue"
    placement="right"
    :mask-closable="maskClosable ?? true"
    :auto-focus="false"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <div class="rdr">
      <header class="rdr__head">
        <div class="rdr__head-left">
          <component :is="icon" v-if="icon" :size="16" class="rdr__icon" />
          <div class="rdr__titles">
            <h2 class="rdr__title">{{ title }}</h2>
            <p v-if="subtitle" class="rdr__subtitle">{{ subtitle }}</p>
          </div>
        </div>
        <button class="rdr__close" :disabled="loading" @click="close" aria-label="关闭">
          <X :size="16" />
        </button>
      </header>
      <div class="rdr__body">
        <slot />
      </div>
      <footer v-if="$slots.footer" class="rdr__foot">
        <slot name="footer" />
      </footer>
    </div>
  </n-drawer>
</template>

<style scoped>
.rdr {
  height: 100%;
  background: #0a0a0a;
  display: flex;
  flex-direction: column;
  font-family: "Geist", system-ui, -apple-system, sans-serif;
}

.rdr__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 20px 24px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;
}
.rdr__head-left {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  flex: 1;
  min-width: 0;
}
.rdr__icon {
  color: #a3a3a3;
  margin-top: 3px;
  flex-shrink: 0;
}
.rdr__titles {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.rdr__title {
  margin: 0;
  color: #ededed;
  font-size: 15px;
  font-weight: 500;
  letter-spacing: -0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rdr__subtitle {
  margin: 0;
  color: #707070;
  font-size: 12px;
  line-height: 1.4;
}

.rdr__close {
  background: transparent;
  border: none;
  color: #707070;
  width: 28px;
  height: 28px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
  flex-shrink: 0;
}
.rdr__close:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.06);
  color: #ededed;
}
.rdr__close:focus-visible {
  outline: 2px solid var(--st-primary, #52a8ff);
  outline-offset: 1px;
}
.rdr__close:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.rdr__body {
  flex: 1;
  overflow-y: auto;
  padding: 22px 24px;
}
.rdr__body::-webkit-scrollbar {
  width: 6px;
}
.rdr__body::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.06);
  border-radius: 3px;
}
.rdr__body::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.1);
}

.rdr__foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 22px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(0, 0, 0, 0.3);
  flex-shrink: 0;
}
</style>
