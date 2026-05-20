package db

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func testDB(t *testing.T) context.Context {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	poolMu.Lock()
	if pool == nil {
		poolOnce = sync.Once{}
		poolErr = nil
	}
	poolMu.Unlock()

	if err := InitPool(ctx, databaseURL); err != nil {
		t.Fatalf("InitPool() error = %v", err)
	}
	if err := RunMigrations(ctx); err != nil && !errors.Is(err, ErrAlreadyAtLatest) {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	return ctx
}

func testID(prefix string) string {
	return prefix + "_" + uuid.NewString()
}

func testDecimal(v string) decimal.Decimal {
	return decimal.RequireFromString(v)
}

func testMeta(op string) WalletMeta {
	return WalletMeta{
		Operation:     op,
		ReservationID: testID("res"),
		RequestID:     testID("req"),
		Operator:      "test",
	}
}

func insertTestApiKey(t *testing.T, ctx context.Context, k ApiKey) ApiKey {
	t.Helper()
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	if k.ID == "" {
		k.ID = testID("key")
	}
	if len(k.KeyHash) == 0 {
		k.KeyHash = []byte(testID("hash"))
	}
	if k.KeyCiphertext == "" {
		k.KeyCiphertext = "v1:gcm:" + testID("cipher")
	}
	if k.Plan == "" {
		k.Plan = "credit"
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now().UTC()
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := InsertApiKey(ctx, tx, k); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return k
}

func insertTestUser(t *testing.T, ctx context.Context, key ApiKey, paid, gift decimal.Decimal) User {
	t.Helper()
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	u := User{
		ID:             testID("usr"),
		Email:          testID("u") + "@example.com",
		Username:       testID("user"),
		PasswordHash:   "hash",
		DefaultKeyID:   key.ID,
		CreatedAt:      now,
		SchemaVersion:  3,
		APIKeyIDs:      []string{key.ID},
		Balance:        paid,
		GiftBalance:    gift,
		TotalRecharged: paid,
		TotalGifted:    gift,
	}
	u.EmailNorm = normalizeEmail(u.Email)
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := InsertUser(ctx, tx, u); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return u
}

func ledgerCountByRequest(t *testing.T, ctx context.Context, requestID string) int {
	t.Helper()
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM wallet_ledger WHERE request_id=$1`, requestID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDeductWallet_BoundUserKey_PaidFirst(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, testDecimal("100"), testDecimal("50"))
	meta := testMeta("deduct")

	res, err := DeductWalletBalance(ctx, key.ID, testDecimal("70"), meta)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.PaidDelta.Equal(testDecimal("70")) || !res.GiftDelta.IsZero() {
		t.Fatalf("unexpected deduct result: %+v", res)
	}
	totals, err := GetWalletTotals(ctx, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !totals.Balance.Equal(testDecimal("30")) || !totals.GiftBalance.Equal(testDecimal("50")) {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestDeductWallet_BoundUserKey_OverflowToGift(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, testDecimal("10"), testDecimal("50"))

	res, err := DeductWalletBalance(ctx, key.ID, testDecimal("30"), testMeta("deduct"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.PaidDelta.Equal(testDecimal("10")) || !res.GiftDelta.Equal(testDecimal("20")) {
		t.Fatalf("unexpected deduct result: %+v", res)
	}
	totals, _ := GetWalletTotals(ctx, key.ID)
	if !totals.Balance.IsZero() || !totals.GiftBalance.Equal(testDecimal("30")) {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestDeductWallet_OrphanKey_RoutesToApiKeys(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{Balance: testDecimal("100")})

	res, err := DeductWalletBalance(ctx, key.ID, testDecimal("30"), testMeta("deduct"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("deduct failed: %+v", res)
	}
	updated, err := GetApiKey(ctx, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Balance.Equal(testDecimal("70")) {
		t.Fatalf("api key balance = %s", updated.Balance)
	}
}

func TestDeductWallet_ResellerChild_RoutesToApiKeys(t *testing.T) {
	ctx := testDB(t)
	parent := insertTestApiKey(t, ctx, ApiKey{Balance: testDecimal("100"), IsReseller: true})
	child := insertTestApiKey(t, ctx, ApiKey{ParentKeyID: parent.ID, Balance: testDecimal("100")})
	insertTestUser(t, ctx, child, testDecimal("500"), decimal.Zero)

	res, err := DeductWalletBalance(ctx, child.ID, testDecimal("30"), testMeta("deduct"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("deduct failed: %+v", res)
	}
	updated, _ := GetApiKey(ctx, child.ID)
	if !updated.Balance.Equal(testDecimal("70")) {
		t.Fatalf("child api key balance = %s", updated.Balance)
	}
	totals, _ := GetWalletTotals(ctx, child.ID)
	if !totals.Balance.Equal(testDecimal("70")) {
		t.Fatalf("child routed totals = %+v", totals)
	}
}

func TestDeductWallet_Insufficient(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, testDecimal("10"), testDecimal("20"))
	meta := testMeta("deduct")

	res, err := DeductWalletBalance(ctx, key.ID, testDecimal("50"), meta)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || res.Reason != "insufficient" {
		t.Fatalf("unexpected result: %+v", res)
	}
	totals, _ := GetWalletTotals(ctx, key.ID)
	if !totals.Balance.Equal(testDecimal("10")) || !totals.GiftBalance.Equal(testDecimal("20")) {
		t.Fatalf("wallet mutated: %+v", totals)
	}
	if got := ledgerCountByRequest(t, ctx, meta.RequestID); got != 0 {
		t.Fatalf("ledger count = %d", got)
	}
}

func TestDeductWallet_ConcurrentSameKey(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{Balance: testDecimal("100")})

	var wg sync.WaitGroup
	results := make(chan DeductResult, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := DeductWalletBalance(ctx, key.ID, testDecimal("60"), testMeta("deduct"))
			results <- res
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	success, insufficient := 0, 0
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for res := range results {
		if res.OK {
			success++
		}
		if res.Reason == "insufficient" {
			insufficient++
		}
	}
	if success != 1 || insufficient != 1 {
		t.Fatalf("success=%d insufficient=%d", success, insufficient)
	}
	totals, _ := GetWalletTotals(ctx, key.ID)
	if !totals.Balance.Equal(testDecimal("40")) {
		t.Fatalf("final balance = %s", totals.Balance)
	}
}

func TestAddWalletRecharge_UpdatesPaidAndTotalRecharged(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, decimal.Zero, decimal.Zero)

	if err := AddWalletRecharge(ctx, key.ID, testDecimal("50"), testMeta("recharge")); err != nil {
		t.Fatal(err)
	}
	totals, _ := GetWalletTotals(ctx, key.ID)
	if !totals.Balance.Equal(testDecimal("50")) || !totals.TotalRecharged.Equal(testDecimal("50")) {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestAddWalletGift_UpdatesGiftAndTotalGifted(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, decimal.Zero, decimal.Zero)

	if err := AddWalletGift(ctx, key.ID, testDecimal("25"), testMeta("gift")); err != nil {
		t.Fatal(err)
	}
	totals, _ := GetWalletTotals(ctx, key.ID)
	if !totals.GiftBalance.Equal(testDecimal("25")) || !totals.TotalGifted.Equal(testDecimal("25")) {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestRefundWalletByReservation_RestoresExactBuckets(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, testDecimal("10"), testDecimal("50"))
	meta := testMeta("deduct")

	res, err := DeductWalletBalance(ctx, key.ID, testDecimal("30"), meta)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("deduct failed: %+v", res)
	}
	if err := RefundWalletByReservation(ctx, meta.ReservationID, WalletMeta{Operation: "refund", RequestID: testID("refund")}); err != nil {
		t.Fatal(err)
	}
	totals, _ := GetWalletTotals(ctx, key.ID)
	if !totals.Balance.Equal(testDecimal("10")) || !totals.GiftBalance.Equal(testDecimal("50")) {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestSetWalletBalances_OverwriteEmitsDeltaLedger(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, testDecimal("50"), testDecimal("20"))
	meta := testMeta("admin_adjust")

	totals, err := SetWalletBalances(ctx, key.ID, testDecimal("200"), testDecimal("100"), meta)
	if err != nil {
		t.Fatal(err)
	}
	if !totals.Balance.Equal(testDecimal("200")) || !totals.GiftBalance.Equal(testDecimal("100")) {
		t.Fatalf("totals = %+v", totals)
	}
	p, _ := requirePool()
	var paidDelta, giftDelta decimal.Decimal
	var paidN, giftN pgtypeNumericPair
	if err := p.QueryRow(ctx,
		`SELECT paid_delta, gift_delta FROM wallet_ledger WHERE request_id=$1`,
		meta.RequestID,
	).Scan(&paidN.Paid, &giftN.Gift); err != nil {
		t.Fatal(err)
	}
	paidDelta, _ = decimalFromNumeric(paidN.Paid)
	giftDelta, _ = decimalFromNumeric(giftN.Gift)
	if !paidDelta.Equal(testDecimal("150")) || !giftDelta.Equal(testDecimal("80")) {
		t.Fatalf("ledger deltas paid=%s gift=%s", paidDelta, giftDelta)
	}
}

type pgtypeNumericPair struct {
	Paid pgtype.Numeric
	Gift pgtype.Numeric
}

func TestRebalanceUserWallets_Multiplier(t *testing.T) {
	ctx := testDB(t)
	for i := 0; i < 3; i++ {
		key := insertTestApiKey(t, ctx, ApiKey{})
		insertTestUser(t, ctx, key, testDecimal("100"), testDecimal("20"))
	}
	before := countLedgerByOperation(t, ctx, "rebalance")
	affected, paidDiff, giftDiff, err := RebalanceUserWallets(ctx, testDecimal("0.5"))
	if err != nil {
		t.Fatal(err)
	}
	if affected < 3 || !paidDiff.IsNegative() || !giftDiff.IsNegative() {
		t.Fatalf("affected=%d paidDiff=%s giftDiff=%s", affected, paidDiff, giftDiff)
	}
	after := countLedgerByOperation(t, ctx, "rebalance")
	if after-before != int(affected) {
		t.Fatalf("rebalance ledger rows delta=%d affected=%d", after-before, affected)
	}
}

func TestWalletLedger_AlwaysAppendsBeforeCommit(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key, testDecimal("100"), decimal.Zero)
	reqs := []string{}
	for _, op := range []string{"deduct", "recharge", "gift"} {
		meta := testMeta(op)
		reqs = append(reqs, meta.RequestID)
		switch op {
		case "deduct":
			if _, err := DeductWalletBalance(ctx, key.ID, testDecimal("10"), meta); err != nil {
				t.Fatal(err)
			}
		case "recharge":
			if err := AddWalletRecharge(ctx, key.ID, testDecimal("10"), meta); err != nil {
				t.Fatal(err)
			}
		case "gift":
			if err := AddWalletGift(ctx, key.ID, testDecimal("10"), meta); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, req := range reqs {
		if got := ledgerCountByRequest(t, ctx, req); got != 1 {
			t.Fatalf("ledger rows for %s = %d", req, got)
		}
	}
}

func countLedgerByOperation(t *testing.T, ctx context.Context, op string) int {
	t.Helper()
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM wallet_ledger WHERE operation=$1`, op).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
