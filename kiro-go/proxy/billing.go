package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
	"strings"
)

const (
	CREDIT_TO_USD = 2.0 // 1 Kiro credit = $2 USD face value
)

// ResolveModelPool determines pool type from model name.
// 4.5 → "free", 4.6/opus → "pro"
func ResolveModelPool(model string) string {
	base := strings.ToLower(model)
	if strings.Contains(base, "4.6") || strings.Contains(base, "opus") {
		return "pro"
	}
	return "free"
}

// PoolPriceUSD returns the USD price per credit for a pool.
func PoolPriceUSD(pool string) float64 {
	p := config.GetPricing()
	if pool == "pro" {
		return p.ProPoolPriceUSD
	}
	return p.FreePoolPriceUSD
}

// CreditsToCostUSD converts credits to USD cost based on pool.
func CreditsToCostUSD(credits float64, pool string) float64 {
	return credits * PoolPriceUSD(pool)
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
// Returns (preChargedUSD, error). Returns 0 if action="free" (no charge needed).
func PreAuthorize(keyID string, model string, maxTokens, estimatedInput int) (float64, error) {
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		return 0, nil
	}

	pool := ResolveModelPool(model)
	action, err := config.ValidateKeyAccessForModel(info, pool)
	if err != nil {
		return 0, err
	}

	if action == "free" {
		return 0, nil // day card covers this, no charge
	}

	// Estimate credits and convert to USD
	estCredits := EstimateCredits(maxTokens, estimatedInput)
	estCostUSD := CreditsToCostUSD(estCredits, pool)

	ok, remaining := config.DeductKeyBalance(keyID, estCostUSD)
	if !ok {
		return 0, fmt.Errorf("insufficient balance (need $%.4f, have $%.4f)", estCostUSD, remaining)
	}

	fmt.Printf("[Billing] PreAuth key=%s model=%s pool=%s est_credits=%.4f est_cost=$%.4f remaining=$%.4f\n",
		keyID[:8], model, pool, estCredits, estCostUSD, remaining)

	return estCostUSD, nil
}

// Reconcile settles the difference between pre-charged and actual cost.
// actualCredits comes from Kiro API response. Returns actual cost in USD.
func Reconcile(keyID, model string, actualCredits, preCharged float64) float64 {
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		return 0
	}

	pool := ResolveModelPool(model)
	action, _ := config.ValidateKeyAccessForModel(info, pool)
	if action == "free" {
		return 0 // no charge for day card users
	}

	actualCostUSD := CreditsToCostUSD(actualCredits, pool)
	diff := actualCostUSD - preCharged

	if diff > 0 {
		ok, _ := config.DeductKeyBalance(keyID, diff)
		if !ok {
			fmt.Printf("[Billing] Reconcile key=%s UNDERPAID by $%.4f\n", keyID[:8], diff)
		}
	} else if diff < 0 {
		config.AddKeyBalance(keyID, -diff)
	}

	fmt.Printf("[Billing] Reconcile key=%s pool=%s actual_credits=%.4f actual_cost=$%.4f preCharged=$%.4f diff=$%.4f\n",
		keyID[:8], pool, actualCredits, actualCostUSD, preCharged, diff)

	return actualCostUSD
}

// RefundPreAuth fully refunds a pre-authorized amount (on request failure).
func RefundPreAuth(keyID string, preCharged float64) {
	if preCharged > 0 && keyID != "" {
		config.AddKeyBalance(keyID, preCharged)
		fmt.Printf("[Billing] Refund key=%s amount=$%.4f\n", keyID[:8], preCharged)
	}
}

// TryDeductBalance is the simple post-request deduction (fallback if pre-auth not used).
// Uses actual credits from Kiro API. Returns (costUSD, error).
func TryDeductBalance(uc *UserContext, model string, actualCredits float64) (float64, error) {
	if uc == nil || uc.KeyID == "" {
		return 0, nil
	}

	info := config.FindApiKeyByID(uc.KeyID)
	if info == nil {
		return 0, nil
	}

	pool := ResolveModelPool(model)
	action, err := config.ValidateKeyAccessForModel(info, pool)
	if err != nil {
		return 0, err
	}
	if action == "free" {
		return 0, nil
	}

	costUSD := CreditsToCostUSD(actualCredits, pool)
	ok, remaining := config.DeductKeyBalance(uc.KeyID, costUSD)
	if !ok {
		return costUSD, fmt.Errorf("insufficient balance (need $%.4f, have $%.4f)", costUSD, remaining)
	}

	fmt.Printf("[Billing] key=%s model=%s pool=%s credits=%.4f cost=$%.4f remaining=$%.4f\n",
		uc.KeyID[:8], model, pool, actualCredits, costUSD, remaining)

	return costUSD, nil
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
