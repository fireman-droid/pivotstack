<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  NInput, NInputNumber, NSwitch, NSelect, NButton, NRadioGroup, NRadio,
  useMessage,
} from 'naive-ui'
import { Plus, KeyRound } from 'lucide-vue-next'
import { createNewAPIChannel } from '../../../api/admin'
import RefinedDialog from '../../common/RefinedDialog.vue'
import RefinedField from '../../common/RefinedField.vue'

const props = defineProps<{
  show: boolean
  providerId: string
  providerName?: string
  groups: Array<{ name: string; ratio?: number; desc?: string }>
}>()

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'created', payload: { alias: string; group: string }): void
}>()

const message = useMessage()
const submitting = ref(false)

const form = ref({
  alias: '',
  group: '',
  unlimitedQuota: true,
  remainQuota: 100000,
  expiredMode: 'never' as 'never' | 'days',
  expiredDays: 30,
  modelLimitsEnabled: false,
  modelLimits: '',
})

watch(() => props.show, v => {
  if (v) {
    form.value = {
      alias: '',
      group: props.groups[0]?.name || '',
      unlimitedQuota: true,
      remainQuota: 100000,
      expiredMode: 'never',
      expiredDays: 30,
      modelLimitsEnabled: false,
      modelLimits: '',
    }
  }
})

const groupOptions = computed(() => props.groups.map(g => ({
  label: `${g.name}${g.ratio ? `   ·   倍率 ${g.ratio}×` : ''}`,
  value: g.name,
})))

async function submit() {
  const f = form.value
  if (!f.alias.trim()) { message.error('渠道名必填'); return }
  if (!f.group) { message.error('请选择上游分组'); return }
  const expiredTime = f.expiredMode === 'never' ? -1 : Math.floor(Date.now() / 1000) + f.expiredDays * 86400
  submitting.value = true
  try {
    await createNewAPIChannel({
      providerId: props.providerId,
      alias: f.alias.trim(),
      group: f.group,
      models: [],
      markup: 1.0,
      remainQuota: f.unlimitedQuota ? 0 : f.remainQuota,
      unlimitedQuota: f.unlimitedQuota,
      expiredTime,
      modelLimitsEnabled: f.modelLimitsEnabled,
      modelLimits: f.modelLimitsEnabled ? f.modelLimits.trim() : '',
      crossGroupRetry: false,
      allowIPs: '',
    })
    message.success(`已创建「${f.alias}」并物化为 PivotStack 渠道`)
    emit('created', { alias: f.alias.trim(), group: f.group })
    emit('update:show', false)
  } catch (e: any) {
    message.error(e?.message || '创建失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <RefinedDialog
    :show="show"
    @update:show="(v: boolean) => emit('update:show', v)"
    :icon="KeyRound"
    title="在上游创建新渠道"
    :subtitle="`上游 = ${providerName || providerId}  ·  会同步生成 PivotStack channel，默认 markup 1×（去分组总览改）`"
    :width="580"
    :mask-closable="!submitting"
  >
    <RefinedField label="渠道名" required hint="同时作为上游 token name 和分组总览里的渠道名">
      <n-input v-model:value="form.alias" placeholder="例如 claude-opus-vip-1" :maxlength="64" />
    </RefinedField>

    <RefinedField label="上游分组" required hint="决定上游计费倍率（aws稳定 2× / kiro引流福利 0.1×）；不同分组单价差异极大">
      <n-select v-model:value="form.group" :options="groupOptions" filterable />
    </RefinedField>

    <RefinedField label="额度" :hint="form.unlimitedQuota ? '不限额度（卖给客户时由 PivotStack 侧管控）' : '限额上游 quota 单位（500000 = $1）'">
      <div class="row">
        <n-switch v-model:value="form.unlimitedQuota" size="small" />
        <span class="row__hint mono">{{ form.unlimitedQuota ? '不限额度' : '限额' }}</span>
        <n-input-number
          v-if="!form.unlimitedQuota"
          v-model:value="form.remainQuota"
          :min="1" :step="100000"
          style="flex: 1; max-width: 200px"
        />
      </div>
    </RefinedField>

    <RefinedField label="有效期" :hint="form.expiredMode === 'never' ? '永不过期（推荐）' : `从现在起 ${form.expiredDays} 天后过期`">
      <div class="row">
        <n-radio-group v-model:value="form.expiredMode">
          <n-radio value="never">永不过期</n-radio>
          <n-radio value="days">N 天后</n-radio>
        </n-radio-group>
        <n-input-number
          v-if="form.expiredMode === 'days'"
          v-model:value="form.expiredDays"
          :min="1" :max="3650" :step="1"
          style="width: 100px"
        />
        <span v-if="form.expiredMode === 'days'" class="row__hint">天</span>
      </div>
    </RefinedField>

    <RefinedField label="模型白名单" :hint="form.modelLimitsEnabled ? '逗号分隔模型名' : '不限：用上游分组的所有模型'">
      <div class="row row--top">
        <n-switch v-model:value="form.modelLimitsEnabled" size="small" />
        <n-input
          v-if="form.modelLimitsEnabled"
          v-model:value="form.modelLimits"
          placeholder="claude-opus-4-7,claude-sonnet-4-6"
          style="flex: 1"
        />
        <span v-else class="row__hint">不限</span>
      </div>
    </RefinedField>

    <template #footer>
      <n-button size="small" quaternary @click="emit('update:show', false)" :disabled="submitting">取消</n-button>
      <n-button size="small" type="primary" :loading="submitting" @click="submit">
        <template #icon><Plus :size="13" /></template>
        创建
      </n-button>
    </template>
  </RefinedDialog>
</template>

<style scoped>
.row { display: flex; align-items: center; gap: 12px; min-height: 30px; }
.row--top { align-items: flex-start; }
.row__hint { color: #707070; font-size: 12px; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; }
</style>
