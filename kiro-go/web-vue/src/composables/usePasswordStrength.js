import { computed } from 'vue'

const STRENGTH_TEXTS = ['极弱', '很弱', '弱', '中', '强', '极强']

const STRENGTH_COLORS = [
  '#9ca3af',
  '#dc2626',
  '#ea580c',
  '#f59e0b',
  '#10b981',
  '#059669',
]

export function usePasswordStrength(password) {
  const strength = computed(() => {
    const pwd = password.value || ''
    if (!pwd) return 0
    let score = 0
    if (pwd.length >= 8) score++
    if (pwd.length >= 12) score++
    if (/[A-Z]/.test(pwd) && /[a-z]/.test(pwd)) score++
    if (/[0-9]/.test(pwd)) score++
    if (/[^A-Za-z0-9]/.test(pwd)) score++
    return score
  })

  const rules = computed(() => {
    const pwd = password.value || ''
    return [
      { label: '至少 12 个字符', met: pwd.length >= 12 },
      { label: '包含大小写字母', met: /[A-Z]/.test(pwd) && /[a-z]/.test(pwd) },
      { label: '包含数字', met: /[0-9]/.test(pwd) },
      { label: '包含特殊符号', met: /[^A-Za-z0-9]/.test(pwd) },
    ]
  })

  const strengthText = computed(() => STRENGTH_TEXTS[strength.value] || '极弱')

  const strengthColor = computed(() => STRENGTH_COLORS[strength.value] || STRENGTH_COLORS[0])

  // 提交门槛：长度 ≥ 12 且强度 ≥ 中（3）
  const canSubmit = computed(() => {
    const pwd = password.value || ''
    return pwd.length >= 12 && strength.value >= 3
  })

  return { strength, rules, strengthText, strengthColor, canSubmit }
}
