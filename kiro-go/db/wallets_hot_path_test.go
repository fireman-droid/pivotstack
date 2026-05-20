package db

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TestRefundByReservation_Idempotent — 同一 reservation 重复 refund 不能重复返钱。
// 这是 plan §8 必须覆盖的 idempotency 保证（防止网络重试 / 上游对账多打）。
func TestRefundByReservation_Idempotent(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	user := insertTestUser(t, ctx, key, testDecimal("10"), testDecimal("0"))
	_ = user

	meta := testMeta("deduct")
	if _, err := DeductWalletBalance(ctx, key.ID, testDecimal("3"), meta); err != nil {
		t.Fatal(err)
	}

	refundMeta := WalletMeta{Operation: "refund", RequestID: testID("req"), Operator: "test"}
	if err := RefundWalletByReservation(ctx, meta.ReservationID, refundMeta); err != nil {
		t.Fatal(err)
	}
	if err := RefundWalletByReservation(ctx, meta.ReservationID, refundMeta); err != nil {
		t.Fatalf("second refund should be no-op: %v", err)
	}
	if err := RefundWalletByReservation(ctx, meta.ReservationID, refundMeta); err != nil {
		t.Fatalf("third refund should be no-op: %v", err)
	}

	// 验证用户钱包余额应为初始 10（扣 3 然后退 3）
	got, err := GetUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Balance.Equal(testDecimal("10")) {
		t.Fatalf("balance after triple refund = %s, want 10", got.Balance)
	}

	// 验证 wallet_ledger 中 refund operation 只追加一次
	entries, err := ListWalletLedger(ctx, WalletLedgerFilter{
		ReservationID: meta.ReservationID,
		Operation:     "refund",
		Limit:         10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("refund ledger count = %d, want exactly 1 (idempotent)", len(entries))
	}
}

// TestDeductWallet_ConcurrentDifferentKeys — 不同 key 的并发扣款不应互相阻塞或死锁。
// plan §3.6 锁顺序保证：sorted key id；不同 key 不竞争 row lock。
func TestDeductWallet_ConcurrentDifferentKeys(t *testing.T) {
	ctx := testDB(t)
	key1 := insertTestApiKey(t, ctx, ApiKey{})
	key2 := insertTestApiKey(t, ctx, ApiKey{})
	insertTestUser(t, ctx, key1, testDecimal("100"), testDecimal("0"))
	insertTestUser(t, ctx, key2, testDecimal("100"), testDecimal("0"))

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	deadline := time.Now().Add(5 * time.Second)
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			meta := testMeta("deduct")
			if _, err := DeductWalletBalance(ctx, key1.ID, testDecimal("1"), meta); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			meta := testMeta("deduct")
			if _, err := DeductWalletBalance(ctx, key2.ID, testDecimal("1"), meta); err != nil {
				errs <- err
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Until(deadline)):
		t.Fatal("concurrent deductions deadlocked")
	}
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent deduct error: %v", err)
		}
	}
}

// BenchmarkDeductWallet 测量单笔 deduct 的端到端 DB 时间。
// plan §7 要求 wallet tx p99 ≤ 5ms。运行：
//
//	DATABASE_URL=... go test ./db -bench BenchmarkDeductWallet -benchtime=10s -run=^$
func BenchmarkDeductWallet(b *testing.B) {
	dsn := requireEnvDSN(b)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := InitPool(ctx, dsn); err != nil {
		b.Fatal(err)
	}
	if err := RunMigrations(ctx); err != nil && err.Error() != "already at latest" {
		// best-effort: 如果 schema 已存在，RunMigrations 内部 idempotent
		_ = err
	}

	k := insertBenchApiKey(b, ctx)
	insertBenchUser(b, ctx, k, decimal.NewFromInt(int64(b.N)+1000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		meta := WalletMeta{
			Operation:     "deduct",
			ReservationID: "res-bench-" + benchID(i),
			RequestID:     "req-bench-" + benchID(i),
			Operator:      "bench",
		}
		_, err := DeductWalletBalance(ctx, k.ID, testDecimal("0.001"), meta)
		if err != nil {
			b.Fatalf("iter %d: %v", i, err)
		}
	}
}

func requireEnvDSN(tb testing.TB) string {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		tb.Skip("DATABASE_URL not set")
	}
	return dsn
}

func insertBenchApiKey(b *testing.B, ctx context.Context) ApiKey {
	b.Helper()
	t := &testing.T{}
	return insertTestApiKey(t, ctx, ApiKey{})
}

func insertBenchUser(b *testing.B, ctx context.Context, k ApiKey, paid decimal.Decimal) User {
	b.Helper()
	t := &testing.T{}
	return insertTestUser(t, ctx, k, paid, testDecimal("0"))
}

// benchID 用 UUID 保证跨进程跨 bench 运行的唯一性，避免 billing_reservations.pkey 冲突。
func benchID(i int) string {
	return uuid.NewString()
}
