package config

import (
	"fmt"
	"strings"
	"time"
)

// ProCostPerCredit returns the admin's cost per credit for PRO accounts (in CNY).
func (p PricingConfig) ProCostPerCredit() float64 {
	var totalCost float64
	var totalCredits float64
	for _, e := range p.ProCostEntries {
		totalCost += e.CostCNY
		totalCredits += float64(e.Count) * e.Credits
	}
	if totalCredits > 0 {
		return totalCost / totalCredits
	}
	// fallback to old fields
	if p.ProAccountCredits > 0 && p.ProAccountPriceCNY > 0 {
		return p.ProAccountPriceCNY / p.ProAccountCredits
	}
	if p.PurchasePriceCNY > 0 {
		return p.PurchasePriceCNY
	}
	return 0.04
}

// FreeCostPerCredit returns the admin's cost per credit for FREE accounts (in CNY).
func (p PricingConfig) FreeCostPerCredit() float64 {
	var totalCost float64
	var totalCredits float64
	for _, e := range p.FreeCostEntries {
		totalCost += e.CostCNY
		totalCredits += float64(e.Count) * FreeAccountCredits
	}
	if totalCredits > 0 {
		return totalCost / totalCredits
	}
	// fallback
	if p.FreeAccountBatchCount > 0 && p.FreeAccountCredits > 0 && p.FreeAccountBatchPrice > 0 {
		return p.FreeAccountBatchPrice / (float64(p.FreeAccountBatchCount) * p.FreeAccountCredits)
	}
	return 0.0002
}

// ProTotalCost returns total investment in PRO accounts (CNY).
func (p PricingConfig) ProTotalCost() float64 {
	var total float64
	for _, e := range p.ProCostEntries {
		total += e.CostCNY
	}
	return total
}

// FreeTotalCost returns total investment in FREE accounts (CNY).
func (p PricingConfig) FreeTotalCost() float64 {
	var total float64
	for _, e := range p.FreeCostEntries {
		total += e.CostCNY
	}
	return total
}

// GetPricing returns the pricing configuration with defaults.
//
// 迁移由 MaybeMigratePricing 在程序启动时显式触发（main.go），此处只做"读 + 兜底默认值"。
func GetPricing() PricingConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	p := cfg.Pricing
	// v2 默认值
	if p.DefaultProPriceUSD == 0 {
		p.DefaultProPriceUSD = 0.20
	}
	if p.DefaultFreePriceUSD == 0 {
		p.DefaultFreePriceUSD = 0.04
	}
	// v1 deprecated 默认值（仍然填，给报表/外部脚本兜底）
	if p.FreePoolPriceUSD == 0 {
		p.FreePoolPriceUSD = 0.40
	}
	if p.ProPoolPriceUSD == 0 {
		p.ProPoolPriceUSD = 2.00
	}
	return p
}

// MaybeMigratePricing 启动时显式调用一次（main.go 在 SetSupportedModels 之后）。
// 检测旧字段是否需要迁移到 v2 ModelPrices，迁移则持久化到磁盘。
//
// 返回 (migrated, err)：migrated=true 表示真的发生了迁移并写盘成功。
func MaybeMigratePricing() (bool, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	pricingMigrated := MigratePricingToModelLevel(&cfg.Pricing)
	promoMigrated := false
	if cfg.Promotion != nil {
		promoMigrated = MigratePromotionToModelLevel(cfg.Promotion)
	}
	if !pricingMigrated && !promoMigrated {
		return false, nil
	}
	if err := Save(); err != nil {
		return true, fmt.Errorf("save after migrate: %w", err)
	}
	return true, nil
}

// AddCostEntry adds a cost entry to PRO or FREE list.
func AddCostEntry(pool string, entry CostEntry) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if pool == "pro" {
		cfg.Pricing.ProCostEntries = append(cfg.Pricing.ProCostEntries, entry)
	} else {
		cfg.Pricing.FreeCostEntries = append(cfg.Pricing.FreeCostEntries, entry)
	}
	return Save()
}

// RemoveCostEntry removes a cost entry by ID.
func RemoveCostEntry(pool, id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if pool == "pro" {
		for i, e := range cfg.Pricing.ProCostEntries {
			if e.ID == id {
				cfg.Pricing.ProCostEntries = append(cfg.Pricing.ProCostEntries[:i], cfg.Pricing.ProCostEntries[i+1:]...)
				return Save()
			}
		}
	} else {
		for i, e := range cfg.Pricing.FreeCostEntries {
			if e.ID == id {
				cfg.Pricing.FreeCostEntries = append(cfg.Pricing.FreeCostEntries[:i], cfg.Pricing.FreeCostEntries[i+1:]...)
				return Save()
			}
		}
	}
	return fmt.Errorf("entry not found: %s", id)
}

// UpdatePricing updates the pricing configuration.
func UpdatePricing(p PricingConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Pricing = p
	return Save()
}

// GetSellPrice 查全局售价（兜底用）。优先用 GetSellPriceForChannel。
func GetSellPrice(model string) (ModelSellPrice, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return lookupSellPriceLocked(cfg.Pricing.SellPrices, model)
}

// GetSellPriceForChannel 渠道感知的售价查找。
//
// v4 严格语义（防漏扣 + 防偷偷用全局价兜底）：
//   - channelID != "" 时：只查 channel.ModelPrices[model]，缺则 ok=false（fail closed，不 fallback 全局）
//   - channelID == "" 时：legacy/孤儿路径，查 pricing.SellPrices[model]
//
// 调用方拿到 ok=false 应该返回 ErrSellPriceMissing 并拒绝请求（不调上游、不扣费）。
//
// Why v4 严格化：channel-routed 请求若 fallback 到全局 SellPrices，同 model 不同渠道
// 会被收一样的钱，违反「按渠道独立定价」核心商业逻辑。
func GetSellPriceForChannel(channelID, model string) (ModelSellPrice, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if channelID != "" {
		for _, ch := range cfg.Channels {
			if ch.ID != channelID {
				continue
			}
			return lookupSellPriceLocked(ch.ModelPrices, model)
		}
		// channelID 指定但 channel 不存在 → fail closed
		return ModelSellPrice{}, false
	}
	// legacy / 无 channel 路径
	return lookupSellPriceLocked(cfg.Pricing.SellPrices, model)
}

// lookupSellPriceLocked 在给定 map 里查 model 单价（'-/.' 互换、大小写不敏感、剥离 thinking 后缀）。
// 调用方必须持有 cfgLock。
func lookupSellPriceLocked(m map[string]ModelSellPrice, model string) (ModelSellPrice, bool) {
	if len(m) == 0 || model == "" {
		return ModelSellPrice{}, false
	}
	low := strings.ToLower(strings.TrimSpace(model))
	if v, ok := m[low]; ok {
		return v, true
	}
	target := normalizeSellPriceKey(low)
	for k, v := range m {
		if normalizeSellPriceKey(strings.ToLower(k)) == target {
			return v, true
		}
	}
	stripped := strings.TrimSuffix(strings.TrimSuffix(low, "-thinking"), "-think")
	if stripped != low {
		if v, ok := m[stripped]; ok {
			return v, true
		}
		st := normalizeSellPriceKey(stripped)
		for k, v := range m {
			if normalizeSellPriceKey(strings.ToLower(k)) == st {
				return v, true
			}
		}
	}
	return ModelSellPrice{}, false
}

func normalizeSellPriceKey(s string) string {
	return strings.ReplaceAll(s, "-", ".")
}
