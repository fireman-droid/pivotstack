<script setup>
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/admin'
import { useToast } from '../composables/useToast'
import { Beaker, Save, AlertTriangle, Plus, Trash2 } from 'lucide-vue-next'
import WorldCard from '../components/world/WorldCard.vue'
import WorldButton from '../components/world/WorldButton.vue'
import WorldInput from '../components/world/WorldInput.vue'
import WorldChip from '../components/world/WorldChip.vue'
import Switch from '../components/ui/Switch.vue'

const { success, error } = useToast()
const cfg = ref({
  enabled: false,
  rules: [],
})
const loading = ref(false)

async function load() {
  try {
    const res = await api('/stealth')
    const data = await res.json()
    cfg.value = {
      enabled: !!data.enabled,
      rules: Array.isArray(data.rules) ? data.rules.map(r => ({
        sourcePattern: r.sourcePattern || '',
        target: r.target || 'claude-sonnet-4.6',
        ratio: typeof r.ratio === 'number' ? r.ratio : 0,
        note: r.note || '',
      })) : [],
    }
    // 如果后端返回的 rules 是空但 legacy 字段非空，前端再做一次兜底迁移
    if (cfg.value.rules.length === 0 && (data.opusFakeRatio > 0 || data.sonnetFakeRatio > 0)) {
      if (data.opusFakeRatio > 0 || data.opusFakeTarget) {
        cfg.value.rules.push({
          sourcePattern: 'opus-4.6',
          target: data.opusFakeTarget || 'claude-sonnet-4.6',
          ratio: data.opusFakeRatio || 0,
          note: '(legacy)',
        })
      }
      if (data.sonnetFakeRatio > 0 || data.sonnetFakeTarget) {
        cfg.value.rules.push({
          sourcePattern: 'sonnet-4.6',
          target: data.sonnetFakeTarget || 'claude-sonnet-4.5',
          ratio: data.sonnetFakeRatio || 0,
          note: '(legacy)',
        })
      }
    }
  } catch { /* default */ }
}

async function save() {
  loading.value = true
  try {
    await api('/stealth', { method: 'PUT', body: JSON.stringify(cfg.value) })
    success('配置已保存')
    load()
  } catch { error('保存失败') }
  loading.value = false
}

function addRule(preset) {
  const presets = {
    opus_46:  { sourcePattern: 'opus-4.6',   target: 'claude-sonnet-4.6', ratio: 0.50, note: '' },
    opus_47:  { sourcePattern: 'opus-4.7',   target: 'claude-sonnet-4.6', ratio: 0.50, note: '' },
    sonnet_46:{ sourcePattern: 'sonnet-4.6', target: 'claude-sonnet-4.5', ratio: 0.50, note: '' },
    blank:    { sourcePattern: '',           target: 'claude-sonnet-4.6', ratio: 0,    note: '' },
  }
  cfg.value.rules.push({ ...(presets[preset] || presets.blank) })
}

function removeRule(idx) {
  cfg.value.rules.splice(idx, 1)
}

function setRatio(rule, percentStr) {
  const v = parseFloat(percentStr) / 100
  rule.ratio = Math.max(0, Math.min(1, isFinite(v) ? v : 0))
}

const ruleCount = computed(() => cfg.value.rules.length)
const activeRuleCount = computed(() => cfg.value.rules.filter(r => r.ratio > 0 && r.sourcePattern).length)

onMounted(load)
</script>

<template>
  <div class="stealth-page">
    <header class="page-head">
      <div class="title-wrap">
        <div class="eyebrow">实验性配置</div>
        <h1 class="page-title"><Beaker :size="22" /> 请求分流（掺水）</h1>
      </div>
      <WorldChip :variant="cfg.enabled ? 'success' : 'neutral'" :dot="true">
        {{ cfg.enabled ? `已启用 · ${activeRuleCount}/${ruleCount} 条生效` : '未启用' }}
      </WorldChip>
    </header>

    <WorldCard padding="lg">
      <div class="row primary-row">
        <div class="row-text">
          <div class="row-title">总开关</div>
          <div class="row-hint">关闭后所有规则失效，不做任何替换</div>
        </div>
        <Switch v-model="cfg.enabled" size="lg" />
      </div>
    </WorldCard>

    <!-- 规则列表 -->
    <WorldCard padding="md">
      <header class="section-head">
        <div>
          <h3>掺水规则</h3>
          <p class="section-hint">
            按顺序匹配用户**原始**请求模型（在归一化之前）。命中第一条 → 按 Ratio 概率替换为 Target。
            <br>
            <strong>SourcePattern</strong> 用小写子串匹配，自动兼容 <code>-/.</code> 互换（4.7 ↔ 4-7）。
            一条用户请求只命中第一条规则。
          </p>
        </div>
        <div class="add-buttons">
          <WorldButton variant="ghost" size="sm" @click="addRule('opus_47')">
            <Plus :size="13" /><span>+ opus-4.7 规则</span>
          </WorldButton>
          <WorldButton variant="ghost" size="sm" @click="addRule('opus_46')">
            <Plus :size="13" /><span>+ opus-4.6</span>
          </WorldButton>
          <WorldButton variant="ghost" size="sm" @click="addRule('sonnet_46')">
            <Plus :size="13" /><span>+ sonnet-4.6</span>
          </WorldButton>
          <WorldButton variant="ghost" size="sm" @click="addRule('blank')">
            <Plus :size="13" /><span>+ 空规则</span>
          </WorldButton>
        </div>
      </header>

      <div class="rules-list">
        <div v-for="(r, i) in cfg.rules" :key="i" class="rule-row">
          <div class="rule-num">{{ i + 1 }}</div>

          <div class="rule-fields">
            <div class="field">
              <label class="field-label">原始模型 SourcePattern</label>
              <WorldInput v-model="r.sourcePattern" placeholder="如 opus-4.7" :monospace="true" size="sm" />
            </div>
            <div class="field arrow-field">→</div>
            <div class="field">
              <label class="field-label">替换为 Target</label>
              <WorldInput v-model="r.target" placeholder="claude-sonnet-4.6" :monospace="true" size="sm" />
            </div>
            <div class="field ratio-field">
              <label class="field-label">概率</label>
              <div class="ratio-inline">
                <input
                  type="range" min="0" max="100" step="1"
                  :value="Math.round(r.ratio * 100)"
                  @input="e => setRatio(r, e.target.value)"
                  :disabled="!cfg.enabled"
                />
                <div class="ratio-value">{{ Math.round(r.ratio * 100) }}%</div>
              </div>
            </div>
          </div>

          <button type="button" class="del-btn" @click="removeRule(i)" aria-label="删除规则">
            <Trash2 :size="14" />
          </button>
        </div>

        <div v-if="!cfg.rules.length" class="empty-state">
          <p>暂无规则。从右上角按钮快速添加 opus-4.6 / opus-4.7 / sonnet-4.6 等常用规则。</p>
        </div>
      </div>
    </WorldCard>

    <div class="warning-row" v-if="cfg.enabled && activeRuleCount > 0">
      <AlertTriangle :size="14" />
      <span>开启后用户请求会按规则替换为别的模型。计费按 stealth 替换后的实际模型走（结合 PRO/FREE 池价 × 模型倍率）。</span>
    </div>

    <div class="save-row">
      <WorldButton variant="primary" :loading="loading" @click="save">
        <Save :size="14" /><span>保存配置</span>
      </WorldButton>
    </div>
  </div>
</template>

<style scoped>
.stealth-page { display: flex; flex-direction: column; gap: 18px; max-width: 1100px; }

.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
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
  padding: 4px 0;
}
.row-text { flex: 1; min-width: 0; }
.row-title { font-size: 0.95rem; font-weight: 800; color: var(--world-text-primary); margin-bottom: 2px; }
.row-hint  { font-size: 0.8125rem; color: var(--world-text-mute); }

.section-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.section-head h3 { margin: 0; font-size: 1rem; font-weight: 800; color: var(--world-text-primary); font-family: var(--world-font-display); }
.section-hint { font-size: 0.78rem; color: var(--world-text-mute); margin: 4px 0 0; line-height: 1.5; }
.add-buttons { display: flex; gap: 6px; flex-wrap: wrap; }

.rules-list { display: flex; flex-direction: column; gap: 10px; }
.rule-row {
  display: flex;
  align-items: stretch;
  gap: 10px;
  padding: 12px;
  background: var(--world-overlay-light);
  border: 1px solid var(--world-glass-border);
  border-radius: var(--world-radius-md);
}
.rule-num {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--world-accent);
  color: white;
  font-weight: 800;
  font-size: 0.75rem;
  border-radius: var(--world-radius-sm);
  flex-shrink: 0;
}
.rule-fields {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr auto 1fr 1.4fr;
  gap: 10px;
  align-items: end;
}
@media (max-width: 880px) {
  .rule-fields { grid-template-columns: 1fr; }
  .arrow-field { display: none; }
}
.field { display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 0.7rem; font-weight: 700; color: var(--world-text-mute); }
.arrow-field {
  align-items: center;
  justify-content: center;
  font-size: 1.2rem;
  color: var(--world-text-mute);
  padding-bottom: 4px;
}
.ratio-inline { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.ratio-inline input[type="range"] { flex: 1; accent-color: var(--world-accent); cursor: pointer; }
.ratio-inline input[type="range"]:disabled { opacity: 0.5; cursor: not-allowed; }
.ratio-value {
  width: 44px;
  text-align: right;
  font-family: var(--world-font-mono);
  font-weight: 800;
  font-size: 0.875rem;
  color: var(--world-accent);
}

.del-btn {
  align-self: center;
  width: 30px; height: 30px;
  background: transparent;
  border: 1px solid transparent;
  color: var(--world-text-mute);
  cursor: pointer;
  border-radius: var(--world-radius-sm);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 200ms ease;
  flex-shrink: 0;
}
.del-btn:hover {
  color: var(--world-error);
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.3);
}

.empty-state {
  text-align: center;
  padding: 28px 16px;
  color: var(--world-text-dim);
  font-size: 0.875rem;
}

.warning-row {
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

.save-row { display: flex; justify-content: flex-end; }
</style>
