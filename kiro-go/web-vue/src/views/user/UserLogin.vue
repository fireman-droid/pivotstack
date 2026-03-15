<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserAuth } from '../../stores/userAuth'

const router = useRouter()
const auth = useUserAuth()
const apiKey = ref('')
const rememberDevice = ref(true)

async function handleLogin() {
  if (!apiKey.value.trim()) return
  const ok = await auth.login(apiKey.value.trim(), rememberDevice.value)
  if (ok) {
    router.replace('/user/dashboard')
  }
}
</script>

<template>
  <div class="user-login-page">
    <div class="login-card">
      <div class="login-header">
        <div class="logo">🪙</div>
        <h1>KiroStack</h1>
        <p class="subtitle">无需注册，API Key 即账户</p>
        <p class="sub-desc">输入您的 API Key 登录用户面板</p>
      </div>

      <form @submit.prevent="handleLogin" class="login-form">
        <div class="input-group">
          <label>API Key</label>
          <input
            v-model="apiKey"
            type="password"
            placeholder="kiro-xxxxxxxxxxxxxxxx"
            autocomplete="off"
            autofocus
          />
        </div>

        <div v-if="auth.error" class="error-msg">
          {{ auth.error }}
        </div>

        <button type="submit" :disabled="auth.loading || !apiKey.trim()">
          <span v-if="auth.loading">验证中...</span>
          <span v-else>登 录</span>
        </button>

        <label class="remember-toggle">
          <input type="checkbox" v-model="rememberDevice" />
          <span>记住此设备</span>
        </label>
      </form>

      <div class="login-footer">
        <p>没有 API Key？请联系管理员获取</p>
        <a href="#/login" class="admin-link">管理员入口 →</a>
      </div>
    </div>
  </div>
</template>

<style scoped>
.user-login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #0f0c29 0%, #1a1a2e 50%, #16213e 100%);
  padding: 1rem;
}

.login-card {
  background: rgba(255,255,255,0.05);
  backdrop-filter: blur(20px);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 20px;
  padding: 3rem 2.5rem;
  width: 100%;
  max-width: 420px;
  box-shadow: 0 25px 50px rgba(0,0,0,0.4);
}

.login-header {
  text-align: center;
  margin-bottom: 2rem;
}

.logo {
  font-size: 3rem;
  margin-bottom: 0.5rem;
  animation: float 3s ease-in-out infinite;
}

@keyframes float {
  0%,100% { transform: translateY(0); }
  50% { transform: translateY(-8px); }
}

.login-header h1 {
  font-size: 1.8rem;
  font-weight: 700;
  background: linear-gradient(135deg, #f5af19, #f12711);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin: 0;
}

.subtitle {
  color: rgba(255,255,255,0.5);
  font-size: 0.9rem;
  margin-top: 0.5rem;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
}

.input-group label {
  display: block;
  color: rgba(255,255,255,0.6);
  font-size: 0.85rem;
  margin-bottom: 0.4rem;
}

.input-group input {
  width: 100%;
  padding: 0.8rem 1rem;
  border: 1px solid rgba(255,255,255,0.15);
  border-radius: 10px;
  background: rgba(255,255,255,0.05);
  color: #fff;
  font-size: 0.95rem;
  transition: all 0.3s;
  box-sizing: border-box;
}

.input-group input:focus {
  outline: none;
  border-color: #f5af19;
  box-shadow: 0 0 0 3px rgba(245,175,25,0.15);
}

.error-msg {
  color: #ff6b6b;
  font-size: 0.85rem;
  padding: 0.5rem 0.8rem;
  background: rgba(255,107,107,0.1);
  border-radius: 8px;
}

button {
  padding: 0.9rem;
  border: none;
  border-radius: 10px;
  background: linear-gradient(135deg, #f5af19, #f12711);
  color: #fff;
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s;
}

button:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(241,39,17,0.3);
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.login-footer {
  text-align: center;
  margin-top: 2rem;
  color: rgba(255,255,255,0.4);
  font-size: 0.8rem;
}

.admin-link {
  color: rgba(245,175,25,0.7);
  text-decoration: none;
  display: inline-block;
  margin-top: 0.5rem;
}

.admin-link:hover {
  color: #f5af19;
}

.sub-desc {
  color: rgba(255,255,255,0.35);
  font-size: 0.8rem;
  margin-top: 0.3rem;
}

.remember-toggle {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.85rem;
  color: rgba(255,255,255,0.5);
  cursor: pointer;
  justify-content: center;
}

.remember-toggle input[type="checkbox"] {
  accent-color: #f5af19;
}
</style>
