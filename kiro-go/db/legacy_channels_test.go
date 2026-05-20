package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestLegacyChannelRepository_CRUD(t *testing.T) {
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	ch := LegacyChannel{
		ID:           testID("legacy"),
		Type:         "openai",
		BaseURL:      "https://example.com",
		APIKeyEnc:    "enc",
		Models:       []string{"gpt"},
		ModelPrices:  map[string]any{"gpt": map[string]any{"inputPerM": 1}},
		ModelAliases: map[string]string{"gpt": "upstream"},
		ExtraHeaders: map[string]string{"x-test": "1"},
		Enabled:      true,
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertLegacyChannel(ctx, tx, ch); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := GetLegacyChannel(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKeyEnc != "enc" || got.ModelAliases["gpt"] != "upstream" || len(got.Models) != 1 {
		t.Fatalf("unexpected legacy channel: %+v", got)
	}
	got.Enabled = false
	got.ModelAliases["gpt"] = "changed"
	if err := UpdateLegacyChannel(ctx, got); err != nil {
		t.Fatal(err)
	}
	updated, _ := GetLegacyChannel(ctx, ch.ID)
	if updated.Enabled || updated.ModelAliases["gpt"] != "changed" {
		t.Fatalf("update failed: %+v", updated)
	}
	list, err := ListLegacyChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range list {
		if row.ID == ch.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("legacy channel not listed")
	}
	if err := DeleteLegacyChannel(ctx, ch.ID); err != nil {
		t.Fatal(err)
	}
}
