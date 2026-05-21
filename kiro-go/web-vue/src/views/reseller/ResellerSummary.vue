<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { userApi } from '../../api/user'
import { useSystemUnit } from '../../composables/useSystemUnit'
import { NDataTable, NSpin, type DataTableColumns } from 'naive-ui'
import PageContainer from '../../components/common/PageContainer.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import MonoValue from '../../components/common/MonoValue.vue'
import EmptyState from '../../components/common/EmptyState.vue'

interface ChildKey {
  id: string
  note?: string
  keyMasked?: string
  totalBalance?: number
  requests?: number
  recentCalls7d?: number
}
interface Summary {
  totalBalance?: number
  childCount?: number
  maxChildKeys?: number
  soldToChildren?: number
  totalRecharged?: number
  childTotalBalance?: number
  childTotalRequests?: number
  childTotalCredits?: number
}

const summary = ref<Summary | null>(null)
const keys = ref<ChildKey[]>([])
const loading = ref(true)

async function load() {
  try {
    const [s, k] = await Promise.all([
      userApi('/reseller/summary'),
      userApi('/reseller/keys'),
    ])
    summary.value = s
    keys.value = k.keys || []
  } catch (e) { /* silent */ }
  loading.value = false
}

onMounted(load)

const { toCny } = useSystemUnit()
const cny = (usd: number) => toCny(usd).toFixed(2)
const balanceClass = computed(() => (summary.value?.totalBalance ?? 0) < 1 ? 'bad' : 'good')

interface TopRow { note: string; keyMasked: string; balance: string; recent7d: string }
const topRows = computed<TopRow[]>(() => {
  return [...keys.value]
    .sort((a, b) => (b.recentCalls7d || 0) - (a.recentCalls7d || 0))
    .slice(0, 6)
    .map(k => ({
      note: k.note || k.id.slice(0, 8),
      keyMasked: k.keyMasked || '-',
      balance: `$${(k.totalBalance || 0).toFixed(2)}`,
      recent7d: (k.recentCalls7d || 0).toLocaleString(),
    }))
})

const columns: DataTableColumns<TopRow> = [
  { title: '备注', key: 'note', width: 240, ellipsis: { tooltip: true }, render: r => r.note },
  { title: 'Key', key: 'keyMasked', width: 200, render: r => h(MonoValue, { value: r.keyMasked }) },
  { title: '余额', key: 'balance', width: 120, align: 'center', render: r => h('span', { class: 'mono' }, r.balance) },
  { title: '近 7 天', key: 'recent7d', width: 120, align: 'center', render: r => h('span', { class: 'mono' }, r.recent7d) },
]
</script>

<template>
  <PageContainer>
    <PageHeader kicker="代理商 · 概览" :kicker-dot="'#707070'" title="代理总览" desc="子 Key 管理与销售汇总" />

    <n-spin v-if="loading" />

    <template v-else>
      <section class="hero-grid">
        <div class="card">
          <span class="card__label">我的余额</span>
          <span class="card__value mono" :class="balanceClass">${{ (summary?.totalBalance ?? 0).toFixed(2) }}</span>
          <span class="card__sub">折合 ¥{{ cny(summary?.totalBalance ?? 0) }}</span>
        </div>
        <div class="card">
          <span class="card__label">子 Key 数量</span>
          <span class="card__value mono">{{ summary?.childCount ?? 0 }}</span>
          <span class="card__sub">{{ summary?.maxChildKeys ? `上限 ${summary.maxChildKeys}` : '无上限' }}</span>
        </div>
        <div class="card">
          <span class="card__label">累计已转出</span>
          <span class="card__value mono">${{ (summary?.soldToChildren ?? 0).toFixed(2) }}</span>
          <span class="card__sub">折合 ¥{{ cny(summary?.soldToChildren ?? 0) }}</span>
        </div>
        <div class="card">
          <span class="card__label">累计进货</span>
          <span class="card__value mono">${{ (summary?.totalRecharged ?? 0).toFixed(2) }}</span>
          <span class="card__sub">折合 ¥{{ cny(summary?.totalRecharged ?? 0) }}</span>
        </div>
      </section>

      <section class="panel">
        <h3 class="section-title">子 Key 活跃排行（近 7 天）</h3>
        <n-data-table
          v-if="topRows.length"
          :columns="columns"
          :data="topRows"
          :row-key="r => r.keyMasked"
          :scroll-x="680"
          size="small"
          striped
        />
        <EmptyState v-else icon="○" title="暂无子 Key" desc="前往「子 Key 管理」创建" />
      </section>

      <section class="panel">
        <h3 class="section-title">子 Key 汇总</h3>
        <dl class="kv">
          <div class="kv__row"><dt>子 Key 总余额</dt><dd class="mono">${{ (summary?.childTotalBalance ?? 0).toFixed(2) }}</dd></div>
          <div class="kv__row"><dt>子 Key 总请求数</dt><dd class="mono">{{ (summary?.childTotalRequests ?? 0).toLocaleString() }}</dd></div>
          <div class="kv__row"><dt>子 Key 总消耗 Credits</dt><dd class="mono">{{ (summary?.childTotalCredits ?? 0).toFixed(2) }}</dd></div>
        </dl>
      </section>
    </template>
  </PageContainer>
</template>

<style scoped>
.hero-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}
.card { display: flex; flex-direction: column; gap: 6px; padding: 16px; border: 1px solid rgba(255,255,255,0.06); border-radius: 6px; background: #0a0a0a; }
.card__label { font-size: 11px; color: #707070; text-transform: uppercase; letter-spacing: 0.06em; }
.card__value { font-size: 24px; font-weight: 600; color: #ededed; }
.card__sub { font-size: 11px; color: #707070; }
.mono { font-family: "Geist Mono", ui-monospace, monospace; }
.mono.good { color: #0bd470; }
.mono.bad { color: #ff7a7a; }

.panel { padding: 16px; border: 1px solid rgba(255,255,255,0.06); border-radius: 6px; background: #0a0a0a; margin-bottom: 16px; }
.section-title { margin: 0 0 12px; color: #ededed; font-size: 14px; font-weight: 600; }

.kv { display: flex; flex-direction: column; gap: 8px; margin: 0; }
.kv__row { display: grid; grid-template-columns: 180px 1fr; font-size: 13px; }
.kv__row dt { color: #707070; }
.kv__row dd { color: #ededed; margin: 0; }

@media (max-width: 900px) {
  .hero-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
