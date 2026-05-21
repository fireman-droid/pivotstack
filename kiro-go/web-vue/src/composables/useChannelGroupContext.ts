// ChannelGroup 详情页数据上下文。
// 集中处理：
//  - 候选 channel 池
//  - 各 NewAPI provider 的 metadata（含 group_ratio / model_ratio / cache_ratio / enable_groups）
//  - 按 (providerId, groupName) 算出每条 NewAPI candidate 的入价摘要
//  - 自营直连按已有 sellPrice 文本展示
//
// 价格公式参见 useNewAPIPricing.ts。

import { ref, computed, type Ref } from 'vue'
import {
  listGroupCandidateChannels,
  type AdminGroupView,
} from '../api/admin/groups'
import { getProviderMetadata } from '../api/admin/providers'
import {
  calcUpstreamPrices,
  formatRange,
  type NewAPIGroup,
  type NewAPIModel,
} from './useNewAPIPricing'

export interface ModelPriceRow {
  name: string
  modelRatio: number
  inputPrice: number   // 上游入价 $/Mtok
  outputPrice: number  // 上游入价 $/Mtok
  cachePrice: number   // 上游入价 $/Mtok
}

export interface CandidatePricing {
  inputMin: number
  inputMax: number
  outputMin: number
  outputMax: number
  groupRatio: number
  modelsCount: number
  modelRows: ModelPriceRow[]   // 该渠道下所有按量模型 + 单价（按 model_ratio 降序）
}

export interface CandidateWithPricing extends AdminGroupView {
  providerId?: string         // NewAPI 才有；从 channelId 拆出
  groupName?: string          // NewAPI 才有；从 sourceDetail 解出
  upstreamPricing: CandidatePricing | null
  topModelExamples: string[]  // 主力 3 模型名（搜索匹配用）
}

export function useChannelGroupContext() {
  const candidates = ref<AdminGroupView[]>([])
  const providerMetadata = ref<Record<string, { groups: NewAPIGroup[]; models: NewAPIModel[] }>>({})
  const loadingCandidates = ref(false)
  const loadingPricing = ref(false)

  async function loadCandidates() {
    loadingCandidates.value = true
    try {
      candidates.value = await listGroupCandidateChannels()
    } finally {
      loadingCandidates.value = false
    }
  }

  // 解析 NewAPI candidate 的 providerId / groupName
  function parseNewAPICandidate(c: AdminGroupView): { providerId: string; groupName: string } | null {
    if (c.sourceType !== 'newapi') return null
    // channelId 格式：<providerId>:tok-<n>
    const colon = c.channelId.indexOf(':')
    const providerId = colon > 0 ? c.channelId.slice(0, colon) : ''
    // sourceDetail 格式：<providerName> / tok-<n> / <groupName>
    const parts = c.sourceDetail.split(' / ')
    const groupName = parts.length >= 3 ? parts.slice(2).join(' / ') : ''
    if (!providerId || !groupName) return null
    return { providerId, groupName }
  }

  async function loadProviderMetadata() {
    loadingPricing.value = true
    try {
      const providerIds = new Set<string>()
      for (const c of candidates.value) {
        const parsed = parseNewAPICandidate(c)
        if (parsed) providerIds.add(parsed.providerId)
      }
      // 跳过已加载过的
      const toLoad = Array.from(providerIds).filter(id => !providerMetadata.value[id])
      if (toLoad.length === 0) return
      const results = await Promise.allSettled(toLoad.map(id => getProviderMetadata(id)))
      const next = { ...providerMetadata.value }
      results.forEach((r, i) => {
        if (r.status === 'fulfilled') {
          const meta = r.value as any
          next[toLoad[i]] = {
            groups: meta?.groups || [],
            models: meta?.models || [],
          }
        }
      })
      providerMetadata.value = next
    } finally {
      loadingPricing.value = false
    }
  }

  function pricingFor(c: AdminGroupView): CandidateWithPricing {
    if (c.sourceType !== 'newapi') {
      return { ...c, upstreamPricing: null, topModelExamples: [] }
    }
    const parsed = parseNewAPICandidate(c)
    if (!parsed) return { ...c, upstreamPricing: null, topModelExamples: [] }
    const meta = providerMetadata.value[parsed.providerId]
    if (!meta) {
      return { ...c, providerId: parsed.providerId, groupName: parsed.groupName, upstreamPricing: null, topModelExamples: [] }
    }
    const group = meta.groups.find(g => g.name === parsed.groupName)
    if (!group) {
      return { ...c, providerId: parsed.providerId, groupName: parsed.groupName, upstreamPricing: null, topModelExamples: [] }
    }
    const ratio = group.ratio ?? 1
    const inGroup = meta.models.filter(m => (m.enable_groups || []).includes(parsed.groupName))
    if (!inGroup.length) {
      return {
        ...c,
        providerId: parsed.providerId,
        groupName: parsed.groupName,
        upstreamPricing: { inputMin: 0, inputMax: 0, outputMin: 0, outputMax: 0, groupRatio: ratio, modelsCount: 0, modelRows: [] },
        topModelExamples: [],
      }
    }
    let inMin = Infinity, inMax = 0, outMin = Infinity, outMax = 0
    const ranked: ModelPriceRow[] = inGroup.map(m => {
      const p = calcUpstreamPrices(m, ratio)
      if (p.input < inMin) inMin = p.input
      if (p.input > inMax) inMax = p.input
      if (p.output < outMin) outMin = p.output
      if (p.output > outMax) outMax = p.output
      return {
        name: m.model_name,
        modelRatio: m.model_ratio,
        inputPrice: p.input,
        outputPrice: p.output,
        cachePrice: p.cache,
      }
    })
    // 按 model_ratio 降序（贵的模型在上），同价按名字稳定排
    ranked.sort((a, b) => b.modelRatio - a.modelRatio || a.name.localeCompare(b.name))
    return {
      ...c,
      providerId: parsed.providerId,
      groupName: parsed.groupName,
      upstreamPricing: {
        inputMin: inMin === Infinity ? 0 : inMin,
        inputMax: inMax,
        outputMin: outMin === Infinity ? 0 : outMin,
        outputMax: outMax,
        groupRatio: ratio,
        modelsCount: inGroup.length,
        modelRows: ranked,
      },
      topModelExamples: ranked.slice(0, 3).map(r => r.name),
    }
  }

  const enriched: Ref<CandidateWithPricing[]> = computed(() => candidates.value.map(c => pricingFor(c)))

  return {
    candidates,
    enriched,
    loadingCandidates,
    loadingPricing,
    loadCandidates,
    loadProviderMetadata,
  }
}

export function formatPricingSummary(p: CandidatePricing | null): string {
  if (!p) return '-'
  if (p.modelsCount === 0) return '该分组下无模型'
  return formatRange(p.inputMin, p.inputMax) + ' /in'
}
