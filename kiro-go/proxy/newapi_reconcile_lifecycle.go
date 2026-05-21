package proxy

import (
	"context"
	"kiro-api-proxy/config"
	"sort"
	"strings"
	"time"
)

func NewNewAPIReconciler(manager *NewAPIManager) *NewAPIReconciler {
	return &NewAPIReconciler{
		manager:        manager,
		queues:         make(map[string]*providerReconcileQueue),
		pollInterval:   defaultNewAPIReconcilePollInterval,
		pollDelayFirst: defaultNewAPIReconcilePollDelay,
		retryBudget:    defaultNewAPIReconcileRetryBudget,
		stopCh:         make(chan struct{}),
		workers:        make(map[string]struct{}),
	}
}

func (r *NewAPIReconciler) Start(ctx context.Context) {
	if r == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	providers := config.GetNewAPIProviders()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	}
	if r.queues == nil {
		r.queues = make(map[string]*providerReconcileQueue)
	}
	if r.workers == nil {
		r.workers = make(map[string]struct{})
	}
	if r.stopCh == nil {
		r.stopCh = make(chan struct{})
	}
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.started = true
	for _, p := range providers {
		r.ensureQueueLocked(p.ID)
		if p.Enabled {
			r.startWorkerLocked(p.ID)
		}
	}
}

func (r *NewAPIReconciler) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.stopCh != nil {
			close(r.stopCh)
		}
		r.wg.Wait()
	})
}

func (r *NewAPIReconciler) Enqueue(providerID string, p *pendingReservation) bool {
	if r == nil || p == nil || p.Reservation == nil || strings.TrimSpace(providerID) == "" || strings.TrimSpace(p.RequestID) == "" {
		return false
	}
	if p.KeyID == "" {
		p.KeyID = p.Reservation.KeyID
	}
	if p.EnqueuedAt.IsZero() {
		p.EnqueuedAt = time.Now()
	}

	q := r.ensureQueue(providerID)
	q.mu.Lock()
	if _, exists := q.pending[p.RequestID]; exists {
		q.mu.Unlock()
		return false
	}
	q.pending[p.RequestID] = p
	q.mu.Unlock()

	r.mu.Lock()
	if r.started {
		r.startWorkerLocked(providerID)
	}
	r.mu.Unlock()
	return true
}

func (r *NewAPIReconciler) Retry(requestID string) bool {
	if r == nil || strings.TrimSpace(requestID) == "" {
		return false
	}
	type queueRef struct {
		q *providerReconcileQueue
	}
	var refs []queueRef
	r.mu.Lock()
	for _, q := range r.queues {
		refs = append(refs, queueRef{q: q})
	}
	r.mu.Unlock()

	for _, ref := range refs {
		ref.q.mu.Lock()
		if p := ref.q.pending[requestID]; p != nil {
			p.Attempt = 0
			p.LastErr = ""
			p.EnqueuedAt = time.Now().Add(-r.pollDelayFirst)
			ref.q.mu.Unlock()
			return true
		}
		ref.q.mu.Unlock()
	}
	return false
}

func (r *NewAPIReconciler) SnapshotStatus() []reconcileProviderStatus {
	if r == nil {
		return []reconcileProviderStatus{}
	}
	type item struct {
		id string
		q  *providerReconcileQueue
	}
	var items []item
	r.mu.Lock()
	for id, q := range r.queues {
		items = append(items, item{id: id, q: q})
	}
	r.mu.Unlock()
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })

	out := make([]reconcileProviderStatus, 0, len(items))
	for _, it := range items {
		it.q.mu.Lock()
		pendingCount := len(it.q.pending)
		recentEvents := it.q.copyRecentEventsLocked(newAPIReconcileAdminEventLimit)
		debtAdded := it.q.totalDebtAddedLocked()
		errCount := it.q.errorCount
		it.q.mu.Unlock()

		// gemini UX: 完全 idle 的 provider 不显示，避免 admin UI 噪声。
		// 有 pending / 有近期事件 / 有 debt / 有 error 任一非零都保留。
		if pendingCount == 0 && len(recentEvents) == 0 && debtAdded == 0 && errCount == 0 {
			continue
		}

		out = append(out, reconcileProviderStatus{
			ProviderID:           it.id,
			PendingCount:         pendingCount,
			RecentEvents:         recentEvents,
			DebtAddedThisSession: debtAdded,
			ErrorCount:           errCount,
		})
	}
	return out
}
