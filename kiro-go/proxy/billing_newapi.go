package proxy

import (
	"fmt"
	"math"
	"strings"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

// NewAPIReservation 是 v5 newapi 渠道的同步预扣 snapshot。
// 一次请求生命周期内，所有用于计价的字段都 freeze 在这里，
// admin 中途改 markup / 改单位换算 / 改全局 PSDPY 不影响在飞行的请求。
type NewAPIReservation struct {
	KeyID           string
	ChannelID       string
	ProviderID      string
	Model           string
	GroupName       string
	UpstreamTokenID int

	Markup                       float64
	QuotaPerUnitDollar           float64
	YuanPerUpstreamDollar        float64
	PivotStackDollarsPerYuanSnap float64

	CompletionRatioSnap float64
	ModelRatioSnap      float64
	GroupRatioSnap      float64

	EstQuota        int64
	PromptTokens    int
	MaxOutputTokens int

	PrePaidUSD float64
	PreGiftUSD float64
	Action     string // "free" | "deduct" | "estimated" | "" (无身份)
	StartedAt  int64

	// 结算状态（billing_audit P0-3）：同一 reservation Reconcile/Refund 只能一次。
	// 防御 defer cleanup、Phase 4b 重跑、worker 重试导致重复扣/退。
	Settled        bool
	SettledPaidUSD float64
	SettledGiftUSD float64
}

type newAPIQuotaRatios struct {
	CompletionRatio float64
	ModelRatio      float64
	GroupRatio      float64
}

// EstimateNewAPIQuota 用上游缓存里的 pricing/group ratio 估算一次请求的 upstream quota。
// 模型必须命中缓存（避免静默按 1.0 默认导致系统性少扣）；group 缺失退化到 1.0 并记 warn。
func EstimateNewAPIQuota(cache *providerCache, model, groupName string, promptTokens, maxOutputTokens int) (int64, error) {
	quota, _, err := estimateNewAPIQuotaWithRatios(cache, model, groupName, promptTokens, maxOutputTokens)
	return quota, err
}

func estimateNewAPIQuotaWithRatios(cache *providerCache, model, groupName string, promptTokens, maxOutputTokens int) (int64, newAPIQuotaRatios, error) {
	groups, models := snapshotNewAPIPricing(cache)
	if len(models) == 0 {
		return 0, newAPIQuotaRatios{}, fmt.Errorf("%w: newapi pricing unavailable", ErrSellPriceMissing)
	}

	target := normalizeChannelModelKey(model)
	var foundModel config.NewAPIModel
	modelFound := false
	for _, m := range models {
		if normalizeChannelModelKey(m.ModelName) == target {
			foundModel = m
			modelFound = true
			break
		}
	}
	if !modelFound {
		return 0, newAPIQuotaRatios{}, fmt.Errorf("%w: model %q not in upstream pricing", ErrSellPriceMissing, model)
	}

	groupRatio := 1.0
	groupFound := false
	trimmedGroup := strings.TrimSpace(groupName)
	for _, g := range groups {
		if strings.TrimSpace(g.Name) == trimmedGroup {
			groupRatio = positiveOrDefault(g.Ratio, 1.0)
			groupFound = true
			break
		}
	}
	if !groupFound && trimmedGroup != "" {
		fmt.Printf("[Billing-NewAPI] group %q missing from cache; using ratio=1.0\n", groupName)
	}

	ratios := newAPIQuotaRatios{
		CompletionRatio: positiveOrDefault(foundModel.CompletionRatio, 1.0),
		ModelRatio:      positiveOrDefault(foundModel.ModelRatio, 1.0),
		GroupRatio:      groupRatio,
	}
	return quotaFromNewAPIRatios(promptTokens, maxOutputTokens, ratios), ratios, nil
}

// PreAuthorizeNewAPIRequest 估算 quota → 计算虚拟$预扣 → 扣 user balance（gift-first 顺序由 DeductKeyBalance 控制）。
//   - keyID == "" / key 不存在 → 返回 nil reservation（无须计费）
//   - 天卡覆盖（action="free"）→ 返回 res.Action="free" 的零扣 reservation
//   - 缓存缺失 / model 缺失 → ErrSellPriceMissing 包装错误
//   - 余额不足 → error，不做部分扣费
func PreAuthorizeNewAPIRequest(
	keyID string,
	ch *NewAPIRuntimeChannel,
	cache *providerCache,
	model string,
	promptTokens, maxOutputTokens int,
) (*NewAPIReservation, error) {
	if keyID == "" {
		return nil, nil
	}
	info := users.OverlayWalletOnKey(config.FindApiKeyByID(keyID))
	if info == nil {
		return nil, nil
	}
	if ch == nil {
		return nil, fmt.Errorf("newapi channel missing")
	}

	pool := ResolveModelPool(model)
	action, err := config.ValidateKeyAccessForModel(info, pool)
	if err != nil {
		return nil, err
	}

	// 防 silent free：provider 单位字段未配 / sync 失败导致 0 时，QuotaToPivotDollars 会返回 0，
	// user 拿到的就是免费请求。必须 fail-closed 拒绝，不允许 fallback 默认值（不同 provider 单位不同）。
	if action != "free" {
		if ch.QuotaPerUnitDollar() <= 0 {
			return nil, fmt.Errorf("%w: provider %q QuotaPerUnitDollar not configured", ErrSellPriceMissing, ch.ProviderID())
		}
		if ch.YuanPerUpstreamDollar() <= 0 {
			return nil, fmt.Errorf("%w: provider %q YuanPerUpstreamDollar not configured", ErrSellPriceMissing, ch.ProviderID())
		}
	}

	res := &NewAPIReservation{
		KeyID:                        keyID,
		ChannelID:                    ch.ID(),
		ProviderID:                   ch.ProviderID(),
		Model:                        model,
		GroupName:                    ch.GroupName(),
		UpstreamTokenID:              ch.UpstreamTokenID(),
		Markup:                       positiveOrDefault(ch.Markup(), 1.0),
		QuotaPerUnitDollar:           ch.QuotaPerUnitDollar(),
		YuanPerUpstreamDollar:        ch.YuanPerUpstreamDollar(),
		PivotStackDollarsPerYuanSnap: config.GetPivotStackDollarsPerYuan(),
		PromptTokens:                 max(0, promptTokens),
		MaxOutputTokens:              max(0, maxOutputTokens),
		Action:                       action,
		StartedAt:                    time.Now().Unix(),
	}

	if action == "free" {
		return res, nil
	}

	estQuota, ratios, err := estimateNewAPIQuotaWithRatios(cache, model, ch.GroupName(), promptTokens, maxOutputTokens)
	if err != nil {
		return nil, err
	}
	res.EstQuota = estQuota
	res.CompletionRatioSnap = ratios.CompletionRatio
	res.ModelRatioSnap = ratios.ModelRatio
	res.GroupRatioSnap = ratios.GroupRatio

	preauthCost := quotaToPivotDollars(estQuota, res)
	ok, remaining, paid, gift := users.DeductWalletBalance(keyID, preauthCost)
	if !ok {
		return nil, fmt.Errorf("insufficient balance (need $%.4f, have $%.4f)", preauthCost, remaining)
	}
	res.PrePaidUSD = paid
	res.PreGiftUSD = gift

	fmt.Printf("[Billing-NewAPI] PreAuth key=%s provider=%s channel=%s model=%s quota=%d cost=$%.6f paid=$%.6f gift=$%.6f\n",
		safeKeyShort(keyID), res.ProviderID, res.ChannelID, model, estQuota, preauthCost, paid, gift)
	return res, nil
}

// ReconcileNewAPIRequest 用实际 token usage 重新计算 quota，差额补/退。
// Phase 4a 用 snapshot ratios 估算 actual quota（上游真实 quota 由 Phase 4b 异步对账拿）。
// 余额不足时只扣可扣部分；剩余 shortfall 仅打印，不入 DebtUSD（Phase 4b 责任）。
func ReconcileNewAPIRequest(res *NewAPIReservation, actual TokenUsage) (paidUSD, giftUSD float64, err error) {
	if res == nil || res.KeyID == "" {
		return 0, 0, nil
	}
	if res.Action == "" || res.Action == "free" {
		return 0, 0, nil
	}
	if res.Settled {
		return res.SettledPaidUSD, res.SettledGiftUSD, nil
	}

	actualQuota := quotaFromNewAPIRatios(actual.InputTokens, actual.OutputTokens, newAPIQuotaRatios{
		CompletionRatio: positiveOrDefault(res.CompletionRatioSnap, 1.0),
		ModelRatio:      positiveOrDefault(res.ModelRatioSnap, 1.0),
		GroupRatio:      positiveOrDefault(res.GroupRatioSnap, 1.0),
	})
	actualCost := quotaToPivotDollars(actualQuota, res)
	preauthCost := res.PrePaidUSD + res.PreGiftUSD
	diff := actualCost - preauthCost

	paid := res.PrePaidUSD
	gift := res.PreGiftUSD

	switch {
	case diff > 0:
		// 少扣 → 补扣 diff。余额不足 → 扣可扣的，剩下 shortfall 等 Phase 4b。
		ok, _, addedPaid, addedGift := users.DeductWalletBalance(res.KeyID, diff)
		if ok {
			paid += addedPaid
			gift += addedGift
			break
		}
		paidAvail, giftAvail, _, _ := users.GetWalletBalance(res.KeyID)
		available := paidAvail + giftAvail
		if available > 0 {
			partial := math.Min(diff, available)
			if ok, _, addedPaid, addedGift := users.DeductWalletBalance(res.KeyID, partial); ok {
				paid += addedPaid
				gift += addedGift
				diff -= partial
			}
		}
		if diff > 0 {
			fmt.Printf("[Billing-NewAPI] Reconcile key=%s UNDERPAID shortfall=$%.6f (debt deferred to Phase 4b)\n",
				safeKeyShort(res.KeyID), diff)
		}
	case diff < 0:
		// 多扣 → 退 over，gift-first（preauth gift 优先消耗 → 退 gift 优先）。
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

	res.Action = "estimated"
	fmt.Printf("[Billing-NewAPI] Reconcile key=%s provider=%s channel=%s model=%s quota=%d cost=$%.6f paid=$%.6f gift=$%.6f\n",
		safeKeyShort(res.KeyID), res.ProviderID, res.ChannelID, res.Model, actualQuota, actualCost, paid, gift)
	res.Settled = true
	res.SettledPaidUSD = paid
	res.SettledGiftUSD = gift
	return paid, gift, nil
}

// RefundNewAPIReservation 全额退回 preauth（请求失败 / 上游 4xx，无成本发生时调用）。
// 幂等（billing_audit P0-3）：Settled 标志守门，重复调用 noop。
func RefundNewAPIReservation(res *NewAPIReservation) {
	if res == nil || res.KeyID == "" {
		return
	}
	if res.Settled {
		return
	}
	if res.PrePaidUSD > 0 || res.PreGiftUSD > 0 {
		_ = users.AddWalletBalance(res.KeyID, res.PrePaidUSD, res.PreGiftUSD)
		fmt.Printf("[Billing-NewAPI] Refund key=%s paid=$%.6f gift=$%.6f\n",
			safeKeyShort(res.KeyID), res.PrePaidUSD, res.PreGiftUSD)
	}
	res.Settled = true
	res.SettledPaidUSD = 0
	res.SettledGiftUSD = 0
}

func quotaToPivotDollars(quota int64, r *NewAPIReservation) float64 {
	if r == nil {
		return 0
	}
	return QuotaToPivotDollars(
		quota,
		r.QuotaPerUnitDollar,
		r.YuanPerUpstreamDollar,
		r.PivotStackDollarsPerYuanSnap,
		r.Markup,
	)
}

func quotaFromNewAPIRatios(promptTokens, outputTokens int, ratios newAPIQuotaRatios) int64 {
	prompt := max(0, promptTokens)
	output := max(0, outputTokens)
	completionRatio := positiveOrDefault(ratios.CompletionRatio, 1.0)
	modelRatio := positiveOrDefault(ratios.ModelRatio, 1.0)
	groupRatio := positiveOrDefault(ratios.GroupRatio, 1.0)
	quotaF := (float64(prompt) + float64(output)*completionRatio) * modelRatio * groupRatio
	return int64(math.Ceil(quotaF))
}

func positiveOrDefault(v, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	return v
}

func snapshotNewAPIPricing(cache *providerCache) ([]config.NewAPIGroup, []config.NewAPIModel) {
	if cache == nil {
		return nil, nil
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return copyNewAPIGroups(cache.Groups), copyNewAPIModels(cache.Models)
}

func newAPIBillingStatus(res *NewAPIReservation) string {
	if res == nil || res.KeyID == "" {
		return ""
	}
	return "estimated"
}
