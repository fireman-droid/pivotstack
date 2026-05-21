<script setup lang="ts">
import { ChevronRight } from 'lucide-vue-next'
import StatusDot from '../stellar/StatusDot.vue'
import Tile from '../stellar/Tile.vue'

export interface PrefChannel {
  id: string
  alias: string
  enabled: boolean
  billing?: string
}
export interface PrefGroup {
  id: string
  name: string
  description?: string
  defaultRuntimeChannelId?: string
  channels: PrefChannel[]
}
export interface PrefSeries {
  id: string
  name: string
  defaultChannelId?: string
  channels: PrefChannel[]
}

const props = defineProps<{
  groups: PrefGroup[]
  series: PrefSeries[]
  preferences: Record<string, string>
  savingKey?: string
}>()

const emit = defineEmits<{
  (e: 'select', payload: { key: string; channelId: string }): void
}>()

function currentId(rowId: string, fallback?: string) {
  return props.preferences[rowId] || fallback || ''
}
function useGroups() { return props.groups.length > 0 }

function onChipClick(key: string, channelId: string) {
  if (props.savingKey === key) return
  emit('select', { key, channelId })
}
</script>

<template>
  <Tile>
    <div class="tile__head tile__head--split">
      <div>
        <div class="t-display">分组路由</div>
        <div class="t-label tertiary">PREFERENCES · 配置走哪个上游</div>
      </div>
    </div>

    <template v-if="useGroups()">
      <template v-for="(g, i) in groups" :key="g.id">
        <div class="route-row">
          <div class="route-row__main">
            <div class="t-body-strong">{{ g.name }}</div>
            <div class="t-label tertiary">{{ g.description || `${(g.channels || []).length} 渠道可选` }}</div>
          </div>
          <div class="route-row__chips">
            <button
              v-for="ch in (g.channels || [])"
              :key="ch.id"
              class="chip"
              :class="currentId(g.id, g.defaultRuntimeChannelId) === ch.id ? 'chip--selected' : 'chip--mono'"
              :disabled="!ch.enabled || savingKey === g.id"
              @click="onChipClick(g.id, ch.id)"
            >
              <StatusDot v-if="currentId(g.id, g.defaultRuntimeChannelId) === ch.id" status="ok" />
              {{ ch.alias }}
            </button>
          </div>
          <ChevronRight :size="14" class="route-row__chev" />
        </div>
        <div v-if="i < groups.length - 1" class="hairline"></div>
      </template>
    </template>

    <template v-else-if="series.length">
      <template v-for="(s, i) in series" :key="s.id">
        <div class="route-row">
          <div class="route-row__main">
            <div class="t-body-strong">{{ s.name }}</div>
            <div class="t-label tertiary">{{ (s.channels || []).length }} 渠道可选</div>
          </div>
          <div class="route-row__chips">
            <button
              v-for="ch in (s.channels || [])"
              :key="ch.id"
              class="chip"
              :class="currentId(s.id, s.defaultChannelId) === ch.id ? 'chip--selected' : 'chip--mono'"
              :disabled="!ch.enabled || savingKey === s.id"
              @click="onChipClick(s.id, ch.id)"
            >
              <StatusDot v-if="currentId(s.id, s.defaultChannelId) === ch.id" status="ok" />
              {{ ch.alias }}
            </button>
          </div>
          <ChevronRight :size="14" class="route-row__chev" />
        </div>
        <div v-if="i < series.length - 1" class="hairline"></div>
      </template>
    </template>

    <div v-else class="t-label tertiary" style="padding: 12px 0">尚未开放渠道选择 · 管理员还没有为该 Key 配置可选渠道</div>
  </Tile>
</template>

<style scoped>
.route-row {
  display: flex; align-items: center; gap: 16px;
  min-height: 64px; padding: 8px;
  cursor: default;
  border-radius: 4px;
  transition: background 150ms ease;
}
.route-row:hover { background: rgba(255,255,255,0.02); }
.route-row__main { width: 200px; flex-shrink: 0; }
.route-row__chips { flex: 1; display: flex; flex-wrap: wrap; gap: 6px; }
.route-row__chips .chip {
  cursor: pointer;
  border: none;
  font-family: inherit;
}
.route-row__chips .chip:disabled { cursor: not-allowed; opacity: 0.5; }
.route-row__chev { color: var(--st-text-ter); transition: color 150ms ease, transform 150ms ease; flex-shrink: 0; }
.route-row:hover .route-row__chev { color: var(--st-text-pri); transform: translateX(2px); }
</style>
