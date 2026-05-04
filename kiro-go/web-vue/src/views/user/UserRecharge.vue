<script setup>
import { ref } from 'vue'
import { useUserAuth } from '../../stores/userAuth'
import { userApi } from '../../api/user'
import { Gift, Sparkles, ShoppingBag, UserCircle, AlertCircle } from 'lucide-vue-next'
import WorldCard from '../../components/world/WorldCard.vue'
import WorldInput from '../../components/world/WorldInput.vue'
import WorldButton from '../../components/world/WorldButton.vue'

const auth = useUserAuth()
const code = ref('')
const loading = ref(false)
const result = ref(null)
const error = ref('')

async function handleRedeem() {
  if (!code.value.trim()) return
  loading.value = true
  error.value = ''
  result.value = null
  try {
    const data = await userApi('/redeem', { method: 'POST', body: { code: code.value.trim() } })
    result.value = data
    code.value = ''
    auth.refresh()
  } catch (e) {
    error.value = e.message
  }
  loading.value = false
}
</script>

<template>
  <div class="recharge-page">
    <div class="page-head">
      <div class="eyebrow">充值中心</div>
      <h1 class="page-title">激活码兑换</h1>
    </div>

    <WorldCard padding="lg" :elevated="true" variant="talisman" class="recharge-card">
      <div class="card-icon-wrap">
        <div class="card-icon">
          <Gift :size="32" stroke-width="2" />
        </div>
      </div>

      <div class="card-body">
        <h2 class="card-title">输入您的激活码</h2>
        <p class="card-helper">兑换成功后余额或使用时间将自动注入您的账户</p>

        <form @submit.prevent="handleRedeem" class="redeem-form">
          <WorldInput
            v-model="code"
            placeholder="XXXX-XXXX-XXXX-XXXX"
            :monospace="true"
            align="center"
            size="lg"
          />
          <WorldButton
            type="submit"
            variant="primary"
            size="md"
            :loading="loading"
            :disabled="code.trim().length < 4"
            :block="true"
          >
            <Sparkles v-if="!loading" :size="16" />
            <span>{{ loading ? '兑换中' : '立即兑换' }}</span>
          </WorldButton>
        </form>

        <Transition name="slide-fade">
          <div v-if="result" class="feedback-msg success">
            <div class="fb-icon"><Sparkles :size="18" /></div>
            <div class="fb-text">
              <div class="fb-title">兑换成功</div>
              <div v-if="result.type === 'balance'" class="fb-detail">
                账户余额 +${{ (result.amount || 0).toFixed(2) }}
              </div>
              <div v-else-if="result.type === 'time'" class="fb-detail">
                账户有效期 +{{ Math.round((result.amount || 0) / 86400) }} 天
              </div>
              <div v-else class="fb-detail">激活码已应用</div>
            </div>
          </div>
        </Transition>

        <Transition name="slide-fade">
          <div v-if="error" class="feedback-msg error">
            <div class="fb-icon"><AlertCircle :size="18" /></div>
            <div class="fb-text">
              <div class="fb-title">兑换失败</div>
              <div class="fb-detail">{{ error }}</div>
            </div>
          </div>
        </Transition>
      </div>
    </WorldCard>

    <!-- 信息提示 -->
    <div class="hints">
      <WorldCard padding="md" class="hint-card">
        <div class="hint-row">
          <div class="hint-icon"><ShoppingBag :size="14" /></div>
          <div class="hint-text">
            <div class="hint-title">在哪里购买？</div>
            <div class="hint-sub">联系您的服务商或社群管理员获取激活码</div>
          </div>
        </div>
      </WorldCard>
      <WorldCard padding="md" class="hint-card">
        <div class="hint-row">
          <div class="hint-icon"><UserCircle :size="14" /></div>
          <div class="hint-text">
            <div class="hint-title">余额查询</div>
            <div class="hint-sub">兑换后请前往「概览」页查看账户状态</div>
          </div>
        </div>
      </WorldCard>
    </div>
  </div>
</template>

<style scoped>
.recharge-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
  max-width: 640px;
  margin: 0 auto;
}

.page-head { margin-bottom: 4px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.75rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 4px 0 0;
  color: var(--world-text-primary);
}
[data-world="daogui"] .page-title {
  background: linear-gradient(135deg, #f3c66e 0%, var(--world-accent) 90%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.recharge-card {
  text-align: center;
  position: relative;
  overflow: hidden;
}
.recharge-card::before {
  content: '';
  position: absolute;
  top: -100px;
  left: 50%;
  transform: translateX(-50%);
  width: 320px;
  height: 320px;
  background: radial-gradient(circle, rgba(2, 132, 199, 0.10), transparent 70%);
  pointer-events: none;
  border-radius: 50%;
}
[data-world="daogui"] .recharge-card::before {
  background: radial-gradient(circle, rgba(196, 30, 58, 0.18), transparent 70%);
}

.card-icon-wrap {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
}
.card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: var(--world-radius-2xl);
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
  color: white;
  box-shadow: 0 8px 28px -8px rgba(2, 132, 199, 0.5);
}
[data-world="daogui"] .card-icon {
  box-shadow: 0 0 32px rgba(196, 30, 58, 0.4);
}

.card-body { position: relative; z-index: 1; }
.card-title {
  font-size: 1.125rem;
  font-weight: 800;
  margin: 0 0 4px;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
.card-helper {
  font-size: 0.8125rem;
  color: var(--world-text-mute);
  margin: 0 0 24px;
}

.redeem-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* Feedback */
.feedback-msg {
  margin-top: 18px;
  padding: 12px 14px;
  border-radius: var(--world-radius-lg);
  display: flex;
  align-items: flex-start;
  gap: 10px;
  text-align: left;
}
.feedback-msg.success {
  background: rgba(16, 185, 129, 0.10);
  border: 1px solid rgba(16, 185, 129, 0.25);
  color: var(--world-success);
}
.feedback-msg.error {
  background: rgba(239, 68, 68, 0.10);
  border: 1px solid rgba(239, 68, 68, 0.25);
  color: var(--world-error);
}
[data-world="daogui"] .feedback-msg.success {
  background: rgba(82, 121, 111, 0.12);
  border-color: rgba(82, 121, 111, 0.32);
  color: #95b5a8;
}
[data-world="daogui"] .feedback-msg.error {
  background: rgba(196, 30, 58, 0.12);
  border-color: rgba(196, 30, 58, 0.32);
  color: #f5707f;
  box-shadow: 0 0 18px rgba(196, 30, 58, 0.15);
}
.fb-icon { flex-shrink: 0; padding-top: 1px; }
.fb-title {
  font-size: 0.875rem;
  font-weight: 800;
  margin-bottom: 2px;
}
.fb-detail {
  font-size: 0.8125rem;
  color: var(--world-text-mute);
}

.slide-fade-enter-active { transition: all 280ms cubic-bezier(0.34, 1.56, 0.64, 1); }
.slide-fade-leave-active { transition: all 200ms ease-in; }
.slide-fade-enter-from   { opacity: 0; transform: translateY(-8px); }
.slide-fade-leave-to     { opacity: 0; transform: translateY(-4px); }

/* Hints */
.hints {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
@media (max-width: 600px) {
  .hints { grid-template-columns: 1fr; }
}
.hint-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}
.hint-icon {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--world-radius-md);
  background: rgba(2, 132, 199, 0.10);
  color: var(--world-accent);
  flex-shrink: 0;
}
[data-world="daogui"] .hint-icon {
  background: rgba(196, 30, 58, 0.12);
  color: var(--world-accent);
}
.hint-title {
  font-size: 0.8125rem;
  font-weight: 800;
  color: var(--world-text-primary);
  margin-bottom: 2px;
}
.hint-sub {
  font-size: 0.75rem;
  color: var(--world-text-mute);
  line-height: 1.4;
}
</style>
