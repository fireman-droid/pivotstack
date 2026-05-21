<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useUserAuth } from '../../stores/userAuth'
import { userApi } from '../../api/user'
import { useMessage } from 'naive-ui'
import { MessageCircle, CircleDollarSign, Ticket, Check } from 'lucide-vue-next'
import Tile from '../../components/user/stellar/Tile.vue'
import CounterNumber from '../../components/user/stellar/CounterNumber.vue'
import StatusDot from '../../components/user/stellar/StatusDot.vue'
import { useSystemUnit } from '../../composables/useSystemUnit'

interface RechargeRecord {
  time?: string
  timestamp?: number
  type?: string
  amountUsd?: number
  amountCny?: number
  balanceBefore?: number
  balanceAfter?: number
  giftBefore?: number
  giftAfter?: number
  code?: string
  note?: string
}

const auth = useUserAuth() as any
const message = useMessage()
const { toCny, dollarsPerYuan } = useSystemUnit()

// 当前余额
const balance = computed(() => Number(auth.userInfo?.balance || 0))
const giftBalance = computed(() => Number(auth.userInfo?.giftBalance || 0))
const totalBalance = computed(() => balance.value + giftBalance.value)
const cny = computed(() => toCny(totalBalance.value).toFixed(2))

// 金额 chip
const preset = [10, 50, 100, 500]
const amount = ref<number>(50)
const customAmount = ref<string>('50')

function pickAmount(n: number) {
  amount.value = n
  customAmount.value = String(n)
}
function onAmountInput(v: string) {
  customAmount.value = v
  const n = parseFloat(v) || 0
  if (n > 0) amount.value = n
}

// 套餐定价（USD 给量；UI 上 1¥ = N 虚拟$ 经 system unit 换算）
// 业务约定：充 ¥10/50/100/500 → 赠送 0/4%/8%/14%
function bonusFor(yuan: number) {
  if (yuan >= 500) return 1.14
  if (yuan >= 100) return 1.08
  if (yuan >= 50) return 1.04
  return 1.0
}
const gotUsd = computed(() => (amount.value * dollarsPerYuan.value * bonusFor(amount.value)).toFixed(2))

// 支付方式
const payMethod = ref<'wechat' | 'alipay' | 'card'>('wechat')

// 兑换码
const code = ref('')
const redeeming = ref(false)
async function doRedeem() {
  const c = code.value.trim()
  if (!c) { message.warning('请输入兑换码'); return }
  redeeming.value = true
  try {
    await userApi('/redeem', { method: 'POST', body: { code: c } })
    message.success('兑换成功')
    code.value = ''
    auth.refresh()
    fetchRecords()
  } catch (e: any) {
    message.error(e?.message || '兑换失败')
  } finally {
    redeeming.value = false
  }
}

// 主 CTA
function doRecharge() {
  message.info(`线上支付 ¥${amount.value} 即将上线。当前请用兑换码或联系客服充值。`)
}

// 充值历史
const records = ref<RechargeRecord[]>([])
const recordsLoading = ref(false)
async function fetchRecords() {
  recordsLoading.value = true
  try {
    const data = await userApi('/recharges?page=1&limit=100')
    records.value = data.records || []
  } catch {
    /* 静默 */
  } finally {
    recordsLoading.value = false
  }
}

function deltaUsd(r: RechargeRecord): number {
  if (r.amountUsd != null) return r.amountUsd
  return ((r.balanceAfter ?? 0) - (r.balanceBefore ?? 0)) + ((r.giftAfter ?? 0) - (r.giftBefore ?? 0))
}
function fmtDate(r: RechargeRecord) {
  const t = r.timestamp ? new Date(r.timestamp * 1000) : (r.time ? new Date(r.time) : null)
  if (!t) return '-'
  return `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, '0')}-${String(t.getDate()).padStart(2, '0')}`
}
function typeLabel(t?: string) {
  const map: Record<string, string> = {
    code_redeem: '兑换码',
    code_redeem_days: '时长卡',
    admin_balance: '管理员',
    admin_gift: '赠送',
    admin_adjust: '调整',
  }
  return map[t || ''] || t || '-'
}

onMounted(() => {
  fetchRecords()
  auth.refresh()
})
</script>

<template>
  <div class="recharge stellar-scope">
    <div class="grid-3 grid-3--recharge">
      <!-- 左：充值入口 -->
      <div class="grid-col">
        <Tile>
          <div class="tile__head"><span class="t-display">充值</span></div>
          <span class="t-label">选择金额</span>
          <div class="amount-chips">
            <button
              v-for="n in preset"
              :key="n"
              class="chip chip--amount"
              :class="{ 'is-selected': amount === n }"
              @click="pickAmount(n)"
            >
              ¥{{ n }}
            </button>
          </div>
          <div class="st-input st-input--lg">
            <span class="st-input__prefix">¥</span>
            <input
              type="number"
              :value="customAmount"
              class="mono"
              @input="(e) => onAmountInput((e.target as HTMLInputElement).value)"
            />
            <span class="st-input__suffix t-label tertiary">= ${{ gotUsd }} 虚拟$</span>
          </div>
        </Tile>

        <Tile>
          <div class="tile__head"><span class="t-label">支付方式</span></div>
          <div class="pay-grid">
            <button
              class="pay"
              :class="{ 'is-selected': payMethod === 'wechat' }"
              @click="payMethod = 'wechat'"
            >
              <MessageCircle :size="18" />
              <div class="t-body-strong">微信支付</div>
              <div class="t-label tertiary">推荐</div>
              <span v-if="payMethod === 'wechat'" class="pay__corner"><Check :size="10" /></span>
            </button>
            <button
              class="pay"
              :class="{ 'is-selected': payMethod === 'alipay' }"
              @click="payMethod = 'alipay'"
            >
              <CircleDollarSign :size="18" />
              <div class="t-body-strong">支付宝</div>
              <div class="t-label tertiary">即时到账</div>
              <span v-if="payMethod === 'alipay'" class="pay__corner"><Check :size="10" /></span>
            </button>
            <button
              class="pay"
              :class="{ 'is-selected': payMethod === 'card' }"
              @click="payMethod = 'card'"
            >
              <Ticket :size="18" />
              <div class="t-body-strong">卡券码</div>
              <div class="t-label tertiary">兑换专用</div>
              <span v-if="payMethod === 'card'" class="pay__corner"><Check :size="10" /></span>
            </button>
          </div>
        </Tile>

        <Tile>
          <div class="tile__head"><span class="t-label">或使用兑换码</span></div>
          <div class="redeem">
            <div class="st-input">
              <input
                v-model="code"
                class="mono"
                placeholder="STELLAR-XXXX-XXXX-XXXX"
                @keyup.enter="doRedeem"
              />
            </div>
            <button class="btn btn--secondary" :disabled="redeeming" @click="doRedeem">
              {{ redeeming ? '兑换中...' : '兑换' }}
            </button>
          </div>
        </Tile>

        <button class="btn btn--primary btn--block btn--lg" @click="doRecharge">
          确认充值 ¥{{ amount }} →
        </button>
      </div>

      <!-- 中：当前余额 + 套餐 -->
      <div class="grid-col">
        <Tile>
          <div class="tile__head"><span class="t-label">CURRENT BALANCE</span></div>
          <div class="hero-num">
            <CounterNumber :value="totalBalance" prefix="$" :decimals="2" class="t-hero-lg mono" />
          </div>
          <div class="t-body sub">≈ ¥{{ cny }}</div>
          <div class="hairline"></div>
          <div class="t-label tertiary">充值 ${{ balance.toFixed(2) }} · 赠送 ${{ giftBalance.toFixed(2) }}</div>
        </Tile>

        <Tile>
          <div class="tile__head"><span class="t-display">套餐</span></div>
          <div class="pkg">
            <div class="pkg__row">
              <span class="pkg__amt mono">¥10</span>
              <span class="pkg__arrow">→</span>
              <span class="pkg__got mono">${{ (10 * dollarsPerYuan).toFixed(0) }}</span>
              <span class="t-label tertiary pkg__note">无优惠</span>
            </div>
            <div class="hairline"></div>
            <div class="pkg__row">
              <span class="pkg__amt mono">¥50</span>
              <span class="pkg__arrow">→</span>
              <span class="pkg__got mono">${{ (50 * dollarsPerYuan * 1.04).toFixed(0) }}</span>
              <span class="t-label pkg__note"><span class="chip chip--up">+4%</span></span>
            </div>
            <div class="hairline"></div>
            <div class="pkg__row">
              <span class="pkg__amt mono">¥100</span>
              <span class="pkg__arrow">→</span>
              <span class="pkg__got mono">${{ (100 * dollarsPerYuan * 1.08).toFixed(0) }}</span>
              <span class="t-label pkg__note"><span class="chip chip--up">+8%</span></span>
            </div>
            <div class="hairline"></div>
            <div class="pkg__row pkg__row--best">
              <span class="pkg__amt mono">¥500</span>
              <span class="pkg__arrow">→</span>
              <span class="pkg__got mono">${{ (500 * dollarsPerYuan * 1.14).toFixed(0) }}</span>
              <span class="t-label pkg__note"><span class="chip chip--up">+14%</span></span>
              <span class="pkg__best">BEST</span>
            </div>
          </div>
        </Tile>
      </div>

      <!-- 右：充值历史 -->
      <div class="grid-col">
        <Tile>
          <div class="tile__head tile__head--split">
            <div>
              <div class="t-display">充值记录</div>
              <div class="t-label tertiary">RECENT {{ records.length }}</div>
            </div>
          </div>
          <div class="recharge-list">
            <div v-for="(r, i) in records.slice(0, 12)" :key="i" class="recharge-row">
              <span class="time">{{ fmtDate(r) }}</span>
              <span class="chip chip--mono">{{ typeLabel(r.type) }}</span>
              <span class="amt">+${{ Math.abs(deltaUsd(r)).toFixed(2) }}</span>
              <StatusDot status="ok" />
            </div>
            <div v-if="!records.length" class="t-label tertiary" style="padding: 12px 4px">还没有充值记录</div>
          </div>
        </Tile>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hero-num { display: flex; align-items: baseline; flex-wrap: wrap; margin: 8px 0; }
.amount-chips { display: flex; flex-wrap: wrap; gap: 6px; margin: 8px 0 12px; }

.pay-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.pay {
  position: relative;
  display: flex; flex-direction: column; align-items: flex-start; gap: 4px;
  padding: 12px;
  min-height: 88px;
  background: rgba(255,255,255,0.04);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  text-align: left;
  font-family: inherit; color: inherit;
  transition: background 150ms ease, box-shadow 150ms ease;
}
.pay:hover { background: rgba(255,255,255,0.07); }
.pay.is-selected { box-shadow: inset 0 0 0 1px rgba(11,212,112,0.4); background: rgba(11,212,112,0.05); }
.pay svg { color: var(--st-text-pri); margin-bottom: 2px; }
.pay__corner {
  position: absolute; top: 6px; right: 6px;
  width: 16px; height: 16px; border-radius: 50%;
  background: var(--st-success); color: var(--st-text-inv);
  display: flex; align-items: center; justify-content: center;
}
.pay__corner svg { color: var(--st-text-inv); margin: 0; }

.redeem { display: flex; gap: 8px; }
.redeem .st-input { flex: 1; }

.pkg { display: flex; flex-direction: column; }
.pkg__row {
  display: grid;
  grid-template-columns: 60px 16px 80px 1fr auto;
  align-items: center;
  gap: 12px;
  height: 44px;
  padding: 0 8px;
  border-radius: 4px;
  position: relative;
  transition: background 150ms ease;
}
.pkg__row:hover { background: rgba(255,255,255,0.02); }
.pkg__amt { font-size: 15px; color: var(--st-text-sec); }
.pkg__arrow { color: var(--st-text-ter); }
.pkg__got { font-size: 15px; font-weight: 500; color: var(--st-text-pri); }
.pkg__row--best { background: rgba(11,212,112,0.05); box-shadow: inset 0 0 0 1px rgba(11,212,112,0.25); padding-right: 60px; }
.pkg__best {
  position: absolute; right: 12px;
  font-size: 10px; font-weight: 600; letter-spacing: 0.12em;
  color: var(--st-success);
  background: rgba(11,212,112,0.12);
  padding: 3px 6px; border-radius: 2px;
}

.recharge-list { display: flex; flex-direction: column; margin-bottom: 12px; }
.recharge-row {
  display: grid;
  grid-template-columns: 90px 70px 1fr 16px;
  align-items: center;
  gap: 12px;
  height: 36px;
}
.recharge-row + .recharge-row { border-top: 1px solid rgba(255,255,255,0.04); }
.recharge-row .time { font-family: var(--st-font-mono); font-size: 11px; color: var(--st-text-ter); }
.recharge-row .amt {
  font-family: var(--st-font-mono); font-variant-numeric: tabular-nums;
  color: var(--st-success); text-align: right; font-size: 13px;
}
</style>
