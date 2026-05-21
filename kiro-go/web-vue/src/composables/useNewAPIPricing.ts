// 上游 NewAPI 定价换算
//
// 三层乘法：input_$/Mtok = model_ratio × 2 × group_ratio
//          output_$/Mtok = model_ratio × completion_ratio × 2 × group_ratio
//          cache_$/Mtok = model_ratio × cache_ratio × 2 × group_ratio
// 验证：claude-sonnet-4-6 (model_ratio=1.5, completion_ratio=5, cache_ratio=0.1)
//        在 kiro引流福利 (group_ratio=0.1) 显示 $0.30/in $1.50/out $0.03/cache
//        1.5 × 2 × 0.1 = $0.30 ✓
import { computed, type Ref } from 'vue'

export interface NewAPIGroup { name: string; desc?: string; ratio: number }
export interface NewAPIModel {
  model_name: string
  model_ratio: number
  completion_ratio: number
  cache_ratio?: number
  enable_groups?: string[]
}
export interface NewAPIToken { id: number; name: string; group: string; status: number }

export interface PriceSummary {
  groupRatio: number
  modelsInGroup: number
  inputMin: number
  inputMax: number
  outputMin: number
  outputMax: number
  topModels: Array<{ name: string; inputPrice: number; outputPrice: number }>
}

const NEWAPI_RATIO_BASE_USD = 2 // NewAPI 约定：model_ratio × $2 = $/Mtok base

export function calcUpstreamPrices(model: NewAPIModel, groupRatio: number) {
  const base = model.model_ratio * NEWAPI_RATIO_BASE_USD * groupRatio
  return {
    input: base,
    output: base * (model.completion_ratio || 1),
    cache: base * (model.cache_ratio || 0),
  }
}

export function useNewAPIPricing(metadata: Ref<{ groups?: NewAPIGroup[]; models?: NewAPIModel[]; tokens?: NewAPIToken[] }>) {
  const groupRatioMap = computed(() => {
    const m = new Map<string, number>()
    for (const g of metadata.value.groups ?? []) m.set(g.name, g.ratio ?? 1)
    return m
  })

  const modelsByGroup = computed(() => {
    const m = new Map<string, NewAPIModel[]>()
    for (const model of metadata.value.models ?? []) {
      for (const g of model.enable_groups ?? []) {
        if (!m.has(g)) m.set(g, [])
        m.get(g)!.push(model)
      }
    }
    return m
  })

  function summaryForGroup(groupName: string): PriceSummary | null {
    if (!groupName) return null
    const ratio = groupRatioMap.value.get(groupName)
    if (ratio === undefined) return null
    const models = modelsByGroup.value.get(groupName) ?? []
    if (!models.length) {
      return {
        groupRatio: ratio,
        modelsInGroup: 0,
        inputMin: 0, inputMax: 0, outputMin: 0, outputMax: 0,
        topModels: [],
      }
    }
    let inputMin = Infinity, inputMax = 0, outputMin = Infinity, outputMax = 0
    const enriched = models.map(model => {
      const p = calcUpstreamPrices(model, ratio)
      if (p.input < inputMin) inputMin = p.input
      if (p.input > inputMax) inputMax = p.input
      if (p.output < outputMin) outputMin = p.output
      if (p.output > outputMax) outputMax = p.output
      return { name: model.model_name, inputPrice: p.input, outputPrice: p.output, modelRatio: model.model_ratio }
    })
    // 按 model_ratio 倒序取 top 3 = 该组下"最贵"的模型，最有代表性
    enriched.sort((a, b) => b.modelRatio - a.modelRatio)
    return {
      groupRatio: ratio,
      modelsInGroup: models.length,
      inputMin: inputMin === Infinity ? 0 : inputMin,
      inputMax,
      outputMin: outputMin === Infinity ? 0 : outputMin,
      outputMax,
      topModels: enriched.slice(0, 3).map(({ name, inputPrice, outputPrice }) => ({ name, inputPrice, outputPrice })),
    }
  }

  return { groupRatioMap, modelsByGroup, summaryForGroup }
}

export function formatPrice(v: number, digits = 2): string {
  if (v === 0) return '$0'
  if (v < 0.01) return `$${v.toFixed(4)}`
  if (v < 1) return `$${v.toFixed(3)}`
  return `$${v.toFixed(digits)}`
}

export function formatRange(min: number, max: number, digits = 2): string {
  if (min === max || min === 0) return formatPrice(max, digits)
  return `${formatPrice(min, digits)} ~ ${formatPrice(max, digits)}`
}
