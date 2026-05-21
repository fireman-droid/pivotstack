package proxy

import (
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func newReconcileTestReservation(keyID string, paid, gift float64) *NewAPIReservation {
	now := time.Now().Unix()
	return &NewAPIReservation{
		KeyID:                        keyID,
		ChannelID:                    "apijing:tok-7",
		ProviderID:                   "apijing",
		Model:                        newAPITestModel,
		GroupName:                    "vip",
		UpstreamTokenID:              7,
		QuotaPerUnitDollar:           1000,
		YuanPerUpstreamDollar:        1,
		PivotStackDollarsPerYuanSnap: 20,
		Markup:                       1,
		EstQuota:                     100,
		PromptTokens:                 100,
		MaxOutputTokens:              50,
		PrePaidUSD:                   paid,
		PreGiftUSD:                   gift,
		Action:                       "estimated",
		StartedAt:                    now,
	}
}

func newReconcilePending(keyID, requestID string, paid, gift float64) *pendingReservation {
	return &pendingReservation{
		Reservation: newReconcileTestReservation(keyID, paid, gift),
		RequestID:   requestID,
		KeyID:       keyID,
		EnqueuedAt:  time.Now().Add(-time.Minute),
	}
}

func TestMatchUpstreamLogPrimaryTokenIDUnique(t *testing.T) {
	p := newReconcilePending("key", "req-1", 2, 0)
	logs := []NewAPILog{
		{TokenID: 8, ModelName: newAPITestModel, PromptTokens: 100, CompletionTokens: 50, CreatedAt: p.Reservation.StartedAt, Group: "vip", Quota: 200},
		{TokenID: 7, ModelName: newAPITestModel, PromptTokens: 102, CompletionTokens: 48, CreatedAt: p.Reservation.StartedAt, Group: "vip", Quota: 201},
	}
	got, ambiguous := matchUpstreamLog(p, logs)
	if ambiguous {
		t.Fatal("match should not be ambiguous")
	}
	if got == nil || got.Quota != 201 {
		t.Fatalf("match = %+v, want quota 201", got)
	}
}

func TestMatchUpstreamLogAmbiguousNeedsFingerprint(t *testing.T) {
	p := newReconcilePending("key", "req-1", 2, 0)
	logs := []NewAPILog{
		{TokenID: 7, ModelName: newAPITestModel, PromptTokens: 100, CompletionTokens: 50, CreatedAt: p.Reservation.StartedAt, Group: "vip", Quota: 201},
		{TokenID: 7, ModelName: newAPITestModel, PromptTokens: 101, CompletionTokens: 49, CreatedAt: p.Reservation.StartedAt, Group: "vip", Quota: 202},
	}
	got, ambiguous := matchUpstreamLog(p, logs)
	if !ambiguous {
		t.Fatalf("ambiguous = false, match = %+v", got)
	}
	if got != nil {
		t.Fatalf("ambiguous match should be nil, got %+v", got)
	}
}

func TestMatchUpstreamLogFingerprintBreakTie(t *testing.T) {
	p := newReconcilePending("key", "req-1", 2, 0)
	logs := []NewAPILog{
		{TokenID: 7, ModelName: newAPITestModel, PromptTokens: 100, CompletionTokens: 50, CreatedAt: p.Reservation.StartedAt, Group: "other", Quota: 201},
		{TokenID: 7, ModelName: newAPITestModel, PromptTokens: 100, CompletionTokens: 50, CreatedAt: p.Reservation.StartedAt, Group: "vip", Quota: 202},
	}
	got, ambiguous := matchUpstreamLog(p, logs)
	if ambiguous {
		t.Fatal("match should not be ambiguous")
	}
	if got == nil || got.Quota != 202 {
		t.Fatalf("match = %+v, want quota 202", got)
	}
}

func TestMatchUpstreamLogNoMatchReturnsNilFalse(t *testing.T) {
	p := newReconcilePending("key", "req-1", 2, 0)
	logs := []NewAPILog{
		{TokenID: 9, ModelName: "other-model", PromptTokens: 300, CompletionTokens: 50, CreatedAt: p.Reservation.StartedAt + 300, Group: "vip", Quota: 201},
	}
	got, ambiguous := matchUpstreamLog(p, logs)
	if ambiguous {
		t.Fatal("no match should not be ambiguous")
	}
	if got != nil {
		t.Fatalf("match = %+v, want nil", got)
	}
}

func TestApplyReconcileUnderpaidDeductsDelta(t *testing.T) {
	newAPITestConfig(t)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	p := newReconcilePending(keyID, "req-underpaid", 2, 0)
	ev := applyUpstreamReconcile(p, &NewAPILog{Quota: 200})
	if ev.Status != "reconciled" {
		t.Fatalf("status = %q", ev.Status)
	}
	tokenTestAssertFloat(t, ev.PaidUSDDelta, 2)
	tokenTestAssertFloat(t, ev.GiftUSDDelta, 0)
	tokenTestAssertFloat(t, ev.DebtUSDAdded, 0)
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 8)
	tokenTestAssertFloat(t, gift, 0)
}

func TestApplyReconcileOverpaidRefundsDelta(t *testing.T) {
	newAPITestConfig(t)
	keyID := tokenTestAddKey(t, "credit", 0, 0, 0)
	p := newReconcilePending(keyID, "req-overpaid", 4, 0)
	ev := applyUpstreamReconcile(p, &NewAPILog{Quota: 100})
	if ev.Status != "reconciled" {
		t.Fatalf("status = %q", ev.Status)
	}
	tokenTestAssertFloat(t, ev.PaidUSDDelta, -2)
	tokenTestAssertFloat(t, ev.GiftUSDDelta, 0)
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 2)
	tokenTestAssertFloat(t, gift, 0)
}

func TestApplyReconcileInsufficientBalanceAccumulatesDebt(t *testing.T) {
	newAPITestConfig(t)
	keyID := tokenTestAddKey(t, "credit", 1, 0, 0)
	p := newReconcilePending(keyID, "req-debt", 2, 0)
	ev := applyUpstreamReconcile(p, &NewAPILog{Quota: 300})
	if ev.Status != "underpaid" {
		t.Fatalf("status = %q", ev.Status)
	}
	tokenTestAssertFloat(t, ev.PaidUSDDelta, 1)
	tokenTestAssertFloat(t, ev.DebtUSDAdded, 3)
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 0)
	tokenTestAssertFloat(t, gift, 0)
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		t.Fatal("missing api key")
	}
	tokenTestAssertFloat(t, info.DebtUSD, 3)
}

func TestEnqueueDedupesByRequestID(t *testing.T) {
	r := NewNewAPIReconciler(nil)
	first := newReconcilePending("key", "req-1", 2, 0)
	second := newReconcilePending("key", "req-1", 3, 0)
	if !r.Enqueue("apijing", first) {
		t.Fatal("first enqueue returned false")
	}
	if r.Enqueue("apijing", second) {
		t.Fatal("duplicate enqueue returned true")
	}
	q, ok := r.queue("apijing")
	if !ok {
		t.Fatal("queue missing")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) != 1 {
		t.Fatalf("pending len = %d, want 1", len(q.pending))
	}
	if q.pending["req-1"] != first {
		t.Fatal("duplicate enqueue replaced original entry")
	}
}

func TestPollDelayFirstDeferred(t *testing.T) {
	r := NewNewAPIReconciler(nil)
	r.pollDelayFirst = time.Hour
	p := newReconcilePending("key", "req-1", 2, 0)
	p.EnqueuedAt = time.Now()
	if !r.Enqueue("apijing", p) {
		t.Fatal("enqueue returned false")
	}
	r.processFetchedLogs("apijing", nil, p.EnqueuedAt.Add(time.Minute))
	q, _ := r.queue("apijing")
	q.mu.Lock()
	defer q.mu.Unlock()
	got := q.pending["req-1"]
	if got == nil {
		t.Fatal("entry should still be pending")
	}
	if got.Attempt != 0 {
		t.Fatalf("attempt = %d, want 0", got.Attempt)
	}
}
