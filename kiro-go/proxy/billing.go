package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
)

// PricingSnapshot captures pricing at request start to avoid mid-request changes.
type PricingSnapshot struct {
	Models         map[string]config.ModelPricing
	DefaultInput   float64
	DefaultOutput  float64
	MinRequestCost float64
}

// SnapshotPricing captures the current pricing config for use during a request.
func SnapshotPricing() PricingSnapshot {
	p := config.GetPricing()
	snap := PricingSnapshot{
		DefaultInput:   p.DefaultInput,
		DefaultOutput:  p.DefaultOutput,
		MinRequestCost: p.MinRequestCost,
		Models:         make(map[string]config.ModelPricing),
	}
	for k, v := range p.Models {
		snap.Models[k] = v
	}
	return snap
}

// CalcCostWithSnapshot calculates the cost in CNY using a pricing snapshot.
func CalcCostWithSnapshot(snap PricingSnapshot, model string, inputTokens, outputTokens int) float64 {
	rate, ok := snap.Models[model]
	if !ok {
		rate = config.ModelPricing{
			InputPricePerM:  snap.DefaultInput,
			OutputPricePerM: snap.DefaultOutput,
		}
	}
	cost := float64(inputTokens)/1e6*rate.InputPricePerM +
		float64(outputTokens)/1e6*rate.OutputPricePerM

	minCost := snap.MinRequestCost
	if minCost <= 0 {
		minCost = 0.0001
	}
	if cost < minCost {
		cost = minCost
	}
	return cost
}

// CalcCost calculates the cost using live pricing (for backward compat).
func CalcCost(model string, inputTokens, outputTokens int) float64 {
	return CalcCostWithSnapshot(SnapshotPricing(), model, inputTokens, outputTokens)
}

// PreAuthorize pre-deducts estimated cost at request start.
// Uses max_tokens to estimate output cost. Returns (preChargedAmount, pricingSnapshot, error).
func PreAuthorize(keyID string, model string, maxTokens int, estimatedInputTokens int) (float64, PricingSnapshot, error) {
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		return 0, PricingSnapshot{}, nil
	}
	if info.Plan != "credit" && info.Plan != "hybrid" {
		return 0, SnapshotPricing(), nil // no pre-auth for timed plans
	}

	snap := SnapshotPricing()

	// Estimate: use requested max_tokens as output estimate, 1000 as default input estimate
	estInput := estimatedInputTokens
	if estInput <= 0 {
		estInput = 1000
	}
	estOutput := maxTokens
	if estOutput <= 0 {
		estOutput = 4096 // default max_tokens
	}

	estimatedCost := CalcCostWithSnapshot(snap, model, estInput, estOutput)

	ok, remaining := config.DeductKeyBalance(keyID, estimatedCost)
	if !ok {
		return 0, snap, fmt.Errorf("insufficient balance (need ¥%.4f estimated, have ¥%.4f)", estimatedCost, remaining)
	}

	fmt.Printf("[Billing] PreAuth key=%s model=%s est_cost=¥%.4f remaining=¥%.4f\n",
		keyID[:8], model, estimatedCost, remaining)

	return estimatedCost, snap, nil
}

// Reconcile settles the difference between pre-charged and actual cost.
// If actual < preCharged, refunds the difference. If actual > preCharged, deducts more.
func Reconcile(keyID string, snap PricingSnapshot, model string, inputTokens, outputTokens int, preCharged float64) float64 {
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		return 0
	}
	if info.Plan != "credit" && info.Plan != "hybrid" {
		return 0
	}

	actualCost := CalcCostWithSnapshot(snap, model, inputTokens, outputTokens)
	diff := actualCost - preCharged

	if diff > 0 {
		// Need to charge more
		ok, _ := config.DeductKeyBalance(keyID, diff)
		if !ok {
			// Can't charge more, but request already completed - accept the loss
			fmt.Printf("[Billing] Reconcile key=%s UNDERPAID by ¥%.4f\n", keyID[:8], diff)
		}
	} else if diff < 0 {
		// Refund overpayment
		config.AddKeyBalance(keyID, -diff)
	}

	fmt.Printf("[Billing] Reconcile key=%s actual=¥%.4f preCharged=¥%.4f diff=¥%.4f\n",
		keyID[:8], actualCost, preCharged, diff)

	return actualCost
}

// TryDeductBalance is the simple post-request deduction (fallback for non-preauth flows).
func TryDeductBalance(uc *UserContext, model string, inputTokens, outputTokens int) (costCNY float64, err error) {
	if uc == nil || uc.KeyID == "" {
		return 0, nil
	}

	info := config.FindApiKeyByID(uc.KeyID)
	if info == nil {
		return 0, nil
	}

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
