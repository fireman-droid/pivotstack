package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func TestAccountRepository_CRUDAndStats(t *testing.T) {
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	a := Account{
		ID:              testID("acct"),
		Email:           testID("acct") + "@example.com",
		AccessTokenEnc:  "enc-access",
		RefreshTokenEnc: "enc-refresh",
		AuthMethod:      "idc",
		Region:          "us-east-1",
		Enabled:         true,
		UsageCurrent:    decimal.RequireFromString("1.5"),
		Metadata:        map[string]any{"source": "test"},
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertAccount(ctx, tx, a); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	got, err := GetAccount(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.EmailNorm == "" || got.AccessTokenEnc != "enc-access" || got.Metadata["source"] != "test" {
		t.Fatalf("unexpected account: %+v", got)
	}
	if err := IncrementAccountUsage(ctx, a.ID, 2, 1, 100, decimal.RequireFromString("3.25")); err != nil {
		t.Fatal(err)
	}
	got, _ = GetAccount(ctx, a.ID)
	if got.RequestCount != 2 || got.ErrorCount != 1 || got.TotalTokens != 100 || !got.TotalCredits.Equal(decimal.RequireFromString("3.25")) {
		t.Fatalf("usage not incremented: %+v", got)
	}
	if err := SetAccountBan(ctx, a.ID, "BANNED", "test"); err != nil {
		t.Fatal(err)
	}
	if err := ClearAccountBan(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := SoftDeleteAccount(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	list, err := ListAccounts(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range list {
		if row.ID == a.ID && row.DeletedAt != nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("deleted account not listed")
	}
}

func TestAccountRepository_UpsertByIdentity(t *testing.T) {
	ctx := testDB(t)
	email := testID("acct") + "@example.com"
	id1, isNew, err := UpsertAccountByIdentity(ctx, Account{ID: testID("acct"), Email: email, AuthMethod: "idc", Enabled: true})
	if err != nil || !isNew {
		t.Fatalf("first upsert id=%s new=%v err=%v", id1, isNew, err)
	}
	id2, isNew, err := UpsertAccountByIdentity(ctx, Account{ID: testID("acct"), Email: email, AuthMethod: "idc", AccessTokenEnc: "rotated", Enabled: true})
	if err != nil || isNew || id2 != id1 {
		t.Fatalf("second upsert id=%s new=%v err=%v", id2, isNew, err)
	}
}
