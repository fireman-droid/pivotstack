<script setup lang="ts">
import { useRouter } from 'vue-router'
import Tile from '../stellar/Tile.vue'
import MonoCopy from '../stellar/MonoCopy.vue'

const props = defineProps<{
  endpoint: string
  apiKey: string
}>()

const router = useRouter()

function maskKey(k: string) {
  if (!k || k.length < 12) return k
  const last = k.slice(-4)
  return `sk-•••••••••••${last}`
}
</script>

<template>
  <Tile>
    <div class="tile__head"><span class="t-display">接入</span></div>
    <div class="kv">
      <span class="t-label">ENDPOINT</span>
      <MonoCopy :value="endpoint" />
    </div>
    <div class="kv">
      <span class="t-label">API KEY</span>
      <MonoCopy :value="apiKey" :display="maskKey(apiKey)" />
    </div>
    <button class="btn btn--secondary btn--block" style="margin-top: 12px" @click="router.push('/user/api-docs')">
      查看接入示例 →
    </button>
  </Tile>
</template>

<style scoped>
.kv { margin-bottom: 12px; display: flex; flex-direction: column; gap: 6px; }
</style>
