<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton, NInput, NSpin,
  NPopconfirm, NDivider, useMessage,
} from 'naive-ui'
import { ArrowLeft, Save, Trash2, Layers } from 'lucide-vue-next'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import GroupMemberPicker from '../../components/admin/groups/GroupMemberPicker.vue'
import GroupMemberList from '../../components/admin/groups/GroupMemberList.vue'
import {
  getChannelGroup, updateChannelGroup, replaceChannelGroupMembers, deleteChannelGroup,
  type ChannelGroupView, type ChannelGroupChannelRef,
} from '../../api/admin/groups'
import { getSystemUnitConfig } from '../../api/admin/unit'
import { useChannelGroupContext, type CandidateWithPricing } from '../../composables/useChannelGroupContext'

const props = defineProps<{ id: string }>()
const router = useRouter()
const message = useMessage()

const loading = ref(false)
const savingMeta = ref(false)
const savingMembers = ref(false)
const group = ref<ChannelGroupView | null>(null)

// 当前已挂载（reactive 编辑态）
const memberRuntimeIds = ref<string[]>([])
const defaultRuntimeId = ref<string>('')

// 元数据编辑态（只保留名称 + 描述；启用走列表行开关；删 ModelPatterns 和 sortOrder UI）
const editName = ref('')
const editDescription = ref('')

// PivotStack 虚拟 $ 换算：sellMultiplier = markup × yuanPerUpstreamDollar × pivotStackDollarsPerYuan
// 每条 channel 单独的 markup（候选 view 返回 markup 字段），自营直连 markup 视为 1×
// 系数来自 admin 全局单位配置（/billing/unit），不再硬编码
const pivotStackDollarsPerYuan = ref(1)
const YUAN_PER_UPSTREAM_DOLLAR = 1 // TODO: 跨 provider 应从 provider.yuanPerUpstreamDollar 拿；目前简化用 1
const sellMultiplierByRuntime = computed<Record<string, number>>(() => {
  const out: Record<string, number> = {}
  for (const c of candidatesEnriched.value) {
    const markup = c.markup ?? 1
    out[c.runtimeId] = markup * YUAN_PER_UPSTREAM_DOLLAR * pivotStackDollarsPerYuan.value
  }
  return out
})

// 上下文：解构出来让 template 自动 unwrap ref（嵌套在 ctx.xxx 时 vue 不会自动 unwrap）
const {
  enriched: candidatesEnriched,
  loadingCandidates,
  loadingPricing,
  loadCandidates,
  loadProviderMetadata,
} = useChannelGroupContext()

// 已选成员明细
const enrichedByRuntime = computed(() => {
  const m = new Map<string, CandidateWithPricing>()
  for (const c of candidatesEnriched.value) m.set(c.runtimeId, c)
  return m
})
const members = computed<CandidateWithPricing[]>(() => {
  return memberRuntimeIds.value
    .map(id => enrichedByRuntime.value.get(id))
    .filter((x): x is CandidateWithPricing => !!x)
})

const excludedRuntimeIds = computed(() => new Set(memberRuntimeIds.value))

async function reload() {
  loading.value = true
  try {
    const [g] = await Promise.all([
      getChannelGroup(props.id),
      loadCandidates(),
      getSystemUnitConfig()
        .then(c => { pivotStackDollarsPerYuan.value = c.pivotStackDollarsPerYuan ?? 1 })
        .catch(() => { /* 单位配置不可用时保持默认 1 */ }),
    ])
    group.value = g
    editName.value = g.name
    editDescription.value = g.description || ''
    memberRuntimeIds.value = g.channels.map(c => c.runtimeId)
    defaultRuntimeId.value = g.defaultRuntimeChannelId || ''
    await loadProviderMetadata()
  } catch (e: any) {
    message.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function saveMetadata() {
  if (!group.value) return
  if (!editName.value.trim()) { message.error('请输入分组名'); return }
  savingMeta.value = true
  try {
    await updateChannelGroup(group.value.id, {
      name: editName.value.trim(),
      description: editDescription.value.trim(),
    })
    message.success('元数据已保存')
    await reload()
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    savingMeta.value = false
  }
}

function addMembers(refs: ChannelGroupChannelRef[]) {
  const next = new Set(memberRuntimeIds.value)
  for (const r of refs) {
    const runtimeId = r.sourceType === 'direct' ? `direct:${r.channelId}` : r.channelId
    next.add(runtimeId)
  }
  memberRuntimeIds.value = Array.from(next)
  if (!defaultRuntimeId.value && memberRuntimeIds.value.length) {
    defaultRuntimeId.value = memberRuntimeIds.value[0]
  }
  message.info(`已加入 ${refs.length} 条候选 — 还未保存`)
}
function removeMember(runtimeId: string) {
  memberRuntimeIds.value = memberRuntimeIds.value.filter(id => id !== runtimeId)
  if (defaultRuntimeId.value === runtimeId) defaultRuntimeId.value = ''
}
function setDefault(runtimeId: string) {
  if (!memberRuntimeIds.value.includes(runtimeId)) return
  defaultRuntimeId.value = runtimeId
}

const dirtyMembers = computed(() => {
  if (!group.value) return false
  const oldIds = new Set(group.value.channels.map(c => c.runtimeId))
  const newIds = new Set(memberRuntimeIds.value)
  if (oldIds.size !== newIds.size) return true
  for (const id of oldIds) if (!newIds.has(id)) return true
  return defaultRuntimeId.value !== (group.value.defaultRuntimeChannelId || '')
})

async function saveMembers() {
  if (!group.value) return
  if (!memberRuntimeIds.value.length) {
    message.warning('请至少挂载一条渠道')
    return
  }
  if (defaultRuntimeId.value && !memberRuntimeIds.value.includes(defaultRuntimeId.value)) {
    message.error('默认渠道不在挂载列表里')
    return
  }
  savingMembers.value = true
  try {
    const refs: ChannelGroupChannelRef[] = []
    for (const id of memberRuntimeIds.value) {
      const cand = enrichedByRuntime.value.get(id)
      if (cand) refs.push({ sourceType: cand.sourceType, channelId: cand.channelId })
    }
    await replaceChannelGroupMembers(group.value.id, {
      channels: refs,
      defaultRuntimeChannelId: defaultRuntimeId.value || undefined,
    })
    message.success(`已挂载 ${refs.length} 条`)
    await reload()
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    savingMembers.value = false
  }
}

async function doDelete() {
  if (!group.value) return
  try {
    await deleteChannelGroup(group.value.id)
    message.success('已删除')
    router.push({ name: 'ChannelsGroups' })
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

const dirtyMeta = computed(() => {
  if (!group.value) return false
  return editName.value.trim() !== group.value.name ||
    editDescription.value.trim() !== (group.value.description || '')
})

watch(() => props.id, reload, { immediate: false })
onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader
      kicker="渠道 · 分组"
      :kicker-dot="'#0bd470'"
      :title="group?.name || props.id"
      :desc="group?.description || `分组 ID: ${props.id}`"
    >
      <template #actions>
        <n-button quaternary size="small" @click="router.push({ name: 'ChannelsGroups' })">
          <template #icon><ArrowLeft :size="14" /></template>
          返回总览
        </n-button>
        <n-popconfirm @positive-click="doDelete" positive-text="删除" negative-text="取消">
          <template #trigger>
            <n-button size="small" quaternary type="error">
              <template #icon><Trash2 :size="13" /></template>
              删除分组
            </n-button>
          </template>
          删除分组「{{ group?.name }}」？user 上指向它的偏好会被一起清除。
        </n-popconfirm>
      </template>
    </PageHeader>

    <n-spin v-if="loading && !group" />

    <template v-else-if="group">
      <!-- 元数据卡片：极简，只保留名称 + 描述 -->
      <section class="meta-card">
        <header class="meta-card__head">
          <h2 class="meta-card__title"><Layers :size="14" /> 基本信息</h2>
          <span class="meta-card__id mono">{{ group.id }}</span>
        </header>
        <div class="meta-form">
          <div class="meta-form__row">
            <label>名称</label>
            <n-input v-model:value="editName" size="small" />
          </div>
          <div class="meta-form__row meta-form__row--top">
            <label>描述</label>
            <n-input v-model:value="editDescription" type="textarea" size="small" :autosize="{ minRows: 2, maxRows: 3 }" />
          </div>
          <div class="meta-form__actions">
            <n-button size="small" type="primary" :disabled="!dirtyMeta" :loading="savingMeta" @click="saveMetadata">
              <template #icon><Save :size="13" /></template>
              保存
            </n-button>
          </div>
        </div>
      </section>

      <n-divider style="margin: 16px 0" />

      <!-- 已挂载渠道 -->
      <section class="block">
        <GroupMemberList
          :members="members"
          :default-runtime-id="defaultRuntimeId"
          :sell-multiplier-by-runtime="sellMultiplierByRuntime"
          @remove="removeMember"
          @set-default="setDefault"
          @channel-changed="loadCandidates"
        />
      </section>

      <!-- 候选 channel 池 -->
      <section class="block">
        <GroupMemberPicker
          :candidates="candidatesEnriched"
          :excluded-runtime-ids="excludedRuntimeIds"
          :loading="loadingCandidates || loadingPricing"
          :pivot-stack-dollars-per-yuan="pivotStackDollarsPerYuan"
          @add="addMembers"
          @channel-changed="loadCandidates"
        />
      </section>

      <!-- 底部 sticky 保存栏 -->
      <footer class="sticky" v-if="dirtyMembers">
        <span class="sticky__hint">⚠ 你的挂载/默认选择尚未保存到后端</span>
        <n-button size="small" :loading="savingMembers" type="primary" @click="saveMembers">
          <template #icon><Save :size="13" /></template>
          保存挂载与默认（{{ members.length }} 条）
        </n-button>
      </footer>
    </template>
  </PageContainer>
</template>

<style scoped>
.meta-card { padding: 18px; background: #0a0a0a; border: 1px solid rgba(255,255,255,0.06); border-radius: 8px; }
.meta-card__head { display: flex; align-items: baseline; justify-content: space-between; margin-bottom: 14px; }
.meta-card__title { color: #ededed; font-size: 14px; font-weight: 500; margin: 0; display: flex; align-items: center; gap: 6px; }
.meta-card__id { color: #707070; font-size: 11px; }
.meta-form { display: grid; grid-template-columns: 1fr 1fr; gap: 14px 24px; }
.meta-form__row { display: grid; grid-template-columns: 120px 1fr; gap: 12px; align-items: center; font-size: 12px; }
.meta-form__row--top { align-items: flex-start; padding-top: 4px; }
.meta-form__row label { color: #707070; }
.meta-form__actions { grid-column: 1 / -1; display: flex; justify-content: flex-end; }
.block { margin-top: 16px; padding: 16px; background: #0a0a0a; border: 1px solid rgba(255,255,255,0.06); border-radius: 8px; }
.sticky {
  position: sticky; bottom: 0; margin-top: 16px;
  padding: 12px 16px; background: rgba(10, 10, 10, 0.96);
  border: 1px solid rgba(255, 200, 0, 0.3); border-radius: 8px;
  display: flex; justify-content: space-between; align-items: center;
  backdrop-filter: blur(8px);
}
.sticky__hint { color: #ffcc66; font-size: 12px; }
.dim { color: #707070; }
.small { font-size: 11px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; font-variant-numeric: tabular-nums; }
@media (max-width: 900px) { .meta-form { grid-template-columns: 1fr; } }
</style>
