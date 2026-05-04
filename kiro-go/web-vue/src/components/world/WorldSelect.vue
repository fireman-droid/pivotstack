<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { ChevronDown, Check, Search, X } from 'lucide-vue-next'

const props = defineProps({
  modelValue: { type: [String, Number, Boolean], default: '' },
  options: { type: Array, required: true },   // [{ value, label, hint? }]
  placeholder: { type: String, default: '请选择' },
  size: { type: String, default: 'sm' },      // sm | md
  searchable: { type: Boolean, default: false },
  maxHeight: { type: String, default: '320px' },
  align: { type: String, default: 'start' },  // start | end
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'change'])

const open = ref(false)
const triggerEl = ref(null)
const panelEl = ref(null)
const searchQuery = ref('')
const highlightIndex = ref(-1)

// 浮层定位：通过 Teleport 到 body + position: fixed，避免被父容器 overflow 裁切
const panelStyle = ref({})
function computePanelPosition() {
  const t = triggerEl.value
  if (!t) return
  const r = t.getBoundingClientRect()
  const vw = window.innerWidth
  const vh = window.innerHeight
  const margin = 8
  const minWidth = Math.max(r.width, 200)
  const maxWidth = 360
  const width = Math.min(maxWidth, Math.max(minWidth, r.width))

  // 水平：默认对齐 trigger 左边；align=end 时对齐右边；超出视口则反向
  let left
  if (props.align === 'end') {
    left = r.right - width
  } else {
    left = r.left
  }
  if (left + width > vw - margin) left = vw - width - margin
  if (left < margin) left = margin

  // 垂直：默认 trigger 下方；下方空间不足时弹到上方
  const panelMaxH = parseInt(props.maxHeight) || 320
  const spaceBelow = vh - r.bottom - margin
  const spaceAbove = r.top - margin
  let top, computedMaxH
  if (spaceBelow >= panelMaxH || spaceBelow >= spaceAbove) {
    top = r.bottom + 6
    computedMaxH = Math.min(panelMaxH, spaceBelow - 4)
  } else {
    computedMaxH = Math.min(panelMaxH, spaceAbove - 4)
    top = r.top - computedMaxH - 6
  }

  panelStyle.value = {
    position: 'fixed',
    top: top + 'px',
    left: left + 'px',
    width: width + 'px',
    maxHeight: computedMaxH + 'px',
    zIndex: 5000,
  }
}

const selected = computed(() =>
  props.options.find(o => o.value === props.modelValue) || null
)

const filteredOptions = computed(() => {
  if (!props.searchable || !searchQuery.value) return props.options
  const q = searchQuery.value.toLowerCase()
  return props.options.filter(o =>
    String(o.label || '').toLowerCase().includes(q) ||
    String(o.hint || '').toLowerCase().includes(q)
  )
})

function toggle() {
  if (props.disabled) return
  open.value ? close() : openPanel()
}

function openPanel() {
  open.value = true
  highlightIndex.value = props.options.findIndex(o => o.value === props.modelValue)
  computePanelPosition()
  nextTick(() => {
    const panel = panelEl.value
    if (!panel) return
    const node = panel.querySelector('.is-selected')
    if (node) node.scrollIntoView({ block: 'nearest' })
  })
}
function close() {
  open.value = false
  searchQuery.value = ''
  highlightIndex.value = -1
}

function onScrollOrResize() {
  if (open.value) computePanelPosition()
}

// 浮层 teleport 到 body 后会脱离父级 [data-world="..."] 选择器链，
// 直接读 root attr 并显式绑定到 panel 上，让 .ws-panel[data-world="daogui"] 选择器生效
const dataWorldAttr = computed(() => {
  if (typeof document === 'undefined') return 'reality'
  return document.documentElement.getAttribute('data-world') || 'reality'
})
const isDaogui = computed(() => dataWorldAttr.value === 'daogui')

function pick(opt) {
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
  close()
}

function onDocClick(e) {
  if (!open.value) return
  const t = triggerEl.value
  const p = panelEl.value
  if (t && t.contains(e.target)) return
  if (p && p.contains(e.target)) return
  close()
}

function onKey(e) {
  if (!open.value) {
    if ((e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') && document.activeElement === triggerEl.value) {
      e.preventDefault()
      openPanel()
    }
    return
  }
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    highlightIndex.value = Math.min(filteredOptions.value.length - 1, highlightIndex.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    highlightIndex.value = Math.max(0, highlightIndex.value - 1)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const opt = filteredOptions.value[highlightIndex.value]
    if (opt) pick(opt)
  }
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKey)
  window.addEventListener('scroll', onScrollOrResize, true)
  window.addEventListener('resize', onScrollOrResize)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('scroll', onScrollOrResize, true)
  window.removeEventListener('resize', onScrollOrResize)
})

watch(() => props.modelValue, () => {
  if (!open.value) return
  highlightIndex.value = props.options.findIndex(o => o.value === props.modelValue)
})
</script>

<template>
  <div class="world-select" :class="[`s-${size}`, { 'is-open': open, 'is-disabled': disabled }]">
    <button
      ref="triggerEl"
      type="button"
      class="ws-trigger"
      :disabled="disabled"
      @click="toggle"
    >
      <span class="ws-label" :class="{ 'is-placeholder': !selected }">
        <slot name="selected" :selected="selected">
          {{ selected ? selected.label : placeholder }}
        </slot>
      </span>
      <ChevronDown class="ws-chev" :size="14" />
    </button>

    <Teleport to="body">
      <Transition name="ws-pop">
        <div
          v-if="open"
          ref="panelEl"
          class="ws-panel"
          :class="[`align-${align}`, { 'theme-daogui': isDaogui }]"
          :data-world-attr="dataWorldAttr"
          :style="panelStyle"
          role="listbox"
        >
          <div v-if="searchable" class="ws-search">
            <Search :size="12" class="ws-search-ic" />
            <input
              v-model="searchQuery"
              class="ws-search-input"
              placeholder="搜索…"
              spellcheck="false"
              @keydown.stop
            />
            <button v-if="searchQuery" class="ws-search-clear" @click="searchQuery = ''">
              <X :size="11" />
            </button>
          </div>
          <ul class="ws-list" role="listbox">
            <li
              v-for="(opt, i) in filteredOptions"
              :key="opt.value"
              role="option"
              :class="[
                'ws-opt',
                { 'is-selected': opt.value === modelValue, 'is-highlight': i === highlightIndex },
              ]"
              @click="pick(opt)"
              @mouseenter="highlightIndex = i"
            >
              <span class="ws-opt-label">
                <slot name="option" :option="opt">
                  {{ opt.label }}
                  <span v-if="opt.hint" class="ws-opt-hint">{{ opt.hint }}</span>
                </slot>
              </span>
              <Check v-if="opt.value === modelValue" class="ws-opt-check" :size="13" />
            </li>
            <li v-if="!filteredOptions.length" class="ws-empty">无匹配项</li>
          </ul>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.world-select {
  position: relative;
  display: inline-flex;
  font-family: var(--world-font-sans);
}
.world-select.is-disabled { opacity: 0.5; cursor: not-allowed; }

.ws-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 34px;
  min-width: 160px;
  padding: 0 10px 0 14px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
  color: var(--world-text-primary);
  font-size: 0.8125rem;
  font-weight: 600;
  cursor: pointer;
  outline: none;
  transition: border-color 200ms, background 200ms;
}
.s-md .ws-trigger { height: 38px; font-size: 0.875rem; padding: 0 12px 0 16px; }

.ws-trigger:hover:not(:disabled) { border-color: var(--world-accent); background: var(--world-bg-card); }
.ws-trigger:focus-visible { box-shadow: var(--world-focus-ring); }
.world-select.is-open .ws-trigger {
  border-color: var(--world-accent);
  background: var(--world-bg-card);
}

.ws-label {
  flex: 1;
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ws-label.is-placeholder { color: var(--world-text-mute); font-weight: 500; }

.ws-chev {
  flex-shrink: 0;
  color: var(--world-text-mute);
  transition: transform 240ms ease;
}
.world-select.is-open .ws-chev {
  transform: rotate(180deg);
  color: var(--world-accent);
}

/* === Panel ===
   被 Teleport 到 body，使用 inline style 完成 position:fixed 定位（见 computePanelPosition）。
   底色用纯 RGB（非半透明），避免与父级背景叠加导致看不清。
*/
.ws-panel {
  display: flex;
  flex-direction: column;
  background: rgb(255, 255, 255);
  color: rgb(15, 23, 42);
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 12px;
  box-shadow:
    0 4px 16px rgba(2, 132, 199, 0.10),
    0 16px 48px rgba(15, 23, 42, 0.18);
  overflow: hidden;
  font-family: var(--world-font-sans);
}

/* daogui 主题：纯墨黑底 + 朱砂红辅光 */
.ws-panel[data-world-attr="daogui"] {
  background: rgb(18, 14, 13);
  color: rgb(231, 215, 193);
  border-color: rgba(184, 134, 11, 0.42);
  box-shadow:
    0 0 0 1px rgba(196, 30, 58, 0.12),
    0 0 28px rgba(196, 30, 58, 0.22),
    0 16px 48px rgba(0, 0, 0, 0.7);
}

/* search */
.ws-search {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(15, 23, 42, 0.03);
  flex-shrink: 0;
}
.ws-panel[data-world-attr="daogui"] .ws-search {
  border-bottom-color: rgba(184, 134, 11, 0.20);
  background: rgba(196, 30, 58, 0.06);
}
.ws-search-ic { color: rgba(15, 23, 42, 0.55); flex-shrink: 0; }
.ws-panel[data-world-attr="daogui"] .ws-search-ic { color: rgba(231, 215, 193, 0.55); }
.ws-search-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  font-size: 0.78rem;
  color: inherit;
  font-family: var(--world-font-sans);
}
.ws-search-clear {
  width: 16px; height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: transparent;
  border: none;
  color: rgba(15, 23, 42, 0.55);
  cursor: pointer;
}
.ws-panel[data-world-attr="daogui"] .ws-search-clear { color: rgba(231, 215, 193, 0.55); }
.ws-search-clear:hover { color: #ef4444; }
.ws-panel[data-world-attr="daogui"] .ws-search-clear:hover { color: #ff6478; }

/* list */
.ws-list {
  list-style: none;
  margin: 0;
  padding: 4px;
  overflow-y: auto;
  flex: 1;
}
.ws-opt {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 0.82rem;
  font-weight: 600;
  color: inherit;
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease;
  user-select: none;
}
.ws-opt-label {
  flex: 1;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ws-opt-hint {
  font-size: 0.7rem;
  color: rgba(15, 23, 42, 0.55);
  font-weight: 500;
  font-family: var(--world-font-mono);
}
.ws-panel[data-world-attr="daogui"] .ws-opt-hint { color: rgba(231, 215, 193, 0.5); }

.ws-opt-check { flex-shrink: 0; color: #0284c7; }
.ws-panel[data-world-attr="daogui"] .ws-opt-check { color: #ff6478; }

/* reality 形态 */
.ws-opt:hover, .ws-opt.is-highlight {
  background: rgba(2, 132, 199, 0.08);
}
.ws-opt.is-selected {
  background: rgba(2, 132, 199, 0.12);
  color: #0284c7;
  box-shadow: inset 0 0 0 1px rgba(2, 132, 199, 0.25);
}

/* daogui 形态 */
.ws-panel[data-world-attr="daogui"] .ws-opt:hover,
.ws-panel[data-world-attr="daogui"] .ws-opt.is-highlight {
  background: rgba(196, 30, 58, 0.16);
  color: #ffd6d6;
}
.ws-panel[data-world-attr="daogui"] .ws-opt.is-selected {
  background: rgba(196, 30, 58, 0.26);
  color: #ff8a98;
  box-shadow: inset 0 0 0 1px rgba(196, 30, 58, 0.45);
}

.ws-empty {
  padding: 18px 12px;
  text-align: center;
  color: rgba(15, 23, 42, 0.4);
  font-size: 0.78rem;
}
.ws-panel[data-world-attr="daogui"] .ws-empty { color: rgba(231, 215, 193, 0.4); }

/* scrollbar */
.ws-list::-webkit-scrollbar { width: 6px; }
.ws-list::-webkit-scrollbar-track { background: transparent; }
.ws-list::-webkit-scrollbar-thumb {
  background: rgba(15, 23, 42, 0.18);
  border-radius: 3px;
}
.ws-list::-webkit-scrollbar-thumb:hover { background: #0284c7; }
.ws-panel[data-world-attr="daogui"] .ws-list::-webkit-scrollbar-thumb {
  background: rgba(184, 134, 11, 0.32);
}
.ws-panel[data-world-attr="daogui"] .ws-list::-webkit-scrollbar-thumb:hover {
  background: rgba(196, 30, 58, 0.7);
}

/* pop transition */
.ws-pop-enter-active { transition: opacity 180ms ease, transform 220ms cubic-bezier(0.16, 1, 0.3, 1); }
.ws-pop-leave-active { transition: opacity 140ms ease, transform 160ms ease; }
.ws-pop-enter-from { opacity: 0; transform: translateY(-6px) scale(0.97); }
.ws-pop-leave-to   { opacity: 0; transform: translateY(-4px) scale(0.98); }
</style>
