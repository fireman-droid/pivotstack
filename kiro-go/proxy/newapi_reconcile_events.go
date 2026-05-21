package proxy

import (
	"time"
)

func (q *providerReconcileQueue) recordEventLocked(ev reconcileEvent) {
	if ev.Timestamp == 0 {
		ev.Timestamp = time.Now().Unix()
	}
	// gemini: 错误类事件累计到 errorCount，admin UI 可以一眼看出 provider 健康度。
	if ev.Status == "match_error" || ev.Status == "ambiguous" || ev.Status == "no_match" {
		q.errorCount++
	}
	q.recent = append(q.recent, ev)
	if len(q.recent) > newAPIReconcileEventLimit {
		q.recent = q.recent[len(q.recent)-newAPIReconcileEventLimit:]
	}
	if ev.KeyID != "" && ev.DebtUSDAdded > 0 {
		if q.debtCounts == nil {
			q.debtCounts = make(map[string]float64)
		}
		q.debtCounts[ev.KeyID] += ev.DebtUSDAdded
	}
}

// copyRecentEventsLocked 返回 reverse-chronological（最新在前），admin UI 不需要再 reverse。
func (q *providerReconcileQueue) copyRecentEventsLocked(limit int) []reconcileEvent {
	if limit <= 0 || limit > len(q.recent) {
		limit = len(q.recent)
	}
	out := make([]reconcileEvent, limit)
	for i := 0; i < limit; i++ {
		out[i] = q.recent[len(q.recent)-1-i]
	}
	return out
}

func (q *providerReconcileQueue) totalDebtAddedLocked() float64 {
	var total float64
	for _, v := range q.debtCounts {
		total += v
	}
	return total
}

func eventFromPending(p *pendingReservation, status, errMsg string, at time.Time) reconcileEvent {
	ev := reconcileEvent{Status: status, Error: errMsg, Timestamp: at.Unix()}
	if p == nil {
		return ev
	}
	ev.RequestID = p.RequestID
	ev.KeyID = p.KeyID
	ev.Attempt = p.Attempt
	if p.Reservation != nil {
		ev.ChannelID = p.Reservation.ChannelID
		ev.ProviderID = p.Reservation.ProviderID
		ev.EstimatedQuota = p.Reservation.EstQuota
	}
	return ev
}
