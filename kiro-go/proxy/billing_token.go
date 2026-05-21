package proxy

import (
	"errors"
	"fmt"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

// ErrSellPriceMissing 表示 token 模式下 model 没配售价 — 必须 fail closed，
// 禁止默认走 0 静默免费（防漏扣）。
var ErrSellPriceMissing = errors.New("no sell price configured for model")

// TokenUsage 描述单次请求的 token 用量。
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// TokenReservation 是一次 token 模式 PreAuthorize 的留存，
// 用于 Reconcile 时回退或补扣。零值（KeyID==""）表示无须结算。
// Action 由 PreAuthorizeTokens 固化（"free" / "deduct"），Reconcile 阶段不再 validate。
// ChannelID 用于标识渠道；InputPerM / OutputPerM 锁住 PreAuth 时价格，
// 避免 admin 中途改价导致预扣与结算价格不一致。
//
// Settled 标识 reservation 已结算或退款 — 重复调用 Reconcile / Refund 直接返回缓存值，
// 防御 defer cleanup、错误重试等场景下的"重复执行 → 重复给钱"漏洞（billing_audit P0-1/2）。
type TokenReservation struct {
	KeyID      string
	ChannelID  string
	Model      string
	Action     string
	EstUsage   TokenUsage
	InputPerM  float64
	OutputPerM float64
	PrePaidUSD float64
	PreGiftUSD float64
	// 结算状态（同一 reservation 只能 Reconcile/Refund 一次）
	Settled         bool
	SettledPaidUSD  float64
	SettledGiftUSD  float64
}

// TokenCost 按售价 × tokens 计算费用（虚拟$）。
// 售价缺失返回 ErrSellPriceMissing（调用方必须 fail closed，禁止 0 静默）。
// 兼容旧调用方 — 仅查全局售价。新代码应使用 TokenCostForChannel。
func TokenCost(model string, usage TokenUsage) (float64, error) {
	return TokenCostForChannel("", model, usage)
}

// TokenCostForChannel 渠道感知的费用计算 — 优先用渠道内部定价，回退到全局售价。
// channelID 为空时退化为 TokenCost 行为。
// 负价格 fail closed（billing_audit P0-5）：admin 误输负数 / config 坏数据不能反向给用户充钱。
func TokenCostForChannel(channelID, model string, usage TokenUsage) (float64, error) {
	price, ok := resolveSellPriceForChannel(channelID, model)
	if !ok {
		return 0, fmt.Errorf("%w: %s (channel=%q)", ErrSellPriceMissing, model, channelID)
	}
	if price.InputPerM < 0 || price.OutputPerM < 0 {
		return 0, fmt.Errorf("invalid sell price (negative): %s in=%.4f out=%.4f (channel=%q)",
			model, price.InputPerM, price.OutputPerM, channelID)
	}
	in := float64(usage.InputTokens) * price.InputPerM / 1_000_000.0
	out := float64(usage.OutputTokens) * price.OutputPerM / 1_000_000.0
	return in + out, nil
}

// PreAuthorizeTokens 兼容旧调用 — 不带 channelID。新代码应使用 PreAuthorizeTokensForChannel。
func PreAuthorizeTokens(keyID, model string, est TokenUsage) (*TokenReservation, error) {
	return PreAuthorizeTokensForChannel(keyID, "", model, est)
}

// PreAuthorizeTokensForChannel 渠道感知的预扣费 — 按渠道内部售价（或全局兜底）计算。
//   - keyID 为空 / key 不存在 → 返回 nil reservation（无须结算）
//   - 天卡覆盖（action=free）→ 返回零值预扣 reservation（结算时跳过）
//   - 售价缺失 → ErrSellPriceMissing
//   - 余额不足 → error
func PreAuthorizeTokensForChannel(keyID, channelID, model string, est TokenUsage) (*TokenReservation, error) {
	if keyID == "" {
		return nil, nil
	}
	info := users.OverlayWalletOnKey(config.FindApiKeyByID(keyID))
	if info == nil {
		return nil, nil
	}

	pool := ResolveModelPool(model)
	action, err := config.ValidateKeyAccessForModel(info, pool)
	if err != nil {
		return nil, err
	}
	price, priceOK := resolveSellPriceForChannel(channelID, model)
	if !priceOK {
		return nil, fmt.Errorf("%w: %s (channel=%q)", ErrSellPriceMissing, model, channelID)
	}
	// 负价格 fail closed（billing_audit P0-5）
	if price.InputPerM < 0 || price.OutputPerM < 0 {
		return nil, fmt.Errorf("invalid sell price (negative): %s in=%.4f out=%.4f (channel=%q)",
			model, price.InputPerM, price.OutputPerM, channelID)
	}
	if action == "free" {
		return &TokenReservation{
			KeyID:      keyID,
			ChannelID:  channelID,
			Model:      model,
			Action:     action,
			EstUsage:   est,
			InputPerM:  price.InputPerM,
			OutputPerM: price.OutputPerM,
		}, nil
	}

	estCost := float64(est.InputTokens)*price.InputPerM/1_000_000.0 +
		float64(est.OutputTokens)*price.OutputPerM/1_000_000.0

	ok, remaining, paid, gift := users.DeductWalletBalance(keyID, estCost)
	if !ok {
		return nil, fmt.Errorf("insufficient balance (need $%.4f, have $%.4f)", estCost, remaining)
	}

	fmt.Printf("[Billing-Token] PreAuth key=%s channel=%s model=%s est_cost=$%.6f paid=$%.6f gift=$%.6f remaining=$%.4f\n",
		safeKeyShort(keyID), channelID, model, estCost, paid, gift, remaining)

	return &TokenReservation{
		KeyID:      keyID,
		ChannelID:  channelID,
		Model:      model,
		Action:     action,
		EstUsage:   est,
		InputPerM:  price.InputPerM,
		OutputPerM: price.OutputPerM,
		PrePaidUSD: paid,
		PreGiftUSD: gift,
	}, nil
}

// ReconcileTokenUsage 按实际 token 用量结算 — 差额补扣 / 多扣退还。
// 返回 (actualPaidUSD, actualGiftUSD, error)。
// Reconcile 阶段不再重复 ValidateKeyAccessForModel — Action 由 PreAuthorize 固化。
//
// 幂等：同一 reservation 重复调用直接返回首次结果（billing_audit P0-2）。
// defer cleanup、上游重试、reconcile worker 重跑等场景都不会重复扣账。
func ReconcileTokenUsage(res *TokenReservation, actual TokenUsage) (float64, float64, error) {
	if res == nil || res.KeyID == "" {
		return 0, 0, nil
	}
	if res.Action == "" || res.Action == "free" {
		return 0, 0, nil
	}
	if res.Settled {
		return res.SettledPaidUSD, res.SettledGiftUSD, nil
	}

	// Reconcile 使用 PreAuth 阶段锁住的价格快照，避免 admin 中途改价影响在途请求。
	actualCost := float64(actual.InputTokens)*res.InputPerM/1_000_000.0 +
		float64(actual.OutputTokens)*res.OutputPerM/1_000_000.0

	pre := res.PrePaidUSD + res.PreGiftUSD
	diff := actualCost - pre

	paid := res.PrePaidUSD
	gift := res.PreGiftUSD

	switch {
	case diff > 0:
		ok, _, addedPaid, addedGift := users.DeductWalletBalance(res.KeyID, diff)
		if !ok {
			fmt.Printf("[Billing-Token] Reconcile key=%s UNDERPAID by $%.6f\n",
				safeKeyShort(res.KeyID), diff)
		} else {
			paid += addedPaid
			gift += addedGift
		}
	case diff < 0:
		over := -diff
		refundGift, refundPaid := 0.0, 0.0
		if over <= res.PreGiftUSD {
			refundGift = over
		} else {
			refundGift = res.PreGiftUSD
			refundPaid = over - res.PreGiftUSD
		}
		_ = users.AddWalletBalance(res.KeyID, refundPaid, refundGift)
		paid -= refundPaid
		gift -= refundGift
	}

	fmt.Printf("[Billing-Token] Reconcile key=%s channel=%s model=%s in=%d out=%d cost=$%.6f paid=$%.6f gift=$%.6f\n",
		safeKeyShort(res.KeyID), res.ChannelID, res.Model, actual.InputTokens, actual.OutputTokens, actualCost, paid, gift)

	res.Settled = true
	res.SettledPaidUSD = paid
	res.SettledGiftUSD = gift
	return paid, gift, nil
}

// RefundTokenReservation 全额退回预扣（请求失败时调用）。
// 幂等：已结算（Reconcile 过或 Refund 过）直接 return（billing_audit P0-1）。
// defer cleanup + 错误退款叠加触发不会多还钱。
func RefundTokenReservation(res *TokenReservation) {
	if res == nil || res.KeyID == "" {
		return
	}
	if res.Settled {
		return
	}
	if res.PrePaidUSD > 0 || res.PreGiftUSD > 0 {
		_ = users.AddWalletBalance(res.KeyID, res.PrePaidUSD, res.PreGiftUSD)
		fmt.Printf("[Billing-Token] Refund key=%s paid=$%.6f gift=$%.6f\n",
			safeKeyShort(res.KeyID), res.PrePaidUSD, res.PreGiftUSD)
	}
	res.Settled = true
	res.SettledPaidUSD = 0
	res.SettledGiftUSD = 0
}

func safeKeyShort(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}
