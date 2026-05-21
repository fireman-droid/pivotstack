package config

import (
	"fmt"
	"time"
)

// ==================== Activation Codes ====================

// GetActivationCodes returns all activation codes.
func GetActivationCodes() []ActivationCode {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	codes := make([]ActivationCode, len(cfg.ActivationCodes))
	copy(codes, cfg.ActivationCodes)
	return codes
}

// AddActivationCode adds a new activation code.
func AddActivationCode(code ActivationCode) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ActivationCodes = append(cfg.ActivationCodes, code)
	return Save()
}

// RedeemActivationCode tries to redeem a code for the given ApiKey ID.
func RedeemActivationCode(codeStr, keyID string) (string, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, ac := range cfg.ActivationCodes {
		if ac.Code == codeStr {
			if ac.Used {
				return "", fmt.Errorf("activation code already used")
			}
			if ac.CodeExpiresAt > 0 && time.Now().Unix() > ac.CodeExpiresAt {
				return "", fmt.Errorf("activation code has expired")
			}

			// Process balance/time addition before deleting
			switch ac.Type {
			case "balance":
				for j, k := range cfg.ApiKeys {
					if k.ID == keyID {
						// 激活码面值即 balance，无任何系统层杠杆。
						// admin 想给 reseller 让利？出卡时手算面值（如客户付 ¥200，admin 给 ¥285 面值卡）。
						// v7: 动态读 PivotStackDollarsPerYuan（已持 cfgLock.Lock），rate 调整后兑换路径自动跟随。
						amountUSD := ac.Amount * pivotStackDollarsPerYuanLocked()

						// v8: 优先把钱写到绑定 user 的钱包（孤儿/子卡走老路径）。
						handled := false
						if boundUserRechargeHook != nil {
							var herr error
							handled, _, herr = boundUserRechargeHook(cfg.ApiKeys[j], WalletDelta{
								Balance:        amountUSD,
								TotalRecharged: amountUSD,
							})
							if herr != nil {
								return "", herr
							}
						}
						if !handled {
							cfg.ApiKeys[j].Balance += amountUSD
							cfg.ApiKeys[j].TotalRecharged += amountUSD
						}

						// Set plan: if already timed → hybrid, otherwise credit
						if cfg.ApiKeys[j].Plan == "timed" || cfg.ApiKeys[j].Plan == "hybrid" {
							cfg.ApiKeys[j].Plan = "hybrid"
						} else {
							cfg.ApiKeys[j].Plan = "credit"
						}
						break
					}
				}
			case "days", "time":
				for j, k := range cfg.ApiKeys {
					if k.ID == keyID {
						base := cfg.ApiKeys[j].ExpiresAt
						now := time.Now().Unix()
						if base < now {
							base = now
						}
						// 单位区分（CodeManagement.vue 行为）：
						//   type=days → ac.Amount 是"天数"，需要 ×86400 转秒
						//   type=time → ac.Amount 已经是"秒"（前端把 天/时/分 折算后送过来），直接加
						// 历史 BUG：曾 days/time 共用 +amount → 30天卡只加30秒；
						// 后修成统一 ×86400 → 反过来 1天卡变 86400天。
						// 回归测试见 TestRedeemActivationCode_DaysAddsCorrectSeconds 与
						// TestRedeemActivationCode_TimeUsesSecondsDirectly。
						var deltaSec int64
						if ac.Type == "days" {
							deltaSec = int64(ac.Amount) * 86400
						} else { // "time"
							deltaSec = int64(ac.Amount)
						}
						cfg.ApiKeys[j].ExpiresAt = base + deltaSec
						// Set plan and tier
						if cfg.ApiKeys[j].Plan == "credit" || cfg.ApiKeys[j].Plan == "hybrid" {
							cfg.ApiKeys[j].Plan = "hybrid"
						} else {
							cfg.ApiKeys[j].Plan = "timed"
						}
						if ac.Tier != "" {
							cfg.ApiKeys[j].Tier = ac.Tier
						}
						break
					}
				}
			default:
				return "", fmt.Errorf("unknown activation code type: %s", ac.Type)
			}

			// Delete the code permanently instead of marking it used
			cfg.ActivationCodes = append(cfg.ActivationCodes[:i], cfg.ActivationCodes[i+1:]...)

			Save()
			return ac.Type, nil
		}
	}
	return "", fmt.Errorf("activation code not found")
}

// DeleteActivationCode deletes an activation code by its code string.
func DeleteActivationCode(codeStr string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, ac := range cfg.ActivationCodes {
		if ac.Code == codeStr {
			cfg.ActivationCodes = append(cfg.ActivationCodes[:i], cfg.ActivationCodes[i+1:]...)
			return Save()
		}
	}
	return nil
}

// CleanupUsedCodes completely removes all voided/used activation codes from the storage.
func CleanupUsedCodes() int {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	var activeCodes []ActivationCode
	removedCount := 0

	for _, ac := range cfg.ActivationCodes {
		if ac.Used {
			removedCount++
		} else {
			activeCodes = append(activeCodes, ac)
		}
	}

	if removedCount > 0 {
		cfg.ActivationCodes = activeCodes
		_ = Save()
	}

	return removedCount
}
