<script setup>
import { ref, computed, onMounted } from 'vue'
import { userApi } from '../../api/user'
import { Wallet, Users, ShoppingCart } from 'lucide-vue-next'
import WorldCard from '../../components/world/WorldCard.vue'
import WorldStat from '../../components/world/WorldStat.vue'
import WorldTable from '../../components/world/WorldTable.vue'
import WorldLoader from '../../components/world/WorldLoader.vue'

const summary = ref(null)
const keys = ref([])
const loading = ref(true)

async function load() {
  try {
    const [s, k] = await Promise.all([
      userApi('/reseller/summary'),
      userApi('/reseller/keys'),
    ])
    summary.value = s
    keys.value = k.keys || []
  } catch (e) {
    console.error('Failed to load reseller summary', e)
  }
  loading.value = false
}

onMounted(load)

const balanceCNY = computed(() => ((summary.value?.totalBalance || 0) * 0.05))
const soldCNY    = computed(() => ((summary.value?.soldToChildren || 0) * 0.05))
const rechargedCNY = computed(() => ((summary.value?.totalRecharged || 0) * 0.05))

const balanceVariant = computed(() => (summary.value?.totalBalance || 0) < 1 ? 'danger' : 'success')

// Top-5 子 key 按 7 天调用排序
const topChildren = computed(() => {
  return [...keys.value]
    .sort((a, b) => (b.recentCalls7d || 0) - (a.recentCalls7d || 0))
    .slice(0, 5)
    .map(k => ({
      note: k.note || k.id.slice(0, 8),
      keyMasked: k.keyMasked,
      balance: '$' + (k.totalBalance || 0).toFixed(2),
      requests: (k.requests || 0).toLocaleString(),
      recent7d: (k.recentCalls7d || 0).toLocaleString(),
    }))
})
</script>

<template>
  <div v-if="!loading" class="summary-page">
    <!-- 3 个核心指标（移除"估算利润"——利润由 admin 出激活码时手算让利，系统不再估算） -->
    <div class="stat-grid">
      <WorldStat
        label="我的余额"
        :value="`$${(summary?.totalBalance || 0).toFixed(2)}`"
        :hint="`折合 ¥${balanceCNY.toFixed(2)}`"
        :variant="balanceVariant"
        :icon="Wallet"
      />
      <WorldStat
        label="子 Key 数量"
        :value="String(summary?.childCount || 0)"
        :hint="summary?.maxChildKeys ? `上限 ${summary.maxChildKeys}` : '无上限'"
        variant="info"
        :icon="Users"
      />
      <WorldStat
        label="累计已转出"
        :value="`$${(summary?.soldToChildren || 0).toFixed(2)}`"
        :hint="`折合 ¥${soldCNY.toFixed(2)} · 累计进货 ¥${rechargedCNY.toFixed(2)}`"
        variant="primary"
        :icon="ShoppingCart"
      />
    </div>

    <!-- 子 key 7 天调用排行 -->
    <WorldCard padding="md">
      <h3 class="section-title">子 Key 活跃排行（近 7 天）</h3>
      <WorldTable
        v-if="topChildren.length > 0"
        :columns="[
          { key: 'note',     label: '备注', mono: false },
          { key: 'keyMasked', label: 'Key', mono: true },
          { key: 'balance',  label: '余额', align: 'right' },
          { key: 'requests', label: '总请求', align: 'right' },
          { key: 'recent7d', label: '近 7 天', align: 'right' },
        ]"
        :rows="topChildren"
        :compact="true"
      />
      <div v-else class="empty-row">
        暂无子 Key。前往「子 Key 管理」创建。
      </div>
    </WorldCard>

    <!-- 子 key 总览 -->
    <WorldCard padding="md">
      <h3 class="section-title">子 Key 汇总</h3>
      <div class="agg-grid">
        <div class="agg-cell">
          <div class="agg-label">子 Key 总余额</div>
          <div class="agg-val">${{ (summary?.childTotalBalance || 0).toFixed(2) }}</div>
        </div>
        <div class="agg-cell">
          <div class="agg-label">子 Key 总请求数</div>
          <div class="agg-val">{{ (summary?.childTotalRequests || 0).toLocaleString() }}</div>
        </div>
        <div class="agg-cell">
          <div class="agg-label">子 Key 总消耗 Credits</div>
          <div class="agg-val">{{ (summary?.childTotalCredits || 0).toFixed(2) }}</div>
        </div>
        <div class="agg-cell">
          <div class="agg-label">累计充值（admin 给我）</div>
          <div class="agg-val">${{ (summary?.totalRecharged || 0).toFixed(2) }}</div>
        </div>
      </div>
    </WorldCard>
  </div>

  <div v-else class="loading-wrap">
    <WorldLoader :size="48" label="载入数据中" />
  </div>
</template>

<style scoped>
.summary-page { display: flex; flex-direction: column; gap: 18px; }
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}
@media (max-width: 920px) { .stat-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 480px) { .stat-grid { grid-template-columns: 1fr; } }

.section-title {
  font-size: 0.95rem;
  font-weight: 800;
  margin: 0 0 14px;
  color: var(--world-text-primary);
  font-family: var(--world-font-display);
}
[data-world="daogui"] .section-title { color: var(--world-paper-aged); }

.empty-row {
  padding: 24px;
  text-align: center;
  color: var(--world-text-mute);
  font-size: 0.85rem;
}

.agg-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 14px;
}
.agg-cell { display: flex; flex-direction: column; gap: 4px; }
.agg-label {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.agg-val {
  font-size: 1.1rem;
  font-weight: 800;
  color: var(--world-text-primary);
  font-family: var(--world-font-mono);
}

.loading-wrap {
  min-height: 50vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
