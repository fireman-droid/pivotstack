package proxy

import (
	"kiro-api-proxy/config"
	"time"
)

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
