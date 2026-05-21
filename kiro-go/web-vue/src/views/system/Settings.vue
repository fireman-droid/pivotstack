<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NButton, NInput, NInputNumber, NSpace, NSwitch, NTabPane, NTabs, useMessage } from 'naive-ui'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import { getSettings, updateSettings, type AdminSettings } from '../../api/admin/settings'

const message = useMessage()
const loading = ref(false)
const saving = ref(false)
const tab = ref('general')
const form = ref<AdminSettings>({})

async function reload() {
  loading.value = true
  try {
    form.value = await getSettings()
  } catch (e: any) {
    message.error(e?.message || '加载设置失败')
  } finally {
    loading.value = false
  }
}
async function save() {
  saving.value = true
  try {
    form.value = await updateSettings(form.value)
    message.success('已保存')
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader kicker="系统" :kicker-dot="'#707070'" title="系统设置">
      <template #tabs>
        <n-tabs v-model:value="tab" size="small">
          <n-tab-pane name="general" tab="常规" />
          <n-tab-pane name="concurrency" tab="并发" />
          <n-tab-pane name="abuse" tab="滥用监控" />
        </n-tabs>
      </template>
    </PageHeader>

    <section class="panel">
      <template v-if="tab === 'general'">
        <div class="field"><label>监听 Host</label><n-input v-model:value="form.host" /><p>默认 0.0.0.0 · 保存后重启生效</p></div>
        <div class="field"><label>监听 Port (配置值)</label><n-input-number v-model:value="form.port" :min="1" /><p>服务 HTTP 端口 · 保存后重启生效</p></div>
        <div class="field"><label>实际监听 Port</label><n-input :value="form.runtimePort || '-'" readonly /><p>当前进程真实监听端口（env / cmdline 可能覆盖配置）</p></div>
        <div class="inline"><n-switch v-model:value="form.requireApiKey" /><span>要求 API Key</span></div>
      </template>

      <template v-else-if="tab === 'concurrency'">
        <div class="field"><label>每 Key 最大并发</label><n-input-number v-model:value="form.maxConcurrentPerKey" :min="1" /></div>
        <div class="field"><label>FREE 账号池并发</label><n-input-number v-model:value="form.maxInFlightPerAccountFree" :min="1" /></div>
        <div class="field"><label>PRO 账号池并发</label><n-input-number v-model:value="form.maxInFlightPerAccountPro" :min="1" /></div>
      </template>

      <template v-else-if="tab === 'abuse'">
        <div class="inline"><n-switch v-model:value="form.abuseEnabled" /><span>启用滥用监控</span></div>
        <div class="field"><label>天卡默认 RPM</label><n-input-number v-model:value="form.timedKeyRPM" :min="1" /></div>
      </template>
    </section>

    <div class="sticky-footer">
      <n-space justify="end">
        <n-button :loading="loading" @click="reload">取消</n-button>
        <n-button type="primary" :loading="saving" @click="save">保存</n-button>
      </n-space>
    </div>
  </PageContainer>
</template>

<style scoped>
.panel { border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; padding: 16px; }
.field { padding: 16px 0; border-bottom: 1px solid rgba(255,255,255,0.06); display: flex; flex-direction: column; gap: 8px; }
.field:first-child { padding-top: 0; }
.field:last-child { border-bottom: 0; }
label { color: #ededed; font-size: 14px; font-weight: 600; }
p { color: #707070; margin: 0; font-size: 12px; }
.inline { display: flex; align-items: center; gap: 8px; color: #a1a1a1; padding: 16px 0; border-bottom: 1px solid rgba(255,255,255,0.06); }
.sticky-footer { position: sticky; bottom: 0; padding: 16px 0; background: #000000; margin-top: 16px; }
</style>
