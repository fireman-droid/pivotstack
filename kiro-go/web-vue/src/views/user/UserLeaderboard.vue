<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { Trophy, Activity, Coins, Cpu, RefreshCw } from 'lucide-vue-next'
import WorldCard from '../../components/world/WorldCard.vue'
import WorldStat from '../../components/world/WorldStat.vue'
import WorldTable from '../../components/world/WorldTable.vue'
import WorldChip from '../../components/world/WorldChip.vue'
import WorldSegment from '../../components/world/WorldSegment.vue'
import WorldButton from '../../components/world/WorldButton.vue'
import WorldLoader from '../../components/world/WorldLoader.vue'

const metric = ref('requests')
const metricOptions = [
  { value: 'requests', label: '请求数' },
  { value: 'credits',  label: 'Credit 消耗' },
  { value: 'tokens',   label: 'Token 数' },
]

const data = ref({ metric: 'requests', updated: 0, top: [], you: null, total: 0 })
const loading = ref(false)
const errMsg = ref('')

const apiKey = computed(() => localStorage.getItem('user_api_key') || '')

async function load() {
  loading.value = true
  errMsg.value = ''
  try {
    const res = await fetch(`/user/api/leaderboard?metric=${metric.value}`, {
      headers: { 'Authorization': `Bearer ${apiKey.value}` },
    })
    if (res.status === 404) {
      errMsg.value = '排行榜暂未开放'
      data.value = { metric: metric.value, updated: 0, top: [], you: null, total: 0 }
    } else if (!res.ok) {
      errMsg.value = `载入失败 (${res.status})`
    } else {
      data.value = await res.json()
    }
  } catch (e) {
    errMsg.value = '网络异常'
  }
  loading.value = false
}

onMounted(load)
watch(metric, load)

const metricLabel = computed(() => {
  const o = metricOptions.find(x => x.value === metric.value)
  return o ? o.label : '数值'
})

function formatValue(v) {
  if (metric.value === 'credits') return Number(v).toFixed(4)
  return Number(v).toLocaleString('zh-CN')
}

function formatUpdated(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

const youRankPercent = computed(() => {
  if (!data.value.you || !data.value.total) return null
  return Math.max(1, Math.round((data.value.you.rank / data.value.total) * 100))
})

const columns = [
  { key: 'rank',  label: '排位',           width: '70px', align: 'center' },
  { key: 'alias', label: '道号',           align: 'left' },
  { key: 'value', label: metricLabel.value, align: 'right', mono: true },
]

watch(metricLabel, () => {
  columns[2].label = metricLabel.value
})
</script>

<template>
  <div class="lb-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">用量榜单</div>
        <h1 class="page-title">用户排行榜</h1>
      </div>
      <WorldButton variant="secondary" size="sm" @click="load" :loading="loading">
        <RefreshCw :size="13" /><span>刷新</span>
      </WorldButton>
    </header>

    <div v-if="errMsg" class="msg-row">{{ errMsg }}</div>

    <template v-else>
      <!-- 自己位次 -->
      <div class="stat-row">
        <WorldStat
          v-if="data.you"
          label="你的位次"
          :value="`#${data.you.rank}`"
          :hint="`共 ${data.total} 位用户`"
          :icon="Trophy"
          variant="primary"
        />
        <WorldStat
          v-else
          label="你的位次"
          value="—"
          hint="尚无消耗，先发起几次请求吧"
          :icon="Trophy"
          variant="neutral"
        />
        <WorldStat
          label="本次指标"
          :value="metricLabel"
          :hint="`数据更新于 ${formatUpdated(data.updated)}`"
          :icon="metric === 'requests' ? Activity : metric === 'credits' ? Coins : Cpu"
          variant="info"
        />
        <WorldStat
          label="超越百分比"
          :value="youRankPercent ? `前 ${youRankPercent}%` : '—'"
          :hint="youRankPercent ? '继续加油' : '使用后才会进入排行'"
          :icon="Activity"
          :variant="youRankPercent && youRankPercent <= 10 ? 'success' : 'info'"
        />
      </div>

      <!-- 切换指标 -->
      <div class="seg-row">
        <WorldSegment v-model="metric" :options="metricOptions" />
      </div>

      <!-- TOP 表 -->
      <WorldCard padding="md">
        <header class="section-head">
          <h3>
            <Trophy :size="16" />
            <span>本日 TOP {{ data.top.length }}</span>
          </h3>
          <WorldChip size="sm" variant="info">每日刷新</WorldChip>
        </header>

        <div v-if="loading && !data.top.length" class="loading-wrap">
          <WorldLoader :size="40" label="加载中" />
        </div>
        <WorldTable
          v-else
          :columns="columns"
          :rows="data.top"
          empty-text="暂无上榜用户"
          max-height="540px"
        >
          <template #cell-rank="{ row }">
            <span :class="['rank-num', row.isYou && 'is-you', row.rank <= 3 && 'is-top']">
              {{ row.rank }}
            </span>
          </template>
          <template #cell-alias="{ row }">
            <span :class="['alias-cell', row.isYou && 'is-you']">
              {{ row.alias }}
              <WorldChip v-if="row.isYou" size="sm" variant="primary">你</WorldChip>
            </span>
          </template>
          <template #cell-value="{ row }">
            <span class="value-cell mono">{{ formatValue(row.value) }}</span>
          </template>
        </WorldTable>
      </WorldCard>
    </template>
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
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
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

.rank-num {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 32px; height: 28px; padding: 0 8px;
  border-radius: var(--world-radius-sm);
  font-family: var(--world-font-mono);
  font-weight: 800;
  color: var(--world-text-mute);
  background: var(--world-overlay-light);
}
.rank-num.is-top { color: var(--world-accent); background: var(--world-overlay-strong, var(--world-overlay-light)); }
.rank-num.is-you {
  color: white;
  background: linear-gradient(135deg, var(--world-accent), var(--world-paper-aged, var(--world-accent-soft, #38bdf8)));
}

.alias-cell {
  display: inline-flex; align-items: center; gap: 8px;
  font-weight: 700;
}
.alias-cell.is-you { color: var(--world-accent); }

.value-cell {
  font-family: var(--world-font-mono);
  font-weight: 700;
  color: var(--world-text-primary);
}

.loading-wrap {
  display: flex; align-items: center; justify-content: center;
  padding: 40px;
}

.msg-row {
  padding: 24px;
  text-align: center;
  color: var(--world-text-mute);
  font-size: 0.95rem;
  background: var(--world-overlay-light);
  border-radius: var(--world-radius-md);
}
</style>
