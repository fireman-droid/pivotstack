<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { Calendar, ChevronDown, ChevronLeft, ChevronRight, X } from 'lucide-vue-next'

const props = defineProps({
  // string 形式与原生 input 兼容：mode='date' → 'YYYY-MM-DD'，mode='datetime' → 'YYYY-MM-DDTHH:mm'
  modelValue: { type: [String, Number], default: '' },
  mode: { type: String, default: 'date' },     // date | datetime
  size: { type: String, default: 'sm' },       // sm | md
  min: { type: String, default: '' },          // YYYY-MM-DD
  max: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  clearable: { type: Boolean, default: true },
  disabled: { type: Boolean, default: false },
  align: { type: String, default: 'start' },   // start | end
  presets: { type: Array, default: () => [] }, // [{ label, value }]
})
const emit = defineEmits(['update:modelValue', 'change'])

const open = ref(false)
const triggerEl = ref(null)
const panelEl = ref(null)
const panelStyle = ref({})

const viewYear = ref(0)
const viewMonth = ref(0)

function pad(n) { return String(n).padStart(2, '0') }
function parseValue(v) {
  if (!v) return null
  if (typeof v === 'number') {
    const d = new Date(v * 1000)
    return isNaN(d.getTime()) ? null : d
  }
  const m = String(v).match(/^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2}))?/)
  if (!m) return null
  const d = new Date(+m[1], +m[2] - 1, +m[3], +m[4] || 0, +m[5] || 0)
  return isNaN(d.getTime()) ? null : d
}
function toModelValue(d, mode) {
  if (!d) return ''
  if (mode === 'datetime') {
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
function formatDisplay(d, mode) {
  if (!d) return ''
  const base = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  return mode === 'datetime' ? `${base} ${pad(d.getHours())}:${pad(d.getMinutes())}` : base
}

const selectedDate = computed(() => parseValue(props.modelValue))
const displayText = computed(() => formatDisplay(selectedDate.value, props.mode))
const placeholderText = computed(() => props.placeholder || (props.mode === 'datetime' ? '选择日期时间' : '选择日期'))

const monthLabel = computed(() => `${viewYear.value}年 ${viewMonth.value + 1}月`)

const calendarRows = computed(() => {
  const y = viewYear.value
  const m = viewMonth.value
  const first = new Date(y, m, 1)
  const offset = (first.getDay() + 6) % 7  // 周一为首列
  const start = new Date(y, m, 1 - offset)
  const today = new Date(); today.setHours(0, 0, 0, 0)
  const sel = selectedDate.value
  const minD = props.min ? parseValue(props.min) : null
  const maxD = props.max ? parseValue(props.max) : null
  const minDay = minD ? new Date(minD.getFullYear(), minD.getMonth(), minD.getDate()) : null
  const maxDay = maxD ? new Date(maxD.getFullYear(), maxD.getMonth(), maxD.getDate()) : null
  const rows = []
  for (let r = 0; r < 6; r++) {
    const row = []
    for (let c = 0; c < 7; c++) {
      const d = new Date(start)
      d.setDate(start.getDate() + r * 7 + c)
      d.setHours(0, 0, 0, 0)
      const inMonth = d.getMonth() === m
      const isToday = d.getTime() === today.getTime()
      const isSelected = !!(sel
        && d.getFullYear() === sel.getFullYear()
        && d.getMonth() === sel.getMonth()
        && d.getDate() === sel.getDate())
      let disabled = false
      if (minDay && d < minDay) disabled = true
      if (maxDay && d > maxDay) disabled = true
      row.push({ date: d, day: d.getDate(), inMonth, isToday, isSelected, disabled })
    }
    rows.push(row)
  }
  return rows
})

const hourInput = ref(0)
const minuteInput = ref(0)
watch(selectedDate, (d) => {
  if (d) { hourInput.value = d.getHours(); minuteInput.value = d.getMinutes() }
}, { immediate: true })

function gotoPrevMonth() {
  let y = viewYear.value, m = viewMonth.value - 1
  if (m < 0) { m = 11; y-- }
  viewYear.value = y; viewMonth.value = m
}
function gotoNextMonth() {
  let y = viewYear.value, m = viewMonth.value + 1
  if (m > 11) { m = 0; y++ }
  viewYear.value = y; viewMonth.value = m
}
function gotoToday() {
  const t = new Date()
  viewYear.value = t.getFullYear()
  viewMonth.value = t.getMonth()
  pickDate(t)
}

function pickDate(d) {
  const result = new Date(d)
  if (props.mode === 'datetime') {
    result.setHours(hourInput.value, minuteInput.value, 0, 0)
  } else {
    result.setHours(0, 0, 0, 0)
  }
  const v = toModelValue(result, props.mode)
  emit('update:modelValue', v)
  emit('change', v)
  if (props.mode === 'date') close()
}

function applyTimeIfSelected() {
  const d = selectedDate.value
  if (!d) return
  const result = new Date(d)
  result.setHours(hourInput.value, minuteInput.value, 0, 0)
  const v = toModelValue(result, props.mode)
  emit('update:modelValue', v)
  emit('change', v)
}
function onHourInput(e) {
  let h = parseInt(e.target.value, 10)
  if (isNaN(h)) h = 0
  h = Math.max(0, Math.min(23, h))
  hourInput.value = h
  applyTimeIfSelected()
}
function onMinuteInput(e) {
  let mi = parseInt(e.target.value, 10)
  if (isNaN(mi)) mi = 0
  mi = Math.max(0, Math.min(59, mi))
  minuteInput.value = mi
  applyTimeIfSelected()
}

function clearValue(e) {
  e?.stopPropagation()
  emit('update:modelValue', '')
  emit('change', '')
}
function applyPreset(p) {
  emit('update:modelValue', p.value)
  emit('change', p.value)
  close()
}
function confirmAndClose() { close() }

function toggle() {
  if (props.disabled) return
  open.value ? close() : openPanel()
}
function openPanel() {
  open.value = true
  const d = selectedDate.value || new Date()
  viewYear.value = d.getFullYear()
  viewMonth.value = d.getMonth()
  computePanelPosition()
}
function close() { open.value = false }

function computePanelPosition() {
  const t = triggerEl.value
  if (!t) return
  const r = t.getBoundingClientRect()
  const vw = window.innerWidth
  const vh = window.innerHeight
  const margin = 8
  const minWidth = 296
  const maxWidth = 360
  const width = Math.max(minWidth, Math.min(maxWidth, r.width))
  let left = props.align === 'end' ? r.right - width : r.left
  if (left + width > vw - margin) left = vw - width - margin
  if (left < margin) left = margin
  const panelMaxH = props.mode === 'datetime' ? 440 : 380
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
function onScrollOrResize() { if (open.value) computePanelPosition() }

const dataWorldAttr = computed(() => {
  if (typeof document === 'undefined') return 'reality'
  return document.documentElement.getAttribute('data-world') || 'reality'
})

function onDocClick(e) {
  if (!open.value) return
  const t = triggerEl.value
  const p = panelEl.value
  if (t && t.contains(e.target)) return
  if (p && p.contains(e.target)) return
  close()
}
function onKey(e) {
  if (open.value && e.key === 'Escape') { e.preventDefault(); close() }
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
</script>

<template>
  <div class="world-datepicker" :class="[`s-${size}`, { 'is-open': open, 'is-disabled': disabled }]">
    <button
      ref="triggerEl"
      type="button"
      class="dp-trigger"
      :disabled="disabled"
      @click="toggle"
    >
      <Calendar class="dp-ic" :size="14" />
      <span class="dp-text" :class="{ 'is-placeholder': !displayText }">
        {{ displayText || placeholderText }}
      </span>
      <X v-if="clearable && displayText" class="dp-clear" :size="13" @click.stop="clearValue" />
      <ChevronDown class="dp-chev" :size="13" />
    </button>

    <Teleport to="body">
      <Transition name="dp-pop">
        <div
          v-if="open"
          ref="panelEl"
          class="dp-panel"
          :class="`align-${align}`"
          :data-world-attr="dataWorldAttr"
          :style="panelStyle"
          role="dialog"
        >
          <div v-if="presets.length" class="dp-presets">
            <button
              v-for="p in presets"
              :key="p.label"
              type="button"
              class="dp-preset-btn"
              @click="applyPreset(p)"
            >{{ p.label }}</button>
          </div>

          <header class="dp-nav">
            <button class="dp-nav-btn" @click="gotoPrevMonth" type="button" aria-label="上月">
              <ChevronLeft :size="14" />
            </button>
            <span class="dp-nav-label">{{ monthLabel }}</span>
            <button class="dp-nav-btn" @click="gotoNextMonth" type="button" aria-label="下月">
              <ChevronRight :size="14" />
            </button>
          </header>

          <div class="dp-week">
            <span v-for="w in ['一','二','三','四','五','六','日']" :key="w">{{ w }}</span>
          </div>

          <div class="dp-grid">
            <template v-for="(row, ri) in calendarRows" :key="ri">
              <button
                v-for="(c, ci) in row"
                :key="`${ri}-${ci}`"
                type="button"
                class="dp-cell"
                :class="{
                  'is-out-month': !c.inMonth,
                  'is-today':     c.isToday,
                  'is-selected':  c.isSelected,
                  'is-disabled':  c.disabled,
                }"
                :disabled="c.disabled"
                @click="pickDate(c.date)"
              >{{ c.day }}</button>
            </template>
          </div>

          <div v-if="mode === 'datetime'" class="dp-time">
            <span class="dp-time-label">时间</span>
            <input
              type="number" min="0" max="23" :value="hourInput"
              class="dp-time-input"
              @input="onHourInput"
            />
            <span class="dp-time-sep">:</span>
            <input
              type="number" min="0" max="59" :value="minuteInput"
              class="dp-time-input"
              @input="onMinuteInput"
            />
          </div>

          <footer class="dp-foot">
            <button class="dp-foot-btn ghost" @click="clearValue" type="button">清除</button>
            <button class="dp-foot-btn ghost" @click="gotoToday" type="button">今天</button>
            <span class="dp-foot-spacer"></span>
            <button v-if="mode === 'datetime'" class="dp-foot-btn primary" @click="confirmAndClose" type="button">完成</button>
          </footer>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.world-datepicker {
  position: relative;
  display: inline-flex;
  font-family: var(--world-font-sans);
}
.world-datepicker.is-disabled { opacity: 0.5; cursor: not-allowed; }

.dp-trigger {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  min-width: 180px;
  padding: 0 10px 0 12px;
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
.s-md .dp-trigger { height: 38px; font-size: 0.875rem; padding: 0 12px 0 14px; }
.dp-trigger:hover:not(:disabled) {
  border-color: var(--world-accent);
  background: var(--world-bg-card);
}
.dp-trigger:focus-visible { box-shadow: var(--world-focus-ring); }
.world-datepicker.is-open .dp-trigger {
  border-color: var(--world-accent);
  background: var(--world-bg-card);
}
.dp-ic, .dp-chev { color: var(--world-text-mute); flex-shrink: 0; }
.dp-text {
  flex: 1;
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-variant-numeric: tabular-nums;
}
.dp-text.is-placeholder {
  color: var(--world-text-mute);
  font-weight: 500;
  font-family: var(--world-font-sans);
}
.dp-clear {
  color: var(--world-text-mute);
  cursor: pointer;
  flex-shrink: 0;
  padding: 1px;
  border-radius: 50%;
}
.dp-clear:hover { color: var(--world-error); background: rgba(239, 68, 68, 0.10); }
.dp-chev { transition: transform 240ms ease; }
.world-datepicker.is-open .dp-chev { transform: rotate(180deg); color: var(--world-accent); }

/* === Panel ===
   Teleport 到 body 后用 inline style 完成 position:fixed 定位（参考 WorldSelect）。
   底色用纯 RGB 避免与父级背景叠加。
*/
.dp-panel {
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
  padding: 12px;
  gap: 8px;
}
.dp-panel[data-world-attr="daogui"] {
  background: rgb(18, 14, 13);
  color: rgb(231, 215, 193);
  border-color: rgba(184, 134, 11, 0.42);
  box-shadow:
    0 0 0 1px rgba(196, 30, 58, 0.12),
    0 0 28px rgba(196, 30, 58, 0.22),
    0 16px 48px rgba(0, 0, 0, 0.7);
}

.dp-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-bottom: 8px;
  border-bottom: 1px dashed rgba(15, 23, 42, 0.12);
}
.dp-panel[data-world-attr="daogui"] .dp-presets {
  border-bottom-color: rgba(184, 134, 11, 0.20);
}
.dp-preset-btn {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(2, 132, 199, 0.08);
  color: #0284c7;
  border: 1px solid rgba(2, 132, 199, 0.18);
  cursor: pointer;
}
.dp-preset-btn:hover { background: rgba(2, 132, 199, 0.18); }
.dp-panel[data-world-attr="daogui"] .dp-preset-btn {
  background: rgba(196, 30, 58, 0.16);
  color: #ff8a98;
  border-color: rgba(196, 30, 58, 0.45);
}
.dp-panel[data-world-attr="daogui"] .dp-preset-btn:hover { background: rgba(196, 30, 58, 0.30); }

.dp-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}
.dp-nav-label {
  font-size: 0.875rem;
  font-weight: 800;
  letter-spacing: 0.02em;
  font-variant-numeric: tabular-nums;
}
.dp-nav-btn {
  width: 28px; height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: inherit;
  cursor: pointer;
}
.dp-nav-btn:hover {
  background: rgba(2, 132, 199, 0.10);
  color: #0284c7;
}
.dp-panel[data-world-attr="daogui"] .dp-nav-btn:hover {
  background: rgba(196, 30, 58, 0.20);
  color: #ff8a98;
}

.dp-week {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px;
}
.dp-week span {
  font-size: 0.7rem;
  font-weight: 700;
  text-align: center;
  padding: 4px 0;
  color: rgba(15, 23, 42, 0.5);
}
.dp-panel[data-world-attr="daogui"] .dp-week span {
  color: rgba(231, 215, 193, 0.55);
}

.dp-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px;
}
.dp-cell {
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  font-size: 0.82rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: inherit;
  cursor: pointer;
  font-family: var(--world-font-mono, ui-monospace, monospace);
  transition: background 120ms, border-color 120ms;
}
.dp-cell:hover:not(.is-disabled) {
  background: rgba(2, 132, 199, 0.08);
  border-color: rgba(2, 132, 199, 0.20);
}
.dp-panel[data-world-attr="daogui"] .dp-cell:hover:not(.is-disabled) {
  background: rgba(196, 30, 58, 0.18);
  border-color: rgba(196, 30, 58, 0.42);
}
.dp-cell.is-out-month { color: rgba(15, 23, 42, 0.30); }
.dp-panel[data-world-attr="daogui"] .dp-cell.is-out-month { color: rgba(231, 215, 193, 0.30); }
.dp-cell.is-today {
  border-color: rgba(2, 132, 199, 0.50);
  font-weight: 800;
}
.dp-panel[data-world-attr="daogui"] .dp-cell.is-today {
  border-color: rgba(196, 30, 58, 0.65);
}
.dp-cell.is-selected {
  background: #0284c7;
  color: #fff;
  border-color: #0284c7;
}
.dp-panel[data-world-attr="daogui"] .dp-cell.is-selected {
  background: rgba(196, 30, 58, 0.85);
  color: #fff;
  border-color: rgba(196, 30, 58, 1);
}
.dp-cell.is-disabled { opacity: 0.30; cursor: not-allowed; }

.dp-time {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px dashed rgba(15, 23, 42, 0.12);
}
.dp-panel[data-world-attr="daogui"] .dp-time {
  border-top-color: rgba(184, 134, 11, 0.20);
}
.dp-time-label {
  font-size: 0.78rem;
  font-weight: 700;
  color: rgba(15, 23, 42, 0.55);
  margin-right: 6px;
}
.dp-panel[data-world-attr="daogui"] .dp-time-label {
  color: rgba(231, 215, 193, 0.55);
}
.dp-time-input {
  width: 56px;
  height: 30px;
  background: rgba(15, 23, 42, 0.04);
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 6px;
  font-size: 0.85rem;
  font-weight: 700;
  text-align: center;
  font-family: var(--world-font-mono, ui-monospace, monospace);
  font-variant-numeric: tabular-nums;
  color: inherit;
  outline: none;
}
.dp-panel[data-world-attr="daogui"] .dp-time-input {
  background: rgba(196, 30, 58, 0.08);
  border-color: rgba(184, 134, 11, 0.30);
}
.dp-time-input:focus { border-color: #0284c7; }
.dp-panel[data-world-attr="daogui"] .dp-time-input:focus { border-color: rgba(196, 30, 58, 0.7); }
.dp-time-sep { font-weight: 800; font-family: var(--world-font-mono, ui-monospace, monospace); }

.dp-foot {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px dashed rgba(15, 23, 42, 0.12);
}
.dp-panel[data-world-attr="daogui"] .dp-foot { border-top-color: rgba(184, 134, 11, 0.20); }
.dp-foot-spacer { flex: 1; }
.dp-foot-btn {
  font-size: 0.78rem;
  font-weight: 700;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid transparent;
  background: transparent;
  color: inherit;
}
.dp-foot-btn.ghost:hover { background: rgba(15, 23, 42, 0.06); }
.dp-panel[data-world-attr="daogui"] .dp-foot-btn.ghost:hover { background: rgba(231, 215, 193, 0.08); }
.dp-foot-btn.primary { background: #0284c7; color: #fff; }
.dp-foot-btn.primary:hover { background: #0369a1; }
.dp-panel[data-world-attr="daogui"] .dp-foot-btn.primary { background: rgba(196, 30, 58, 0.85); color: #fff; }
.dp-panel[data-world-attr="daogui"] .dp-foot-btn.primary:hover { background: rgba(196, 30, 58, 1); }

.dp-pop-enter-active { transition: opacity 180ms ease, transform 220ms cubic-bezier(0.16, 1, 0.3, 1); }
.dp-pop-leave-active { transition: opacity 140ms ease, transform 160ms ease; }
.dp-pop-enter-from { opacity: 0; transform: translateY(-6px) scale(0.97); }
.dp-pop-leave-to   { opacity: 0; transform: translateY(-4px) scale(0.98); }
</style>
