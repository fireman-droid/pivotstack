package db

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func insertDirectChannelForTest(t *testing.T, ch DirectChannel) DirectChannel {
	t.Helper()
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID == "" {
		ch.ID = testID("direct")
	}
	if ch.Type == "" {
		ch.Type = "openai"
	}
	if ch.Alias == "" {
		ch.Alias = testID("alias")
	}
	ch.Enabled = true
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertDirectChannel(ctx, tx, ch); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return ch
}

func TestDirectChannelRepository_CRUD(t *testing.T) {
	ctx := testDB(t)
	ch := insertDirectChannelForTest(t, DirectChannel{
		Models:       []string{"gpt-4.1"},
		APIKeyEnc:    "enc-key",
		ModelMapping: map[string]string{"gpt-4.1": "upstream-gpt"},
		ExtraHeaders: map[string]string{"x-test": "1"},
		SellPrice: DirectSellPrice{
			Default: DirectSellPriceRow{InputPerM: 1, OutputPerM: 2},
		},
	})
	got, err := GetDirectChannel(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKeyEnc != "enc-key" || got.ModelMapping["gpt-4.1"] != "upstream-gpt" || len(got.Models) != 1 {
		t.Fatalf("unexpected channel: %+v", got)
	}
	got.Alias = testID("alias")
	got.Enabled = false
	if err := UpdateDirectChannel(ctx, got); err != nil {
		t.Fatal(err)
	}
	updated, _ := GetDirectChannel(ctx, ch.ID)
	if updated.Enabled || updated.Alias != got.Alias {
		t.Fatalf("update failed: %+v", updated)
	}
	if err := SetDirectChannelAPIKey(ctx, ch.ID, ""); err != nil {
		t.Fatal(err)
	}
	updated, _ = GetDirectChannel(ctx, ch.ID)
	if updated.APIKeyEnc != "" {
		t.Fatalf("api key not cleared: %+v", updated)
	}
	if err := SoftDeleteDirectChannel(ctx, ch.ID); err != nil {
		t.Fatal(err)
	}
	updated, _ = GetDirectChannel(ctx, ch.ID)
	if updated.DeletedAt == nil {
		t.Fatal("expected soft delete timestamp")
	}
}

func TestDirectChannelRepository_AliasConflictWithNewAPI(t *testing.T) {
	ctx := testDB(t)
	pvd := insertNewAPIProviderForTest(t)
	alias := testID("alias")
	insertNewAPIChannelForTest(t, NewAPIChannel{
		ProviderID:      pvd.ID,
		Alias:           alias,
		UpstreamTokenID: 1,
		GroupName:       "default",
		Markup:          testDecimal("1.2"),
		Models:          []string{"model"},
		Enabled:         true,
	})
	p, _ := requirePool()
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	err = InsertDirectChannel(ctx, tx, DirectChannel{
		ID:      testID("direct"),
		Type:    "openai",
		Alias:   alias,
		Enabled: true,
	})
	if !errors.Is(err, ErrAliasConflict) {
		t.Fatalf("expected ErrAliasConflict, got %v", err)
	}
}
