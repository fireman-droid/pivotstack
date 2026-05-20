package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func insertTestBillingReservation(t *testing.T, ctx context.Context, id, status, ownerType, ownerID, apiKeyID, requestID string, createdAt time.Time) {
	t.Helper()
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, `
		INSERT INTO billing_reservations (
			id, request_id, api_key_id, owner_type, owner_id, channel_id, model,
			status, action, est_cost_usd, pre_paid_usd, pre_gift_usd, price_snapshot,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'deduct',$9,$10,0,'{}'::jsonb,$11)
	`, id, textFromString(requestID), apiKeyID, ownerType, ownerID,
		textFromString("ch_"+ownerID), textFromString("model"),
		status, numericFromDecimal(decimal.RequireFromString("0.50")),
		numericFromDecimal(decimal.RequireFromString("0.50")), createdAt.UTC())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctxBg := context.Background()
		_, _ = p.Exec(ctxBg, `DELETE FROM billing_reservations WHERE id=$1`, id)
	})
}

func TestBillingReservation_GetList(t *testing.T) {
	ctx := testDB(t)
	keyID := testID("key")
	insertTestApiKey(t, ctx, ApiKey{ID: keyID})
	resID := testID("res")
	now := time.Now().UTC()
	insertTestBillingReservation(t, ctx, resID, "pending", "api_key", keyID, keyID, testID("req"), now)

	got, err := GetBillingReservation(ctx, resID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != resID || got.OwnerID != keyID || got.Status != "pending" {
		t.Fatalf("unexpected reservation: %+v", got)
	}
	if !got.EstCostUSD.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("est_cost = %s", got.EstCostUSD)
	}

	rows, err := ListBillingReservations(ctx, BillingReservationFilter{
		Status:   "pending",
		OwnerID:  keyID,
		APIKeyID: keyID,
		Limit:    10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != resID {
		t.Fatalf("list returned %+v", rows)
	}
}

func TestBillingReservation_NotFound(t *testing.T) {
	ctx := testDB(t)
	if _, err := GetBillingReservation(ctx, "nonexistent_"+testID("none")); err == nil || err.Error() != ErrNotFound.Error() {
		if err == nil || !errIs(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	}
}

func TestBillingReservation_ExpireStale(t *testing.T) {
	ctx := testDB(t)
	keyID := testID("key")
	insertTestApiKey(t, ctx, ApiKey{ID: keyID})
	staleID := testID("res-stale")
	freshID := testID("res-fresh")
	past := time.Now().UTC().AddDate(-10, 0, 0)
	future := time.Now().UTC().AddDate(10, 0, 0)
	insertTestBillingReservation(t, ctx, staleID, "pending", "api_key", keyID, keyID, testID("req"), past)
	insertTestBillingReservation(t, ctx, freshID, "pending", "api_key", keyID, keyID, testID("req"), future)

	affected, err := ExpireStaleReservations(ctx, time.Now().UTC().AddDate(-5, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if affected < 1 {
		t.Fatalf("expected >= 1 affected, got %d", affected)
	}

	stale, err := GetBillingReservation(ctx, staleID)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Status != "expired" || stale.SettledAt == nil {
		t.Fatalf("stale not expired: %+v", stale)
	}
	fresh, err := GetBillingReservation(ctx, freshID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Status != "pending" {
		t.Fatalf("fresh wrongly expired: %+v", fresh)
	}
}

// errIs 用 errors.Is 风格但因为部分 wrap 需要兼容。
func errIs(err, target error) bool {
	if err == nil {
		return false
	}
	// pgx may wrap ErrNotFound directly; we keep tests resilient.
	for e := err; e != nil; {
		if e == target {
			return true
		}
		unwrapper, ok := e.(interface{ Unwrap() error })
		if !ok {
			break
		}
		e = unwrapper.Unwrap()
	}
	return err == target || err.Error() == target.Error()
}

// unused import guard (pgx) — silence if helper above doesn't reference it.
var _ pgx.Tx
