package db

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestApiKeyRepository_CRUD(t *testing.T) {
	ctx := testDB(t)
	expires := time.Now().UTC().Add(time.Hour)
	k := insertTestApiKey(t, ctx, ApiKey{
		Tier:              "pro",
		Plan:              "hybrid",
		ExpiresAt:         &expires,
		Balance:           decimal.RequireFromString("100"),
		GiftBalance:       decimal.RequireFromString("10"),
		TotalRecharged:    decimal.RequireFromString("100"),
		TotalGifted:       decimal.RequireFromString("10"),
		Note:              "test key",
		Models:            map[string]int64{"claude": 1},
		SeriesPreferences: map[string]any{"series": "channel"},
		Metadata:          map[string]any{"source": "test"},
	})

	got, err := GetApiKey(ctx, k.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != k.ID || got.Tier != "pro" || !got.Balance.Equal(decimal.RequireFromString("100")) {
		t.Fatalf("unexpected key: %+v", got)
	}
	byHash, err := GetApiKeyByHash(ctx, k.KeyHash)
	if err != nil {
		t.Fatal(err)
	}
	if byHash.ID != k.ID {
		t.Fatalf("GetApiKeyByHash returned %s", byHash.ID)
	}

	got.Note = "updated"
	got.Enabled = false
	got.ChannelPreferences = map[string]any{"group": "direct:test"}
	if err := UpdateApiKey(ctx, got); err != nil {
		t.Fatal(err)
	}
	updated, _ := GetApiKey(ctx, k.ID)
	if updated.Note != "updated" || updated.Enabled {
		t.Fatalf("update failed: %+v", updated)
	}
	if err := SetApiKeyEnabled(ctx, k.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := SetApiKeyExpiry(ctx, k.ID, nil); err != nil {
		t.Fatal(err)
	}
	updated, _ = GetApiKey(ctx, k.ID)
	if !updated.Enabled || updated.ExpiresAt != nil {
		t.Fatalf("targeted updates failed: %+v", updated)
	}
}

func TestApiKeyRepository_ListAndChildren(t *testing.T) {
	ctx := testDB(t)
	parent := insertTestApiKey(t, ctx, ApiKey{IsReseller: true})
	child := insertTestApiKey(t, ctx, ApiKey{ParentKeyID: parent.ID})

	children, err := ListChildKeys(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range children {
		if c.ID == child.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("child %s not found in %+v", child.ID, children)
	}

	keys, err := ListApiKeys(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("expected api keys")
	}
}

func TestApiKeyRepository_IncrementUsage(t *testing.T) {
	ctx := testDB(t)
	k := insertTestApiKey(t, ctx, ApiKey{Models: map[string]int64{"a": 1}})

	if err := IncrementApiKeyUsage(ctx, k.ID, 2, 1, 100, decimal.RequireFromString("3.5"), map[string]int64{"a": 2, "b": 1}); err != nil {
		t.Fatal(err)
	}
	got, err := GetApiKey(ctx, k.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Requests != 2 || got.Errors != 1 || got.Tokens != 100 || !got.Credits.Equal(decimal.RequireFromString("3.5")) {
		t.Fatalf("usage counters = %+v", got)
	}
	if got.Models["a"] != 3 || got.Models["b"] != 1 {
		t.Fatalf("models = %+v", got.Models)
	}
}

func TestApiKeyRepository_SoftDelete(t *testing.T) {
	ctx := testDB(t)
	k := insertTestApiKey(t, ctx, ApiKey{})

	if err := SoftDeleteApiKey(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := GetApiKeyByHash(ctx, k.KeyHash); err != ErrNotFound {
		t.Fatalf("expected deleted key hidden from hash lookup, got %v", err)
	}
	keys, err := ListApiKeys(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	foundDeleted := false
	for _, row := range keys {
		if row.ID == k.ID && row.DeletedAt != nil {
			foundDeleted = true
		}
	}
	if !foundDeleted {
		t.Fatalf("deleted key not listed with includeDeleted")
	}
}

func TestUpdateApiKeySoldToChildren(t *testing.T) {
	ctx := testDB(t)
	p, _ := requirePool()
	k := insertTestApiKey(t, ctx, ApiKey{})
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := UpdateApiKeySoldToChildren(ctx, tx, k.ID, decimal.RequireFromString("12.5")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := GetApiKey(ctx, k.ID)
	if !got.SoldToChildren.Equal(decimal.RequireFromString("12.5")) {
		t.Fatalf("sold_to_children = %s", got.SoldToChildren)
	}
}
