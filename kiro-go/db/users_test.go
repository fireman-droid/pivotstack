package db

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func TestUserRepository_CRUDAndBindings(t *testing.T) {
	ctx := testDB(t)
	key := insertTestApiKey(t, ctx, ApiKey{})
	u := insertTestUser(t, ctx, key, decimal.RequireFromString("12.5"), decimal.RequireFromString("2.5"))

	got, err := GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID || got.DefaultKeyID != key.ID || len(got.APIKeyIDs) != 1 {
		t.Fatalf("unexpected user: %+v", got)
	}
	if !got.Balance.Equal(decimal.RequireFromString("12.5")) || !got.GiftBalance.Equal(decimal.RequireFromString("2.5")) {
		t.Fatalf("wallet overlay missing: %+v", got)
	}

	byEmail, err := GetUserByEmail(ctx, u.EmailNorm)
	if err != nil {
		t.Fatal(err)
	}
	if byEmail.ID != u.ID {
		t.Fatalf("GetUserByEmail returned %s", byEmail.ID)
	}
	byUsername, err := GetUserByUsername(ctx, u.Username)
	if err != nil {
		t.Fatal(err)
	}
	if byUsername.ID != u.ID {
		t.Fatalf("GetUserByUsername returned %s", byUsername.ID)
	}

	u.Username = testID("renamed")
	u.Metadata = map[string]any{"role": "test"}
	if err := UpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	updated, err := GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != u.Username || updated.Metadata["role"] != "test" {
		t.Fatalf("update not persisted: %+v", updated)
	}

	if err := UpdateUserPassword(ctx, u.ID, "new-hash"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := MarkUserLogin(ctx, u.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := SetUserDisabled(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	updated, _ = GetUser(ctx, u.ID)
	if updated.PasswordHash != "new-hash" || updated.LastLoginAt == nil || !updated.Disabled {
		t.Fatalf("targeted updates failed: %+v", updated)
	}
	if err := DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUserRepository_BindAndUnbindKey(t *testing.T) {
	ctx := testDB(t)
	key1 := insertTestApiKey(t, ctx, ApiKey{})
	key2 := insertTestApiKey(t, ctx, ApiKey{})
	u := insertTestUser(t, ctx, key1, decimal.Zero, decimal.Zero)

	if err := BindKeyToUser(ctx, u.ID, key2.ID); err != nil {
		t.Fatal(err)
	}
	got, err := GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.APIKeyIDs) != 2 {
		t.Fatalf("key bindings = %+v", got.APIKeyIDs)
	}
	if err := UnbindKeyFromUser(ctx, u.ID, key1.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = GetUser(ctx, u.ID)
	if len(got.APIKeyIDs) != 1 || got.APIKeyIDs[0] != key2.ID || got.DefaultKeyID != key2.ID {
		t.Fatalf("unbind/default repair failed: %+v", got)
	}
}

func TestInsertUser_TxRollback(t *testing.T) {
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	key := insertTestApiKey(t, ctx, ApiKey{})
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	u := User{
		ID:           testID("usr"),
		Email:        testID("u") + "@example.com",
		Username:     testID("user"),
		PasswordHash: "hash",
		DefaultKeyID: key.ID,
		CreatedAt:    time.Now().UTC(),
		APIKeyIDs:    []string{key.ID},
	}
	u.EmailNorm = normalizeEmail(u.Email)
	if err := InsertUser(ctx, tx, u); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(ctx)
	if _, err := GetUser(ctx, u.ID); err != ErrNotFound {
		t.Fatalf("expected rollback not found, got %v", err)
	}
}
