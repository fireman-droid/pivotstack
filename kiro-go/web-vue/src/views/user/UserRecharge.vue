<script setup>
import { ref } from 'vue'
import { useUserAuth } from '../../stores/userAuth'
import { userApi } from '../../api/user'
import { Gift, Loader2, Sparkles, ShoppingBag, UserCircle } from 'lucide-vue-next'

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
    <div class="recharge-card glass">
      <div class="card-header">
        <div class="icon-wrapper">
          <Gift :size="28" color="#818cf8" />
        </div>
        <h3>激活码兑换</h3>
        <p class="helper-text">请在下方输入您的充值激活码</p>
      </div>

      <form @submit.prevent="handleRedeem" class="redeem-form">
        <div class="input-group">
          <input
            v-model="code"
            placeholder="XXXX-XXXX-XXXX-XXXX"
            class="code-input"
            maxlength="19"
            spellcheck="false"
            autocomplete="off"
          />
        </div>

        <button type="submit" :disabled="loading || code.trim().length < 10" class="submit-btn">
          <Loader2 v-if="loading" :size="18" class="animate-spin" style="margin-right:8px" />
          <span>{{ loading ? '处理中...' : '立即兑换' }}</span>
        </button>
      </form>

      <div v-if="result" class="feedback-msg success">
        <div class="feedback-content">
          <Sparkles :size="18" style="margin-right:12px;flex-shrink:0" />
          <div v-if="result.type === 'balance'">
            <strong>兑换成功！</strong>
            <p>账户余额已增加 ¥{{ (result.amount || 0).toFixed(2) }}</p>
          </div>
          <div v-else-if="result.type === 'time'">
            <strong>兑换成功！</strong>
            <p>账户有效期已延长 {{ Math.round((result.amount || 0) / 86400) }} 天</p>
          </div>
          <div v-else>
            <strong>兑换成功！</strong>
          </div>
        </div>
      </div>

      <div v-if="error" class="feedback-msg error">
        <div class="feedback-content">
          <p>{{ error }}</p>
        </div>
      </div>
    </div>

    <!-- Purchase Guide -->
    <div class="guide-container">
      <div class="guide-card glass">
        <ShoppingBag :size="20" class="guide-icon" />
        <div class="guide-info">
          <h4>闲鱼购买</h4>
          <p>前往闲鱼搜索「KiroStack激活码」获取充值卡</p>
        </div>
      </div>
      <div class="guide-card glass">
        <UserCircle :size="20" class="guide-icon" />
        <div class="guide-info">
          <h4>联系管理员</h4>
          <p>通过管理员直接获取专属激活码</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.recharge-page {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2rem;
  padding: 2rem 1rem;
  min-height: 100%;
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.glass {
  background: rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
}

.recharge-card {
  width: 100%;
  max-width: 480px;
  border-radius: 12px;
  padding: 2.5rem;
}

.card-header {
  text-align: center;
  margin-bottom: 2.5rem;
}

.icon-wrapper {
  width: 64px;
  height: 64px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.1), rgba(168, 85, 247, 0.1));
  border: 1px solid rgba(99, 102, 241, 0.2);
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 1rem;
  box-shadow: 0 0 20px rgba(99, 102, 241, 0.15);
}

h3 {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.5rem;
  font-weight: 700;
  color: #f8fafc;
  margin: 0 0 0.5rem;
}

.helper-text {
  color: #94a3b8;
  font-size: 0.875rem;
  margin: 0;
}

.redeem-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.code-input {
  width: 100%;
  height: 52px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  padding: 0 1rem;
  color: #f8fafc;
  font-family: ui-monospace, monospace;
  font-size: 1.25rem;
  text-align: center;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  transition: all 0.2s;
  box-sizing: border-box;
}

.code-input::placeholder {
  color: #475569;
  letter-spacing: 0.05em;
}

.code-input:focus {
  outline: none;
  border-color: #6366f1;
  background: rgba(99, 102, 241, 0.05);
  box-shadow: 0 0 0 4px rgba(99, 102, 241, 0.1);
}

.submit-btn {
  height: 48px;
  background: linear-gradient(to right, #6366f1, #4f46e5);
  border: none;
  border-radius: 8px;
  color: #fff;
  font-weight: 600;
  font-size: 1rem;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.2);
}

.submit-btn:hover:not(:disabled) {
  background: linear-gradient(to right, #4f46e5, #4338ca);
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(99, 102, 241, 0.3);
}

.submit-btn:active:not(:disabled) {
  transform: translateY(0);
}

.submit-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.feedback-msg {
  margin-top: 1rem;
  padding: 1rem;
  border-radius: 10px;
  animation: slideIn 0.3s cubic-bezier(0.18, 0.89, 0.32, 1.28);
}

.feedback-content {
  display: flex;
  align-items: center;
}

.feedback-content strong {
  display: block;
  font-size: 0.9375rem;
  margin-bottom: 0.125rem;
}

.feedback-content p {
  margin: 0;
  font-size: 0.8125rem;
  opacity: 0.9;
}

.feedback-msg.success {
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.2);
  color: #10b981;
}

.feedback-msg.error {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.2);
  color: #ef4444;
}

@keyframes slideIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.guide-container {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1.5rem;
  width: 100%;
  max-width: 480px;
}

.guide-card {
  border-radius: 14px;
  padding: 1.25rem;
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
}

.guide-icon {
  color: #6366f1;
  flex-shrink: 0;
  margin-top: 2px;
}

.guide-info h4 {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #f8fafc;
  margin: 0 0 0.375rem;
}

.guide-info p {
  font-size: 0.8125rem;
  color: #94a3b8;
  margin: 0;
  line-height: 1.4;
}

.animate-spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 480px) {
  .guide-container { grid-template-columns: 1fr; }
  .recharge-card { padding: 1.5rem; }
}
</style>
