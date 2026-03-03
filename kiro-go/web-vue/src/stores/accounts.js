import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api/admin'

export const useAccountsStore = defineStore('accounts', () => {
  const accounts = ref([])
  const selectedIds = ref(new Set())
  const filterKeyword = ref('')
  const filterStatus = ref('all')
  const sortField = ref('')
  const sortAsc = ref(true)
  const loading = ref(false)

  const filtered = computed(() => {
    let list = accounts.value.filter(a => {
      if (filterStatus.value === 'enabled' && !a.enabled) return false
      if (filterStatus.value === 'disabled' && (a.enabled || (a.banStatus && a.banStatus !== 'ACTIVE'))) return false
      if (filterStatus.value === 'banned' && (!a.banStatus || a.banStatus === 'ACTIVE')) return false
      if (filterKeyword.value) {
        const kw = filterKeyword.value.toLowerCase()
        if (!(a.email || '').toLowerCase().includes(kw)) return false
      }
      return true
    })
    if (sortField.value) {
      list = [...list].sort((a, b) => {
        const va = a[sortField.value] ?? 0
        const vb = b[sortField.value] ?? 0
        return sortAsc.value ? (va > vb ? 1 : -1) : (va < vb ? 1 : -1)
      })
    }
    return list
  })

  async function load() {
    loading.value = true
    const res = await api('/accounts')
    if (res.ok) accounts.value = await res.json()
    loading.value = false
  }

  function toggleSelect(id) {
    const s = new Set(selectedIds.value)
    s.has(id) ? s.delete(id) : s.add(id)
    selectedIds.value = s
  }

  function selectAll() {
    selectedIds.value = new Set(filtered.value.map(a => a.id))
  }

  function clearSelection() {
    selectedIds.value = new Set()
  }

  async function batchAction(action, extra = {}) {
    const ids = Array.from(selectedIds.value)
    if (!ids.length) return null
    const res = await api('/accounts/batch', {
      method: 'POST',
      body: JSON.stringify({ ids, action, ...extra }),
    })
    const data = await res.json()
    clearSelection()
    await load()
    return data
  }

  return {
    accounts, selectedIds, filterKeyword, filterStatus, sortField, sortAsc, loading,
    filtered, load, toggleSelect, selectAll, clearSelection, batchAction,
  }
})
