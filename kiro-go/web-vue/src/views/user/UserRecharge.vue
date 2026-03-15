<script setup>
import { ref } from 'vue'
import { useUserAuth } from '../../stores/userAuth'
import { userApi } from '../../api/user'

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
    <!-- Redeem Section -->
    <div class="section">
      <h3>🎫 激活码兑换</h3>
      <p class="hint">输入激活码为您的账户充值余额或延长有效期</p>

      <form @submit.prevent="handleRedeem" class="redeem-form">
        <input
          v-model="code"
          placeholder="KIRO-XXXX-XXXX-XXXX"
          class="code-input"
          maxlength="19"
        />
        <button type="submit" :disabled="loading || !code.trim()" class="redeem-btn">
          {{ loading ? '兑换中...' : '立即兑换' }}
        </button>
      </form>

      <div v-if="result" class="result success">
        <div class="receipt-title">✅ 兑换成功！</div>
        <div class="receipt-detail" v-if="result.type === 'balance'">
          余额：¥{{ (result.balanceBefore || 0).toFixed(2) }} → ¥{{ (result.balanceAfter || 0).toFixed(2) }}
          <span class="receipt-add">(+¥{{ (result.amount || 0).toFixed(2) }})</span>
        </div>
        <div class="receipt-detail" v-if="result.type === 'days'">
          有效期延长 <span class="receipt-add">+{{ result.amount }}天</span>
        </div>
      </div>

      <div v-if="error" class="result error">
        ❌ {{ error }}
      </div>
    </div>

    <!-- Purchase Guide -->
    <div class="section guide">
      <h3>💡 如何获取激活码</h3>
      <div class="guide-items">
        <div class="guide-item">
          <span class="guide-icon">🛒</span>
          <div>
            <strong>闲鱼购买</strong>
            <p>搜索「KiroStack激活卡」购买充值卡</p>
          </div>
        </div>
        <div class="guide-item">
          <span class="guide-icon">👤</span>
          <div>
            <strong>联系管理员</strong>
            <p>通过管理员直接获取激活码</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.recharge-page {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  max-width: 600px;
}

.section {
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 14px;
  padding: 1.5rem;
}

.section h3 {
  margin: 0 0 0.5rem 0;
  font-size: 1.1rem;
  color: #fff;
}

.hint {
  color: rgba(255,255,255,0.4);
  font-size: 0.85rem;
  margin-bottom: 1.2rem;
}

.redeem-form {
  display: flex;
  gap: 0.8rem;
}

.code-input {
  flex: 1;
  padding: 0.8rem 1rem;
  border: 1px solid rgba(255,255,255,0.15);
  border-radius: 10px;
  background: rgba(255,255,255,0.05);
  color: #fff;
  font-size: 1rem;
  font-family: monospace;
  letter-spacing: 1px;
  text-transform: uppercase;
}

.code-input:focus {
  outline: none;
  border-color: #f5af19;
}

.redeem-btn {
  padding: 0.8rem 1.5rem;
  border: none;
  border-radius: 10px;
  background: linear-gradient(135deg, #f5af19, #f12711);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s;
}

.redeem-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 15px rgba(241,39,17,0.3);
}

.redeem-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.result {
  margin-top: 1rem;
  padding: 0.8rem 1rem;
  border-radius: 10px;
  font-size: 0.9rem;
}

.result.success {
  background: rgba(34,197,94,0.1);
  color: #22c55e;
  border: 1px solid rgba(34,197,94,0.2);
}

.result.error {
  background: rgba(239,68,68,0.1);
  color: #ef4444;
  border: 1px solid rgba(239,68,68,0.2);
}

.guide-items {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.guide-item {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
}

.guide-icon {
  font-size: 1.5rem;
}

.guide-item strong {
  color: #fff;
  font-size: 0.95rem;
}

.guide-item p {
  color: rgba(255,255,255,0.4);
  font-size: 0.8rem;
  margin: 0.2rem 0 0 0;
}

@media (max-width: 500px) {
  .redeem-form {
    flex-direction: column;
  }
}
</style>
