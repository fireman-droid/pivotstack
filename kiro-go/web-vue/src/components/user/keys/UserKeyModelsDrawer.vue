<script setup lang="ts">
// 抽屉：展示一把 user key 当前路由偏好对应渠道支持的模型清单 + 单价 + 一键复制完整模型名。
// 目的：user 在 Claude Code / Cursor 配 model 时，要的是「这把 key 调时该写哪个模型名」，
// 后端因 normalize 失败时（4-7 vs 4.7）会 model_not_found，所以把上游真实模型名暴露给 user 直接复制。
import { computed, ref, watch } from 'vue'
import { NButton, useMessage } from 'naive-ui'
import { Copy, Check, BookOpen, ChevronDown, ChevronRight } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import { getChannelOptions } from '../../../api/user'

interface ModelRow {
  name: string
  inputPerM?: number
  outputPerM?: number
}
interface ChannelOption {
  id: string
  alias: string
  sourceType: string
  models: ModelRow[]
}
interface GroupOption {
  id: string
  name: string
  description: string
  defaultChannel: string
  channels: ChannelOption[]
}
interface ApiKey {
  id: string
  note?: string
  channelPreferences?: Record<string, string>
}

const props = defineProps<{ show: boolean; apiKey: ApiKey | null }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const message = useMessage()
const loading = ref(false)
const groups = ref<GroupOption[]>([])
const psdpy = ref<number>(20)
const groupOpen = ref<Record<string, boolean>>({})

const psdpyHint = computed(() => `站内虚拟单位（1¥ = ${psdpy.value}$）`)

interface Resolved {
  group: GroupOption
  channel: ChannelOption | null
  source: 'preference' | 'default' | 'none'
}

const resolved = computed<Resolved[]>(() => {
  const prefs = props.apiKey?.channelPreferences || {}
  return groups.value.map(g => {
    let channel: ChannelOption | null = null
    let source: 'preference' | 'default' | 'none' = 'none'
    const prefID = prefs[g.id]
    if (prefID) {
      channel = g.channels.find(c => c.id === prefID) || null
      if (channel) source = 'preference'
    }
    if (!channel && g.defaultChannel) {
      channel = g.channels.find(c => c.id === g.defaultChannel) || null
      if (channel) source = 'default'
    }
    if (!channel && g.channels.length > 0) {
      channel = g.channels[0]
      source = 'default'
    }
    return { group: g, channel, source }
  })
})

async function loadOptions() {
  loading.value = true
  try {
    const data = await getChannelOptions()
    if (Array.isArray(data)) {
      groups.value = data
    } else if (data && typeof data === 'object') {
      groups.value = Array.isArray(data.groups) ? data.groups : []
      if (typeof data.pivotStackDollarsPerYuan === 'number' && data.pivotStackDollarsPerYuan > 0) {
        psdpy.value = data.pivotStackDollarsPerYuan
      }
    }
    // 默认全开
    const open: Record<string, boolean> = {}
    for (const g of groups.value) open[g.id] = true
    groupOpen.value = open
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

watch(() => props.show, on => {
  if (on) loadOptions()
})

function toggle(gid: string) {
  groupOpen.value = { ...groupOpen.value, [gid]: !groupOpen.value[gid] }
}

function isOpen(gid: string): boolean { return groupOpen.value[gid] !== false }

function fmtPrice(v: number | undefined): string {
  if (v == null) return '—'
  if (v >= 1) return `$${v.toFixed(2)}`
  return `$${v.toFixed(4)}`
}

const copiedModel = ref<string>('')
async function copyModel(name: string) {
  try {
    await navigator.clipboard.writeText(name)
    copiedModel.value = name
    setTimeout(() => { if (copiedModel.value === name) copiedModel.value = '' }, 1200)
  } catch {
    message.error('复制失败')
  }
}

function sourceTag(s: 'preference' | 'default' | 'none'): { label: string; cls: string } {
  if (s === 'preference') return { label: '你的偏好', cls: 'tag--pref' }
  if (s === 'default') return { label: '系统默认', cls: 'tag--auto' }
  return { label: '未配置', cls: 'tag--none' }
}
</script>

<template>
  <RefinedDrawer
    :show="show"
    title="可用模型清单"
    :subtitle="apiKey?.note ? `Key「${apiKey.note}」实际路由的渠道与模型；价格 ${psdpyHint}` : `Key 实际路由的渠道与模型；价格 ${psdpyHint}`"
    :icon="BookOpen"
    :width="720"
    @update:show="(v) => emit('update:show', v)"
  >
    <div v-if="loading" class="ukm-loading">加载中…</div>
    <div v-else-if="!groups.length" class="ukm-empty">无可用分组</div>
    <div v-else class="ukm-list">
      <section v-for="r in resolved" :key="r.group.id" class="ukm-group">
        <button type="button" class="ukm-group__head" @click="toggle(r.group.id)">
          <component :is="isOpen(r.group.id) ? ChevronDown : ChevronRight" :size="14" class="ukm-chev" />
          <span class="ukm-group__name">{{ r.group.name }}</span>
          <span class="ukm-group__alias">{{ r.channel?.alias || r.channel?.id || '—' }}</span>
          <span class="ukm-tag" :class="sourceTag(r.source).cls">{{ sourceTag(r.source).label }}</span>
          <span class="ukm-group__count">{{ r.channel?.models?.length || 0 }} 模型</span>
        </button>
        <div v-show="isOpen(r.group.id)" class="ukm-models">
          <template v-if="r.channel?.models?.length">
            <div class="ukm-models__head">
              <span>模型名（点击复制）</span><span>输入 / 1M</span><span>输出 / 1M</span>
            </div>
            <div
              v-for="m in r.channel.models"
              :key="m.name"
              class="ukm-models__row"
              :class="{ 'is-copied': copiedModel === m.name }"
              @click="copyModel(m.name)"
              :title="`点击复制：${m.name}`"
            >
              <span class="ukm-model-name">
                <component :is="copiedModel === m.name ? Check : Copy" :size="11" class="ukm-copy-icon" />
                {{ m.name }}
              </span>
              <span class="ukm-price">{{ fmtPrice(m.inputPerM) }}</span>
              <span class="ukm-price">{{ fmtPrice(m.outputPerM) }}</span>
            </div>
          </template>
          <div v-else class="ukm-empty-row">该渠道未配置模型</div>
        </div>
      </section>
    </div>

    <template #footer>
      <n-button size="small" quaternary @click="emit('update:show', false)">关闭</n-button>
    </template>
  </RefinedDrawer>
</template>

<style scoped>
.ukm-loading, .ukm-empty {
  color: #707070; font-size: 13px; padding: 24px 0; text-align: center;
}
.ukm-list { display: flex; flex-direction: column; gap: 14px; }

.ukm-group { display: flex; flex-direction: column; gap: 6px; }
.ukm-group__head {
  all: unset;
  display: flex; align-items: center; gap: 10px;
  padding: 8px 6px;
  cursor: pointer;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.12s;
}
.ukm-group__head:hover { background: rgba(255, 255, 255, 0.02); }
.ukm-chev { color: #707070; flex-shrink: 0; }
.ukm-group__name {
  color: #ededed; font-size: 13px; font-weight: 600;
}
.ukm-group__alias {
  color: #a3a3a3; font-size: 12px;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  flex: 1; min-width: 0;
}
.ukm-tag {
  font-size: 10px; padding: 1px 6px; border-radius: 3px;
  letter-spacing: 0.04em; flex-shrink: 0;
}
.tag--pref { color: #52a8ff; background: rgba(82, 168, 255, 0.12); }
.tag--auto { color: #707070; background: rgba(255, 255, 255, 0.06); }
.tag--none { color: #f5a623; background: rgba(245, 166, 35, 0.10); }
.ukm-group__count { color: #707070; font-size: 11px; flex-shrink: 0; }

.ukm-models {
  margin-left: 24px;
  border: 1px solid rgba(255, 255, 255, 0.04);
  border-radius: 4px;
  overflow: hidden;
}
.ukm-models__head, .ukm-models__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 90px 90px;
  gap: 12px;
  padding: 7px 12px;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
  font-variant-numeric: tabular-nums;
  align-items: center;
}
.ukm-models__head {
  background: rgba(255, 255, 255, 0.02);
  color: #707070; font-size: 10px;
  text-transform: uppercase; letter-spacing: 0.06em;
}
.ukm-models__head > span:nth-child(2),
.ukm-models__head > span:nth-child(3) { text-align: center; }
.ukm-models__row {
  color: #d4d4d4; font-size: 11.5px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
}
.ukm-models__row:hover {
  background: rgba(255, 255, 255, 0.035);
  color: #ededed;
}
.ukm-models__row.is-copied {
  background: rgba(11, 212, 112, 0.08);
  color: #0bd470;
}
.ukm-model-name {
  display: inline-flex; align-items: center; gap: 6px;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.ukm-copy-icon { color: #707070; flex-shrink: 0; }
.ukm-models__row:hover .ukm-copy-icon { color: #ededed; }
.ukm-models__row.is-copied .ukm-copy-icon { color: #0bd470; }
.ukm-price { text-align: center; color: #ededed; }
.ukm-empty-row { color: #707070; font-size: 11px; padding: 10px 12px; }
</style>
