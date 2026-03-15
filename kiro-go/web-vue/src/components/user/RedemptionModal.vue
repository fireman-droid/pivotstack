<script setup>
import { ref } from 'vue'
import { userApi } from '../../api/user'

const props = defineProps({ show: Boolean })
const emit = defineEmits(['close', 'redeemed'])

const code = ref('')
const loading = ref(false)
const receipt = ref(null)
const error = ref('')

async function redeem() {
  if (!code.value.trim()) return
  loading.value = true
  error.value = ''
  try {
    const data = await userApi('/redeem', {
      method: 'POST',
      body: JSON.stringify({ code: code.value.trim() })
    })
    if (data.error) {
      error.value = data.error
    } else {
      receipt.value = data
      emit('redeemed', data)
    }
  } catch (e) {
    error.value = e.message || '网络错误'
  }
  loading.value = false
}

function close() {
  emit('close')
  setTimeout(() => { code.value = ''; receipt.value = null; error.value = '' }, 300)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="modal-overlay" @click.self="close">
      <div class="modal-backdrop" />
      <div class="modal-box">
        <div class="modal-header">
          <h3>🎁 激活码兑换</h3>
          <button @click="close" class="close-btn">✕</button>
        </div>

        <!-- Input -->
        <div v-if="!receipt" class="modal-body">
          <input v-model="code" placeholder="KIRO-XXXX-XXXX-XXXX"
            class="code-input" @keyup.enter="redeem" autofocus />
          <div v-if="error" class="error-msg">{{ error }}</div>
          <button @click="redeem" :disabled="loading || !code.trim()" class="redeem-btn">
            {{ loading ? '验证中...' : '立即兑换' }}
          </button>
        </div>

        <!-- Receipt -->
        <div v-else class="modal-body receipt">
          <div class="receipt-icon">✅</div>
          <div class="receipt-title">兑换成功！</div>
          <div class="receipt-card">
            <div v-if="receipt.type === 'balance'" class="receipt-row">
              <span class="label">余额变化</span>
              <span class="value">
                ¥{{ (receipt.balanceBefore || 0).toFixed(2) }} → ¥{{ (receipt.balanceAfter || 0).toFixed(2) }}
                <span class="add">(+¥{{ (receipt.amount || 0).toFixed(2) }})</span>
              </span>
            </div>
            <div v-if="receipt.type === 'days'" class="receipt-row">
              <span class="label">有效期延长</span>
              <span class="value add">+{{ receipt.amount }} 天</span>
            </div>
          </div>
          <button @click="close" class="redeem-btn">完成</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.modal-overlay {
  position: fixed; inset: 0; z-index: 100;
  display: flex; align-items: center; justify-content: center; padding: 1rem;
}
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.5); backdrop-filter: blur(6px); }
.modal-box {
  position: relative; width: 100%; max-width: 420px;
  background: #1a1a2e; border: 1px solid rgba(255,255,255,0.1);
  border-radius: 16px; overflow: hidden; box-shadow: 0 25px 50px rgba(0,0,0,0.5);
}
.modal-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.5rem; border-bottom: 1px solid rgba(255,255,255,0.08);
}
.modal-header h3 { margin: 0; font-size: 0.95rem; color: #fff; }
.close-btn { background: none; border: none; color: rgba(255,255,255,0.5); cursor: pointer; font-size: 1rem; }
.modal-body { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.code-input {
  width: 100%; height: 48px; padding: 0 1rem;
  background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.12);
  border-radius: 10px; color: #fff; font-family: monospace; font-size: 0.95rem;
  text-align: center; letter-spacing: 2px; text-transform: uppercase;
  outline: none; box-sizing: border-box;
}
.code-input:focus { border-color: #f5af19; }
.error-msg { color: #ff6b6b; font-size: 0.85rem; background: rgba(255,107,107,0.1); padding: 0.5rem; border-radius: 8px; text-align: center; }
.redeem-btn {
  padding: 0.8rem; border: none; border-radius: 10px;
  background: linear-gradient(135deg, #f5af19, #f12711); color: #fff;
  font-weight: 600; cursor: pointer; font-size: 0.95rem;
}
.redeem-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.receipt { text-align: center; }
.receipt-icon { font-size: 2.5rem; }
.receipt-title { font-size: 1rem; font-weight: 700; color: #fff; }
.receipt-card {
  background: rgba(255,255,255,0.04); border-radius: 10px; padding: 1rem;
  text-align: left;
}
.receipt-row { display: flex; justify-content: space-between; font-size: 0.85rem; color: rgba(255,255,255,0.7); }
.label { color: rgba(255,255,255,0.45); }
.value { font-weight: 600; }
.add { color: #22c55e; }
</style>
