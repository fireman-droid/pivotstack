<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { NInput, NInputNumber, NDatePicker, NSwitch, NButton, useMessage } from 'naive-ui'
import { KeyRound, Plus, Pencil } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import { userApi } from '../../../api/user'

export interface ChildKey {
  id: string
  note?: string
  key?: string
  keyMasked?: string
  balance?: number
  giftBalance?: number
  totalBalance?: number
  enabled: boolean
  expiresAt?: number
  requests?: number
  recentCalls7d?: number
}

const props = defineProps<{
  show: boolean
  /** 编辑模式传入；create 模式传 null */
  row: ChildKey | null
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'saved'): void
  (e: 'created', row: ChildKey): void
}>()

const message = useMessage()
const submitting = ref(false)

const CNY_PER_USD = 0.05

interface FormState {
  note: string
  balanceCNY: number
  expiresAtMs: number | null
  enabled: boolean
}

const initial: FormState = { note: '', balanceCNY: 0, expiresAtMs: null, enabled: true }
const form = ref<FormState>({ ...initial })

const mode = computed<'create' | 'edit'>(() => (props.row ? 'edit' : 'create'))
const title = computed(() => mode.value === 'create' ? '创建子 Key' : `编辑 · ${props.row?.note || props.row?.id}`)
const subtitle = computed(() => mode.value === 'create'
  ? '初始余额从你的代理账户划转过去'
  : '余额改动会立即从账户充入或退回')

watch(() => props.show, v => {
  if (!v) return
  if (props.row) {
    form.value = {
      note: props.row.note || '',
      balanceCNY: Number(((props.row.balance || 0) * CNY_PER_USD).toFixed(2)),
      expiresAtMs: props.row.expiresAt ? props.row.expiresAt * 1000 : null,
      enabled: props.row.enabled,
    }
  } else {
    form.value = { ...initial }
  }
})

const editPreview = computed(() => {
  if (mode.value !== 'edit' || !props.row) return null
  const oldCNY = (props.row.balance || 0) * CNY_PER_USD
  const newCNY = Number(form.value.balanceCNY) || 0
  const delta = newCNY - oldCNY
  return { oldCNY, newCNY, delta }
})

function close() {
  if (submitting.value) return
  emit('update:show', false)
}

async function submit() {
  if (mode.value === 'create' && !form.value.note.trim()) {
    message.warning('请填写备注')
    return
  }
  submitting.value = true
  try {
    const newExpiresAt = form.value.expiresAtMs ? Math.floor(form.value.expiresAtMs / 1000) : 0
    if (mode.value === 'create') {
      const initialUSD = (Number(form.value.balanceCNY) || 0) / CNY_PER_USD
      const data = await userApi('/reseller/keys', {
        method: 'POST',
        body: { note: form.value.note.trim(), initialBalanceUSD: initialUSD, expiresAt: newExpiresAt },
      })
      message.success('已创建')
      emit('created', data as ChildKey)
    } else {
      const newBalanceUSD = (Number(form.value.balanceCNY) || 0) / CNY_PER_USD
      await userApi(`/reseller/keys/${props.row!.id}`, {
        method: 'PATCH',
        body: {
          note: form.value.note.trim(),
          balance: newBalanceUSD,
          expiresAt: newExpiresAt,
          enabled: form.value.enabled,
        },
      })
      message.success('已保存')
      emit('saved')
    }
    emit('update:show', false)
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <RefinedDrawer
    :show="show"
    :title="title"
    :subtitle="subtitle"
    :icon="mode === 'create' ? Plus : Pencil"
    :loading="submitting"
    :width="500"
    @update:show="(v) => emit('update:show', v)"
  >
    <RefinedField label="备注" required hint="客户名 / 用途，用于识别">
      <n-input v-model:value="form.note" placeholder="如：李总 / 9月推广" maxlength="60" show-count />
    </RefinedField>

    <RefinedField
      :label="mode === 'create' ? '初始余额 ¥' : '当前余额 ¥'"
      :hint="mode === 'create' ? '从你的代理账户划转' : '覆盖式：调高=充入；调低=退回'"
    >
      <n-input-number
        v-model:value="form.balanceCNY"
        :min="0"
        :precision="2"
        :step="10"
        style="width: 100%"
      >
        <template #prefix>¥</template>
      </n-input-number>
    </RefinedField>

    <div v-if="editPreview && editPreview.delta !== 0" class="rckd-preview" :class="{ 'rckd-preview--neg': editPreview.delta < 0 }">
      从 ¥{{ editPreview.oldCNY.toFixed(2) }} 调整为 ¥{{ editPreview.newCNY.toFixed(2) }}
      （{{ editPreview.delta > 0 ? '充入' : '退回' }} ¥{{ Math.abs(editPreview.delta).toFixed(2) }}）
    </div>

    <RefinedField label="过期时间" hint="留空 = 永久；过期后子 Key 不可用">
      <n-date-picker
        v-model:value="form.expiresAtMs"
        type="datetime"
        clearable
        format="yyyy-MM-dd HH:mm"
        style="width: 100%"
      />
    </RefinedField>

    <RefinedField v-if="mode === 'edit'" label="启用" hint="关闭后立即停用">
      <n-switch v-model:value="form.enabled" />
    </RefinedField>

    <template #footer>
      <n-button :disabled="submitting" quaternary @click="close">取消</n-button>
      <n-button type="primary" :loading="submitting" @click="submit">
        {{ mode === 'create' ? '创建' : '保存' }}
      </n-button>
    </template>
  </RefinedDrawer>
</template>

<style scoped>
.rckd-preview {
  margin: -8px 0 16px;
  padding: 8px 12px;
  border-radius: 4px;
  background: rgba(11, 212, 112, 0.08);
  border: 1px solid rgba(11, 212, 112, 0.20);
  color: #0bd470;
  font-size: 12px;
}
.rckd-preview--neg {
  background: rgba(245, 166, 35, 0.08);
  border-color: rgba(245, 166, 35, 0.20);
  color: #f5a623;
}
</style>
