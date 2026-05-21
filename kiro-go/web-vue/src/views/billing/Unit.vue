<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { NButton, NInput, NInputNumber, NSpace, NSwitch, useMessage } from 'naive-ui'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import { getSystemUnitConfig, updateSystemUnitConfig, type UnitHistory } from '../../api/admin/unit'

const message = useMessage()
const loading = ref(false)
const saving = ref(false)
const yuanPerUSD = ref(7.2)
const pivotDollarsPerYuan = ref(20)
const adminPassword = ref('')
const rebalance = ref(false)
const history = ref<UnitHistory[]>([])

const preview = computed(() => {
  const virtual = yuanPerUSD.value * pivotDollarsPerYuan.value
  return `1 USD = ¥${yuanPerUSD.value.toFixed(2)} = ${virtual.toFixed(2)} 虚拟$`
})

async function reload() {
  loading.value = true
  try {
    const data = await getSystemUnitConfig()
    yuanPerUSD.value = data.yuanPerUSD ?? yuanPerUSD.value
    pivotDollarsPerYuan.value = data.pivotStackDollarsPerYuan ?? pivotDollarsPerYuan.value
    history.value = data.history || []
  } catch (e: any) {
    message.error(e?.message || '加载单位配置失败')
  } finally {
    loading.value = false
  }
}
async function save() {
  saving.value = true
  try {
    await updateSystemUnitConfig({
      newValue: pivotDollarsPerYuan.value,
      adminPassword: adminPassword.value,
      rebalanceUserBalances: rebalance.value,
    })
    message.success('已保存')
    adminPassword.value = ''
    reload()
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
function fmtTime(ts?: number) {
  return ts ? new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'
}

onMounted(reload)
</script>

<template>
  <PageContainer>
    <PageHeader kicker="销售 & 计费" :kicker-dot="'#707070'" title="单位换算" desc="虚拟 $ ↔ ¥ ↔ 上游 unit" />

    <section class="panel">
      <div class="field">
        <label>1 USD = N ¥</label>
        <n-input-number v-model:value="yuanPerUSD" :min="0" :step="0.01" />
        <p>用于运营侧展示，实际用户余额以虚拟 $ 记账。</p>
      </div>
      <div class="field">
        <label>1 ¥ = N 虚拟$</label>
        <n-input-number v-model:value="pivotDollarsPerYuan" :min="0" :step="1" />
        <p>改动会影响后续充值和显示口径。</p>
      </div>
      <div class="preview">{{ preview }}</div>
      <div class="field">
        <label>二次确认密码</label>
        <n-input v-model:value="adminPassword" type="password" placeholder="admin password" />
      </div>
      <div class="inline-field">
        <n-switch v-model:value="rebalance" />
        <span>同步重算现有用户余额</span>
      </div>
    </section>

    <section class="panel">
      <h3>历史变更</h3>
      <ul class="timeline">
        <li v-for="(item, i) in history" :key="i">
          <span class="time">{{ fmtTime(item.time) }}</span>
          <span>{{ item.oldValue ?? '-' }} → {{ item.newValue ?? '-' }}</span>
          <span class="actor">{{ item.actor || 'system' }}</span>
        </li>
        <li v-if="!history.length" class="empty">暂无历史</li>
      </ul>
    </section>

    <div class="sticky-footer">
      <n-space justify="end">
        <n-button @click="reload" :loading="loading">取消</n-button>
        <n-button type="primary" @click="save" :loading="saving">保存</n-button>
      </n-space>
    </div>
  </PageContainer>
</template>

<style scoped>
.panel { border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; padding: 16px; margin-bottom: 24px; }
.field { padding: 16px 0; border-bottom: 1px solid rgba(255,255,255,0.06); display: flex; flex-direction: column; gap: 8px; }
.field:first-child { padding-top: 0; }
.field:last-child { border-bottom: 0; }
label, h3 { color: #ededed; font-size: 14px; font-weight: 600; margin: 0; }
p, .actor, .empty { color: #707070; margin: 0; font-size: 12px; }
.preview { margin: 16px 0; padding: 14px; border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; color: #ededed; font-family: "Geist Mono", ui-monospace, monospace; }
.inline-field { display: flex; align-items: center; gap: 8px; color: #a1a1a1; padding-top: 16px; }
.timeline { list-style: none; margin: 12px 0 0; padding: 0; display: flex; flex-direction: column; gap: 10px; }
.timeline li { display: flex; gap: 16px; color: #a1a1a1; }
.time { font-family: "Geist Mono", ui-monospace, monospace; color: #707070; }
.sticky-footer { position: sticky; bottom: 0; padding: 16px 0; background: #000000; }
</style>
