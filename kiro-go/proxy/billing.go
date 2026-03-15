package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
)

// CalcCost calculates the cost in CNY for a given model and token usage.
func CalcCost(model string, inputTokens, outputTokens int) float64 {
	pricing := config.GetPricing()
	rate, ok := pricing.Models[model]
	if !ok {
		rate = config.ModelPricing{
			InputPricePerM:  pricing.DefaultInput,
			OutputPricePerM: pricing.DefaultOutput,
		}
	}
	cost := float64(inputTokens)/1e6*rate.InputPricePerM +
		float64(outputTokens)/1e6*rate.OutputPricePerM

	// Minimum cost per request (fixed overhead)
	minCost := 0.0001
	if cost < minCost {
		cost = minCost
	}
	return cost
}

// TryDeductBalance checks the key plan and deducts balance if needed.
// Returns (shouldDeduct, success, remaining, cost, error).
// shouldDeduct=false means the key doesn't use credit billing.
func TryDeductBalance(uc *UserContext, model string, inputTokens, outputTokens int) (costCNY float64, err error) {
	if uc == nil || uc.KeyID == "" {
		return 0, nil // no key context, skip billing
	}

	info := config.FindApiKeyByID(uc.KeyID)
	if info == nil {
		return 0, nil
	}

	// Only deduct for credit and hybrid plans
	if info.Plan != "credit" && info.Plan != "hybrid" {
		return 0, nil
	}

	cost := CalcCost(model, inputTokens, outputTokens)
	ok, remaining := config.DeductKeyBalance(uc.KeyID, cost)
	if !ok {
		return cost, fmt.Errorf("insufficient balance (need ¥%.4f, have ¥%.4f)", cost, remaining)
	}

	fmt.Printf("[Billing] key=%s model=%s in=%d out=%d cost=¥%.4f remaining=¥%.4f\n",
		uc.KeyID[:8], model, inputTokens, outputTokens, cost, remaining)

	return cost, nil
}
