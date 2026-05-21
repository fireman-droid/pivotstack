<script setup lang="ts">
import { ref } from 'vue'
import { NPopover, NButton, NInput, NInputNumber, NSwitch, NSpace, useMessage } from 'naive-ui'
import { Pencil, Save } from 'lucide-vue-next'
import { patchNewAPIChannel } from '../../../api/admin'
import { patchDirectChannel } from '../../../api/admin/directChannels'

const props = defineProps<{
  sourceType: 'newapi' | 'direct'
  channelId: string
  alias: string
  markup?: number
  enabled: boolean
}>()

const emit = defineEmits<{ (e: 'saved'): void }>()

const message = useMessage()
const open = ref(false)
const editAlias = ref('')
const editMarkup = ref(1)
const editEnabled = ref(true)
const saving = ref(false)

function resetForm() {
  editAlias.value = props.alias
  editMarkup.value = props.markup ?? 1
  editEnabled.value = !!props.enabled
}

// popover show change handler — 同步 reset，确保 NSwitch 拿到当前 props 值不滞后
function handleShowChange(v: boolean) {
  open.value = v
  if (v) resetForm()
}

async function save() {
  if (!editAlias.value.trim()) {
    message.error('渠道名不能为空')
    return
  }
  saving.value = true
  try {
    if (props.sourceType === 'newapi') {
      await patchNewAPIChannel(props.channelId, {
        alias: editAlias.value.trim(),
        markup: editMarkup.value,
        enabled: editEnabled.value,
      })
    } else {
      await patchDirectChannel(props.channelId, {
        alias: editAlias.value.trim(),
        enabled: editEnabled.value,
      } as any)
    }
    message.success('已保存')
    open.value = false
    emit('saved')
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <n-popover
    :show="open"
    @update:show="handleShowChange"
    trigger="click"
    placement="left"
    :show-arrow="false"
    style="padding: 14px; min-width: 280px"
  >
    <template #trigger>
      <n-button size="tiny" quaternary>
        <template #icon><Pencil :size="12" /></template>
        编辑
      </n-button>
    </template>
    <div class="pop">
      <div class="pop__title">编辑渠道</div>
      <div class="pop__row">
        <label>渠道名</label>
        <n-input v-model:value="editAlias" size="small" placeholder="user 看到的名字" />
      </div>
      <div v-if="sourceType === 'newapi'" class="pop__row">
        <label>Markup</label>
        <n-input-number v-model:value="editMarkup" :min="0.1" :max="50" :step="0.1" :precision="2" size="small" style="width: 100%" />
      </div>
      <div class="pop__row">
        <label>启用</label>
        <n-switch v-model:value="editEnabled" size="small" />
      </div>
      <n-space justify="end" style="margin-top: 4px">
        <n-button size="tiny" quaternary @click="open = false">取消</n-button>
        <n-button size="tiny" type="primary" :loading="saving" @click="save">
          <template #icon><Save :size="11" /></template>
          保存
        </n-button>
      </n-space>
    </div>
  </n-popover>
</template>

<style scoped>
.pop { display: flex; flex-direction: column; gap: 10px; }
.pop__title { color: #ededed; font-size: 12px; font-weight: 500; padding-bottom: 6px; border-bottom: 1px solid rgba(255,255,255,0.08); }
.pop__row { display: grid; grid-template-columns: 80px 1fr; gap: 10px; align-items: center; font-size: 12px; }
.pop__row label { color: #707070; }
/* switch / 短控件靠左对齐，不要 stretch 占满整列 */
.pop__row :deep(.n-switch) { justify-self: start; }
</style>
