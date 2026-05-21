package proxy

import (
	"fmt"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

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
	info := users.OverlayWalletOnKey(config.FindApiKeyByID(keyID))
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

	ok, remaining, paidDeducted, giftDeducted := users.DeductWalletBalance(keyID, estCostUSD)
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
	info := users.OverlayWalletOnKey(config.FindApiKeyByID(keyID))
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
		ok, _, addedPaid, addedGift := users.DeductWalletBalance(keyID, diff)
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

		_ = users.AddWalletBalance(keyID, refundPaid, refundGift)
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
		_ = users.AddWalletBalance(keyID, preChargedPaid, preChargedGift)
		fmt.Printf("[Billing] Refund key=%s paid=$%.4f gift=$%.4f\n", keyID[:8], preChargedPaid, preChargedGift)
	}
}

// TryDeductBalance is the simple post-request deduction (fallback if pre-auth not used).
// Uses actual credits from Kiro API. Returns (paidCostUSD, giftCostUSD, error).
func TryDeductBalance(uc *UserContext, model string, actualCredits float64) (float64, float64, error) {
	if uc == nil || uc.KeyID == "" {
		return 0, 0, nil
	}

	info := users.OverlayWalletOnKey(config.FindApiKeyByID(uc.KeyID))
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
	ok, remaining, paidCost, giftCost := users.DeductWalletBalance(uc.KeyID, costUSD)
	if !ok {
		return 0, 0, fmt.Errorf("insufficient balance (need $%.4f, have $%.4f)", costUSD, remaining)
	}

	fmt.Printf("[Billing] key=%s model=%s pool=%s credits=%.4f cost=$%.4f remaining=$%.4f (paid=$%.4f, gift=$%.4f)\n",
		uc.KeyID[:8], model, pool, actualCredits, costUSD, remaining, paidCost, giftCost)

	return paidCost, giftCost, nil
}
