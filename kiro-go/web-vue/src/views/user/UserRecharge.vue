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
    auth.refresh() // refresh balance
  } catch (e) {
    error.value = e.message
  }
  loading.value = false
}
</script>

<template>
  <div class="recharge-page">
    <div class="recharge-card">
      <div class="card-header">
        <div class="icon-wrapper">
          <Gift class="w-6 h-6 text-indigo-400" />
        </div>
        <h3>激活码兑换</h3>
        <p class="helper-text">输入激活码兑换余额或时间</p>
      </div>

      <form @submit.prevent="handleRedeem" class="redeem-form">
        <div class="input-group">
          <input
            v-model="code"
            placeholder="KIRO-XXXX-XXXX-XXXX"
            class="code-input"
            maxlength="19"
            spellcheck="false"
          />
        </div>
        
        <button type="submit" :disabled="loading || !code.trim()" class="submit-btn">
          <Loader2 v-if="loading" class="w-4 h-4 animate-spin mr-2" />
          <span>{{ loading ? '处理中...' : '立即兑换' }}</span>
        </button>
      </form>

      <div v-if="result" class="feedback-msg success">
        <Sparkles class="w-4 h-4 mr-2" />
        <span v-if="result.type === 'balance'">
          兑换成功！余额已增加 ¥{{ (result.amount || 0).toFixed(2) }}
        </span>
        <span v-else-if="result.type === 'days'">
          兑换成功！有效期已延长 {{ result.amount }} 天
        </span>
      </div>

      <div v-if="error" class="feedback-msg error">
        <span>{{ error }}</span>
      </div>
    </div>

    <!-- Purchase Guide Section -->
    <div class="guide-container">
      <div class="guide-card">
        <ShoppingBag class="guide-icon" />
        <div class="guide-info">
          <h4>闲鱼购买</h4>
          <p>搜索「KiroStack激活卡」获取充值卡</p>
        </div>
      </div>
      <div class="guide-card">
        <UserCircle class="guide-icon" />
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
  gap: 1.5rem;
  padding: 2rem 1rem;
  min-height: 100%;
}

.recharge-card {
  width: 100%;
  max-width: 480px;
  background: rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  padding: 2rem;
  box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.3);
}

.card-header {
  text-align: center;
  margin-bottom: 2rem;
}

.icon-wrapper {
  width: 56px;
  height: 56px;
  background: rgba(99, 102, 241, 0.1);
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 1rem;
}

h3 {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.5rem;
  font-weight: 700;
  color: #fff;
  margin: 0 0 0.5rem;
}

.helper-text {
  font-family: 'DM Sans', sans-serif;
  color: #6b7280;
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
  color: #fff;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 1.125rem;
  text-align: center;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  transition: all 150ms ease;
}

.code-input:focus {
  outline: none;
  border-color: #6366f1;
  background: rgba(99, 102, 241, 0.05);
  box-shadow: 0 0 0 4px rgba(99, 102, 241, 0.1);
}

.submit-btn {
  height: 48px;
  background: #6366f1;
  border: none;
  border-radius: 8px;
  color: #fff;
  font-weight: 600;
  font-size: 1rem;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 150ms ease;
}

.submit-btn:hover:not(:disabled) {
  background: #4f46e5;
  transform: translateY(-1px);
}

.submit-btn:active:not(:disabled) {
  transform: translateY(0);
}

.submit-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.feedback-msg {
  margin-top: 1rem;
  padding: 0.875rem;
  border-radius: 8px;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  justify-content: center;
  animation: slideDown 200ms ease;
}

.feedback-msg.success {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.feedback-msg.error {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.2);
}

.guide-container {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
  width: 100%;
  max-width: 480px;
}

.guide-card {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  padding: 1rem;
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
}

.guide-icon {
  width: 20px;
  height: 20px;
  color: #6b7280;
  flex-shrink: 0;
  margin-top: 2px;
}

.guide-info h4 {
  font-size: 0.875rem;
  font-weight: 600;
  color: #fff;
  margin: 0 0 0.25rem;
}

.guide-info p {
  font-size: 0.75rem;
  color: #6b7280;
  margin: 0;
  line-height: 1.4;
}

@keyframes slideDown {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.animate-spin {
  animation: spin 1s linear infinite;
}

.mr-2 { margin-right: 0.5rem; }
.w-4 { width: 1rem; }
.h-4 { height: 1rem; }
.w-6 { width: 1.5rem; }
.h-6 { height: 1.5rem; }

@media (max-width: 480px) {
  .guide-container {
    grid-template-columns: 1fr;
  }
}
</style>
