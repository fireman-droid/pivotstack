<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { NInput, NInputNumber, NSwitch, NButton, useMessage } from 'naive-ui'
import { Server, Plus, Pencil } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import { createProvider, updateProvider, type NewAPIProvider, type NewAPIProviderUpsert } from '../../../api/admin/providers'

const props = defineProps<{
  show: boolean
  /** 编辑时传入；create 模式传 null */
  row: NewAPIProvider | null
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'saved'): void
}>()

const message = useMessage()
const submitting = ref(false)

const mode = computed<'create' | 'edit'>(() => (props.row ? 'edit' : 'create'))
const title = computed(() => (mode.value === 'create' ? '接入新 NewAPI 上游' : `编辑上游 · ${props.row?.name || props.row?.id}`))
const subtitle = computed(() =>
  mode.value === 'create'
    ? '配置上游 baseURL + 登录凭据，渠道 ID 创建后不可改'
    : '调整上游元数据；密码留空表示不修改',
)

interface FormState {
  id: string
  name: string
  baseUrl: string
  username: string
  password: string
  quotaPerUnitDollar: number
  yuanPerUpstreamDollar: number
  syncIntervalSec: number
  enabled: boolean
}

const initial: FormState = {
  id: '',
  name: '',
  baseUrl: '',
  username: '',
  password: '',
  quotaPerUnitDollar: 500000,
  yuanPerUpstreamDollar: 7.2,
  syncIntervalSec: 600,
  enabled: true,
}
const form = ref<FormState>({ ...initial })

watch(() => props.show, v => {
  if (!v) return
  if (props.row) {
    form.value = {
      id: props.row.id || '',
      name: props.row.name || '',
      baseUrl: props.row.baseUrl || '',
      username: '',
      password: '',
      quotaPerUnitDollar: props.row.quotaPerUnitDollar ?? 500000,
      yuanPerUpstreamDollar: props.row.yuanPerUpstreamDollar ?? 7.2,
      syncIntervalSec: props.row.syncIntervalSec ?? 600,
      enabled: props.row.enabled ?? true,
    }
  } else {
    form.value = { ...initial }
  }
})

function close() {
  if (submitting.value) return
  emit('update:show', false)
}

async function submit() {
  const f = form.value
  if (mode.value === 'create') {
    if (!f.id.trim()) {
      message.warning('请填写上游 ID（如 apijing），后续渠道 ID 会用此前缀')
      return
    }
    if (!f.baseUrl.trim()) {
      message.warning('请填写上游 baseURL')
      return
    }
  }
  submitting.value = true
  try {
    if (mode.value === 'create') {
      const req: NewAPIProviderUpsert = {
        id: f.id.trim(),
        name: f.name.trim() || f.id.trim(),
        baseUrl: f.baseUrl.trim(),
        username: f.username.trim() || undefined,
        password: f.password || undefined,
        quotaPerUnitDollar: f.quotaPerUnitDollar,
        yuanPerUpstreamDollar: f.yuanPerUpstreamDollar,
        syncIntervalSec: f.syncIntervalSec,
        enabled: f.enabled,
      }
      await createProvider(req)
      message.success('已接入')
    } else {
      const req: Partial<NewAPIProviderUpsert> = {
        name: f.name.trim() || undefined,
        baseUrl: f.baseUrl.trim(),
        quotaPerUnitDollar: f.quotaPerUnitDollar,
        yuanPerUpstreamDollar: f.yuanPerUpstreamDollar,
        syncIntervalSec: f.syncIntervalSec,
        enabled: f.enabled,
      }
      if (f.username.trim()) req.username = f.username.trim()
      if (f.password) req.password = f.password
      await updateProvider(props.row!.id, req)
      message.success('已保存')
    }
    emit('saved')
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
    :width="560"
    @update:show="(v) => emit('update:show', v)"
  >
    <RefinedField label="上游 ID" required hint="小写、URL-safe，创建后不可改">
      <n-input
        v-model:value="form.id"
        :disabled="mode === 'edit'"
        placeholder="apijing / tcdmx / oneapi-mirror"
      />
    </RefinedField>

    <RefinedField label="显示名">
      <n-input v-model:value="form.name" placeholder="比如 APIJing 主线" maxlength="40" show-count />
    </RefinedField>

    <RefinedField label="Base URL" required>
      <n-input v-model:value="form.baseUrl" placeholder="https://api.example.com" />
    </RefinedField>

    <RefinedField label="上游用户名">
      <n-input v-model:value="form.username" placeholder="登录账号" />
    </RefinedField>

    <RefinedField
      label="上游密码"
      :hint="mode === 'edit' ? '留空 = 不修改原密码' : '用于 NewAPI 登录拉 token'"
    >
      <n-input
        v-model:value="form.password"
        type="password"
        show-password-on="click"
        placeholder="••••••••"
      />
    </RefinedField>

    <RefinedField label="quota / $" hint="quota 算法系数；apijing 默认 500000">
      <n-input-number v-model:value="form.quotaPerUnitDollar" :min="1" :step="1000" style="width: 100%" />
    </RefinedField>

    <RefinedField label="¥ / 上游 $ 汇率" hint="⚠ 真上游 $ 兑 ¥ 汇率（常用 7.2），不是 PivotStack 1:20 单位">
      <n-input-number
        v-model:value="form.yuanPerUpstreamDollar"
        :min="0.01"
        :precision="4"
        :step="0.1"
        style="width: 100%"
      />
    </RefinedField>

    <RefinedField label="同步间隔" hint="秒；最小 60s">
      <n-input-number v-model:value="form.syncIntervalSec" :min="60" :step="60" style="width: 100%" />
    </RefinedField>

    <RefinedField label="启用">
      <n-switch v-model:value="form.enabled" />
    </RefinedField>

    <template #footer>
      <n-button :disabled="submitting" quaternary @click="close">取消</n-button>
      <n-button type="primary" :loading="submitting" @click="submit">
        {{ mode === 'create' ? '接入' : '保存' }}
      </n-button>
    </template>
  </RefinedDrawer>
</template>
