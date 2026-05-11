package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
	"strings"
	"time"
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

// promotionInTimeWindow 判断当前时间是否在活动有效期内。
// StartTs/EndTs 为 0 视为无下界/无上界。
func promotionInTimeWindow(p *config.PromotionConfig) bool {
	if p == nil {
		return false
	}
	now := time.Now().Unix()
	if p.StartTs > 0 && now < p.StartTs {
		return false
	}
	if p.EndTs > 0 && now > p.EndTs {
		return false
	}
	return true
}

// isInPromotionWhitelist 判断 keyID 是否在活动白名单。
func isInPromotionWhitelist(p *config.PromotionConfig, keyID string) bool {
	if p == nil || keyID == "" {
		return false
	}
	for _, k := range p.Whitelist {
		if k == keyID {
			return true
		}
	}
	return false
}

// keyEligibleForPromotion 综合判定一个 key 是否够资格享受活动价。
//
// 排除规则：
//   1. 子 key（ParentKeyID != ""）— 钱来自 reseller，享活动价会导致 reseller 套利
//      （reseller 卖给真用户按标价收，但子 key 按活动价扣，差价吃定）。
//   2. Reseller key（IsReseller=true）— 已享 ResellerDiscount 折扣进货，再叠活动 = 双重套利。
// 这两类 key 调用永远走标价；活动只面向普通直购用户。
func keyEligibleForPromotion(p *config.PromotionConfig, keyID string) bool {
	if p == nil || !p.Enabled || keyID == "" {
		return false
	}
	if info := config.FindApiKeyByID(keyID); info != nil {
		if info.ParentKeyID != "" {
			return false // 子 key 不参与活动
		}
		if info.IsReseller {
			return false // reseller 已享折扣，不参与活动
		}
	}
	// 1. 白名单
	if isInPromotionWhitelist(p, keyID) {
		return true
	}
	// 2. 充值门槛
	if p.MinMonthlyRechargeCNY > 0 {
		if monthlyRechargeSumCNY(keyID) >= p.MinMonthlyRechargeCNY {
			return true
		}
	}
	// 3. 活跃度门槛
	if p.MinRecentCalls > 0 && p.RecentCallsDays > 0 {
		if recentCallCount(keyID, p.RecentCallsDays) >= p.MinRecentCalls {
			return true
		}
	}
	return false
}

// promotionEligible 给 user/admin endpoint 用的公开版本（接受预先算好的 monthCNY 和 recentCalls，避免重复扫描）。
// 排除规则同 keyEligibleForPromotion：子 key + reseller 都不参与活动。
func promotionEligible(p *config.PromotionConfig, keyID string, monthCNY float64, recentCalls int) bool {
	if p == nil || !p.Enabled {
		return false
	}
	if info := config.FindApiKeyByID(keyID); info != nil {
		if info.ParentKeyID != "" {
			return false // 子 key 不参与活动
		}
		if info.IsReseller {
			return false // reseller 已享折扣，不参与活动
		}
	}
	if isInPromotionWhitelist(p, keyID) {
		return true
	}
	if p.MinMonthlyRechargeCNY > 0 && monthCNY >= p.MinMonthlyRechargeCNY {
		return true
	}
	if p.MinRecentCalls > 0 && p.RecentCallsDays > 0 && recentCalls >= p.MinRecentCalls {
		return true
	}
	return false
}

// EstimateCredits estimates credits from max_tokens (rough estimation for pre-auth).
// Based on empirical formula: credit ≈ 0.0036268×(inputK) + 0.0001092735×(outputK) + 0.00948186
func EstimateCredits(maxTokens, estimatedInput int) float64 {
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	if estimatedInput <= 0 {
		estimatedInput = 1000
	}
	return 0.0036268*float64(estimatedInput)/1000 +
		0.0001092735*float64(maxTokens)/1000 + 0.00948186
}

// PreAuthorize pre-deducts estimated cost (in USD) at request start.
// Returns (preChargedPaidUSD, preChargedGiftUSD, error). Returns (0,0) if action="free".
func PreAuthorize(keyID string, model string, maxTokens, estimatedInput int) (float64, float64, error) {
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		return 0, 0, nil
	}

	pool := ResolveModelPool(model)
	action, err := config.ValidateKeyAccessForModel(info, pool)
	if err != nil {
		return 0, 0, err
	}

	if action == "free" {
		return 0, 0, nil // day card covers this, no charge
	}

	// Estimate credits and convert to USD（用 ForKey 版本，让活动门槛 + model 倍率生效）
	estCredits := EstimateCredits(maxTokens, estimatedInput)
	estCostUSD := CreditsToCostUSDForKey(estCredits, pool, keyID, model)

	ok, remaining, paidDeducted, giftDeducted := config.DeductKeyBalance(keyID, estCostUSD)
	if !ok {
		return 0, 0, fmt.Errorf("insufficient balance (need $%.4f, have $%.4f)", estCostUSD, remaining)
	}

	fmt.Printf("[Billing] PreAuth key=%s model=%s pool=%s est_credits=%.4f est_cost=$%.4f remaining=$%.4f (paid=$%.4f, gift=$%.4f)\n",
		keyID[:8], model, pool, estCredits, estCostUSD, remaining, paidDeducted, giftDeducted)

	return paidDeducted, giftDeducted, nil
}

// ApplyLowOutputProtection caps credits when output is abnormally low but cost is high.
// This protects users from being overcharged for failed/truncated responses.
func ApplyLowOutputProtection(outputTokens int, actualCredits float64, inputTokens int) float64 {
	if outputTokens < 30 && actualCredits > 1.0 {
		capped := EstimateCredits(100, inputTokens)
		if capped < actualCredits {
			fmt.Printf("[Billing] LowOutputProtection: out=%d credits=%.4f → capped=%.4f (saved %.4f)\n",
				outputTokens, actualCredits, capped, actualCredits-capped)
			return capped
		}
	}
	return actualCredits
}

// Reconcile settles the difference between pre-charged and actual cost.
// Returns (actualPaidCostUSD, actualGiftCostUSD).
//
// model: 用户原始请求的模型（用于查 ModelMultipliers 倍率，保证全链路一致）。
// pool 单独按 billingModel 决定（如未传 billingModel 则按 model 决定）。
func Reconcile(keyID, model string, actualCredits, preChargedPaid, preChargedGift float64) (float64, float64) {
	return ReconcileWithBillingModel(keyID, model, model, actualCredits, preChargedPaid, preChargedGift)
}

// ReconcileWithBillingModel 是 Reconcile 的扩展版本，可以分别指定：
//   - originalModel: 用户原始请求模型（用于查倍率）
//   - billingModel:  实际计费模型（用于决定 pool）
//
// 当 stealth 把 opus-4.7 替换为 sonnet-4.6 时：originalModel="claude-opus-4.7"，billingModel="claude-sonnet-4.6"。
// 倍率按 originalModel 走（保证用户配的 4.7=1.5 在结算时也生效）。
func ReconcileWithBillingModel(keyID, originalModel, billingModel string, actualCredits, preChargedPaid, preChargedGift float64) (float64, float64) {
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		return 0, 0
	}

	pool := ResolveModelPool(billingModel)
	action, _ := config.ValidateKeyAccessForModel(info, pool)
	if action == "free" {
		return 0, 0 // no charge for day card users
	}

	// 关键：用 originalModel 查倍率（不是 billingModel），保证用户配置的模型倍率在结算时也生效。
	actualCostUSD := CreditsToCostUSDForKey(actualCredits, pool, keyID, originalModel)
	preChargedTotal := preChargedPaid + preChargedGift
	diff := actualCostUSD - preChargedTotal

	actualPaid := preChargedPaid
	actualGift := preChargedGift

	if diff > 0 {
		ok, _, addedPaid, addedGift := config.DeductKeyBalance(keyID, diff)
		if !ok {
			fmt.Printf("[Billing] Reconcile key=%s UNDERPAID by $%.4f\n", keyID[:8], diff)
		} else {
			actualPaid += addedPaid
			actualGift += addedGift
		}
	} else if diff < 0 {
		overCharge := -diff
		refundGift := 0.0
		refundPaid := 0.0

		// Refund logic: refund gift first, then paid
		if overCharge <= preChargedGift {
			refundGift = overCharge
		} else {
			refundGift = preChargedGift
			refundPaid = overCharge - preChargedGift
		}

		config.AddKeyBalance(keyID, refundPaid, refundGift)
		actualPaid -= refundPaid
		actualGift -= refundGift
	}

	fmt.Printf("[Billing] Reconcile key=%s pool=%s actual_credits=%.4f actual_cost=$%.4f actual_paid=$%.4f actual_gift=$%.4f\n",
		keyID[:8], pool, actualCredits, actualCostUSD, actualPaid, actualGift)

	return actualPaid, actualGift
}

// RefundPreAuth fully refunds a pre-authorized amount (on request failure).
func RefundPreAuth(keyID string, preChargedPaid float64, preChargedGift float64) {
	if (preChargedPaid > 0 || preChargedGift > 0) && keyID != "" {
		config.AddKeyBalance(keyID, preChargedPaid, preChargedGift)
		fmt.Printf("[Billing] Refund key=%s paid=$%.4f gift=$%.4f\n", keyID[:8], preChargedPaid, preChargedGift)
	}
}

// TryDeductBalance is the simple post-request deduction (fallback if pre-auth not used).
// Uses actual credits from Kiro API. Returns (paidCostUSD, giftCostUSD, error).
func TryDeductBalance(uc *UserContext, model string, actualCredits float64) (float64, float64, error) {
	if uc == nil || uc.KeyID == "" {
		return 0, 0, nil
	}

	info := config.FindApiKeyByID(uc.KeyID)
	if info == nil {
		return 0, 0, nil
	}

	pool := ResolveModelPool(model)
	action, err := config.ValidateKeyAccessForModel(info, pool)
	if err != nil {
		return 0, 0, err
	}
	if action == "free" {
		return 0, 0, nil
	}

	// Bug #6: 用 ForKey 版本，保证活动门槛 + 模型倍率生效（虽然此函数当前 dead code，
	// 一旦未来 fallback 启用，必须跟 PreAuth/Reconcile 同口径）
	costUSD := CreditsToCostUSDForKey(actualCredits, pool, uc.KeyID, model)
	ok, remaining, paidCost, giftCost := config.DeductKeyBalance(uc.KeyID, costUSD)
	if !ok {
		return 0, 0, fmt.Errorf("insufficient balance (need $%.4f, have $%.4f)", costUSD, remaining)
	}

	fmt.Printf("[Billing] key=%s model=%s pool=%s credits=%.4f cost=$%.4f remaining=$%.4f (paid=$%.4f, gift=$%.4f)\n",
		uc.KeyID[:8], model, pool, actualCredits, costUSD, remaining, paidCost, giftCost)

	return paidCost, giftCost, nil
}

// CalcAdminProfit calculates profit for admin dashboard using per-pool costs.
func CalcAdminProfit(totalUSDConsumed, proCreditConsumed, freeCreditConsumed float64) map[string]float64 {
	pricing := config.GetPricing()
	proCostCNY := proCreditConsumed * pricing.ProCostPerCredit()
	freeCostCNY := freeCreditConsumed * pricing.FreeCostPerCredit()
	totalCostCNY := proCostCNY + freeCostCNY

	// Convert face-value USD to real CNY ($1 face = ¥0.05 real)
	revenueCNY := totalUSDConsumed * config.CNYPerUSDFace
	profitCNY := revenueCNY - totalCostCNY

	margin := 0.0
	if revenueCNY > 0 {
		margin = profitCNY / revenueCNY * 100
	}
	return map[string]float64{
		"revenue_usd":    totalUSDConsumed,
		"revenue_cny":    revenueCNY,
		"pro_cost_cny":   proCostCNY,
		"free_cost_cny":  freeCostCNY,
		"total_cost_cny": totalCostCNY,
		"profit_cny":     profitCNY,
		"margin_percent": margin,
	}
}

// stealthCreditRate returns the typical upstream credit cost for one "unit" of work
// for the given model. Used to upscale upstream-reported credits when the model
// was secretly swapped, so the user is billed at the original model's rate.
//
// 经验值：sonnet-4.5 与 sonnet-4.6 在 AWS Kiro 上游消耗一致（都按 1.3 计 credit）。
// sonnet-4.6 → sonnet-4.5 掺水的利润不靠 multiplier，靠 FREE 池账号成本低于 PRO 池。
// opus-4.6 上游真实消耗高于 sonnet（1.77x），multiplier 用于把 sonnet upstream credits
// 还原成 opus 等价值，使用户按 opus 收费。
func stealthCreditRate(model string) float64 {
	b := strings.ToLower(model)
	switch {
	case strings.Contains(b, "opus-4.6"), strings.Contains(b, "opus-4-6"):
		return 2.3
	case strings.Contains(b, "sonnet-4.6"), strings.Contains(b, "sonnet-4-6"):
		return 1.3
	case strings.Contains(b, "sonnet-4.5"), strings.Contains(b, "sonnet-4-5"):
		return 1.3
	}
	return 1.0
}

// StealthCreditMultiplier scales upstream credits to billing-model equivalent.
// If the request was swapped (e.g. user asked opus, we served sonnet), upstream
// returned credits for the cheap model; multiply by ratio so the user is billed
// as if the original (expensive) model was used.
func StealthCreditMultiplier(billingModel, upstreamModel string) float64 {
	if billingModel == "" || upstreamModel == "" || billingModel == upstreamModel {
		return 1.0
	}
	up := stealthCreditRate(upstreamModel)
	if up <= 0 {
		return 1.0
	}
	return stealthCreditRate(billingModel) / up
}
