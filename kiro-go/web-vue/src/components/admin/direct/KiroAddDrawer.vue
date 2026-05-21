<script setup lang="ts">
import { ref, watch } from 'vue'
import { NTabs, NTabPane, NInput, NInputNumber, NSelect, NButton, useMessage } from 'naive-ui'
import { UserPlus } from 'lucide-vue-next'
import RefinedDrawer from '../../common/RefinedDrawer.vue'
import RefinedField from '../../common/RefinedField.vue'
import KiroOAuthPanel from './KiroOAuthPanel.vue'
import { useKiroDeviceCode } from './useKiroDeviceCode'
import { addAccount, type AddAccountRequest } from '../../../api/admin/accounts'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'added'): void
}>()

const message = useMessage()
const submitting = ref(false)
const tab = ref<'oauth' | 'single' | 'batch'>('oauth')

const { state: oauthState, startOAuth, cancelOAuth, copyUserCode, setRegion, setSuccessTimer } = useKiroDeviceCode({
  onSuccess: () => {
    emit('added')
    setSuccessTimer(() => emit('update:show', false), 1200)
  },
})

interface Single {
  email: string
  refreshToken: string
  authMethod: 'idc' | 'social'
  region: string
  nickname: string
  weight: number
}
const single = ref<Single>({
  email: '', refreshToken: '', authMethod: 'social',
  region: 'us-east-1', nickname: '', weight: 100,
})
const batchText = ref('')

const authOptions = [
  { label: 'Social (Google/GitHub)', value: 'social' },
  { label: 'IDC (Builder/Enterprise)', value: 'idc' },
]
const regionOptions = [
  { label: 'us-east-1', value: 'us-east-1' },
  { label: 'us-west-2', value: 'us-west-2' },
  { label: 'eu-central-1', value: 'eu-central-1' },
  { label: 'ap-southeast-1', value: 'ap-southeast-1' },
]

watch(() => props.show, v => {
  if (!v) { cancelOAuth(); return }
  tab.value = 'oauth'
  single.value = {
    email: '', refreshToken: '', authMethod: 'social',
    region: 'us-east-1', nickname: '', weight: 100,
  }
  batchText.value = ''
  cancelOAuth()
})

function close() {
  if (submitting.value) return
  emit('update:show', false)
}

async function submitSingle() {
  if (!single.value.refreshToken.trim()) {
    message.warning('请填写 refresh token')
    return
  }
  submitting.value = true
  try {
    await addAccount({
      email: single.value.email.trim() || undefined,
      refreshToken: single.value.refreshToken.trim(),
      authMethod: single.value.authMethod,
      region: single.value.region,
      nickname: single.value.nickname.trim() || undefined,
      weight: single.value.weight,
    })
    message.success('已添加，正在异步拉取账号信息…')
    emit('added')
    emit('update:show', false)
  } catch (e: any) {
    message.error(e?.message || '添加失败')
  } finally {
    submitting.value = false
  }
}

async function submitBatch() {
  const lines = batchText.value.split('\n').map(s => s.trim()).filter(Boolean)
  if (!lines.length) { message.warning('请粘贴至少一条'); return }
  let items: AddAccountRequest[] = []
  const raw = batchText.value.trim()
  if (raw.startsWith('[')) {
    try {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) {
        items = parsed.map((p: any) => ({
          email: p.email,
          refreshToken: p.refreshToken || p.refresh_token,
          accessToken: p.accessToken || p.access_token,
          authMethod: p.authMethod || p.auth_method || 'social',
          region: p.region || 'us-east-1',
          nickname: p.nickname,
        }))
      }
    } catch {
      message.error('JSON 解析失败，请检查格式')
      return
    }
  } else {
    items = lines.map(token => ({ refreshToken: token, authMethod: 'social', region: 'us-east-1' }))
  }
  if (!items.length) { message.warning('未能解析出可用条目'); return }
  submitting.value = true
  try {
    const results = await Promise.allSettled(items.map(it => addAccount(it)))
    const ok = results.filter(r => r.status === 'fulfilled').length
    const fail = results.length - ok
    if (fail === 0) message.success(`批量添加 ${ok} 条成功`)
    else if (ok === 0) message.error(`全部 ${fail} 条失败`)
    else message.warning(`${ok} 成功 / ${fail} 失败`)
    emit('added')
    emit('update:show', false)
  } catch (e: any) {
    message.error(e?.message || '批量失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <RefinedDrawer
    :show="show"
    title="添加 Kiro 账号"
    subtitle="推荐 OAuth 流程一键完成；高级用户可手粘 refresh token 或批量导入"
    :icon="UserPlus"
    :loading="submitting"
    :width="600"
    @update:show="(v) => emit('update:show', v)"
  >
    <n-tabs v-model:value="tab" size="small" type="line" class="kad-tabs">
      <n-tab-pane name="oauth" tab="OAuth 登录（推荐）">
        <KiroOAuthPanel
          :state="oauthState"
          @start="startOAuth"
          @cancel="cancelOAuth"
          @copy-code="copyUserCode"
          @update:region="setRegion"
        />
      </n-tab-pane>

      <n-tab-pane name="single" tab="单条粘贴">
        <RefinedField label="Refresh Token" required hint="粘贴 kiro 账号的 refresh token">
          <n-input
            v-model:value="single.refreshToken"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 6 }"
            placeholder="粘贴 refresh token"
            style="font-family: var(--st-font-mono, 'Geist Mono', monospace); font-size: 12px"
          />
        </RefinedField>
        <RefinedField label="邮箱" hint="选填，留空会自动从上游拉取">
          <n-input v-model:value="single.email" placeholder="user@example.com" />
        </RefinedField>
        <RefinedField label="认证方式">
          <n-select v-model:value="single.authMethod" :options="authOptions" />
        </RefinedField>
        <RefinedField label="区域">
          <n-select v-model:value="single.region" :options="regionOptions" />
        </RefinedField>
        <RefinedField label="备注" hint="便于识别的标签">
          <n-input v-model:value="single.nickname" placeholder="例如：客户 A / 备用号" />
        </RefinedField>
        <RefinedField label="权重" hint="账号池调度权重，默认 100">
          <n-input-number v-model:value="single.weight" :min="0" :step="10" style="width: 100%" />
        </RefinedField>
      </n-tab-pane>

      <n-tab-pane name="batch" tab="批量导入">
        <div class="kad-hint">
          <strong>两种格式：</strong><br />
          • <code>每行一个 refresh token</code>（自动用默认 social / us-east-1）<br />
          • <code>JSON 数组</code>，每项可含 email / refreshToken / authMethod / region / nickname
        </div>
        <n-input
          v-model:value="batchText"
          type="textarea"
          :autosize="{ minRows: 10, maxRows: 18 }"
          placeholder="粘贴 refresh token 列表或 JSON 数组"
          style="font-family: var(--st-font-mono, 'Geist Mono', monospace); font-size: 12px; margin-top: 8px"
        />
      </n-tab-pane>
    </n-tabs>

    <template #footer>
      <n-button :disabled="submitting" quaternary @click="close">关闭</n-button>
      <n-button v-if="tab === 'single'" type="primary" :loading="submitting" @click="submitSingle">添加</n-button>
      <n-button v-else-if="tab === 'batch'" type="primary" :loading="submitting" @click="submitBatch">批量添加</n-button>
      <!-- OAuth tab 由 KiroOAuthPanel 内部按钮驱动 -->
    </template>
  </RefinedDrawer>
</template>

<style scoped>
.kad-tabs { margin-top: -4px; }
.kad-hint {
  padding: 10px 12px;
  background: rgba(82, 168, 255, 0.05);
  border: 1px solid rgba(82, 168, 255, 0.20);
  border-radius: 4px;
  color: #a1a1a1;
  font-size: 12px;
  line-height: 1.6;
}
.kad-hint code {
  padding: 1px 5px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 3px;
  font-size: 11px;
  color: #ededed;
}
</style>
