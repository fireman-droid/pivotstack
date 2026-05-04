<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { Trophy, RefreshCw, Sparkles } from 'lucide-vue-next'
import { api } from '../api/admin'
import WorldCard from '../components/world/WorldCard.vue'
import WorldStat from '../components/world/WorldStat.vue'
import WorldTable from '../components/world/WorldTable.vue'
import WorldChip from '../components/world/WorldChip.vue'
import WorldSegment from '../components/world/WorldSegment.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldLoader from '../components/world/WorldLoader.vue'

const metric = ref('requests')
const metricOptions = [
  { value: 'requests', label: '请求数' },
  { value: 'credits',  label: 'Credit 消耗' },
  { value: 'tokens',   label: 'Token 数' },
]

const data = ref({ metric: 'requests', updated: 0, top: [], total: 0 })
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await api(`/leaderboard?metric=${metric.value}`)
    if (res.ok) {
      data.value = await res.json()
    }
  } catch {}
  loading.value = false
}

onMounted(load)
watch(metric, load)

const fakeCount = computed(() => data.value.top.filter(r => r.isFake).length)
const realCount = computed(() => data.value.total || 0)

function formatValue(v) {
  if (metric.value === 'credits') return Number(v).toFixed(4)
  return Number(v).toLocaleString('zh-CN')
}
function formatUpdated(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

const columns = [
  { key: 'rank',     label: '排位',   width: '60px', align: 'center' },
  { key: 'alias',    label: '展示名', align: 'left' },
  { key: 'note',     label: '备注',   align: 'left' },
  { key: 'realId',   label: '真实ID', align: 'left', mono: true },
  { key: 'isFake',   label: '类型',   width: '80px', align: 'center' },
  { key: 'value',    label: '数值',   align: 'right', mono: true },
]
</script>

<template>
  <div class="lb-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">Engagement</div>
        <h1 class="page-title">用户排行榜（管理视图）</h1>
      </div>
      <WorldButton variant="secondary" size="sm" @click="load" :loading="loading">
        <RefreshCw :size="13" /><span>刷新</span>
      </WorldButton>
    </header>

    <div class="stat-row">
      <WorldStat
        label="真实用户上榜"
        :value="realCount"
        hint="metric > 0 的真实 keys"
        :icon="Trophy"
        variant="success"
      />
      <WorldStat
        label="虚拟条目"
        :value="fakeCount"
        :hint="fakeCount > 0 ? '当前在前端混入展示' : '未启用虚拟条目'"
        :icon="Sparkles"
        :variant="fakeCount > 0 ? 'warning' : 'neutral'"
      />
      <WorldStat
        label="数据时间"
        :value="formatUpdated(data.updated)"
        hint="点击刷新拉取最新"
        :icon="RefreshCw"
        variant="info"
      />
    </div>

    <div class="seg-row">
      <WorldSegment v-model="metric" :options="metricOptions" />
    </div>

    <WorldCard padding="md">
      <header class="section-head">
        <h3><Trophy :size="16" /><span>完整排行</span></h3>
        <WorldChip size="sm" variant="warning">含虚拟</WorldChip>
      </header>

      <div v-if="loading && !data.top.length" class="loading-wrap">
        <WorldLoader :size="40" label="加载中" />
      </div>
      <WorldTable
        v-else
        :columns="columns"
        :rows="data.top"
        empty-text="无上榜数据"
        max-height="620px"
      >
        <template #cell-isFake="{ row }">
          <WorldChip v-if="row.isFake" size="sm" variant="warning">虚拟</WorldChip>
          <WorldChip v-else size="sm" variant="success">真实</WorldChip>
        </template>
        <template #cell-realId="{ row }">
          <span class="mono small">{{ row.realId ? row.realId.slice(0, 12) + '…' : '—' }}</span>
        </template>
        <template #cell-note="{ row }">
          <span>{{ row.note || '—' }}</span>
        </template>
        <template #cell-value="{ row }">
          <span class="mono">{{ formatValue(row.value) }}</span>
        </template>
      </WorldTable>
    </WorldCard>
  </div>
</template>

<style scoped>
.lb-page { display: flex; flex-direction: column; gap: 16px; }
.page-head {
  display: flex; align-items: flex-end; justify-content: space-between;
  gap: 12px; flex-wrap: wrap;
}
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem; font-weight: 800;
  letter-spacing: 0.18em; text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  font-family: var(--world-font-display);
  font-size: 1.5rem; font-weight: 800;
  margin: 0; color: var(--world-text-primary);
}
.stat-row {
  display: grid; gap: 12px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
@media (max-width: 720px) { .stat-row { grid-template-columns: 1fr; } }

.seg-row { display: flex; justify-content: flex-end; }

.section-head {
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; margin-bottom: 12px;
}
.section-head h3 {
  display: flex; align-items: center; gap: 8px;
  margin: 0; font-size: 0.875rem; font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}

.mono {
  font-family: var(--world-font-mono);
  font-weight: 700;
  color: var(--world-text-primary);
}
.mono.small { font-size: 0.78rem; color: var(--world-text-mute); }

.loading-wrap {
  display: flex; align-items: center; justify-content: center;
  padding: 40px;
}
</style>
