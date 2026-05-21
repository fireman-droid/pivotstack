package proxy

import (
	"kiro-api-proxy/config"
	"strings"
	"time"
)

// parseUsageData 解析 kiro-account-manager 导出的 usageData 字段
func parseUsageData(account *config.Account, data map[string]interface{}) {
	// 解析 subscriptionInfo
	if subInfo, ok := data["subscriptionInfo"].(map[string]interface{}); ok {
		if t, ok := subInfo["type"].(string); ok {
			// Q_DEVELOPER_STANDALONE_FREE -> FREE, Q_DEVELOPER_STANDALONE_PRO -> PRO
			switch {
			case strings.Contains(t, "FREE"):
				account.SubscriptionType = "FREE"
			case strings.Contains(t, "PRO_PLUS"):
				account.SubscriptionType = "PRO_PLUS"
			case strings.Contains(t, "PRO"):
				account.SubscriptionType = "PRO"
			default:
				account.SubscriptionType = t
			}
		}
		if title, ok := subInfo["subscriptionTitle"].(string); ok {
			account.SubscriptionTitle = title
		}
	}

	// 解析 daysUntilReset
	if days, ok := data["daysUntilReset"].(float64); ok {
		account.DaysRemaining = int(days)
	}

	// 解析 nextDateReset
	if resetTs, ok := data["nextDateReset"].(float64); ok {
		t := time.Unix(int64(resetTs), 0)
		account.NextResetDate = t.Format("2006-01-02")
	}

	// 解析 usageBreakdownList
	if breakdowns, ok := data["usageBreakdownList"].([]interface{}); ok && len(breakdowns) > 0 {
		if bd, ok := breakdowns[0].(map[string]interface{}); ok {
			// 主额度
			if usage, ok := bd["currentUsage"].(float64); ok {
				account.UsageCurrent = usage
			}
			if limit, ok := bd["usageLimit"].(float64); ok {
				account.UsageLimit = limit
				if limit > 0 {
					account.UsagePercent = account.UsageCurrent / limit
				}
			}

			// 试用额度
			if trial, ok := bd["freeTrialInfo"].(map[string]interface{}); ok {
				if usage, ok := trial["currentUsage"].(float64); ok {
					account.TrialUsageCurrent = usage
				}
				if limit, ok := trial["usageLimit"].(float64); ok {
					account.TrialUsageLimit = limit
					if limit > 0 {
						account.TrialUsagePercent = account.TrialUsageCurrent / limit
					}
				}
				if status, ok := trial["freeTrialStatus"].(string); ok {
					account.TrialStatus = status
				}
				if expiry, ok := trial["freeTrialExpiry"].(float64); ok {
					account.TrialExpiresAt = int64(expiry)
				}
			}
		}
	}
}
