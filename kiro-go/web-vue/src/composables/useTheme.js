import { ref, watchEffect } from 'vue'

const theme = ref(localStorage.getItem('kiro_theme') || 'system')

function applyTheme() {
  const root = document.documentElement
  if (theme.value === 'dark' || (theme.value === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    root.classList.add('dark')
  } else {
    root.classList.remove('dark')
  }
}

export function useTheme() {
  watchEffect(applyTheme)

  function setTheme(t) {
    theme.value = t
    localStorage.setItem('kiro_theme', t)
    applyTheme()
  }

  return { theme, setTheme }
}
