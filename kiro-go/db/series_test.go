package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestSeriesRepository_CRUD(t *testing.T) {
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	s := Series{ID: testID("series"), Name: "Claude", ModelPatterns: []string{"claude-*"}, SortOrder: 1}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertSeries(ctx, tx, s); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := GetSeries(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Claude" || len(got.ModelPatterns) != 1 {
		t.Fatalf("unexpected series: %+v", got)
	}
	got.Name = "Claude Updated"
	got.SortOrder = 2
	if err := UpdateSeries(ctx, got); err != nil {
		t.Fatal(err)
	}
	list, err := ListSeries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range list {
		if row.ID == s.ID && row.Name == "Claude Updated" {
			found = true
		}
	}
	if !found {
		t.Fatal("updated series not listed")
	}
	if err := DeleteSeries(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSeries(ctx, s.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
