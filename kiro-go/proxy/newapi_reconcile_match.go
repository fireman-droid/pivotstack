package proxy

import (
	"math"
	"strings"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

func matchUpstreamLog(p *pendingReservation, logs []NewAPILog) (*NewAPILog, bool) {
	if p == nil || p.Reservation == nil {
		return nil, false
	}
	var primary []*NewAPILog
	if p.Reservation.UpstreamTokenID > 0 {
		// codex High 1: token_id 主键路径也必须走 full fingerprint（含 group + ts±30s），
		// 否则同 token 多租户 / stale 同 model 请求会被误唯一匹配 → 扣错钱。
		for i := range logs {
			log := &logs[i]
			if log.TokenID != p.Reservation.UpstreamTokenID {
				continue
			}
			if !newAPILogMatchesFingerprint(log, p, 0.10) {
				continue
			}
			primary = append(primary, log)
		}
	}
	if len(primary) == 1 {
		return primary[0], false
	}
	if len(primary) > 1 {
		if winner := narrowByFingerprint(primary, p); winner != nil {
			return winner, false
		}
		return nil, true
	}

	var fallback []*NewAPILog
	for i := range logs {
		log := &logs[i]
		if newAPILogMatchesFingerprint(log, p, 0.20) {
			fallback = append(fallback, log)
		}
	}
	if len(fallback) == 1 {
		return fallback[0], false
	}
	if len(fallback) > 1 {
		return nil, true
	}
	return nil, false
}

func narrowByFingerprint(candidates []*NewAPILog, p *pendingReservation) *NewAPILog {
	var matches []*NewAPILog
	for _, c := range candidates {
		if newAPILogMatchesFingerprint(c, p, 0.20) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return nil
}

func newAPILogMatchesFingerprint(log *NewAPILog, p *pendingReservation, tokenPct float64) bool {
	if log == nil || p == nil || p.Reservation == nil {
		return false
	}
	res := p.Reservation
	if !newAPIOptionalEqual(res.GroupName, log.Group) {
		return false
	}
	if !newAPIModelCompatible(logModelName(log), res.Model) {
		return false
	}
	if !newAPILogTokensClose(log, p, tokenPct) {
		return false
	}
	created := normalizeNewAPILogCreatedAt(log.CreatedAt)
	if created > 0 && res.StartedAt > 0 && math.Abs(float64(created-res.StartedAt)) > newAPIReconcileTimeWindowSec {
		return false
	}
	return true
}

func newAPILogTokensClose(log *NewAPILog, p *pendingReservation, pct float64) bool {
	if log == nil || p == nil || p.Reservation == nil {
		return false
	}
	return intWithinPercent(log.PromptTokens, p.Reservation.PromptTokens, pct) &&
		intWithinPercent(log.CompletionTokens, p.Reservation.MaxOutputTokens, pct)
}

func intWithinPercent(actual, expected int, pct float64) bool {
	// codex Medium: zero 不应该当作通配符。
	// expected=0：actual 也必须为 0 才匹配（缺失字段对齐）。
	// actual=0 but expected>0：上游 0 token 而 reservation 期望非 0 → 不应匹配。
	if expected <= 0 {
		return actual <= 0
	}
	if actual <= 0 {
		return false
	}
	tolerance := math.Ceil(float64(expected) * pct)
	return math.Abs(float64(actual-expected)) <= tolerance
}

func newAPIOptionalEqual(want, got string) bool {
	want = strings.TrimSpace(want)
	got = strings.TrimSpace(got)
	if want == "" || got == "" {
		return true
	}
	return strings.EqualFold(want, got)
}

func newAPIModelCompatible(logModel, want string) bool {
	logModel = strings.TrimSpace(logModel)
	want = strings.TrimSpace(want)
	if logModel == "" || want == "" {
		return true
	}
	return normalizeChannelModelKey(logModel) == normalizeChannelModelKey(want)
}

func logModelName(log *NewAPILog) string {
	if log == nil {
		return ""
	}
	if strings.TrimSpace(log.ModelName) != "" {
		return log.ModelName
	}
	return log.Model
}

func normalizeNewAPILogCreatedAt(ts int64) int64 {
	switch {
	case ts > 1_000_000_000_000_000:
		return ts / 1_000_000_000
	case ts > 1_000_000_000_000:
		return ts / 1000
	default:
		return ts
	}
}

func applyUpstreamReconcile(p *pendingReservation, m *NewAPILog) reconcileEvent {
	now := time.Now()
	if p == nil || p.Reservation == nil || m == nil {
		return reconcileEvent{Status: "match_error", Error: "missing reservation or upstream match", Timestamp: now.Unix()}
	}
	res := p.Reservation
	keyID := p.KeyID
	if keyID == "" {
		keyID = res.KeyID
	}
	upstreamRealCost := QuotaToPivotDollars(m.Quota, res.QuotaPerUnitDollar, res.YuanPerUpstreamDollar, res.PivotStackDollarsPerYuanSnap, res.Markup)
	bookedSoFar := res.PrePaidUSD + res.PreGiftUSD
	diff := upstreamRealCost - bookedSoFar

	var paidDelta, giftDelta, debtDelta float64
	var errMsg string
	switch {
	case diff > newAPIReconcileMoneyEpsilon:
		ok, _, paid, gift := users.DeductWalletBalance(keyID, diff)
		if ok {
			paidDelta = paid
			giftDelta = gift
			break
		}
		paidAvail, giftAvail, _, _ := users.GetWalletBalance(keyID)
		available := paidAvail + giftAvail
		if available > 0 {
			partial := math.Min(diff, available)
			if ok, _, paid, gift := users.DeductWalletBalance(keyID, partial); ok {
				paidDelta = paid
				giftDelta = gift
			}
		}
		remaining := diff - paidDelta - giftDelta
		if remaining > newAPIReconcileMoneyEpsilon {
			// codex Medium: AccumulateDebtUSD 失败时 debtDelta 不能算"已记账"，
			// 报告值要反映真实状态，否则 admin 报表会以为 debt 已经累积而实际没扣。
			if err := config.AccumulateDebtUSD(keyID, remaining); err != nil {
				errMsg = err.Error()
			} else {
				debtDelta = remaining
			}
		}
	case diff < -newAPIReconcileMoneyEpsilon:
		over := -diff
		refundGift, refundPaid := 0.0, 0.0
		if over <= res.PreGiftUSD {
			refundGift = over
		} else {
			refundGift = res.PreGiftUSD
			refundPaid = over - res.PreGiftUSD
		}
		if err := users.AddWalletBalance(keyID, refundPaid, refundGift); err != nil {
			errMsg = err.Error()
			// codex Medium: refund 失败时 paid/gift delta 不能报告为已退，
			// admin 报表才能准确反映「这笔多扣实际没退回」。
			refundPaid = 0
			refundGift = 0
		}
		paidDelta = -refundPaid
		giftDelta = -refundGift
	}

	res.PrePaidUSD += paidDelta
	res.PreGiftUSD += giftDelta
	status := "reconciled"
	switch {
	case errMsg != "":
		status = "match_error"
	case debtDelta > newAPIReconcileMoneyEpsilon:
		status = "underpaid"
	}
	appendCallLogReconcileEvent(p.RequestID, status, m.Quota, paidDelta, giftDelta, debtDelta)
	return reconcileEvent{
		RequestID:      p.RequestID,
		KeyID:          keyID,
		ChannelID:      res.ChannelID,
		ProviderID:     res.ProviderID,
		Status:         status,
		Attempt:        p.Attempt,
		UpstreamQuota:  m.Quota,
		EstimatedQuota: res.EstQuota,
		PaidUSDDelta:   paidDelta,
		GiftUSDDelta:   giftDelta,
		DebtUSDAdded:   debtDelta,
		Error:          errMsg,
		Timestamp:      now.Unix(),
	}
}
