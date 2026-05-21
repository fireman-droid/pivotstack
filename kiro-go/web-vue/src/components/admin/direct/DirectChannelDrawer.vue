<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { NInput, NInputNumber, NDynamicTags, NButton, NSwitch, useMessage } from 'naive-ui'
import { Plug, Plus, Pencil } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import {
  createDirectChannel, patchDirectChannel,
  type DirectChannel, type DirectChannelCreateRequest,
} from '../../../api/admin/directChannels'

const props = defineProps<{
  show: boolean
  /** 类型：openai 透传 / kiro 账号池；create 模式必填；edit 模式从 row 推断 */
  type: 'openai' | 'kiro'
  /** 编辑时传入；create 模式传 null */
  row: DirectChannel | null
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'saved'): void
}>()

const message = useMessage()
const submitting = ref(false)

const mode = computed<'create' | 'edit'>(() => (props.row ? 'edit' : 'create'))
const channelType = computed<'openai' | 'kiro'>(() => props.row?.type || props.type)
const title = computed(() => {
  const t = channelType.value === 'openai' ? 'OpenAI 透传' : 'Kiro 账号池'
  return mode.value === 'create' ? `创建 ${t} 渠道` : `编辑 ${t} · ${props.row?.alias}`
})
const subtitle = computed(() => mode.value === 'create'
  ? '配置 alias + baseURL/API Key + 售价；售价两项都 0 = 沿用全局'
  : '调整 alias / 模型 / 售价 / 启用；API Key 留空 = 不修改')

interface FormState {
  alias: string
  baseUrl: string
  apiKey: string
  models: string[]
  inputPerM: number
  outputPerM: number
  costInputPerM: number
  costOutputPerM: number
  enabled: boolean
}

const initial: FormState = {
  alias: '',
  baseUrl: '',
  apiKey: '',
  models: [],
  inputPerM: 0,
  outputPerM: 0,
  costInputPerM: 0,
  costOutputPerM: 0,
  enabled: true,
}
const form = ref<FormState>({ ...initial })

watch(() => props.show, v => {
  if (!v) return
  if (props.row) {
    const def = props.row.sellPrice?.default
    form.value = {
      alias: props.row.alias || '',
      baseUrl: props.row.baseUrl || '',
      apiKey: '',
      models: [...(props.row.models || [])],
      inputPerM: def?.inputPerM ?? 0,
      outputPerM: def?.outputPerM ?? 0,
      costInputPerM: def?.costInputPerM ?? 0,
      costOutputPerM: def?.costOutputPerM ?? 0,
      enabled: props.row.enabled,
    }
  } else {
    form.value = { ...initial, baseUrl: channelType.value === 'openai' ? 'https://api.openai.com/v1' : '' }
  }
})

function close() {
  if (submitting.value) return
  emit('update:show', false)
}

async function submit() {
  if (!form.value.alias.trim()) {
    message.warning('请填写渠道 alias，便于路由识别')
    return
  }
  if (channelType.value === 'openai' && mode.value === 'create' && !form.value.apiKey.trim()) {
    message.warning('OpenAI 渠道首次创建需要 API Key')
    return
  }

  submitting.value = true
  try {
    // edit 模式始终提交 sellPrice.default —— 否则把 4 个字段全清成 0 时 patch 会省略 sellPrice，
    // 后端 isZeroSellPrice 检测到会保留旧值，违反"显式 0 就是 0"的语义。
    // create 模式可以省略（admin 全 0 = 不写 SellPrice 节点，沿用全局兜底）。
    const hasAnyPriceField = form.value.inputPerM > 0
      || form.value.outputPerM > 0
      || form.value.costInputPerM > 0
      || form.value.costOutputPerM > 0
    const sellPrice = (mode.value === 'edit' || hasAnyPriceField)
      ? {
          default: {
            inputPerM: form.value.inputPerM,
            outputPerM: form.value.outputPerM,
            costInputPerM: form.value.costInputPerM,
            costOutputPerM: form.value.costOutputPerM,
          },
        }
      : undefined

    if (mode.value === 'create') {
      const req: DirectChannelCreateRequest = {
        type: channelType.value,
        alias: form.value.alias.trim(),
        models: form.value.models.length ? form.value.models : undefined,
        sellPrice,
        enabled: form.value.enabled,
      }
      if (channelType.value === 'openai') {
        req.baseUrl = form.value.baseUrl.trim()
        req.apiKey = form.value.apiKey.trim()
      }
      await createDirectChannel(req)
      message.success('已创建')
    } else {
      const id = props.row!.id
      const patch: Partial<DirectChannel> = {
        alias: form.value.alias.trim(),
        models: form.value.models,
        sellPrice,
        enabled: form.value.enabled,
      }
      if (channelType.value === 'openai') {
        patch.baseUrl = form.value.baseUrl.trim()
        // apiKey 仅在用户填了新值时才覆盖；留空 = 不修改原 key
        if (form.value.apiKey.trim()) {
          (patch as any).apiKey = form.value.apiKey.trim()
        }
      }
      await patchDirectChannel(id, patch)
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
    <RefinedField label="Alias" required hint="路由识别名">
      <n-input
        v-model:value="form.alias"
        placeholder="例：openai-prod / kiro-pool-cn"
        maxlength="60"
        show-count
      />
    </RefinedField>

    <template v-if="channelType === 'openai'">
      <RefinedField label="Base URL" required>
        <n-input v-model:value="form.baseUrl" placeholder="https://api.openai.com/v1" />
      </RefinedField>
      <RefinedField
        label="API Key"
        :required="mode === 'create'"
        :hint="mode === 'edit' ? '留空 = 不修改原 key' : '上游 sk-... 凭据'"
      >
        <n-input
          v-model:value="form.apiKey"
          type="password"
          show-password-on="click"
          placeholder="sk-..."
        />
      </RefinedField>
    </template>

    <div v-else class="dcd-kiro-hint">
      Kiro 账号池渠道复用全局 kiro account pool。这里只管 alias / 透传模型清单 / 定价 / 启用状态；账号本体在「上游账号」管理。
    </div>

    <RefinedField label="支持的模型" hint="留空 = 透传所有模型">
      <n-dynamic-tags v-model:value="form.models" />
    </RefinedField>

    <RefinedField label="售价 IN" hint="虚拟$/Mtok（1¥ = 20 虚拟$）；经营看板展示时按系统汇率换算成 ¥">
      <n-input-number
        v-model:value="form.inputPerM"
        :min="0"
        :precision="6"
        :step="0.5"
        style="width: 100%"
      >
        <template #suffix>虚拟$/Mtok</template>
      </n-input-number>
    </RefinedField>

    <RefinedField label="售价 OUT" hint="两项都 0 = 沿用全局定价表">
      <n-input-number
        v-model:value="form.outputPerM"
        :min="0"
        :precision="6"
        :step="0.5"
        style="width: 100%"
      >
        <template #suffix>虚拟$/Mtok</template>
      </n-input-number>
    </RefinedField>

    <RefinedField label="成本 IN" hint="虚拟$/Mtok；上游真实成本，经营看板会换算成 ¥">
      <n-input-number
        v-model:value="form.costInputPerM"
        :min="0"
        :precision="6"
        :step="0.5"
        style="width: 100%"
      >
        <template #suffix>虚拟$/Mtok</template>
      </n-input-number>
    </RefinedField>

    <RefinedField label="成本 OUT" hint="留 0 = 该渠道在经营看板里成本计 0（admin 显式选择）">
      <n-input-number
        v-model:value="form.costOutputPerM"
        :min="0"
        :precision="6"
        :step="0.5"
        style="width: 100%"
      >
        <template #suffix>虚拟$/Mtok</template>
      </n-input-number>
    </RefinedField>

    <RefinedField label="启用">
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
.dcd-kiro-hint {
  padding: 10px 12px;
  margin-bottom: 16px;
  background: rgba(82, 168, 255, 0.05);
  border: 1px solid rgba(82, 168, 255, 0.15);
  border-radius: 4px;
  color: #a1a1a1;
  font-size: 12px;
  line-height: 1.5;
}
</style>
