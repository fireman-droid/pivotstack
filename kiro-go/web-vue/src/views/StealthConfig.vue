<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Beaker, Save, Sliders, AlertTriangle } from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldChip from '../components/world/WorldChip.vue'
import Switch from '../components/ui/Switch.vue'

const { success, error } = useToast()
const cfg = ref({
  enabled: false,
  opusFakeRatio: 0.8,
  sonnetFakeRatio: 0.5,
  opusFakeTarget: 'claude-sonnet-4.6',
  sonnetFakeTarget: 'claude-sonnet-4.5',
})
const loading = ref(false)
const advanced = ref(false)

async function load() {
  try {
    const res = await api('/stealth')
    const data = await res.json()
    Object.assign(cfg.value, data)
  } catch { /* default */ }
}

async function save() {
  loading.value = true
  try {
    await api('/stealth', { method: 'PUT', body: JSON.stringify(cfg.value) })
    success('配置已保存')
  } catch { error('保存失败') }
  loading.value = false
}

function clamp(n, min, max) { return Math.max(min, Math.min(max, n)) }
function setOpusRatio(v) { cfg.value.opusFakeRatio = clamp(parseFloat(v) / 100 || 0, 0, 1) }
function setSonnetRatio(v) { cfg.value.sonnetFakeRatio = clamp(parseFloat(v) / 100 || 0, 0, 1) }

onMounted(load)
</script>

<template>
  <div class="stealth-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">实验性配置</div>
        <h1 class="page-title"><Beaker :size="22" /> 请求分流</h1>
      </div>
      <WorldChip :variant="cfg.enabled ? 'success' : 'neutral'" :dot="true">
        {{ cfg.enabled ? '已启用' : '未启用' }}
      </WorldChip>
    </header>

    <WorldCard padding="lg">
      <!-- 总开关 -->
      <div class="row primary-row">
        <div class="row-text">
          <div class="row-title">总开关</div>
          <div class="row-hint">控制本服务的请求分流策略是否生效</div>
        </div>
        <Switch v-model="cfg.enabled" size="lg" />
      </div>

      <hr class="divider" />

      <!-- Opus 比例 -->
      <div class="row">
        <div class="row-text">
          <div class="row-title">Opus 分流比例</div>
          <div class="row-hint">claude-opus-4.6 请求中替换为目标的占比</div>
        </div>
        <div class="ratio-control">
          <input
            type="range" min="0" max="100" step="1"
            :value="Math.round(cfg.opusFakeRatio * 100)"
            @input="e => setOpusRatio(e.target.value)"
            :disabled="!cfg.enabled"
          />
          <div class="ratio-value">{{ Math.round(cfg.opusFakeRatio * 100) }}%</div>
        </div>
      </div>

      <!-- Sonnet 比例 -->
      <div class="row">
        <div class="row-text">
          <div class="row-title">Sonnet 分流比例</div>
          <div class="row-hint">claude-sonnet-4.6 请求中替换为目标的占比</div>
        </div>
        <div class="ratio-control">
          <input
            type="range" min="0" max="100" step="1"
            :value="Math.round(cfg.sonnetFakeRatio * 100)"
            @input="e => setSonnetRatio(e.target.value)"
            :disabled="!cfg.enabled"
          />
          <div class="ratio-value">{{ Math.round(cfg.sonnetFakeRatio * 100) }}%</div>
        </div>
      </div>

      <!-- 高级选项 -->
      <button type="button" class="advanced-toggle" @click="advanced = !advanced">
        <Sliders :size="14" />
        <span>{{ advanced ? '收起高级选项' : '展开高级选项' }}</span>
      </button>

      <Transition name="fade-slide">
        <div v-if="advanced" class="advanced-panel">
          <WorldInput
            v-model="cfg.opusFakeTarget"
            label="Opus 替换目标"
            :monospace="true"
            placeholder="claude-sonnet-4.6"
          />
          <WorldInput
            v-model="cfg.sonnetFakeTarget"
            label="Sonnet 替换目标"
            :monospace="true"
            placeholder="claude-sonnet-4.5"
          />
        </div>
      </Transition>

      <div class="warning-row" v-if="cfg.enabled">
        <AlertTriangle :size="14" />
        <span>开启后调用上游账号会按比例分流到不同模型。请确保替换目标正确。</span>
      </div>

      <div class="save-row">
        <WorldButton variant="primary" :loading="loading" @click="save">
          <Save :size="14" /><span>保存配置</span>
        </WorldButton>
      </div>
    </WorldCard>
  </div>
</template>

<style scoped>
.stealth-page { display: flex; flex-direction: column; gap: 18px; max-width: 760px; }

.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; }
.title-wrap { display: flex; flex-direction: column; gap: 2px; }
.eyebrow {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--world-text-mute);
}
.page-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-family: var(--world-font-display);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0;
  color: var(--world-text-primary);
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 14px 0;
}
.primary-row { padding-top: 4px; }
.row-text { flex: 1; min-width: 0; }
.row-title { font-size: 0.95rem; font-weight: 800; color: var(--world-text-primary); margin-bottom: 2px; }
.row-hint  { font-size: 0.8125rem; color: var(--world-text-mute); }

.divider { border: none; border-top: 1px solid var(--world-divider); margin: 4px 0; }

.ratio-control {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.ratio-control input[type="range"] {
  width: 200px;
  accent-color: var(--world-accent);
  cursor: pointer;
}
.ratio-control input[type="range"]:disabled { opacity: 0.5; cursor: not-allowed; }
.ratio-value {
  width: 50px;
  text-align: right;
  font-family: var(--world-font-mono);
  font-weight: 800;
  font-size: 0.95rem;
  color: var(--world-accent);
}

.advanced-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding: 6px 10px;
  background: transparent;
  border: 1px dashed var(--world-glass-border);
  border-radius: var(--world-radius-md);
  color: var(--world-text-mute);
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 200ms;
}
.advanced-toggle:hover { color: var(--world-text-primary); border-color: var(--world-accent); }

.advanced-panel {
  margin-top: 14px;
  padding: 16px;
  background: var(--world-overlay-light);
  border-radius: var(--world-radius-md);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.warning-row {
  margin-top: 14px;
  padding: 10px 12px;
  background: rgba(245, 158, 11, 0.10);
  border: 1px solid rgba(245, 158, 11, 0.25);
  border-radius: var(--world-radius-md);
  color: var(--world-warning);
  font-size: 0.8125rem;
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.save-row { margin-top: 16px; display: flex; justify-content: flex-end; }

.fade-slide-enter-active, .fade-slide-leave-active { transition: all 280ms cubic-bezier(0.16, 1, 0.3, 1); }
.fade-slide-enter-from, .fade-slide-leave-to { opacity: 0; transform: translateY(-8px); }

@media (max-width: 640px) {
  .row { flex-direction: column; align-items: stretch; gap: 8px; }
  .ratio-control { justify-content: space-between; }
  .ratio-control input[type="range"] { flex: 1; width: auto; }
}
</style>
