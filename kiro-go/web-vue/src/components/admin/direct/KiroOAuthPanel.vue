<script setup lang="ts">
import { NSelect, NButton, NTag } from 'naive-ui'
import { ExternalLink, Copy, RefreshCw } from 'lucide-vue-next'
import RefinedField from '../../common/RefinedField.vue'
import type { KiroOAuthState } from './useKiroDeviceCode'

defineProps<{
  state: KiroOAuthState
}>()
const emit = defineEmits<{
  (e: 'start'): void
  (e: 'cancel'): void
  (e: 'copy-code'): void
  (e: 'update:region', v: string): void
}>()

const regionOptions = [
  { label: 'us-east-1', value: 'us-east-1' },
  { label: 'us-west-2', value: 'us-west-2' },
  { label: 'eu-central-1', value: 'eu-central-1' },
  { label: 'ap-southeast-1', value: 'ap-southeast-1' },
]
</script>

<template>
  <div class="kop">
    <p class="kop__intro">
      使用 AWS Builder ID 设备码流程：点击下方按钮 → 浏览器新窗口打开 Amazon 登录页 → 在该页输入下方 user code → 完成后此处自动到账。
    </p>

    <RefinedField label="区域" hint="选与 Kiro 套餐绑定的 AWS region">
      <n-select
        :value="state.region"
        :options="regionOptions"
        :disabled="state.status !== 'idle'"
        @update:value="(v: string) => emit('update:region', v)"
      />
    </RefinedField>

    <RefinedField label="" hint="点击后会新开浏览器窗口完成登录">
      <n-button
        type="primary"
        :loading="state.starting"
        :disabled="state.status === 'awaiting' || state.status === 'success'"
        @click="emit('start')"
      >
        开始 OAuth 登录
      </n-button>
    </RefinedField>

    <section v-if="state.status === 'awaiting'" class="kop__box">
      <header class="kop__head">
        <n-tag size="small" :bordered="false" type="info">
          <RefreshCw :size="11" :class="{ 'kop-spin': state.polling }" style="vertical-align: -2px; margin-right: 4px" />
          等待你完成登录…
        </n-tag>
        <n-button quaternary size="tiny" @click="emit('cancel')">取消</n-button>
      </header>

      <div class="kop__step">
        <span class="kop__step-num">1</span>
        <div>
          <a :href="state.verificationUri" target="_blank" rel="noopener noreferrer" class="kop__link">
            {{ state.verificationUri }}
            <ExternalLink :size="12" />
          </a>
          <span class="kop__step-hint">（新窗口会自动打开，没打开就点这个）</span>
        </div>
      </div>

      <div class="kop__step">
        <span class="kop__step-num">2</span>
        <div>
          在打开的页面输入 user code：
          <div class="kop__code">
            <span class="kop__code-value">{{ state.userCode }}</span>
            <n-button quaternary size="tiny" @click="emit('copy-code')">
              <template #icon><Copy :size="13" /></template>
              复制
            </n-button>
          </div>
        </div>
      </div>

      <div class="kop__step">
        <span class="kop__step-num">3</span>
        <div>登录授权后回到这里即可，无需手动操作</div>
      </div>
    </section>

    <section v-else-if="state.status === 'success'" class="kop__box kop__box--ok">
      <div>✓ 已添加账号 <strong>{{ state.resultEmail || '(email pending)' }}</strong>，正在异步拉取套餐信息…</div>
    </section>

    <section v-else-if="state.status === 'error'" class="kop__box kop__box--err">
      <div>✕ {{ state.errorMsg }}</div>
    </section>
  </div>
</template>

<style scoped>
.kop { display: flex; flex-direction: column; }
.kop__intro { color: #a1a1a1; font-size: 13px; line-height: 1.6; margin: 0 0 16px; }

.kop__box {
  margin-top: 16px;
  padding: 16px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 4px;
}
.kop__box--ok { border-color: rgba(11, 212, 112, 0.30); color: #0bd470; font-size: 13px; }
.kop__box--err { border-color: rgba(255, 77, 77, 0.30); color: #ff7a7a; font-size: 13px; }

.kop__head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }

.kop__step {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 0;
  color: #a1a1a1;
  font-size: 13px;
  line-height: 1.55;
}
.kop__step-num {
  flex-shrink: 0;
  width: 20px; height: 20px;
  display: flex; align-items: center; justify-content: center;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 50%;
  color: #ededed; font-size: 11px; font-weight: 600;
}
.kop__step-hint { color: #707070; font-size: 11px; margin-left: 4px; }

.kop__link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #52a8ff;
  text-decoration: none;
  font-size: 13px;
  word-break: break-all;
}
.kop__link:hover { text-decoration: underline; }

.kop__code {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  padding: 8px 12px;
  background: rgba(82, 168, 255, 0.08);
  border: 1px solid rgba(82, 168, 255, 0.30);
  border-radius: 4px;
}
.kop__code-value {
  font-family: var(--st-font-mono, "Geist Mono", ui-monospace, monospace);
  font-size: 18px;
  font-weight: 600;
  color: #ededed;
  letter-spacing: 0.06em;
  user-select: all;
}

.kop-spin { animation: kop-spin 1.2s linear infinite; }
@keyframes kop-spin { to { transform: rotate(360deg); } }
</style>
