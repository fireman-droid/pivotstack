package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
	"math"
	"os"
	"testing"
)

// TestMain initializes config before running tests
func TestMain(m *testing.M) {
	// Use temp file so config.Save() inside UpdatePricing doesn't panic
	tmpDir, _ := os.MkdirTemp("", "billing_test")
	defer os.RemoveAll(tmpDir)
	config.Init(tmpDir + "/config.json") // Init expects file path
	config.UpdatePricing(config.PricingConfig{
		FreePoolPriceUSD: 0.40,
		ProPoolPriceUSD:  2.00,
		ProCostEntries: []config.CostEntry{
			{ID: "t1", Count: 1, CostCNY: 60, Credits: 1500},
		},
		FreeCostEntries: []config.CostEntry{
			{ID: "t2", Count: 100, CostCNY: 9},
		},
	})
	os.Exit(m.Run())
}

// ==================== Pure Function Tests ====================

func TestResolveModelPool(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"claude-sonnet-4.5", "free"},
		{"claude-sonnet-4-5-20250514", "free"},
		{"claude-sonnet-4.5-20250601", "free"},
		{"sonnet-4.5", "free"},
		{"claude-3-sonnet", "free"},
		{"claude-sonnet-4.6", "pro"},
		{"claude-sonnet-4.6-20250601", "pro"},
		{"claude-opus-4.6", "pro"},
		{"claude-opus-4-6", "pro"},
		{"opus-model", "pro"},
		{"gpt-4o", "free"},
		{"", "free"},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := ResolveModelPool(tt.model); got != tt.expected {
				t.Errorf("ResolveModelPool(%q) = %q, want %q", tt.model, got, tt.expected)
			}
		})
	}
}

func TestEstimateCredits(t *testing.T) {
	tests := []struct {
		name       string
		maxTokens  int
		estInput   int
		minCredits float64
		maxCredits float64
	}{
		{"defaults (0,0)", 0, 0, 0.005, 0.02},
		{"small", 1000, 500, 0.005, 0.02},
		{"medium", 4096, 5000, 0.01, 0.05},
		{"large", 16000, 50000, 0.1, 0.3},
		{"huge (100k input)", 16000, 100000, 0.3, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateCredits(tt.maxTokens, tt.estInput)
			if got < tt.minCredits || got > tt.maxCredits {
				t.Errorf("EstimateCredits(%d,%d) = %.6f, want [%.4f,%.4f]", tt.maxTokens, tt.estInput, got, tt.minCredits, tt.maxCredits)
			}
			t.Logf("EstimateCredits(%d,%d) = %.6f credits", tt.maxTokens, tt.estInput, got)
		})
	}
}

func TestPoolPriceAndCost(t *testing.T) {
	// v2 起 PoolPriceUSD 是 deprecated wrapper，返回 DefaultProPriceUSD/DefaultFreePriceUSD（按 pool 兜底）。
	// 默认值贴近实际生产用的（FREE=0.04, PRO=0.20）而非远古 1.0 时代的 0.40/2.00。
	freePrice := PoolPriceUSD("free")
	proPrice := PoolPriceUSD("pro")

	if freePrice != 0.04 {
		t.Errorf("FREE pool price = $%.4f, want $0.04", freePrice)
	}
	if proPrice != 0.20 {
		t.Errorf("PRO pool price = $%.4f, want $0.20", proPrice)
	}

	// CreditsToCostUSD
	if cost := CreditsToCostUSD(10.0, "free"); math.Abs(cost-0.40) > 0.001 {
		t.Errorf("10 credits FREE = $%.4f, want $0.40", cost)
	}
	if cost := CreditsToCostUSD(10.0, "pro"); math.Abs(cost-2.00) > 0.001 {
		t.Errorf("10 credits PRO = $%.4f, want $2.00", cost)
	}
	t.Logf("✅ FREE $%.2f/cr, PRO $%.2f/cr, ratio=%.1fx", freePrice, proPrice, proPrice/freePrice)
}

func TestCalcAdminProfit(t *testing.T) {
	tests := []struct {
		name       string
		totalUSD   float64
		proCredit  float64
		freeCredit float64
	}{
		{"no usage", 0, 0, 0},
		{"free only", 1.0, 0, 25.0},
		{"pro only", 5.0, 25.0, 0},
		{"mixed", 10.0, 30.0, 10.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CalcAdminProfit(tt.totalUSD, tt.proCredit, tt.freeCredit)
			for _, f := range []string{"revenue_usd", "revenue_cny", "pro_cost_cny", "free_cost_cny", "total_cost_cny", "profit_cny", "margin_percent"} {
				if _, ok := r[f]; !ok {
					t.Errorf("missing field %q", f)
				}
			}
			if r["revenue_usd"] != tt.totalUSD {
				t.Errorf("revenue_usd = %.4f, want %.4f", r["revenue_usd"], tt.totalUSD)
			}
			expectedProCost := tt.proCredit * 0.04 // 60/1500
			if math.Abs(r["pro_cost_cny"]-expectedProCost) > 0.001 {
				t.Errorf("pro_cost_cny = %.4f, want %.4f", r["pro_cost_cny"], expectedProCost)
			}
			t.Logf("USD=$%.2f CNY=¥%.2f 成本=¥%.2f 利润=¥%.2f 利润率=%.1f%%",
				r["revenue_usd"], r["revenue_cny"], r["total_cost_cny"], r["profit_cny"], r["margin_percent"])
		})
	}
}

// ==================== Billing Flow Simulation (no real API calls) ====================

func simulateBilling(t *testing.T, name, model string, inputToks, outputToks int, actualCredits, balance float64) {
	t.Run(name, func(t *testing.T) {
		pool := ResolveModelPool(model)
		price := PoolPriceUSD(pool)
		t.Logf("模型=%s 池=%s 单价=$%.2f/cr 余额=$%.2f", model, pool, price, balance)

		// 1. Pre-auth
		estCr := EstimateCredits(outputToks, inputToks)
		estCost := estCr * price
		if balance < estCost {
			t.Logf("❌ 余额不足 (需$%.4f, 有$%.4f) → 拒绝请求", estCost, balance)
			return
		}
		bal1 := balance - estCost
		t.Logf("[预扣] %.4f cr → $%.4f, 余额→$%.4f", estCr, estCost, bal1)

		// 2. Actual cost
		actCost := actualCredits * price
		diff := actCost - estCost
		var balFinal float64
		if diff > 0 {
			balFinal = bal1 - diff
			t.Logf("[结算] 实际 %.4f cr → $%.4f, 补扣$%.4f → 余额$%.4f", actualCredits, actCost, diff, balFinal)
		} else {
			balFinal = bal1 + (-diff)
			t.Logf("[结算] 实际 %.4f cr → $%.4f, 退回$%.4f → 余额$%.4f", actualCredits, actCost, -diff, balFinal)
		}

		totalCharged := balance - balFinal
		if math.Abs(totalCharged-actCost) > 0.0001 {
			t.Errorf("扣费$%.4f ≠ 实际$%.4f", totalCharged, actCost)
		}
		t.Logf("✅ 本次扣费=$%.4f", totalCharged)
	})
}

func TestBillingScenarios(t *testing.T) {
	fmt.Println("\n══════ 计费场景模拟 (零消耗) ══════")
	simulateBilling(t, "FREE池-短对话", "claude-sonnet-4.5", 1000, 2000, 0.015, 10.0)
	simulateBilling(t, "PRO池-短对话", "claude-sonnet-4.6", 1000, 2000, 0.015, 10.0)
	simulateBilling(t, "PRO池-代码审查", "claude-opus-4.6", 50000, 8000, 0.25, 10.0)
	simulateBilling(t, "PRO池-超大Context", "claude-sonnet-4.6", 100000, 16000, 0.8, 5.0)
	simulateBilling(t, "余额不足", "claude-sonnet-4.6", 5000, 4096, 0.05, 0.001)

	t.Run("请求失败-全额退款", func(t *testing.T) {
		bal := 10.0
		est := EstimateCredits(4096, 5000) * PoolPriceUSD("pro")
		bal1 := bal - est
		balRefund := bal1 + est
		if math.Abs(balRefund-bal) > 0.0001 {
			t.Errorf("退款后$%.4f ≠ 原始$%.4f", balRefund, bal)
		}
		t.Logf("✅ 退款正确: $%.4f→预扣→$%.4f→退款→$%.4f", bal, bal1, balRefund)
	})
}

// ==================== Price Comparison Table ====================

func TestPriceTable(t *testing.T) {
	fmt.Println("\n══════ 定价对比表 ══════")
	fp, pp := PoolPriceUSD("free"), PoolPriceUSD("pro")
	fmt.Printf("%-10s | %-14s | %-14s\n", "Credits", "FREE(USD)", "PRO(USD)")
	fmt.Println("-------------------------------------------")
	for _, cr := range []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 50} {
		fmt.Printf("%-10.2f | $%-13.4f | $%-13.4f\n", cr, cr*fp, cr*pp)
	}
	fmt.Printf("\nFREE=$%.2f/cr  PRO=$%.2f/cr  倍率=%.1fx\n", fp, pp, pp/fp)
}

func TestProfitSim(t *testing.T) {
	fmt.Println("\n══════ 利润模拟 ══════")
	for _, s := range []struct {
		n    string
		u, p float64
	}{
		{"轻度(1天)", 2, 5}, {"中度(1天)", 10, 30}, {"重度(1天)", 50, 150}, {"月度(30天)", 300, 900},
	} {
		r := CalcAdminProfit(s.u, s.p, 0)
		fmt.Printf("[%s] 收入$%.0f/¥%.0f 成本¥%.1f 利润¥%.1f 率%.0f%%\n",
			s.n, r["revenue_usd"], r["revenue_cny"], r["total_cost_cny"], r["profit_cny"], r["margin_percent"])
	}
}
