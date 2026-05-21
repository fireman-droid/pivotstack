package proxy

// billingAmount 从 CallLog 推出该次请求"扣了用户多少钱"（虚拟$ 或 legacy credit，看 BillingMode）。
//
// v9 移除 billing_profit.go 后，这个 helper 仍被 billing_audit_test 使用，
// 同时也是 audit / 对账时跨 BillingMode 的统一入口。
//
// 优先级：
//   - "token"        : ChargedUSD > 0 → ChargedUSD；否则回退 CostUSD（早期 token 日志没写 ChargedUSD）
//   - "newapi"       : 同上（NewAPI 渠道也按 token 计费记账）；不读 UpstreamCredits/Credits
//   - "legacy_credits": Credits（旧 credit 时代）
//   - ""(空)          : Credits 兜底（不带 BillingMode 的远古日志）
func billingAmount(log CallLog) float64 {
	switch log.BillingMode {
	case "token", "newapi":
		if log.ChargedUSD != 0 {
			return log.ChargedUSD
		}
		return log.CostUSD
	}
	return log.Credits
}
