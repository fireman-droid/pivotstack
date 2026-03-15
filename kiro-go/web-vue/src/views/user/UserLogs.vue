<script setup>
import { ref, onMounted } from 'vue'
import { userApi } from '../../api/user'
import { FileX, CheckCircle2, XCircle, Clock, Database, Coins, Timer } from 'lucide-vue-next'

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
    <div class="page-header">
      <div class="title-section">
        <h3>请求日志</h3>
        <span class="count-badge">{{ logs.length }}</span>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div v-for="i in 3" :key="i" class="skeleton-card shimmer"></div>
    </div>

    <div v-else-if="logs.length === 0" class="empty-state">
      <div class="empty-icon-wrapper">
        <FileX class="w-12 h-12 text-slate-600" />
      </div>
      <h4>暂无请求记录</h4>
      <p>开始使用 API 后，您的请求日志将在此显示</p>
    </div>

    <div v-else class="log-list">
      <div 
        v-for="log in logs" 
        :key="log.request_id"
        :class="['log-card', log.status === 'error' ? 'status-error' : 'status-success']"
      >
        <div class="log-main">
          <div class="log-info">
            <div class="model-name">
              {{ log.actual_model || log.original_model }}
            </div>
            <div class="status-indicator">
              <CheckCircle2 v-if="log.status !== 'error'" class="status-icon success" />
              <XCircle v-else class="status-icon error" />
            </div>
          </div>
          <div class="log-time">
            <Clock class="w-3 h-3 mr-1" />
            {{ log.time || fmtTime(log.timestamp * 1000) }}
          </div>
        </div>

        <div class="log-meta">
          <div class="meta-item">
            <Database class="w-3.5 h-3.5" />
            <span>Tokens: {{ ((log.input_tokens || 0) + (log.output_tokens || 0)) / 1000 }}K</span>
          </div>
          <div class="meta-item">
            <Coins class="w-3.5 h-3.5" />
            <span>Credits: {{ (log.credits || 0).toFixed(4) }}</span>
          </div>
          <div class="meta-item" v-if="log.duration_ms">
            <Timer class="w-3.5 h-3.5" />
            <span>{{ log.duration_ms }}ms</span>
          </div>
          <div v-if="log.stop_reason" class="stop-reason-badge">
            {{ log.stop_reason }}
          </div>
        </div>

        <div v-if="log.error" class="error-detail">
          {{ log.error }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  padding: 1rem 0;
}

.page-header {
  margin-bottom: 2rem;
}

.title-section {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

h3 {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.5rem;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.count-badge {
  background: rgba(99, 102, 241, 0.15);
  color: #818cf8;
  padding: 2px 10px;
  border-radius: 20px;
  font-size: 0.75rem;
  font-weight: 600;
  border: 1px solid rgba(99, 102, 241, 0.2);
}

.log-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.log-card {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  padding: 1rem 1.25rem;
  transition: all 200ms ease;
  position: relative;
  overflow: hidden;
}

.log-card:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
}

.status-success { border-left: 3px solid #22c55e; }
.status-error { border-left: 3px solid #ef4444; }

.log-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;
}

.log-info {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.model-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: #c084fc;
  font-weight: 600;
  font-size: 0.9375rem;
}

.status-icon {
  width: 16px;
  height: 16px;
}

.status-icon.success { color: #22c55e; }
.status-icon.error { color: #ef4444; }

.log-time {
  display: flex;
  align-items: center;
  color: #6b7280;
  font-size: 0.75rem;
}

.log-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 1.25rem;
  align-items: center;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  color: #94a3b8;
  font-size: 0.8125rem;
}

.stop-reason-badge {
  background: rgba(255, 255, 255, 0.06);
  color: #94a3b8;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.025em;
}

.error-detail {
  margin-top: 0.75rem;
  padding: 0.75rem;
  background: rgba(239, 68, 68, 0.05);
  border-radius: 8px;
  color: #f87171;
  font-size: 0.8125rem;
  border: 1px solid rgba(239, 68, 68, 0.1);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem 2rem;
  text-align: center;
}

.empty-icon-wrapper {
  width: 80px;
  height: 80px;
  background: rgba(255, 255, 255, 0.03);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 1.5rem;
}

.empty-state h4 {
  color: #f8fafc;
  font-size: 1.125rem;
  font-weight: 600;
  margin: 0 0 0.5rem;
}

.empty-state p {
  color: #64748b;
  font-size: 0.875rem;
  max-width: 240px;
  margin: 0;
}

.loading-state {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.skeleton-card {
  height: 80px;
  background: rgba(255, 255, 255, 0.03);
  border-radius: 12px;
}

.shimmer {
  position: relative;
  overflow: hidden;
}

.shimmer::after {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  transform: translateX(-100%);
  background-image: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0,
    rgba(255, 255, 255, 0.03) 20%,
    rgba(255, 255, 255, 0.06) 60%,
    rgba(255, 255, 255, 0)
  );
  animation: shimmer 2s infinite;
  content: '';
}

@keyframes shimmer {
  100% { transform: translateX(100%); }
}

.w-12 { width: 3rem; }
.h-12 { height: 3rem; }
.mr-1 { margin-right: 0.25rem; }
</style>
