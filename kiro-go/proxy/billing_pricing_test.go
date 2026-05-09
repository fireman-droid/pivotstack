package proxy

import (
	"kiro-api-proxy/config"
	"math"
	"testing"
	"time"
)

// 这些测试针对 v2 per-model 定价：
//   - ModelPriceUSD / ModelPriceUSDForKey 三层 fallback
//   - 活动期 + 资格判定 + per-model 单价 + 兜底
//   - stealth 跨 pool 时按 originalModel 取价
//   - PricingConfig / PromotionConfig v1→v2 迁移幂等性 + 等价性
//
// 每个 Test 末尾用 testRestoreConfig 恢复 cfg 状态，防互相污染。

// ============== 基础设施 ==============

// testSetPricing 设置 cfg.Pricing 给单测专用，返回 restore 函数。
func testSetPricing(p config.PricingConfig, supportedModels map[string][]string) func() {
	config.SetSupportedModels(supportedModels)
	return testApplyConfig(&p, nil)
}

func testSetPricingAndPromo(p config.PricingConfig, promo *config.PromotionConfig, supportedModels map[string][]string) func() {
	config.SetSupportedModels(supportedModels)
	return testApplyConfig(&p, promo)
}

func testApplyConfig(p *config.PricingConfig, promo *config.PromotionConfig) func() {
	// 直接通过 UpdatePricing/UpdatePromotion 注入（这俩 API 是公开的，用它们更稳）
	old := config.GetPricing()
	_ = config.UpdatePricing(*p)
	var oldPromo *config.PromotionConfig
	if existing := config.GetPromotion(); existing != nil {
		cp := *existing
		oldPromo = &cp
	}
	if promo != nil {
		_ = config.UpdatePromotion(promo, "test")
	} else {
		_ = config.UpdatePromotion(nil, "test")
	}
	return func() {
		_ = config.UpdatePricing(old)
		_ = config.UpdatePromotion(oldPromo, "test")
	}
}

func defaultSupportedModels() map[string][]string {
	return map[string][]string{
		"pro":  {"claude-sonnet-4.6", "claude-opus-4.6", "claude-opus-4.7"},
		"free": {"claude-sonnet-4.5"},
	}
}

// ============== ModelPriceUSD（无活动）==============

func TestModelPriceUSD_DirectMatch(t *testing.T) {
	restore := testSetPricing(config.PricingConfig{
		ModelPrices: map[string]float64{
			"claude-opus-4.6": 0.30,
		},
		DefaultProPriceUSD:  0.20,
		DefaultFreePriceUSD: 0.04,
	}, defaultSupportedModels())
	defer restore()

	if got := ModelPriceUSD("claude-opus-4.6"); math.Abs(got-0.30) > 0.0001 {
		t.Errorf("ModelPriceUSD(opus-4.6) = %.4f, want 0.30", got)
	}
}

func TestModelPriceUSD_DashDotEquivalent(t *testing.T) {
	restore := testSetPricing(config.PricingConfig{
		ModelPrices: map[string]float64{
			"claude-opus-4-6": 0.30, // 配 '-' 形式
		},
		DefaultProPriceUSD:  0.20,
		DefaultFreePriceUSD: 0.04,
	}, defaultSupportedModels())
	defer restore()

	// 查 '.' 形式应同样命中
	if got := ModelPriceUSD("claude-opus-4.6"); math.Abs(got-0.30) > 0.0001 {
		t.Errorf("expected dash<->dot equivalence, got %.4f", got)
	}
}

func TestModelPriceUSD_FallbackToProDefault(t *testing.T) {
	restore := testSetPricing(config.PricingConfig{
		// ModelPrices 故意不含 opus-4.6
		DefaultProPriceUSD:  0.25,
		DefaultFreePriceUSD: 0.05,
	}, defaultSupportedModels())
	defer restore()

	if got := ModelPriceUSD("claude-opus-4.6"); math.Abs(got-0.25) > 0.0001 {
		t.Errorf("expected fallback to DefaultProPriceUSD=0.25, got %.4f", got)
	}
	if got := ModelPriceUSD("claude-sonnet-4.5"); math.Abs(got-0.05) > 0.0001 {
		t.Errorf("expected fallback to DefaultFreePriceUSD=0.05, got %.4f", got)
	}
}

func TestModelPriceUSD_ThinkingSuffixMatch(t *testing.T) {
	restore := testSetPricing(config.PricingConfig{
		ModelPrices: map[string]float64{
			"claude-opus-4.6": 0.30,
		},
		DefaultProPriceUSD:  0.20,
		DefaultFreePriceUSD: 0.04,
	}, defaultSupportedModels())
	defer restore()

	// thinking 后缀剥离匹配
	if got := ModelPriceUSD("claude-opus-4.6-thinking"); math.Abs(got-0.30) > 0.0001 {
		t.Errorf("expected thinking suffix to match base price 0.30, got %.4f", got)
	}
}

// ============== ModelPriceUSDForKey（含活动）==============

func TestModelPriceUSDForKey_NoPromo(t *testing.T) {
	restore := testSetPricing(config.PricingConfig{
		ModelPrices: map[string]float64{"claude-opus-4.6": 0.30},
		DefaultProPriceUSD: 0.20, DefaultFreePriceUSD: 0.04,
	}, defaultSupportedModels())
	defer restore()

	// 没活动 → 跟 ModelPriceUSD 一样
	if got := ModelPriceUSDForKey("any-key", "claude-opus-4.6"); math.Abs(got-0.30) > 0.0001 {
		t.Errorf("got %.4f, want 0.30", got)
	}
}

func TestModelPriceUSDForKey_PromoEligibleViaWhitelist(t *testing.T) {
	restore := testSetPricingAndPromo(
		config.PricingConfig{
			ModelPrices:        map[string]float64{"claude-opus-4.6": 0.30},
			DefaultProPriceUSD: 0.20, DefaultFreePriceUSD: 0.04,
		},
		&config.PromotionConfig{
			Enabled:             true,
			ModelPrices:         map[string]float64{"claude-opus-4.6": 0.05},
			DefaultProPriceUSD:  0.10,
			DefaultFreePriceUSD: 0.005,
			Whitelist:           []string{"vip-key"},
			RecentCallsDays:     7,
		},
		defaultSupportedModels())
	defer restore()

	// 白名单 key + 该 model 有专属活动价
	if got := ModelPriceUSDForKey("vip-key", "claude-opus-4.6"); math.Abs(got-0.05) > 0.0001 {
		t.Errorf("vip key should hit promo.ModelPrices=0.05, got %.4f", got)
	}
	// 非白名单 → 走原价
	if got := ModelPriceUSDForKey("nope-key", "claude-opus-4.6"); math.Abs(got-0.30) > 0.0001 {
		t.Errorf("non-vip key should hit original 0.30, got %.4f", got)
	}
}

func TestModelPriceUSDForKey_PromoFallbackToDefault(t *testing.T) {
	restore := testSetPricingAndPromo(
		config.PricingConfig{
			ModelPrices:        map[string]float64{"claude-opus-4.6": 0.30, "claude-sonnet-4.6": 0.20},
			DefaultProPriceUSD: 0.20, DefaultFreePriceUSD: 0.04,
		},
		&config.PromotionConfig{
			Enabled: true,
			// promo.ModelPrices 故意不列出 sonnet-4.6
			ModelPrices:         map[string]float64{"claude-opus-4.6": 0.05},
			DefaultProPriceUSD:  0.08, // PRO 池兜底
			DefaultFreePriceUSD: 0.002,
			Whitelist:           []string{"vip-key"},
			RecentCallsDays:     7,
		},
		defaultSupportedModels())
	defer restore()

	// sonnet-4.6 没单独配 promo → 走 promo.DefaultProPriceUSD=0.08
	if got := ModelPriceUSDForKey("vip-key", "claude-sonnet-4.6"); math.Abs(got-0.08) > 0.0001 {
		t.Errorf("expected promo default 0.08, got %.4f", got)
	}
}

func TestModelPriceUSDForKey_PromoOutOfTimeWindow(t *testing.T) {
	now := time.Now().Unix()
	restore := testSetPricingAndPromo(
		config.PricingConfig{
			ModelPrices:        map[string]float64{"claude-opus-4.6": 0.30},
			DefaultProPriceUSD: 0.20, DefaultFreePriceUSD: 0.04,
		},
		&config.PromotionConfig{
			Enabled:             true,
			ModelPrices:         map[string]float64{"claude-opus-4.6": 0.05},
			DefaultProPriceUSD:  0.08,
			Whitelist:           []string{"vip-key"},
			StartTs:             now + 86400, // 还没开始
			RecentCallsDays:     7,
		},
		defaultSupportedModels())
	defer restore()

	if got := ModelPriceUSDForKey("vip-key", "claude-opus-4.6"); math.Abs(got-0.30) > 0.0001 {
		t.Errorf("promo not yet in window, expected original 0.30, got %.4f", got)
	}
}

// ============== Migrate v1→v2 ==============

func TestMigratePricingToModelLevel_OldConfig(t *testing.T) {
	config.SetSupportedModels(defaultSupportedModels())
	p := config.PricingConfig{
		ProPoolPriceUSD:  0.20,
		FreePoolPriceUSD: 0.04,
		ModelMultipliers: map[string]float64{
			"claude-opus-4.7": 1.5, // opus-4.7 加价 50%
		},
	}
	migrated := config.MigratePricingToModelLevel(&p)
	if !migrated {
		t.Fatal("expected migration to happen")
	}
	// opus-4.7 应该是 0.20 × 1.5 = 0.30
	if got := p.ModelPrices["claude-opus-4.7"]; math.Abs(got-0.30) > 0.0001 {
		t.Errorf("opus-4.7 migrated price = %.4f, want 0.30", got)
	}
	// opus-4.6 没乘数 → 0.20 × 1.0 = 0.20
	if got := p.ModelPrices["claude-opus-4.6"]; math.Abs(got-0.20) > 0.0001 {
		t.Errorf("opus-4.6 migrated price = %.4f, want 0.20", got)
	}
	// sonnet-4.5 走 free 池 → 0.04
	if got := p.ModelPrices["claude-sonnet-4.5"]; math.Abs(got-0.04) > 0.0001 {
		t.Errorf("sonnet-4.5 migrated price = %.4f, want 0.04", got)
	}
	// Default 兜底也填了
	if math.Abs(p.DefaultProPriceUSD-0.20) > 0.0001 {
		t.Errorf("DefaultProPriceUSD = %.4f, want 0.20", p.DefaultProPriceUSD)
	}
	if math.Abs(p.DefaultFreePriceUSD-0.04) > 0.0001 {
		t.Errorf("DefaultFreePriceUSD = %.4f, want 0.04", p.DefaultFreePriceUSD)
	}
}

func TestMigratePricingToModelLevel_Idempotent(t *testing.T) {
	config.SetSupportedModels(defaultSupportedModels())
	p := config.PricingConfig{
		ModelPrices:        map[string]float64{"claude-opus-4.6": 0.30},
		DefaultProPriceUSD: 0.20,
		// 故意还带旧字段
		ProPoolPriceUSD: 0.20,
		FreePoolPriceUSD: 0.04,
	}
	if migrated := config.MigratePricingToModelLevel(&p); migrated {
		t.Errorf("already-migrated config should not migrate again")
	}
	// 原 ModelPrices 不动
	if got := p.ModelPrices["claude-opus-4.6"]; math.Abs(got-0.30) > 0.0001 {
		t.Errorf("ModelPrices unchanged: got %.4f, want 0.30", got)
	}
}

func TestMigratePricingToModelLevel_EmptyConfigSkips(t *testing.T) {
	config.SetSupportedModels(defaultSupportedModels())
	p := config.PricingConfig{}
	if migrated := config.MigratePricingToModelLevel(&p); migrated {
		t.Errorf("empty config should not trigger migration")
	}
}

func TestMigratePromotionToModelLevel_OldConfig(t *testing.T) {
	config.SetSupportedModels(defaultSupportedModels())
	p := config.PromotionConfig{
		Enabled:          true,
		ProPoolPriceUSD:  0.05,
		FreePoolPriceUSD: 0.005,
	}
	if migrated := config.MigratePromotionToModelLevel(&p); !migrated {
		t.Fatal("expected migration")
	}
	if math.Abs(p.DefaultProPriceUSD-0.05) > 0.0001 {
		t.Errorf("DefaultProPriceUSD migrated = %.4f, want 0.05", p.DefaultProPriceUSD)
	}
	if math.Abs(p.DefaultFreePriceUSD-0.005) > 0.0001 {
		t.Errorf("DefaultFreePriceUSD migrated = %.4f, want 0.005", p.DefaultFreePriceUSD)
	}
}

// ============== Shadow 校验：LegacyModelPriceUSD ==============

func TestLegacyModelPriceUSD_MatchesV1Formula(t *testing.T) {
	restore := testSetPricing(config.PricingConfig{
		ModelPrices:        map[string]float64{"claude-opus-4.6": 999}, // 故意篡改 v2，确保 legacy 不读它
		DefaultProPriceUSD: 0.20, DefaultFreePriceUSD: 0.04,
		ProPoolPriceUSD:  0.20,
		FreePoolPriceUSD: 0.04,
		ModelMultipliers: map[string]float64{"claude-opus-4.7": 1.5},
	}, defaultSupportedModels())
	defer restore()

	// LegacyModelPriceUSD("opus-4.7") = ProPoolPriceUSD × 1.5 = 0.30
	if got := LegacyModelPriceUSD("claude-opus-4.7"); math.Abs(got-0.30) > 0.0001 {
		t.Errorf("legacy opus-4.7 = %.4f, want 0.30", got)
	}
	// opus-4.6 没乘数 = 0.20 × 1.0 = 0.20
	if got := LegacyModelPriceUSD("claude-opus-4.6"); math.Abs(got-0.20) > 0.0001 {
		t.Errorf("legacy opus-4.6 = %.4f, want 0.20", got)
	}
	// sonnet-4.5 走 FREE = 0.04
	if got := LegacyModelPriceUSD("claude-sonnet-4.5"); math.Abs(got-0.04) > 0.0001 {
		t.Errorf("legacy sonnet-4.5 = %.4f, want 0.04", got)
	}
}

// ============== shadow 一致性：迁移后新公式 == 旧公式 ==============

func TestPricingMigration_ShadowEquivalence(t *testing.T) {
	config.SetSupportedModels(defaultSupportedModels())
	// 模拟 prod 场景：旧 config 有 PoolPrice + Multiplier
	p := config.PricingConfig{
		ProPoolPriceUSD:  0.20,
		FreePoolPriceUSD: 0.04,
		ModelMultipliers: map[string]float64{
			"claude-opus-4.7": 1.5,
		},
	}
	// 迁移
	if !config.MigratePricingToModelLevel(&p) {
		t.Fatal("migration must trigger")
	}
	// 注入 cfg
	restore := testSetPricing(p, defaultSupportedModels())
	defer restore()

	// 对每个 supported model，新公式 ModelPriceUSD vs 旧公式 LegacyModelPriceUSD 必须一致
	allModels := []string{"claude-opus-4.6", "claude-opus-4.7", "claude-sonnet-4.6", "claude-sonnet-4.5"}
	for _, m := range allModels {
		newPrice := ModelPriceUSD(m)
		legacyPrice := LegacyModelPriceUSD(m)
		if math.Abs(newPrice-legacyPrice) > 0.0001 {
			t.Errorf("%s: new=%.4f legacy=%.4f (must be equal after migration)", m, newPrice, legacyPrice)
		}
	}
}
