package config

import (
	"fmt"
	"time"
)

// ==================== Promotion ====================

// GetPromotion 返回当前活动配置（线程安全副本）。未配置则返回 nil。
func GetPromotion() *PromotionConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if cfg.Promotion == nil {
		return nil
	}
	// 返回副本（含白名单深拷贝）
	cp := *cfg.Promotion
	if len(cfg.Promotion.Whitelist) > 0 {
		cp.Whitelist = make([]string, len(cfg.Promotion.Whitelist))
		copy(cp.Whitelist, cfg.Promotion.Whitelist)
	}
	return &cp
}

// UpdatePromotion 更新活动配置（传 nil 视为关闭）。
func UpdatePromotion(p *PromotionConfig, operator string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if p != nil {
		p.UpdatedAt = time.Now().Unix()
		p.UpdatedBy = operator
		// 默认窗口
		if p.RecentCallsDays <= 0 {
			p.RecentCallsDays = 7
		}
	}
	cfg.Promotion = p
	return Save()
}

// AddPromotionWhitelist 把 keyID 加入白名单（去重）。
func AddPromotionWhitelist(keyID, operator string) error {
	if keyID == "" {
		return fmt.Errorf("keyID required")
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if cfg.Promotion == nil {
		cfg.Promotion = &PromotionConfig{Enabled: false, RecentCallsDays: 7}
	}
	for _, k := range cfg.Promotion.Whitelist {
		if k == keyID {
			return nil // 已在
		}
	}
	cfg.Promotion.Whitelist = append(cfg.Promotion.Whitelist, keyID)
	cfg.Promotion.UpdatedAt = time.Now().Unix()
	cfg.Promotion.UpdatedBy = operator
	return Save()
}

// RemovePromotionWhitelist 把 keyID 从白名单移除。
func RemovePromotionWhitelist(keyID, operator string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if cfg.Promotion == nil {
		return nil
	}
	out := cfg.Promotion.Whitelist[:0]
	for _, k := range cfg.Promotion.Whitelist {
		if k != keyID {
			out = append(out, k)
		}
	}
	cfg.Promotion.Whitelist = out
	cfg.Promotion.UpdatedAt = time.Now().Unix()
	cfg.Promotion.UpdatedBy = operator
	return Save()
}
