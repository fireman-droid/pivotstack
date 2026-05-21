<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import {
  NInput, NInputNumber, NSelect, NSwitch, NButton, NDatePicker, useMessage,
} from 'naive-ui'
import { Pencil, Plus } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import { createApiKey, updateApiKey, type ApiKeyRow } from '../../../api/admin/keys'

const props = defineProps<{
  show: boolean
  /** 编辑时传入；create 模式传 null */
  row: ApiKeyRow | null
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'created', row: ApiKeyRow): void
  (e: 'updated', row: ApiKeyRow): void
}>()

const message = useMessage()
const submitting = ref(false)

const mode = computed<'create' | 'edit'>(() => (props.row ? 'edit' : 'create'))
const title = computed(() =>
  mode.value === 'create' ? '创建 API Key' : `编辑 Key · ${props.row?.note || props.row?.id}`,
)
const subtitle = computed(() =>
  mode.value === 'create'
    ? '配置 plan、初始余额、过期时间。后续可在列表里随时编辑。'
    : '调整 plan / 余额 / 过期 / 启用 / 分销开关。',
)

const planOptions = [
  { label: '余额卡 (按消费扣)', value: 'credit' },
  { label: '天卡 (按时间订阅)', value: 'timed' },
  { label: '混合 (余额 + 天卡)', value: 'hybrid' },
]

interface FormState {
  note: string
  plan: string
  balance: number
  giftBalance: number
  enabled: boolean
  isReseller: boolean
  maxChildKeys: number
  expiresAt: number | null
}

const initial: FormState = {
  note: '',
  plan: 'credit',
  balance: 0,
  giftBalance: 0,
  enabled: true,
  isReseller: false,
  maxChildKeys: 0,
  expiresAt: null,
}
const form = ref<FormState>({ ...initial })

watch(() => props.show, v => {
  if (!v) return
  if (props.row) {
    form.value = {
      note: props.row.note || '',
      plan: props.row.plan || 'credit',
      balance: props.row.balance ?? 0,
      giftBalance: props.row.giftBalance ?? 0,
      enabled: props.row.enabled,
      isReseller: !!props.row.isReseller,
      maxChildKeys: props.row.maxChildKeys ?? 0,
      expiresAt: props.row.expiresAt ?? null,
    }
  } else {
    form.value = { ...initial }
  }
})

const isChild = computed(() => !!props.row?.parentKeyId)

// NDatePicker 需要 ms 时间戳；form.expiresAt 存 Unix 秒。
const expiresAtMs = computed<number | null>({
  get: () => (form.value.expiresAt != null && form.value.expiresAt > 0 ? form.value.expiresAt * 1000 : null),
  set: (ms) => { form.value.expiresAt = ms ? Math.floor(ms / 1000) : null },
})

function close() {
  if (submitting.value) return
  emit('update:show', false)
}

async function submit() {
  if (!form.value.note.trim()) {
    message.warning('请填写备注，方便后续识别')
    return
  }
  submitting.value = true
  try {
    if (mode.value === 'create') {
      const created = await createApiKey(form.value.note.trim())
      const needPatch = form.value.balance > 0 || form.value.giftBalance > 0
        || form.value.plan !== 'credit' || form.value.isReseller || form.value.expiresAt
      if (needPatch) {
        await updateApiKey(created.id, {
          plan: form.value.plan,
          balance: form.value.balance,
          giftBalance: form.value.giftBalance,
          isReseller: form.value.isReseller,
          maxChildKeys: form.value.maxChildKeys,
          expiresAt: form.value.expiresAt ?? undefined,
        })
      }
      message.success('已创建')
      emit('created', created)
    } else {
      const id = props.row!.id
      const updated = await updateApiKey(id, {
        note: form.value.note.trim(),
        plan: form.value.plan,
        balance: form.value.balance,
        giftBalance: form.value.giftBalance,
        enabled: form.value.enabled,
        isReseller: isChild.value ? undefined : form.value.isReseller,
        maxChildKeys: form.value.isReseller ? form.value.maxChildKeys : undefined,
        expiresAt: form.value.expiresAt ?? undefined,
      })
      message.success('已保存')
      emit('updated', updated)
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
    :width="520"
    @update:show="(v) => emit('update:show', v)"
  >
    <RefinedField label="备注" required hint="客户名 / 用途，用于运营识别">
      <n-input v-model:value="form.note" placeholder="如：玉米地大佬" maxlength="80" show-count />
    </RefinedField>

    <RefinedField label="套餐" hint="credit 用余额扣费；timed 用天卡；hybrid 两者并存">
      <n-select v-model:value="form.plan" :options="planOptions" />
    </RefinedField>

    <RefinedField label="付费余额 ¥">
      <n-input-number v-model:value="form.balance" :min="0" :precision="2" :step="10" style="width: 100%">
        <template #prefix>¥</template>
      </n-input-number>
    </RefinedField>

    <RefinedField label="赠送余额 ¥" hint="赠送优先消耗，不计入实际收入">
      <n-input-number v-model:value="form.giftBalance" :min="0" :precision="2" :step="5" style="width: 100%">
        <template #prefix>¥</template>
      </n-input-number>
    </RefinedField>

    <RefinedField label="过期时间" hint="留空 = 永久；过期后 Key 立即不可用">
      <n-date-picker
        v-model:value="expiresAtMs"
        type="datetime"
        clearable
        format="yyyy-MM-dd HH:mm"
        placeholder="选择过期日期"
        style="width: 100%"
      />
    </RefinedField>

    <RefinedField v-if="mode === 'edit'" label="启用" hint="关闭后该 Key 立即停用，再开启需手动切回">
      <n-switch v-model:value="form.enabled" />
    </RefinedField>

    <RefinedField v-if="!isChild" label="代理商" hint="允许该 Key 派生子 Key 进行二级分销">
      <n-switch v-model:value="form.isReseller" />
    </RefinedField>

    <RefinedField v-if="form.isReseller && !isChild" label="子 Key 上限" hint="0 = 无上限">
      <n-input-number v-model:value="form.maxChildKeys" :min="0" :step="10" style="width: 100%" />
    </RefinedField>

    <template #footer>
      <n-button :disabled="submitting" quaternary @click="close">取消</n-button>
      <n-button type="primary" :loading="submitting" @click="submit">
        {{ mode === 'create' ? '创建' : '保存' }}
      </n-button>
    </template>
  </RefinedDrawer>
</template>
