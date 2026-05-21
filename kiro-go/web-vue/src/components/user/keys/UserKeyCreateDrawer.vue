<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NInput, NButton, NSelect, NInputNumber, useMessage } from 'naive-ui'
import { Key, Copy, Check, ChevronDown, ChevronRight } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import { createUserKey, patchUserKey, getChannelOptions } from '../../../api/user'

interface ModelPriceRow {
  name: string
  inputPerM?: number   // virtual $/1M tokens（站内单位，1¥=20$）
  outputPerM?: number
}
interface ChannelOption {
  id: string
  alias: string
  sourceType: string
  models: ModelPriceRow[]
}
interface GroupOption {
  id: string
  name: string
  description: string
  defaultChannel: string
  channels: ChannelOption[]
}
interface EditableKey {
  id: string
  note?: string
  channelPreferences?: Record<string, string>
}

const props = defineProps<{
  show: boolean
  editKey?: EditableKey | null
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'created'): void
  (e: 'updated'): void
}>()

const isEdit = computed(() => !!props.editKey)

const message = useMessage()
const submitting = ref(false)
const loadingOptions = ref(false)

const note = ref('')
const expiresAtPreset = ref<string>('never')
const expiresCustom = ref<string>('') // YYYY-MM-DD
const rateLimit = ref<number | null>(null)

// 路由偏好：groupID → channelID
const groups = ref<GroupOption[]>([])
const groupPrefs = ref<Record<string, string>>({})
const psdpy = ref<number>(20) // 1¥ = N$ 站内虚拟单位换算（admin 可调）

const psdpyHint = computed(() => `价格 $ 为站内虚拟单位（1¥ = ${psdpy.value}$）`)
// 折叠状态：默认全开
const groupExpanded = ref<Record<string, boolean>>({})
const channelExpanded = ref<Record<string, boolean>>({}) // key = `${groupId}:${channelId}`
function isGroupOpen(gid: string): boolean { return groupExpanded.value[gid] !== false }
function isChannelOpen(gid: string, cid: string): boolean {
  return channelExpanded.value[`${gid}:${cid}`] !== false
}
function toggleGroup(gid: string) {
  // 用当前可见状态取反，而不是 raw map 值取反 —
  // 初始 undefined 时 !undefined === true，会让首次点击没效果。
  groupExpanded.value = { ...groupExpanded.value, [gid]: !isGroupOpen(gid) }
}
function toggleChannel(gid: string, cid: string) {
  const k = `${gid}:${cid}`
  channelExpanded.value = { ...channelExpanded.value, [k]: !isChannelOpen(gid, cid) }
}

const createdKey = ref('')
const copied = ref(false)

const presetOptions = [
  { label: '永不过期', value: 'never' },
  { label: '7 天', value: '7d' },
  { label: '30 天', value: '30d' },
  { label: '90 天', value: '90d' },
  { label: '自定义日期', value: 'custom' },
]

const expiresAtComputed = computed<number>(() => {
  if (expiresAtPreset.value === 'never') return 0
  if (expiresAtPreset.value === 'custom') {
    if (!expiresCustom.value) return 0
    const t = new Date(expiresCustom.value + 'T23:59:59').getTime()
    return Math.floor(t / 1000)
  }
  const map: Record<string, number> = { '7d': 7, '30d': 30, '90d': 90 }
  const days = map[expiresAtPreset.value] || 0
  if (!days) return 0
  return Math.floor((Date.now() + days * 86400_000) / 1000)
})

async function loadOptions() {
  loadingOptions.value = true
  try {
    const data = await getChannelOptions()
    // 后端 v7.1+ 返回 { groups, pivotStackDollarsPerYuan }；旧版直接返回 array — 都兼容
    if (Array.isArray(data)) {
      groups.value = data
    } else if (data && typeof data === 'object') {
      groups.value = Array.isArray(data.groups) ? data.groups : []
      if (typeof data.pivotStackDollarsPerYuan === 'number' && data.pivotStackDollarsPerYuan > 0) {
        psdpy.value = data.pivotStackDollarsPerYuan
      }
    } else {
      groups.value = []
    }
    // 默认值：edit 模式从 editKey 预填；create 模式取 defaultChannel / 第一个
    const existing = props.editKey?.channelPreferences || {}
    const next: Record<string, string> = {}
    for (const g of groups.value) {
      if (existing[g.id]) next[g.id] = existing[g.id]
      else if (!isEdit.value && g.defaultChannel) next[g.id] = g.defaultChannel
      else if (!isEdit.value && g.channels.length) next[g.id] = g.channels[0].id
      // edit 模式下未设置过的 group 保持空（= 走系统默认）
    }
    groupPrefs.value = next
  } catch (e: any) {
    // 静默：无 group 配置时直接隐藏路由区
  } finally {
    loadingOptions.value = false
  }
}

watch(() => props.show, on => {
  if (!on) {
    note.value = ''
    expiresAtPreset.value = 'never'
    expiresCustom.value = ''
    rateLimit.value = null
    createdKey.value = ''
    copied.value = false
  } else {
    // 进入 edit 模式时预填名称；create 模式保持默认空
    note.value = isEdit.value ? (props.editKey?.note || '') : ''
    loadOptions()
  }
})

function fmtPrice(v: number | undefined): string {
  if (v == null) return '—'
  if (v >= 1) return `$${v.toFixed(2)}`
  return `$${v.toFixed(4)}`
}
function channelLabel(c: ChannelOption): string {
  return c.alias && c.alias !== c.id ? `${c.alias} · ${c.id}` : c.id
}
function selectChannel(groupId: string, channelId: string) {
  groupPrefs.value = { ...groupPrefs.value, [groupId]: channelId }
}

async function submit() {
  submitting.value = true
  try {
    // 收集路由偏好（仅写非空 channel）
    const prefs: Record<string, string> = {}
    for (const g of groups.value) {
      const v = groupPrefs.value[g.id]
      if (v) prefs[g.id] = v
    }
    if (isEdit.value && props.editKey) {
      await patchUserKey(props.editKey.id, {
        note: note.value.trim(),
        channelPreferences: prefs,
      })
      emit('updated')
      message.success('已保存')
      emit('update:show', false)
      return
    }
    const payload: Record<string, unknown> = {
      note: note.value.trim(),
      expiresAt: expiresAtComputed.value,
      channelPreferences: prefs,
    }
    if (rateLimit.value && rateLimit.value > 0) {
      payload.rateLimitPerMin = rateLimit.value
    }
    const out = await createUserKey(payload)
    createdKey.value = out.key
    emit('created')
    message.success('已创建')
  } catch (e: any) {
    message.error(e?.message || (isEdit.value ? '保存失败' : '创建失败'))
  } finally {
    submitting.value = false
  }
}

async function copy() {
  await navigator.clipboard.writeText(createdKey.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 1500)
}

function close() {
  if (submitting.value) return
  emit('update:show', false)
}
</script>

<template>
  <RefinedDrawer
    :show="show"
    :title="isEdit ? '编辑 API Key' : '新建 API Key'"
    :subtitle="isEdit ? '修改名称和路由偏好；过期时间和速率限制由 admin 管理' : '一次性配置完整参数。创建后会显示 Key，仅此一次'"
    :icon="Key"
    :loading="submitting"
    :width="640"
    @update:show="(v) => emit('update:show', v)"
  >
    <template v-if="!createdKey">
      <RefinedField label="名称" hint="便于识别这把 Key 的用途（如 Claude Code 工作机、Cursor IDE 等）">
        <n-input
          v-model:value="note"
          placeholder="留空将使用默认名"
          :maxlength="64"
        />
      </RefinedField>

      <RefinedField v-if="!isEdit" label="过期时间" hint="永不过期适合长期机器；短期分发建议设过期">
        <div class="ukd-row">
          <n-select
            v-model:value="expiresAtPreset"
            :options="presetOptions"
            style="flex: 1"
          />
          <input
            v-if="expiresAtPreset === 'custom'"
            v-model="expiresCustom"
            type="date"
            class="ukd-date"
          />
        </div>
      </RefinedField>

      <RefinedField
        v-if="groups.length"
        label="路由偏好"
        :hint="`为每个模型分组挑一条渠道；${psdpyHint}`"
      >
        <div class="ukd-routes">
          <section v-for="g in groups" :key="g.id" class="ukd-group">
            <button type="button" class="ukd-group__head" @click="toggleGroup(g.id)">
              <component :is="isGroupOpen(g.id) ? ChevronDown : ChevronRight" :size="14" class="ukd-group__chev" />
              <span class="ukd-group__name">{{ g.name }}</span>
              <span class="ukd-group__desc">{{ g.description || g.id }}</span>
              <span class="ukd-group__pick">
                {{ groupPrefs[g.id]
                  ? (g.channels.find(c => c.id === groupPrefs[g.id])?.alias || groupPrefs[g.id])
                  : '系统默认' }}
              </span>
            </button>
            <div v-show="isGroupOpen(g.id)" class="ukd-cards">
              <label
                class="ukd-card ukd-card--auto"
                :class="{ 'ukd-card--active': !groupPrefs[g.id] }"
                @click="selectChannel(g.id, '')"
              >
                <input type="radio" :checked="!groupPrefs[g.id]" :name="`g-${g.id}`" />
                <div class="ukd-card__body">
                  <div class="ukd-card__head">
                    <span class="ukd-card__alias">系统默认</span>
                    <span class="ukd-card__tag ukd-card__tag--auto">auto</span>
                    <span class="ukd-card__hint-inline">由系统按 group 默认渠道分发（推荐）</span>
                  </div>
                </div>
              </label>
              <div
                v-for="c in g.channels"
                :key="c.id"
                class="ukd-card"
                :class="{ 'ukd-card--active': groupPrefs[g.id] === c.id }"
              >
                <label class="ukd-card__pick" @click="selectChannel(g.id, c.id)">
                  <input type="radio" :checked="groupPrefs[g.id] === c.id" :name="`g-${g.id}`" />
                </label>
                <div class="ukd-card__body">
                  <button
                    type="button"
                    class="ukd-card__head ukd-card__head--btn"
                    @click="toggleChannel(g.id, c.id)"
                  >
                    <component :is="isChannelOpen(g.id, c.id) ? ChevronDown : ChevronRight" :size="13" class="ukd-card__chev" />
                    <span class="ukd-card__alias">{{ channelLabel(c) }}</span>
                    <span class="ukd-card__tag">{{ c.sourceType }}</span>
                    <span class="ukd-card__count">{{ c.models?.length || 0 }} 模型</span>
                  </button>
                  <div v-show="isChannelOpen(g.id, c.id)" class="ukd-card__detail">
                    <div v-if="c.models?.length" class="ukd-models">
                      <div class="ukd-models__head">
                        <span>模型</span><span>输入 / 1M</span><span>输出 / 1M</span>
                      </div>
                      <div v-for="m in c.models" :key="m.name" class="ukd-models__row">
                        <span class="ukd-models__name">{{ m.name }}</span>
                        <span class="ukd-models__price">{{ fmtPrice(m.inputPerM) }}</span>
                        <span class="ukd-models__price">{{ fmtPrice(m.outputPerM) }}</span>
                      </div>
                    </div>
                    <div v-else class="ukd-card__hint">未配置模型清单</div>
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>
      </RefinedField>

      <RefinedField v-if="!isEdit" label="速率限制（可选）" hint="每分钟请求数上限，留空走全局默认（一般 200/min）">
        <n-input-number
          v-model:value="rateLimit"
          :min="0"
          :step="10"
          placeholder="例如 60"
          style="width: 100%"
        />
      </RefinedField>
    </template>

    <div v-else class="ukd-result">
      <div class="ukd-result__title">✓ 创建成功</div>
      <div class="ukd-result__hint">这是你的新 API Key。关闭后也可在列表的「API Key」列点击 mask 文本随时复制完整 key。</div>
      <div class="ukd-result__key">
        <code>{{ createdKey }}</code>
        <n-button quaternary size="small" @click="copy">
          <template #icon>
            <Check v-if="copied" :size="13" />
            <Copy v-else :size="13" />
          </template>
          {{ copied ? '已复制' : '复制' }}
        </n-button>
      </div>
    </div>

    <template #footer>
      <n-button :disabled="submitting" quaternary @click="close">{{ createdKey ? '完成' : '取消' }}</n-button>
      <n-button v-if="!createdKey" type="primary" :loading="submitting" @click="submit">
        {{ isEdit ? '保存' : '创建' }}
      </n-button>
    </template>
  </RefinedDrawer>
</template>

<style scoped>
.ukd-row { display: flex; gap: 8px; align-items: center; }
.ukd-date {
  flex: 1;
  height: 34px;
  padding: 0 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 4px;
  color: #ededed;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
}
.ukd-date:focus { outline: none; border-color: rgba(82, 168, 255, 0.6); }

.ukd-routes { display: flex; flex-direction: column; gap: 14px; }
.ukd-group { display: flex; flex-direction: column; gap: 6px; }
.ukd-group__head {
  all: unset;
  display: flex; align-items: baseline; gap: 10px;
  padding: 6px 4px;
  cursor: pointer;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.12s;
}
.ukd-group__head:hover { background: rgba(255, 255, 255, 0.02); }
.ukd-group__chev { color: #707070; align-self: center; flex-shrink: 0; }
.ukd-group__name { color: #ededed; font-size: 13px; font-weight: 600; }
.ukd-group__desc { color: #707070; font-size: 11px; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ukd-group__pick {
  color: #52a8ff; font-size: 11px;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
}

.ukd-cards { display: flex; flex-direction: column; gap: 6px; padding-left: 4px; }
.ukd-card {
  display: flex; gap: 10px; align-items: flex-start;
  padding: 8px 10px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 4px;
  transition: background 0.12s, border-color 0.12s;
}
.ukd-card:hover { background: rgba(255, 255, 255, 0.035); border-color: rgba(255, 255, 255, 0.10); }
.ukd-card--active { background: rgba(82, 168, 255, 0.06); border-color: rgba(82, 168, 255, 0.45); }
.ukd-card--auto { cursor: pointer; }

.ukd-card__pick { padding: 2px 2px 0; cursor: pointer; flex-shrink: 0; }
.ukd-card input[type="radio"] {
  margin: 0; flex-shrink: 0;
  accent-color: #52a8ff;
  width: 14px; height: 14px;
}
.ukd-card__body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }

.ukd-card__head { display: flex; align-items: center; gap: 8px; }
.ukd-card__head--btn {
  all: unset;
  display: flex; align-items: center; gap: 8px; width: 100%;
  cursor: pointer;
  padding: 2px 0;
}
.ukd-card__chev { color: #707070; flex-shrink: 0; }
.ukd-card__alias {
  color: #ededed; font-size: 13px; font-weight: 500;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.ukd-card__tag {
  font-size: 10px; padding: 1px 6px; border-radius: 3px;
  background: rgba(255, 255, 255, 0.05); color: #a3a3a3;
  text-transform: uppercase; letter-spacing: 0.06em;
  flex-shrink: 0;
}
.ukd-card__tag--auto { background: rgba(82, 168, 255, 0.12); color: #52a8ff; }
.ukd-card__count { color: #707070; font-size: 11px; margin-left: auto; flex-shrink: 0; }
.ukd-card__hint { color: #707070; font-size: 11px; }
.ukd-card__hint-inline { color: #707070; font-size: 11px; }

.ukd-card__detail { margin-top: 2px; }

.ukd-models {
  border: 1px solid rgba(255, 255, 255, 0.04);
  border-radius: 3px;
  overflow: hidden;
}
.ukd-models__head, .ukd-models__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 96px 96px;
  gap: 10px;
  padding: 6px 10px;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
  font-variant-numeric: tabular-nums;
  align-items: center;
}
.ukd-models__head {
  background: rgba(255, 255, 255, 0.02);
  color: #707070; font-size: 10px;
  text-transform: uppercase; letter-spacing: 0.06em;
}
.ukd-models__head > span:nth-child(1) { text-align: left; }
.ukd-models__head > span:nth-child(2),
.ukd-models__head > span:nth-child(3) { text-align: center; }
.ukd-models__row { color: #d4d4d4; font-size: 11px; border-top: 1px solid rgba(255, 255, 255, 0.03); }
.ukd-models__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-align: left; }
.ukd-models__price { text-align: center; color: #ededed; }

.ukd-result { display: flex; flex-direction: column; gap: 12px; }
.ukd-result__title { color: #0bd470; font-size: 14px; font-weight: 600; }
.ukd-result__hint {
  color: #a1a1a1; font-size: 13px; line-height: 1.6;
  padding: 10px 12px;
  background: rgba(245, 166, 35, 0.06);
  border: 1px solid rgba(245, 166, 35, 0.30);
  border-radius: 4px;
}
.ukd-result__key {
  display: flex; align-items: center; gap: 8px;
  padding: 14px 16px;
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 6px;
}
.ukd-result__key code {
  flex: 1;
  font-family: var(--st-font-mono, "Geist Mono", monospace);
  font-size: 13px;
  color: #ededed;
  word-break: break-all;
  user-select: all;
}
</style>
