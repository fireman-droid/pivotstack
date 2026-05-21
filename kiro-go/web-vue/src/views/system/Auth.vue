<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NSwitch, useMessage } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { getUserPolicy, setUserPolicy, type UserPolicy } from '../../api/admin/users'

const message = useMessage()

const policy = ref<UserPolicy>({ allowSelfRegister: true, requireActivationCode: false })
const policyLoading = ref(false)

async function loadPolicy() {
  policyLoading.value = true
  try {
    policy.value = await getUserPolicy()
  } catch (e: any) {
    message.error(e?.message || '加载策略失败')
  } finally {
    policyLoading.value = false
  }
}

async function updatePolicy(patch: Partial<UserPolicy>) {
  try {
    policy.value = await setUserPolicy(patch)
    message.success('策略已更新')
  } catch (e: any) {
    message.error(e?.message || '更新失败')
    loadPolicy()
  }
}

onMounted(loadPolicy)
</script>

<template>
  <div class="admin-page">
    <header class="page-head">
      <div>
        <div class="page-head__crumb"><b>SYSTEM</b> / 认证 / 登录策略</div>
        <div class="page-head__title">
          <div class="t-display-admin">登录策略</div>
          <div class="page-head__sub">登录方式开关</div>
        </div>
      </div>
      <div class="page-head__right">
        <button class="a-btn a-btn--ghost" :disabled="policyLoading" @click="loadPolicy">
          <RefreshCw :size="14" :class="{ 'is-spinning': policyLoading }" />
          刷新
        </button>
      </div>
    </header>

    <section class="a-section">
      <div class="a-section-title">登录策略</div>
      <div class="a-toggles">
        <div class="a-toggle">
          <div class="a-toggle-text">
            <div class="a-toggle-title">允许自助注册</div>
            <div class="a-toggle-desc">未开放注册时只能通过 admin 端手动建账号或 API Key 登录</div>
          </div>
          <NSwitch
            :value="policy.allowSelfRegister"
            :loading="policyLoading"
            @update:value="(v: boolean) => updatePolicy({ allowSelfRegister: v })"
          />
        </div>
        <div class="a-toggle">
          <div class="a-toggle-text">
            <div class="a-toggle-title">注册必须激活码</div>
            <div class="a-toggle-desc">关闭后任何邮箱可自由注册；开启后需要使用 admin 派发的激活码（兑换码）</div>
          </div>
          <NSwitch
            :value="policy.requireActivationCode"
            :loading="policyLoading"
            @update:value="(v: boolean) => updatePolicy({ requireActivationCode: v })"
          />
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.a-btn {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--st-border);
  border-radius: 4px;
  color: var(--st-text-pri);
  font-size: 12px; font-family: inherit; cursor: pointer;
}
.a-btn--ghost { background: transparent; }
.a-btn:hover:not(:disabled) { background: rgba(255, 255, 255, 0.08); }
.is-spinning { animation: a-spin 0.8s linear infinite; }
@keyframes a-spin { to { transform: rotate(360deg); } }

.a-section {
  background: var(--st-bg-surface);
  border: 1px solid var(--st-border);
  border-radius: 6px;
  padding: 20px 24px;
  margin-bottom: 16px;
}
.a-section-title {
  font-size: 14px; font-weight: 600; color: var(--st-text-pri);
  margin-bottom: 16px;
}

.a-toggles { display: flex; flex-direction: column; gap: 12px; }
.a-toggle {
  display: flex; align-items: center; justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--st-border);
}
.a-toggle:last-child { border-bottom: none; }
.a-toggle-text { flex: 1; min-width: 0; }
.a-toggle-title { font-size: 13px; font-weight: 500; color: var(--st-text-pri); margin-bottom: 2px; }
.a-toggle-desc { font-size: 11px; color: var(--st-text-ter); line-height: 1.4; }
</style>
