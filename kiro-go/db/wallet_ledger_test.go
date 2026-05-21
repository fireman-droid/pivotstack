package db

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func insertTestLedgerEntry(t *testing.T, ctx context.Context, e WalletLedgerEntry) {
	t.Helper()
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	meta, err := jsonObjectParam(e.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, `
		INSERT INTO wallet_ledger (
			id, occurred_at, api_key_id, owner_type, owner_id, operation,
			reservation_id, request_id, paid_delta, gift_delta, paid_after, gift_after, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, e.ID, e.OccurredAt.UTC(), e.APIKeyID, e.OwnerType, e.OwnerID, e.Operation,
		textFromString(e.ReservationID), textFromString(e.RequestID),
		numericFromDecimal(e.PaidDelta), numericFromDecimal(e.GiftDelta),
		numericFromDecimal(e.PaidAfter), numericFromDecimal(e.GiftAfter), meta)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = p.Exec(context.Background(), `DELETE FROM wallet_ledger WHERE id=$1`, e.ID)
	})
}

func TestWalletLedger_ListByFilter(t *testing.T) {
	ctx := testDB(t)
	ownerID := testID("usr")
	otherID := testID("usr")
	keyID := testID("key")
	now := time.Now().UTC()
	target := WalletLedgerEntry{
		ID: testID("ledger"), OccurredAt: now, APIKeyID: keyID,
		OwnerType: "user", OwnerID: ownerID, Operation: "deduct",
		ReservationID: testID("res"), RequestID: testID("req"),
		PaidDelta: decimal.RequireFromString("-1.5"), GiftDelta: decimal.Zero,
		PaidAfter: decimal.RequireFromString("8.5"), GiftAfter: decimal.RequireFromString("2"),
		Metadata: map[string]any{"note": "test"},
	}
	noise := WalletLedgerEntry{
		ID: testID("ledger"), OccurredAt: now.Add(-time.Minute), APIKeyID: testID("key"),
		OwnerType: "user", OwnerID: otherID, Operation: "deduct",
		PaidDelta: decimal.RequireFromString("-0.1"), PaidAfter: decimal.RequireFromString("1"),
	}
	insertTestLedgerEntry(t, ctx, target)
	insertTestLedgerEntry(t, ctx, noise)

	rows, err := ListWalletLedger(ctx, WalletLedgerFilter{
		OwnerType: "user", OwnerID: ownerID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != target.ID {
		t.Fatalf("filter by owner returned %+v", rows)
	}
	if !rows[0].PaidDelta.Equal(target.PaidDelta) || rows[0].Metadata["note"] != "test" {
		t.Fatalf("decoded ledger entry mismatch: %+v", rows[0])
	}

	byRes, err := ListWalletLedger(ctx, WalletLedgerFilter{ReservationID: target.ReservationID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byRes) != 1 || byRes[0].ID != target.ID {
		t.Fatalf("filter by reservation returned %+v", byRes)
	}
}

func TestWalletLedger_AggregateByOperation(t *testing.T) {
	ctx := testDB(t)
	ownerID := testID("usr")
	keyID := testID("key")
	base := time.Now().UTC().AddDate(50, 0, 0) // 用未来时间隔离
	entries := []WalletLedgerEntry{
		{ID: testID("ledger"), OccurredAt: base, APIKeyID: keyID, OwnerType: "user", OwnerID: ownerID,
			Operation: "deduct", PaidDelta: decimal.RequireFromString("-1.0"),
			PaidAfter: decimal.RequireFromString("9")},
		{ID: testID("ledger"), OccurredAt: base.Add(time.Minute), APIKeyID: keyID, OwnerType: "user", OwnerID: ownerID,
			Operation: "deduct", PaidDelta: decimal.RequireFromString("-0.5"),
			PaidAfter: decimal.RequireFromString("8.5")},
		{ID: testID("ledger"), OccurredAt: base.Add(2 * time.Minute), APIKeyID: keyID, OwnerType: "user", OwnerID: ownerID,
			Operation: "refund", PaidDelta: decimal.RequireFromString("0.3"),
			PaidAfter: decimal.RequireFromString("8.8")},
	}
	for _, e := range entries {
		insertTestLedgerEntry(t, ctx, e)
	}
	aggs, err := AggregateWalletLedgerByOperation(ctx, "user", ownerID, base.Add(-time.Hour), base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	byOp := map[string]WalletLedgerAggregate{}
	for _, a := range aggs {
		byOp[a.Operation] = a
	}
	if byOp["deduct"].Count != 2 || !byOp["deduct"].PaidSum.Equal(decimal.RequireFromString("-1.5")) {
		t.Fatalf("deduct agg: %+v", byOp["deduct"])
	}
	if byOp["refund"].Count != 1 || !byOp["refund"].PaidSum.Equal(decimal.RequireFromString("0.3")) {
		t.Fatalf("refund agg: %+v", byOp["refund"])
	}
}
