<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Eye, EyeOff, X, ShieldCheck } from 'lucide-vue-next'
import { useMessage } from 'naive-ui'
import { useUserAuth } from '../../stores/userAuth'

const props = defineProps<{
  /** 是否显示 */
  show: boolean
  /** 当前跳过次数；≥ 3 时强制升级（不可跳过/关闭） */
  skipCount: number
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  /** 用户点了"暂时跳过" */
  (e: 'skip'): void
  /** 升级成功（前端可以 auth.refresh + 关闭） */
  (e: 'success'): void
}>()

const message = useMessage()
const auth = useUserAuth() as any

const email = ref('')
const username = ref('')
const password = ref('')
const confirmPwd = ref('')
const showPwd = ref(false)
const showPwd2 = ref(false)
const submitting = ref(false)
const errMsg = ref('')

const canSkip = computed(() => props.skipCount < 3)
const isForced = computed(() => !canSkip.value)

const pwdMismatch = computed(() => confirmPwd.value !== '' && password.value !== confirmPwd.value)
const pwdTooShort = computed(() => password.value !== '' && password.value.length < 8)

// v7: username 默认从当前 key 的 note 派生（NormalizeUsername 的前端镜像）
function normalizeUsername(s: string): string {
  s = (s || '').trim()
  let out = ''
  let lastUnderscore = false
  for (const ch of s) {
    const code = ch.charCodeAt(0)
    if (ch >= 'A' && ch <= 'Z') {
      out += String.fromCharCode(code + 32); lastUnderscore = false
    } else if ((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch === '-') {
      out += ch; lastUnderscore = false
    } else if (ch === '_' || code > 127 || ch === ' ') {
      if (!lastUnderscore) out += '_'
      lastUnderscore = true
    } else {
      if (!lastUnderscore) out += '_'
      lastUnderscore = true
    }
  }
  out = out.replace(/^[_-]+|[_-]+$/g, '')
  return out || 'user'
}

const canSubmit = computed(() =>
  email.value.trim() !== '' &&
  username.value.trim().length >= 1 &&
  password.value.length >= 8 &&
  password.value === confirmPwd.value &&
  !submitting.value,
)

watch(() => props.show, on => {
  if (!on) {
    email.value = ''
    username.value = ''
    password.value = ''
    confirmPwd.value = ''
    showPwd.value = false
    showPwd2.value = false
    errMsg.value = ''
  } else {
    // 打开时预填 username（来自当前 key.note）
    const note = (auth.userInfo?.note as string) || ''
    username.value = normalizeUsername(note)
  }
})

async function onSubmit() {
  if (!canSubmit.value) return
  submitting.value = true
  errMsg.value = ''
  try {
    const res = await fetch('/user/api/bind-account', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${auth.apiKey}`,
      },
      body: JSON.stringify({
        email: email.value.trim(),
        password: password.value,
        username: username.value.trim(),
      }),
    })
    if (res.status === 409) {
      errMsg.value = '此邮箱已被注册'
      return
    }
    if (!res.ok) {
      let msg = `HTTP ${res.status}`
      try { const d = await res.json(); if (d?.error) msg = d.error } catch {}
      errMsg.value = msg
      return
    }
    message.success('账号绑定成功')
    emit('success')
  } catch (e: any) {
    errMsg.value = e?.message || '升级失败'
  } finally {
    submitting.value = false
  }
}

function onSkip() {
  if (!canSkip.value) return
  emit('skip')
}
function onCloseAttempt() {
  if (isForced.value) return // 强制升级时禁用关闭
  emit('update:show', false)
}
</script>

<template>
  <Transition name="uam">
    <div v-if="show" class="uam__backdrop" @click.self="onCloseAttempt">
      <div class="uam" role="dialog" aria-modal="true">
        <button
          v-if="!isForced"
          class="uam__close"
          type="button"
          @click="onCloseAttempt"
          aria-label="关闭"
        >
          <X :size="16" />
        </button>

        <div class="uam__head">
          <div class="uam__icon">
            <ShieldCheck :size="20" />
          </div>
          <div class="uam__head-text">
            <h2 class="uam__title">升级账号</h2>
            <div class="uam__subtitle">
              {{ isForced
                ? '已多次跳过，请绑定邮箱和密码以继续使用'
                : '为您的账号绑定邮箱和密码（仅作登录凭证，不发激活邮件）' }}
            </div>
          </div>
        </div>

        <form class="uam__form" @submit.prevent="onSubmit">
          <label class="uam__label">
            用户名
            <span class="uam__hint">注册后不可修改 · 默认来自你 Key 的名字</span>
          </label>
          <input
            v-model="username"
            type="text"
            class="uam__input"
            placeholder="登录用的用户名"
            autocomplete="username"
            required
          />

          <label class="uam__label">邮箱</label>
          <input
            v-model="email"
            type="email"
            class="uam__input"
            placeholder="your@email.com"
            autocomplete="email"
            required
          />

          <label class="uam__label">
            密码
            <span class="uam__hint">≥ 8 位</span>
          </label>
          <div class="uam__field">
            <input
              v-model="password"
              :type="showPwd ? 'text' : 'password'"
              class="uam__input"
              placeholder="设置一个安全的密码"
              autocomplete="new-password"
              required
            />
            <button type="button" class="uam__eye" @click="showPwd = !showPwd" tabindex="-1">
              <Eye v-if="!showPwd" :size="14" />
              <EyeOff v-else :size="14" />
            </button>
          </div>
          <div v-if="pwdTooShort" class="uam__field-err">密码至少 8 位</div>

          <label class="uam__label">确认密码</label>
          <div class="uam__field">
            <input
              v-model="confirmPwd"
              :type="showPwd2 ? 'text' : 'password'"
              class="uam__input"
              placeholder="再输入一次"
              autocomplete="new-password"
              required
            />
            <button type="button" class="uam__eye" @click="showPwd2 = !showPwd2" tabindex="-1">
              <Eye v-if="!showPwd2" :size="14" />
              <EyeOff v-else :size="14" />
            </button>
          </div>
          <div v-if="pwdMismatch" class="uam__field-err">两次密码不一致</div>

          <div v-if="errMsg" class="uam__err">{{ errMsg }}</div>

          <ul class="uam__notes">
            <li>升级后您原有的 API Key 不变，余额、套餐、调用记录全部不动</li>
            <li>下次可用邮箱+密码登录，也仍可用原 API Key 登录</li>
          </ul>

          <div class="uam__foot">
            <button
              v-if="canSkip"
              type="button"
              class="uam__btn uam__btn--ghost"
              :disabled="submitting"
              @click="onSkip"
            >暂时跳过</button>
            <button
              type="submit"
              class="uam__btn uam__btn--primary"
              :disabled="!canSubmit"
            >{{ submitting ? '提交中…' : '立即升级 →' }}</button>
          </div>
        </form>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.uam__backdrop {
  position: fixed; inset: 0;
  background: rgba(0, 0, 0, 0.72);
  backdrop-filter: blur(4px);
  display: flex; align-items: center; justify-content: center;
  z-index: 1000;
  padding: 24px;
}
.uam {
  position: relative;
  width: 100%;
  max-width: 440px;
  background: #0a0a0a;
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 10px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.7);
  padding: 28px 32px;
}
.uam__close {
  position: absolute; top: 12px; right: 12px;
  width: 28px; height: 28px;
  display: flex; align-items: center; justify-content: center;
  background: transparent; border: none;
  border-radius: 4px;
  color: #707070; cursor: pointer;
  transition: background 160ms ease, color 160ms ease;
}
.uam__close:hover { background: rgba(255, 255, 255, 0.06); color: #ededed; }

.uam__head { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 24px; }
.uam__icon {
  width: 40px; height: 40px;
  border-radius: 8px;
  background: rgba(11, 212, 112, 0.10);
  color: #0bd470;
  display: inline-flex; align-items: center; justify-content: center;
  flex-shrink: 0;
}
.uam__head-text { flex: 1; min-width: 0; }
.uam__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #ededed;
  letter-spacing: -0.01em;
}
.uam__subtitle {
  margin-top: 6px;
  font-size: 12px;
  color: #a1a1a1;
  line-height: 1.5;
}

.uam__form { display: flex; flex-direction: column; }
.uam__label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: #707070;
  text-transform: uppercase;
  margin: 12px 0 6px;
}
.uam__hint {
  margin-left: 6px;
  font-size: 10px;
  letter-spacing: 0;
  text-transform: none;
  color: #4d4d4d;
  font-weight: 400;
}
.uam__field { position: relative; display: flex; align-items: center; }
.uam__input {
  width: 100%;
  height: 38px;
  padding: 0 12px;
  background: #050505;
  border: 1px solid rgba(255, 255, 255, 0.10);
  border-radius: 4px;
  color: #ededed;
  font-size: 13px;
  font-family: inherit;
  outline: none;
  transition: border-color 160ms ease;
}
.uam__field .uam__input { padding-right: 38px; }
.uam__input::placeholder { color: #4d4d4d; }
.uam__input:focus { border-color: rgba(11, 212, 112, 0.45); }
.uam__eye {
  position: absolute; right: 6px;
  width: 28px; height: 28px;
  background: transparent; border: none;
  display: flex; align-items: center; justify-content: center;
  color: #707070; cursor: pointer;
  border-radius: 3px;
  transition: color 160ms ease;
}
.uam__eye:hover { color: #ededed; background: rgba(255, 255, 255, 0.04); }

.uam__field-err {
  margin-top: 4px;
  font-size: 11px;
  color: #ff7a7a;
}
.uam__err {
  margin-top: 14px;
  padding: 6px 10px;
  background: rgba(255, 77, 77, 0.08);
  border: 1px solid rgba(255, 77, 77, 0.30);
  border-radius: 4px;
  color: #ff7a7a;
  font-size: 12px;
}

.uam__notes {
  margin: 16px 0 0;
  padding-left: 16px;
  list-style: disc;
  font-size: 11px;
  color: #707070;
  line-height: 1.6;
}

.uam__foot {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.uam__btn {
  height: 36px;
  padding: 0 16px;
  border-radius: 4px;
  border: none;
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  cursor: pointer;
  transition: background 160ms ease, opacity 160ms ease;
}
.uam__btn--ghost { background: transparent; color: #a1a1a1; }
.uam__btn--ghost:hover:not(:disabled) { background: rgba(255, 255, 255, 0.06); color: #ededed; }
.uam__btn--primary { background: #ededed; color: #000; }
.uam__btn--primary:hover:not(:disabled) { background: #fff; }
.uam__btn:disabled { opacity: 0.5; cursor: not-allowed; }

.uam-enter-active, .uam-leave-active { transition: opacity 200ms ease; }
.uam-enter-active .uam, .uam-leave-active .uam {
  transition: opacity 200ms ease, transform 200ms cubic-bezier(0.16, 1, 0.3, 1);
}
.uam-enter-from, .uam-leave-to { opacity: 0; }
.uam-enter-from .uam, .uam-leave-to .uam { opacity: 0; transform: translateY(8px) scale(0.98); }
</style>
