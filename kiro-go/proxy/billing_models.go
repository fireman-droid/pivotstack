package proxy

import (
	"kiro-api-proxy/config"
	"strings"
)

const (
	CREDIT_TO_USD = 2.0 // 1 Kiro credit = $2 USD face value
)

// SupportedModels 返回各账号池支持的模型清单（权威源，前端展示用）。
//
// ⚠️ 维护原则：每加一个新模型，必须同时更新这里 + ResolveModelPool 关键字判定，
// 才能让"前端展示 / 池路由 / pricing UI 提示"三处同步。
// 反向：ResolveModelPool 用关键字模糊匹配（4.6/4-6/opus 等），所以底层路由
// 一直是动态的；这里仅是给前端展示用的"展示清单"。
func SupportedModels() map[string][]string {
	// Anthropic 只发布了 opus-4.7（无 sonnet-4.7）。展示清单与实际可调模型保持一致。
	return map[string][]string{
		"free": {"claude-sonnet-4.5"},
		"pro":  {"claude-opus-4.7", "claude-opus-4.6", "claude-sonnet-4.6"},
	}
}

// ResolveModelPool determines pool type from model name.
// 4.5 → "free", 4.6/4.7/opus → "pro"
// 同时匹配 . 和 - 分隔符（如 "claude-sonnet-4-6" 与 "opus 4.7"）。
// 4.7 系列上游已支持，直传不再做 4.6 降级。
func ResolveModelPool(model string) string {
	base := strings.ToLower(model)
	if strings.Contains(base, "4.6") || strings.Contains(base, "4-6") ||
		strings.Contains(base, "4.7") || strings.Contains(base, "4-7") ||
		strings.Contains(base, "opus") {
		return "pro"
	}
	return "free"
}

// PoolPriceUSD returns the USD price per credit for a pool (deprecated).
//
// 🚮 v2 起 pool 不再作为定价维度。此函数读 DefaultProPriceUSD/DefaultFreePriceUSD
// 作为 pool 兜底价，仅供 admin 报表（/profit, /pricing-analysis）兼容旧字段使用。
// 业务调用应该用 ModelPriceUSDForKey(keyID, model)。
func PoolPriceUSD(pool string) float64 {
	p := config.GetPricing()
	if pool == "pro" {
		return p.DefaultProPriceUSD
	}
	return p.DefaultFreePriceUSD
}

// lookupModelPrice 在 ModelPrices map 里查 model 单价（'-/.' 互换、大小写不敏感、thinking 后缀剥离匹配）。
// 找不到返回 0（让调用方走兜底）。
//
// 匹配优先级（命中即返回）：
//  1. 完整名小写直查
//  2. 完整名归一化（'-' ↔ '.'）扫描
//  3. 剥离 -thinking/-think 后缀重试
func lookupModelPrice(m map[string]float64, model string) float64 {
	if len(m) == 0 || model == "" {
		return 0
	}
	low := strings.ToLower(strings.TrimSpace(model))
	if v, ok := m[low]; ok && v > 0 {
		return v
	}
	target := normalizeModelKey(low)
	for k, v := range m {
		if v <= 0 {
			continue
		}
		if normalizeModelKey(strings.ToLower(k)) == target {
			return v
		}
	}
	// 剥离 thinking 后缀重试
	stripped := strings.TrimSuffix(strings.TrimSuffix(low, "-thinking"), "-think")
	if stripped != low {
		if v, ok := m[stripped]; ok && v > 0 {
			return v
		}
		st := normalizeModelKey(stripped)
		for k, v := range m {
			if v <= 0 {
				continue
			}
			if normalizeModelKey(strings.ToLower(k)) == st {
				return v
			}
		}
	}
	return 0
}

func lookupMultiplier(m map[string]float64, model string) float64 {
	key := strings.ToLower(strings.TrimSpace(model))
	if v, ok := m[key]; ok && v > 0 {
		return v
	}
	// 兼容：'-' / '.' 互换匹配（claude-opus-4.7 vs claude-opus-4-7）
	for k, v := range m {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk == "" || v <= 0 {
			continue
		}
		if normalizeModelKey(kk) == normalizeModelKey(key) {
			return v
		}
	}
	return 0
}

// normalizeModelKey 把 model 名标准化（点和减号统一）便于匹配。
func normalizeModelKey(s string) string {
	return strings.ReplaceAll(s, "-", ".")
}

// ModelPriceUSD 返回某 model 的当前售价（不含活动判断），admin 报表用。
//
// 三层 fallback：
//  1. pricing.ModelPrices[model] 命中 → 直接返回
//  2. 没命中 → 按 ResolveModelPool(model) 用 DefaultProPriceUSD/DefaultFreePriceUSD
func ModelPriceUSD(model string) float64 {
	pricing := config.GetPricing()
	if v := lookupModelPrice(pricing.ModelPrices, model); v > 0 {
		return v
	}
	if ResolveModelPool(model) == "pro" {
		return pricing.DefaultProPriceUSD
	}
	return pricing.DefaultFreePriceUSD
}

// ModelPriceUSDForKey 返回某 key 调用某 model 的当前实际售价（含活动判断）。
//
// 资格判定（OR 关系）— 满足任一即享活动价：
//  1. key 在活动白名单
//  2. 本月充值 ≥ MinMonthlyRechargeCNY
//  3. 过去 RecentCallsDays 天调用次数 ≥ MinRecentCalls
//
// 活动期价格三层 fallback：
//  1. promo.ModelPrices[model] 命中
//  2. 否则按 pool 用 promo.DefaultProPriceUSD / promo.DefaultFreePriceUSD
//  3. 否则掉到原价（pricing.ModelPrices[model] / pricing.Default*）
//
// keyID 为空 / 资格不通过 / 不在活动期窗口 → 走原价。
func ModelPriceUSDForKey(keyID, model string) float64 {
	promo := config.GetPromotion()
	if promo != nil && promo.Enabled && promotionInTimeWindow(promo) && keyID != "" && keyEligibleForPromotion(promo, keyID) {
		if v := lookupModelPrice(promo.ModelPrices, model); v > 0 {
			return v
		}
		pool := ResolveModelPool(model)
		if pool == "pro" && promo.DefaultProPriceUSD > 0 {
			return promo.DefaultProPriceUSD
		}
		if pool == "free" && promo.DefaultFreePriceUSD > 0 {
			return promo.DefaultFreePriceUSD
		}
		// 活动期未定价 → 掉到原价
	}
	return ModelPriceUSD(model)
}

// PoolPriceUSDForKey deprecated wrapper — pool 参数不再用作定价（仅保留向后兼容）。
// 内部直接走 v2 ModelPriceUSDForKey；当 model 为空时按 pool 用兜底默认价。
//
// ⚠️ 业务调用应该用 ModelPriceUSDForKey(keyID, model)。
func PoolPriceUSDForKey(pool, keyID string) float64 {
	// pool 参数无 model 信息时用兜底默认价
	pricing := config.GetPricing()
	promo := config.GetPromotion()
	if promo != nil && promo.Enabled && promotionInTimeWindow(promo) && keyID != "" && keyEligibleForPromotion(promo, keyID) {
		if pool == "pro" && promo.DefaultProPriceUSD > 0 {
			return promo.DefaultProPriceUSD
		}
		if pool == "free" && promo.DefaultFreePriceUSD > 0 {
			return promo.DefaultFreePriceUSD
		}
	}
	if pool == "pro" {
		return pricing.DefaultProPriceUSD
	}
	return pricing.DefaultFreePriceUSD
}

// EffectivePriceUSD deprecated — pool 参数不再用作定价；新代码请直接用 ModelPriceUSDForKey。
func EffectivePriceUSD(pool, keyID, model string) float64 {
	return ModelPriceUSDForKey(keyID, model)
}

// CreditsToCostUSD 旧版：不带活动判断（admin 报表使用）。pool 参数 deprecated，仅做兜底。
//
// ⚠️ 业务路径应该用 CreditsToCostUSDForKey。
func CreditsToCostUSD(credits float64, pool string) float64 {
	return credits * PoolPriceUSD(pool)
}

// CreditsToCostUSDForKey 业务用：含活动门槛 + per-model 单价。
//
// ⚠️ pool 参数保留是为了不破坏调用方签名，但内部不再用作定价（v2 起 ModelPrices 优先），
// 仅当 model 为空时退化为按 pool 用兜底默认价。
func CreditsToCostUSDForKey(credits float64, pool, keyID, model string) float64 {
	if model == "" {
		return credits * PoolPriceUSDForKey(pool, keyID)
	}
	return credits * ModelPriceUSDForKey(keyID, model)
}

// LegacyModelPriceUSD 按 v1 旧公式（PoolPriceUSD × ModelMultiplier）算 model 价格。
// **shadow 校验专用** — 部署后 24 小时内 call_log 写两份 cost：新公式实际生效，旧公式 shadow 写 cost_usd_legacy。
// grep call_logs.jsonl 看新旧值是否始终相等，确认迁移无偏差。
func LegacyModelPriceUSD(model string) float64 {
	pricing := config.GetPricing()
	pool := ResolveModelPool(model)
	var poolPrice float64
	if pool == "pro" {
		poolPrice = pricing.ProPoolPriceUSD // v1 deprecated 字段，GetPricing 仍兜底 2.00
	} else {
		poolPrice = pricing.FreePoolPriceUSD
	}
	mult := lookupMultiplier(pricing.ModelMultipliers, model)
	if mult <= 0 {
		mult = 1.0
	}
	return poolPrice * mult
}
