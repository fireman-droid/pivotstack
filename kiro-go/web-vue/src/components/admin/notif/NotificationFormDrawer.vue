<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NInput, NRadioGroup, NRadioButton, NSwitch, NButton, NDatePicker, NPopconfirm, useMessage,
} from 'naive-ui'
import { Megaphone, Plus, Pencil } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import {
  adminCreateNotification, adminUpdateNotification,
  type AdminNotification, type NotificationInput,
} from '../../../api/notifications'

const props = defineProps<{
  show: boolean
  editing: AdminNotification | null
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'saved', n: AdminNotification): void
}>()

const message = useMessage()

const form = ref<NotificationInput>({
  title: '',
  body: '',
  level: 'info',
  targetType: 'all',
  targetValue: [],
  status: 'draft',
  publishAt: 0,
  expireAt: 0,
  dismissible: true,
})
const targetValueText = ref('')
const publishLocal = ref<number | null>(null)
const expireLocal = ref<number | null>(null)
const submitting = ref(false)

watch(() => props.show, on => {
  if (!on) return
  if (props.editing) {
    const e = props.editing
    form.value = {
      title: e.title,
      body: e.body,
      level: e.level,
      targetType: e.targetType,
      targetValue: e.targetValue ? [...e.targetValue] : [],
      status: e.status,
      publishAt: e.publishAt || 0,
      expireAt: e.expireAt || 0,
      dismissible: e.dismissible,
    }
    targetValueText.value = (e.targetValue || []).join(', ')
    publishLocal.value = e.publishAt ? e.publishAt * 1000 : null
    expireLocal.value = e.expireAt ? e.expireAt * 1000 : null
  } else {
    form.value = {
      title: '', body: '', level: 'info', targetType: 'all', targetValue: [],
      status: 'draft', publishAt: 0, expireAt: 0, dismissible: true,
    }
    targetValueText.value = ''
    publishLocal.value = null
    expireLocal.value = null
  }
}, { immediate: true })

const mode = computed<'create' | 'edit'>(() => (props.editing ? 'edit' : 'create'))
const title = computed(() => mode.value === 'create' ? '新建通知' : '编辑通知')
const subtitle = computed(() => mode.value === 'create'
  ? '草稿可暂存；立即发布会向所有命中目标的 user 推送'
  : '修改后已读状态保留，user 端会重新展示')

function close() {
  if (submitting.value) return
  emit('update:show', false)
}

function parseTargetValue() {
  if (form.value.targetType === 'all') return []
  return targetValueText.value
    .split(/[,，\n]/)
    .map(s => s.trim())
    .filter(Boolean)
}

async function submit(status: 'draft' | 'published') {
  if (submitting.value) return
  if (!form.value.title.trim()) {
    message.warning('请填写标题')
    return
  }
  if (!form.value.body.trim()) {
    message.warning('请填写正文')
    return
  }
  submitting.value = true
  try {
    const payload: NotificationInput = {
      ...form.value,
      title: form.value.title.trim(),
      body: form.value.body,
      status,
      targetValue: parseTargetValue(),
      publishAt: publishLocal.value ? Math.floor(publishLocal.value / 1000) : 0,
      expireAt: expireLocal.value ? Math.floor(expireLocal.value / 1000) : 0,
    }
    const saved = props.editing
      ? await adminUpdateNotification(props.editing.id, payload)
      : await adminCreateNotification(payload)
    message.success(status === 'published' ? '已发布' : '已保存草稿')
    emit('saved', saved)
    emit('update:show', false)
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

const targetValuePlaceholder = computed(() => {
  switch (form.value.targetType) {
    case 'plan': return '逗号分隔：free, credit, hybrid'
    case 'group': return '逗号分隔的 group id'
    case 'userIds': return '逗号分隔的 API Key id'
    default: return ''
  }
})
</script>

<template>
  <RefinedDrawer
    :show="show"
    :title="title"
    :subtitle="subtitle"
    :icon="mode === 'create' ? Plus : Pencil"
    :loading="submitting"
    :width="600"
    @update:show="(v) => emit('update:show', v)"
  >
    <RefinedField label="标题" required hint="≤ 80 字，user 端会作为通知头部">
      <n-input v-model:value="form.title" maxlength="80" show-count placeholder="如：Claude 维护通告" />
    </RefinedField>

    <RefinedField label="正文" required hint="支持 *斜* **粗** `code`；≤ 2000 字">
      <n-input
        v-model:value="form.body"
        type="textarea"
        :autosize="{ minRows: 6, maxRows: 14 }"
        maxlength="2000"
        show-count
        placeholder="Markdown 简化语法"
      />
    </RefinedField>

    <RefinedField label="级别" hint="critical 强弹窗 + 不可关闭">
      <n-radio-group v-model:value="form.level">
        <n-radio-button value="info">INFO</n-radio-button>
        <n-radio-button value="warn">WARN</n-radio-button>
        <n-radio-button value="critical">CRITICAL</n-radio-button>
      </n-radio-group>
    </RefinedField>

    <RefinedField label="目标">
      <n-radio-group v-model:value="form.targetType">
        <n-radio-button value="all">全部 user</n-radio-button>
        <n-radio-button value="plan">按 plan</n-radio-button>
        <n-radio-button value="group">按 group</n-radio-button>
        <n-radio-button value="userIds">指定 user</n-radio-button>
      </n-radio-group>
    </RefinedField>

    <RefinedField v-if="form.targetType !== 'all'" label="目标值">
      <n-input
        v-model:value="targetValueText"
        type="textarea"
        :autosize="{ minRows: 2, maxRows: 5 }"
        :placeholder="targetValuePlaceholder"
      />
    </RefinedField>

    <RefinedField label="发布时间" hint="留空 + 立即发布 = 即时；带时间 = 定时发布">
      <n-date-picker v-model:value="publishLocal" type="datetime" clearable style="width: 100%" />
    </RefinedField>

    <RefinedField label="过期时间" hint="留空 = 永久；过期后 user 端隐藏">
      <n-date-picker v-model:value="expireLocal" type="datetime" clearable style="width: 100%" />
    </RefinedField>

    <RefinedField label="允许用户关闭" hint="critical 通常关闭此项">
      <n-switch v-model:value="form.dismissible" />
    </RefinedField>

    <template #footer>
      <n-button :disabled="submitting" quaternary @click="close">取消</n-button>
      <n-button :loading="submitting" @click="submit('draft')">保存草稿</n-button>
      <n-popconfirm
        positive-text="立即发布"
        negative-text="取消"
        @positive-click="submit('published')"
      >
        <template #trigger>
          <n-button type="primary" :loading="submitting">立即发布</n-button>
        </template>
        确认推送给所有命中目标的 user？发布后可在列表里再编辑。
      </n-popconfirm>
    </template>
  </RefinedDrawer>
</template>
