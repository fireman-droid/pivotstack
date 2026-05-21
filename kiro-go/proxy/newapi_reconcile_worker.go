package proxy

import (
	"context"
	"kiro-api-proxy/config"
	"strings"
	"time"
)

func (r *NewAPIReconciler) ensureQueue(providerID string) *providerReconcileQueue {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ensureQueueLocked(providerID)
}

func (r *NewAPIReconciler) ensureQueueLocked(providerID string) *providerReconcileQueue {
	if r.queues == nil {
		r.queues = make(map[string]*providerReconcileQueue)
	}
	q := r.queues[providerID]
	if q == nil {
		q = &providerReconcileQueue{
			pending:    make(map[string]*pendingReservation),
			debtCounts: make(map[string]float64),
		}
		r.queues[providerID] = q
	}
	return q
}

func (r *NewAPIReconciler) queue(providerID string) (*providerReconcileQueue, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q, ok := r.queues[providerID]
	return q, ok
}

func (r *NewAPIReconciler) startWorkerLocked(providerID string) {
	if r.manager == nil || r.ctx == nil || providerID == "" {
		return
	}
	if _, exists := r.workers[providerID]; exists {
		return
	}
	r.workers[providerID] = struct{}{}
	ctx := r.ctx
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer func() {
			r.mu.Lock()
			delete(r.workers, providerID)
			r.mu.Unlock()
			if recovered := recover(); recovered != nil {
				r.recordProviderError(providerID, "reconcile worker panic")
			}
		}()
		r.runProvider(ctx, providerID)
	}()
}

func (r *NewAPIReconciler) runProvider(ctx context.Context, providerID string) {
	interval := r.pollInterval
	if interval <= 0 {
		interval = defaultNewAPIReconcilePollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.pollProvider(ctx, providerID)
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		}
	}
}

func (r *NewAPIReconciler) pollProvider(ctx context.Context, providerID string) {
	if r == nil || r.manager == nil || !r.providerHasPending(providerID) {
		return
	}
	if _, ok := r.manager.Cache(providerID); !ok {
		return
	}
	provider, ok := config.GetNewAPIProvider(providerID)
	if !ok || !provider.Enabled || provider.AccessTokenEnc == "" {
		return
	}
	accessToken, err := config.DecryptSecret(provider.AccessTokenEnc)
	if err != nil {
		r.recordProviderError(providerID, err.Error())
		return
	}
	client := r.manager.client
	if client == nil {
		client = NewNewAPIClient()
	}
	pollCtx, cancel := context.WithTimeout(ctx, newAPIReconcileFetchTimeout)
	defer cancel()
	logs, err := client.FetchRecentLogs(pollCtx, provider.BaseURL, accessToken, provider.UserID, map[string]string{
		"page_size": newAPIReconcileLogPageSize,
	})
	if err != nil {
		r.recordProviderError(providerID, err.Error())
		return
	}
	r.processFetchedLogs(providerID, logs, time.Now())
}

func (r *NewAPIReconciler) providerHasPending(providerID string) bool {
	q, ok := r.queue(providerID)
	if !ok {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending) > 0
}

func (r *NewAPIReconciler) processFetchedLogs(providerID string, logs []NewAPILog, now time.Time) {
	q, ok := r.queue(providerID)
	if !ok {
		return
	}
	retryBudget := r.retryBudget
	if retryBudget <= 0 {
		retryBudget = 1
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for reqID, p := range q.pending {
		if p == nil || p.Reservation == nil {
			q.recordEventLocked(reconcileEvent{RequestID: reqID, ProviderID: providerID, Status: "match_error", Error: "missing pending reservation", Timestamp: now.Unix()})
			delete(q.pending, reqID)
			continue
		}
		if p.EnqueuedAt.IsZero() {
			p.EnqueuedAt = now
		}
		if p.Attempt == 0 && r.pollDelayFirst > 0 && now.Sub(p.EnqueuedAt) < r.pollDelayFirst {
			continue
		}
		match, ambiguous := matchUpstreamLog(p, logs)
		if ambiguous {
			p.Attempt++
			p.LastErr = "ambiguous upstream log match"
			if p.Attempt >= retryBudget {
				q.recordEventLocked(eventFromPending(p, "ambiguous", p.LastErr, now))
				// codex High 2 + gemini: 给放弃对账的请求也写 reconcile event，
				// admin UI 才能区分「待对账」vs「已放弃」（jsonl status=expired_estimated）。
				appendCallLogReconcileEvent(reqID, "expired_estimated", 0, 0, 0, 0)
				delete(q.pending, reqID)
			}
			continue
		}
		if match == nil {
			p.Attempt++
			p.LastErr = "no upstream log match"
			if p.Attempt >= retryBudget {
				q.recordEventLocked(eventFromPending(p, "no_match", p.LastErr, now))
				appendCallLogReconcileEvent(reqID, "expired_estimated", 0, 0, 0, 0)
				delete(q.pending, reqID)
			}
			continue
		}
		ev := applyUpstreamReconcile(p, match)
		if ev.Timestamp == 0 {
			ev.Timestamp = now.Unix()
		}
		q.recordEventLocked(ev)
		delete(q.pending, reqID)
	}
}

func (r *NewAPIReconciler) recordProviderError(providerID, msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	msg = bearerSecretRegex.ReplaceAllString(msg, "Bearer sk-***")
	q := r.ensureQueue(providerID)
	q.mu.Lock()
	q.recordEventLocked(reconcileEvent{ProviderID: providerID, Status: "match_error", Error: msg, Timestamp: time.Now().Unix()})
	q.mu.Unlock()
}
