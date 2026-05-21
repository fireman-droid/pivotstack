package db

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func insertActivationCodeForTest(t *testing.T, c ActivationCode) ActivationCode {
	t.Helper()
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	if c.Code == "" {
		c.Code = testID("code")
	}
	if c.Type == "" {
		c.Type = "balance"
	}
	if c.Amount.IsZero() {
		c.Amount = testDecimal("10")
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := InsertActivationCode(ctx, tx, c); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestActivationCodeRepository_InsertGet(t *testing.T) {
	ctx := testDB(t)
	expires := time.Now().UTC().Add(time.Hour)
	c := insertActivationCodeForTest(t, ActivationCode{
		Type:            "days",
		Amount:          testDecimal("30"),
		Tier:            "pro",
		CodeExpiresAt:   &expires,
		Note:            "test",
		RateLimitPerMin: 20,
		SalePriceCNY:    testDecimal("99"),
	})
	got, err := GetActivationCode(ctx, c.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != c.Code || got.Type != "days" || !got.Amount.Equal(testDecimal("30")) || got.RateLimitPerMin != 20 {
		t.Fatalf("unexpected activation code: %+v", got)
	}
}

func TestActivationCodeRepository_ListFilters(t *testing.T) {
	ctx := testDB(t)
	active := insertActivationCodeForTest(t, ActivationCode{Type: "balance", Amount: testDecimal("1")})
	past := time.Now().UTC().Add(-time.Hour)
	insertActivationCodeForTest(t, ActivationCode{Type: "time", Amount: testDecimal("2"), CodeExpiresAt: &past})

	rows, err := ListActivationCodes(ctx, ActivationCodeFilter{Type: "balance", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Code == active.Code {
			found = true
		}
		if row.Type != "balance" {
			t.Fatalf("unexpected type in filtered list: %+v", row)
		}
	}
	if !found {
		t.Fatal("active balance code not listed")
	}
}

func TestActivationCodeRepository_MarkUsed(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	c := insertActivationCodeForTest(t, ActivationCode{})
	p, _ := requirePool()
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockActivationCodeForRedeem(ctx, tx, c.Code); err != nil {
		t.Fatal(err)
	}
	if err := MarkActivationCodeUsed(ctx, tx, c.Code, key.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := GetActivationCode(ctx, c.Code)
	if !got.Used || got.UsedByKeyID != key.ID || got.UsedAt == nil {
		t.Fatalf("code not marked used: %+v", got)
	}
}

func TestActivationCodeRepository_LockForRedeemConcurrent(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	c := insertActivationCodeForTest(t, ActivationCode{})
	p, _ := requirePool()

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
			if err != nil {
				errs <- err
				return
			}
			defer tx.Rollback(ctx)
			if _, err := LockActivationCodeForRedeem(ctx, tx, c.Code); err != nil {
				errs <- err
				return
			}
			if err := MarkActivationCodeUsed(ctx, tx, c.Code, key.ID, time.Now().UTC()); err != nil {
				errs <- err
				return
			}
			errs <- tx.Commit(ctx)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	wins, used := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrCodeAlreadyUsed):
			used++
		default:
			t.Fatal(err)
		}
	}
	if wins != 1 || used != 1 {
		t.Fatalf("wins=%d used=%d", wins, used)
	}
}

func TestActivationCodeRepository_Delete(t *testing.T) {
	ctx := testDB(t)
	c := insertActivationCodeForTest(t, ActivationCode{})
	if err := DeleteActivationCode(ctx, c.Code); err != nil {
		t.Fatal(err)
	}
	if _, err := GetActivationCode(ctx, c.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
