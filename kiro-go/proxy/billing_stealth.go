package proxy

import (
	"strings"
)

// stealthCreditRate returns the typical upstream credit cost for one "unit" of work
// for the given model. Used to upscale upstream-reported credits when the model
// was secretly swapped, so the user is billed at the original model's rate.
//
// 经验值：sonnet-4.5 与 sonnet-4.6 在 AWS Kiro 上游消耗一致（都按 1.3 计 credit）。
// sonnet-4.6 → sonnet-4.5 掺水的利润不靠 multiplier，靠 FREE 池账号成本低于 PRO 池。
// opus-4.6 上游真实消耗高于 sonnet（1.77x），multiplier 用于把 sonnet upstream credits
// 还原成 opus 等价值，使用户按 opus 收费。
func stealthCreditRate(model string) float64 {
	b := strings.ToLower(model)
	switch {
	case strings.Contains(b, "opus-4.6"), strings.Contains(b, "opus-4-6"):
		return 2.3
	case strings.Contains(b, "sonnet-4.6"), strings.Contains(b, "sonnet-4-6"):
		return 1.3
	case strings.Contains(b, "sonnet-4.5"), strings.Contains(b, "sonnet-4-5"):
		return 1.3
	}
	return 1.0
}

// StealthCreditMultiplier scales upstream credits to billing-model equivalent.
// If the request was swapped (e.g. user asked opus, we served sonnet), upstream
// returned credits for the cheap model; multiply by ratio so the user is billed
// as if the original (expensive) model was used.
func StealthCreditMultiplier(billingModel, upstreamModel string) float64 {
	if billingModel == "" || upstreamModel == "" || billingModel == upstreamModel {
		return 1.0
	}
	up := stealthCreditRate(upstreamModel)
	if up <= 0 {
		return 1.0
	}
	return stealthCreditRate(billingModel) / up
}
