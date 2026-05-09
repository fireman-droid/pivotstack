<script setup>
import { ref } from 'vue'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../composables/useToast'
import { Play, Pause, RefreshCw, Trash2, Settings, Download, X } from 'lucide-vue-next'
import WorldButton from './world/WorldButton.vue'
import WorldChip from './world/WorldChip.vue'

const store = useAccountsStore()
const { success, error } = useToast()
const weightValue = ref(0)

async function doBatch(action, extra = {}) {
  const count = store.selectedIds.size
  if (!count) return
  const confirmMap = {
    enable:  `确定启用 ${count} 个账号？`,
    disable: `确定禁用 ${count} 个账号？`,
    refresh: `确定刷新 ${count} 个账号？`,
    delete:  `确定删除 ${count} 个账号？此操作不可撤销！`,
    setWeight: `确定将 ${count} 个账号权重设为 ${extra.weight}？`,
  }
  if (confirmMap[action] && !confirm(confirmMap[action])) return
  try {
    const data = await store.batchAction(action, extra)
    if (data) {
      if (action === 'refresh') success(`刷新完成：成功 ${data.refreshed}，失败 ${data.failed}`)
      else if (action === 'delete') success(`删除完成：${data.deleted} 个`)
      else if (action === 'export') {
        const blob = new Blob([JSON.stringify(data.accounts, null, 2)], { type: 'application/json' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `pivotstack-accounts-${new Date().toISOString().slice(0, 10)}.json`
        a.click()
        URL.revokeObjectURL(url)
        success('导出成功')
      } else success('操作完成')
    }
  } catch { error('操作失败') }
}
</script>

<template>
  <div v-if="store.selectedIds.size > 0" class="batch-bar">
    <WorldChip variant="accent" :dot="true">已选 {{ store.selectedIds.size }} 项</WorldChip>

    <span class="divider" />

    <div class="btn-group">
      <WorldButton variant="primary" size="sm" @click="doBatch('enable')">
        <Play :size="13" /><span>启用</span>
      </WorldButton>
      <WorldButton variant="secondary" size="sm" @click="doBatch('disable')">
        <Pause :size="13" /><span>禁用</span>
      </WorldButton>
      <WorldButton variant="secondary" size="sm" @click="doBatch('refresh')">
        <RefreshCw :size="13" /><span>刷新</span>
      </WorldButton>
      <WorldButton variant="danger" size="sm" @click="doBatch('delete')">
        <Trash2 :size="13" /><span>删除</span>
      </WorldButton>
    </div>

    <span class="divider" />

    <div class="weight-group">
      <select v-model="weightValue" class="weight-select">
        <option v-for="w in [0,1,2,3,4,5]" :key="w" :value="w">W: {{ w }}</option>
      </select>
      <WorldButton variant="secondary" size="sm" @click="doBatch('setWeight', { weight: weightValue })">
        <Settings :size="13" /><span>设权重</span>
      </WorldButton>
    </div>

    <WorldButton variant="secondary" size="sm" @click="doBatch('export')">
      <Download :size="13" /><span>导出</span>
    </WorldButton>

    <button type="button" class="cancel-btn" @click="store.clearSelection()">
      <X :size="13" /><span>取消选择</span>
    </button>
  </div>
</template>

<style scoped>
.batch-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 14px;
  background: var(--world-glass-bg-strong);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-lg);
  backdrop-filter: blur(var(--world-glass-blur));
  -webkit-backdrop-filter: blur(var(--world-glass-blur));
  box-shadow: var(--world-shadow-sm);
}
.divider {
  width: 1px;
  height: 22px;
  background: var(--world-divider);
}
.btn-group, .weight-group { display: flex; gap: 6px; align-items: center; }

.weight-select {
  padding: 0 10px;
  height: 30px;
  border-radius: var(--world-radius-sm);
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  color: var(--world-text-primary);
  font-family: var(--world-font-mono);
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
  outline: none;
}
.weight-select:focus { border-color: var(--world-accent); }

.cancel-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  background: transparent;
  border: none;
  color: var(--world-text-mute);
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
  transition: color 200ms;
}
.cancel-btn:hover { color: var(--world-error); }
</style>
