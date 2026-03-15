<script setup>
import { ref, onMounted } from 'vue'
import { userApi } from '../../api/user'

const logs = ref([])
const loading = ref(true)

onMounted(async () => {
  try {
    const data = await userApi('/logs')
    logs.value = data.logs || []
  } catch {}
  loading.value = false
})

function fmtTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="logs-page">
    <h3>📋 请求日志</h3>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else-if="logs.length === 0" class="empty">
      <div class="empty-icon">📭</div>
      <div class="empty-title">暂无请求记录</div>
      <div class="empty-desc">开始使用 API 后，您的请求日志将在此显示</div>
    </div>

    <div v-else class="log-list">
      <div class="log-item" v-for="log in logs" :key="log.request_id">
        <div class="log-header">
          <span class="log-model">{{ log.actual_model || log.original_model }}</span>
          <span :class="['log-status', log.status === 'error' ? 'err' : 'ok']">
            {{ log.status === 'error' ? '❌' : '✅' }}
          </span>
          <span class="log-time">{{ log.time || fmtTime(log.timestamp * 1000) }}</span>
        </div>
        <div class="log-meta">
          <span v-if="log.input_tokens">入: {{ (log.input_tokens/1000).toFixed(1) }}K</span>
          <span v-if="log.output_tokens">出: {{ (log.output_tokens/1000).toFixed(1) }}K</span>
          <span v-if="log.credits">Credits: {{ log.credits.toFixed(2) }}</span>
          <span v-if="log.duration_ms">{{ log.duration_ms }}ms</span>
          <span v-if="log.stop_reason" class="stop-reason">{{ log.stop_reason }}</span>
        </div>
        <div v-if="log.error" class="log-error">{{ log.error }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.logs-page h3 {
  margin: 0 0 1rem 0;
  color: rgba(255,255,255,0.8);
}

.log-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.log-item {
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 10px;
  padding: 0.8rem 1rem;
  transition: background 0.2s;
}

.log-item:hover {
  background: rgba(255,255,255,0.06);
}

.log-header {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  margin-bottom: 0.3rem;
}

.log-model {
  font-family: monospace;
  color: #a78bfa;
  font-size: 0.85rem;
  font-weight: 600;
}

.log-status.ok { color: #22c55e; }
.log-status.err { color: #ef4444; }

.log-time {
  margin-left: auto;
  color: rgba(255,255,255,0.3);
  font-size: 0.75rem;
}

.log-meta {
  display: flex;
  gap: 1rem;
  font-size: 0.75rem;
  color: rgba(255,255,255,0.4);
}

.stop-reason {
  padding: 0.1rem 0.4rem;
  border-radius: 4px;
  background: rgba(139,92,246,0.1);
  color: #a78bfa;
}

.log-error {
  margin-top: 0.3rem;
  font-size: 0.8rem;
  color: #ef4444;
  background: rgba(239,68,68,0.08);
  padding: 0.3rem 0.6rem;
  border-radius: 6px;
}

.loading, .empty {
  text-align: center;
  padding: 3rem;
  color: rgba(255,255,255,0.3);
}

.empty-icon { font-size: 2.5rem; margin-bottom: 0.8rem; }
.empty-title { font-size: 1rem; font-weight: 600; color: rgba(255,255,255,0.5); margin-bottom: 0.4rem; }
.empty-desc { font-size: 0.8rem; color: rgba(255,255,255,0.25); }
</style>
