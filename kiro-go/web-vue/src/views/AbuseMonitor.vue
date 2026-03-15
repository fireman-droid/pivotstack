<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { ShieldAlert, XCircle, RefreshCw } from 'lucide-vue-next'

const { success, error: toastError } = useToast()
const flagged = ref([])
const loading = ref(true)

async function loadFlagged() {
  loading.value = true
  try {
    const res = await api('/abuse')
    if (res.ok) flagged.value = await res.json()
  } catch { toastError('加载失败') }
  loading.value = false
}

async function clearFlag(keyId) {
  try {
    await api(`/abuse/${keyId}/clear`, { method: 'POST' })
    flagged.value = flagged.value.filter(f => f.keyId !== keyId)
    success('已清除标记')
  } catch { toastError('清除失败') }
}

onMounted(loadFlagged)
</script>

<template>
  <div class="space-y-6 max-w-[1200px] mx-auto pb-20">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-black tracking-tight text-[var(--text)]">滥用监控</h1>
        <p class="text-sm text-[var(--text-secondary)]">被标记的异常 API Key（IP多样性过高等）</p>
      </div>
      <button @click="loadFlagged"
        class="flex items-center gap-2 px-4 py-2 bg-[var(--card)] border border-[var(--border)] rounded-xl text-sm font-bold hover:border-[var(--primary)] transition-all">
        <RefreshCw class="w-4 h-4" />
        刷新
      </button>
    </div>

    <!-- Flagged List -->
    <div v-if="flagged.length" class="space-y-3">
      <div v-for="item in flagged" :key="item.keyId" class="modern-card p-5">
        <div class="flex items-center justify-between mb-3">
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-amber-500/10 flex items-center justify-center">
              <ShieldAlert class="w-5 h-5 text-amber-500" />
            </div>
            <div>
              <div class="text-sm font-bold font-mono text-[var(--text)]">{{ item.keyId }}</div>
              <div class="text-[10px] text-[var(--text-secondary)]">{{ item.reason || '异常行为' }}</div>
            </div>
          </div>
          <button @click="clearFlag(item.keyId)"
            class="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-emerald-500/10 text-emerald-500 hover:bg-emerald-500/20 transition-all">
            <XCircle class="w-3.5 h-3.5" />
            清除标记
          </button>
        </div>

        <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div class="p-3 bg-[var(--bg)] rounded-xl">
            <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">活跃流</div>
            <div class="text-sm font-black">{{ item.activeStreams || 0 }}</div>
          </div>
          <div class="p-3 bg-[var(--bg)] rounded-xl">
            <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">IP 数</div>
            <div class="text-sm font-black" :class="item.distinctIPs > 10 ? 'text-amber-500' : ''">{{ item.distinctIPs || 0 }}</div>
          </div>
          <div class="p-3 bg-[var(--bg)] rounded-xl">
            <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">近期请求</div>
            <div class="text-sm font-black">{{ item.recentRequests || 0 }}</div>
          </div>
          <div class="p-3 bg-[var(--bg)] rounded-xl">
            <div class="text-[9px] font-bold text-[var(--text-secondary)] uppercase mb-1">标记时间</div>
            <div class="text-xs font-bold">{{ item.flaggedAt ? new Date(item.flaggedAt).toLocaleString('zh-CN') : '-' }}</div>
          </div>
        </div>
      </div>
    </div>

    <div v-else-if="!loading" class="text-center py-20">
      <ShieldAlert class="w-12 h-12 text-[var(--text-secondary)] opacity-20 mx-auto mb-3" />
      <div class="text-sm font-bold text-[var(--text-secondary)]">没有被标记的 Key</div>
      <div class="text-xs text-[var(--text-secondary)] mt-1">当 Key 出现异常行为时会自动标记</div>
    </div>
  </div>
</template>
