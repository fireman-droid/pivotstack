import { ref, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import { startBuilderIdLogin, pollBuilderIdAuth } from '../../../api/admin/accounts'

export interface KiroOAuthState {
  region: string
  starting: boolean
  polling: boolean
  sessionId: string
  userCode: string
  verificationUri: string
  status: 'idle' | 'awaiting' | 'success' | 'error'
  errorMsg: string
  resultEmail: string
}

/**
 * AWS Builder ID device-code OAuth 流程封装。
 * - startOAuth() 启动会话，新窗口打开授权页
 * - 内部轮询 pollBuilderIdAuth；成功触发 onSuccess
 * - cancelOAuth() / 组件 unmount 自动 cleanup timer
 */
export function useKiroDeviceCode(options: {
  onSuccess: (email: string) => void
}) {
  const message = useMessage()
  const state = ref<KiroOAuthState>({
    region: 'us-east-1',
    starting: false,
    polling: false,
    sessionId: '',
    userCode: '',
    verificationUri: '',
    status: 'idle',
    errorMsg: '',
    resultEmail: '',
  })
  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let successTimer: ReturnType<typeof setTimeout> | null = null

  function stopPolling() {
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
    if (successTimer) {
      clearTimeout(successTimer)
      successTimer = null
    }
    state.value.polling = false
  }

  function setRegion(v: string) {
    state.value.region = v
  }

  function cancelOAuth() {
    stopPolling()
    state.value.status = 'idle'
    state.value.sessionId = ''
    state.value.userCode = ''
    state.value.verificationUri = ''
    state.value.errorMsg = ''
  }

  async function startOAuth() {
    state.value.starting = true
    state.value.status = 'idle'
    state.value.errorMsg = ''
    try {
      const resp = await startBuilderIdLogin(state.value.region)
      state.value.sessionId = resp.sessionId
      state.value.userCode = resp.userCode
      state.value.verificationUri = resp.verificationUri
      state.value.status = 'awaiting'
      state.value.polling = true
      window.open(resp.verificationUri, '_blank', 'noopener,noreferrer')
      scheduleNextPoll(resp.interval || 5)
    } catch (e: any) {
      state.value.errorMsg = e?.message || '启动失败'
      state.value.status = 'error'
    } finally {
      state.value.starting = false
    }
  }

  function scheduleNextPoll(interval: number) {
    pollTimer = setTimeout(async () => {
      if (!state.value.sessionId || state.value.status !== 'awaiting') return
      try {
        const resp = await pollBuilderIdAuth(state.value.sessionId)
        if (resp.completed) {
          state.value.status = 'success'
          state.value.polling = false
          state.value.resultEmail = resp.account?.email || ''
          message.success(`已添加 ${resp.account?.email || '账号'}`)
          // 让 onSuccess 决定何时关闭；这里只暴露 setSuccessTimer 给调用方排队 cleanup
          options.onSuccess(state.value.resultEmail)
          return
        }
        scheduleNextPoll(resp.interval || interval)
      } catch (e: any) {
        state.value.errorMsg = e?.message || '轮询失败'
        state.value.status = 'error'
        state.value.polling = false
      }
    }, interval * 1000)
  }

  async function copyUserCode() {
    if (!state.value.userCode) return
    await navigator.clipboard.writeText(state.value.userCode)
    message.success('已复制 user code')
  }

  onUnmounted(stopPolling)

  /** 排一个延迟回调（如 success 1.2s 后关闭 drawer），随 stopPolling 一起 cleanup */
  function setSuccessTimer(fn: () => void, ms: number) {
    if (successTimer) clearTimeout(successTimer)
    successTimer = setTimeout(() => {
      successTimer = null
      fn()
    }, ms)
  }

  return {
    state,
    startOAuth,
    cancelOAuth,
    stopPolling,
    copyUserCode,
    setRegion,
    setSuccessTimer,
  }
}
