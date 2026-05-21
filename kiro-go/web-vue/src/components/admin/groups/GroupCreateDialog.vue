<script setup lang="ts">
import { ref, watch } from 'vue'
import { NInput, NSwitch, NButton, useMessage } from 'naive-ui'
import { Layers, Plus } from 'lucide-vue-next'
import { createChannelGroup } from '../../../api/admin/groups'
import RefinedDialog from '../../common/RefinedDialog.vue'
import RefinedField from '../../common/RefinedField.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'created', id: string): void
}>()

const message = useMessage()
const submitting = ref(false)
const form = ref({ id: '', name: '', description: '', enabled: true })
const idError = ref('')

watch(() => props.show, v => {
  if (v) {
    form.value = { id: '', name: '', description: '', enabled: true }
    idError.value = ''
  }
})

function validateId(): boolean {
  const id = form.value.id.trim()
  if (!id) { idError.value = '请输入英文 ID'; return false }
  if (!/^[a-zA-Z0-9_-]{1,64}$/.test(id)) { idError.value = '只能用字母数字 _ -，最多 64 字符'; return false }
  idError.value = ''
  return true
}

async function submit() {
  if (!validateId()) return
  if (!form.value.name.trim()) { message.error('请输入分组名'); return }
  submitting.value = true
  try {
    const created = await createChannelGroup({
      id: form.value.id.trim(),
      name: form.value.name.trim(),
      description: form.value.description.trim() || undefined,
      enabled: form.value.enabled,
    })
    message.success(`已创建「${created.name}」`)
    emit('created', created.id)
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
    :icon="Layers"
    title="新建分组"
    subtitle="分组是 admin 自定义的对外卖品——决定 user 能挑哪些 channel"
    :width="500"
    :mask-closable="!submitting"
  >
    <RefinedField label="ID" required :error="idError" hint="英文/数字/_/-，例如 claude、codex、gemini">
      <n-input v-model:value="form.id" placeholder="claude / codex / gemini" :maxlength="64" :on-blur="validateId" />
    </RefinedField>

    <RefinedField label="分组名" required hint="user 在 Dashboard 上看到的名字">
      <n-input v-model:value="form.name" placeholder="Claude 分组 / Codex 分组" :maxlength="80" />
    </RefinedField>

    <RefinedField label="描述" hint="可选：说明本分组用途、目标客户、定价档位">
      <n-input v-model:value="form.description" type="textarea" placeholder="例如：高质量 claude 系，opus 推荐用" :maxlength="200" :autosize="{ minRows: 2, maxRows: 4 }" />
    </RefinedField>

    <RefinedField label="启用">
      <div class="gcd-toggle">
        <n-switch v-model:value="form.enabled" size="small" />
        <span class="gcd-toggle__hint">禁用时 user 看不到此分组（但已挂载的偏好仍保留）</span>
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
.gcd-toggle { display: flex; align-items: center; gap: 10px; min-width: 0; }
.gcd-toggle__hint {
  color: #707070;
  font-size: 11px;
  line-height: 1.4;
  flex: 1;
  min-width: 0;
}
</style>
