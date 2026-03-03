<script setup>
import { ref } from 'vue'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../composables/useToast'
import { VideoPlay, VideoPause, Refresh, Delete, Setting, Download } from '@element-plus/icons-vue'

const store = useAccountsStore()
const { success, error } = useToast()
const weightValue = ref(0)

async function doBatch(action, extra = {}) {
  const count = store.selectedIds.size
  if (!count) return
  const confirmMap = {
    enable: `确定启用 ${count} 个账号？`,
    disable: `确定禁用 ${count} 个账号？`,
    refresh: `确定刷新 ${count} 个账号？`,
    delete: `确定删除 ${count} 个账号？此操作不可撤销！`,
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
        a.download = `kiro-accounts-${new Date().toISOString().slice(0, 10)}.json`
        a.click()
        URL.revokeObjectURL(url)
        success('导出成功')
      } else success('操作完成')
    }
  } catch { error('操作失败') }
}
</script>

<template>
  <div v-if="store.selectedIds.size > 0" class="flex items-center gap-3">
    <span class="text-sm font-medium text-blue-600">已选 {{ store.selectedIds.size }} 项</span>
    <el-divider direction="vertical" />
    
    <el-button-group>
      <el-button type="primary" :icon="VideoPlay" @click="doBatch('enable')">启用</el-button>
      <el-button type="info" :icon="VideoPause" @click="doBatch('disable')">禁用</el-button>
      <el-button type="primary" plain :icon="Refresh" @click="doBatch('refresh')">刷新</el-button>
      <el-button type="danger" :icon="Delete" @click="doBatch('delete')">删除</el-button>
    </el-button-group>
    
    <div class="flex items-center gap-1">
      <el-select v-model="weightValue" class="w-20">
        <el-option v-for="w in [0,1,2,3,4,5]" :key="w" :label="'W:'+w" :value="w" />
      </el-select>
      <el-button type="warning" :icon="Setting" @click="doBatch('setWeight', { weight: weightValue })">设权重</el-button>
    </div>
    
    <el-button type="success" :icon="Download" @click="doBatch('export')">导出</el-button>
    
    <el-button link type="info" @click="store.clearSelection()">取消选择</el-button>
  </div>
</template>
