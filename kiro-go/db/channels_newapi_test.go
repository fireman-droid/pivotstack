package db

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func insertNewAPIProviderForTest(t *testing.T) NewAPIProvider {
	t.Helper()
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	pvd := NewAPIProvider{
		ID:                    testID("pvd"),
		Name:                  "provider",
		BaseURL:               "https://example.com",
		Username:              "admin",
		PasswordEnc:           "enc-password",
		QuotaPerUnitDollar:    testDecimal("500000"),
		YuanPerUpstreamDollar: testDecimal("7.2"),
		Enabled:               true,
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertNewAPIProvider(ctx, tx, pvd); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return pvd
}

func insertNewAPIChannelForTest(t *testing.T, ch NewAPIChannel) NewAPIChannel {
	t.Helper()
	ctx := testDB(t)
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID == "" {
		ch.ID = testID("newapi")
	}
	if ch.Alias == "" {
		ch.Alias = testID("alias")
	}
	if ch.Markup.IsZero() {
		ch.Markup = testDecimal("1.1")
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertNewAPIChannel(ctx, tx, ch); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return ch
}

func TestNewAPIProviderRepository_CRUDAndTokenRotation(t *testing.T) {
	ctx := testDB(t)
	pvd := insertNewAPIProviderForTest(t)
	got, err := GetNewAPIProvider(ctx, pvd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PasswordEnc != "enc-password" || !got.QuotaPerUnitDollar.Equal(testDecimal("500000")) {
		t.Fatalf("unexpected provider: %+v", got)
	}
	exp := time.Now().UTC().Add(time.Hour)
	if err := RotateNewAPIProviderToken(ctx, pvd.ID, "enc-token", &exp); err != nil {
		t.Fatal(err)
	}
	if err := UpdateNewAPIProviderSync(ctx, pvd.ID, time.Now().UTC(), ""); err != nil {
		t.Fatal(err)
	}
	got, _ = GetNewAPIProvider(ctx, pvd.ID)
	if got.AccessTokenEnc != "enc-token" || got.LastSyncAt == nil {
		t.Fatalf("targeted updates failed: %+v", got)
	}
}

func TestNewAPIChannelRepository_CRUDQuotaSoftDelete(t *testing.T) {
	ctx := testDB(t)
	pvd := insertNewAPIProviderForTest(t)
	ch := insertNewAPIChannelForTest(t, NewAPIChannel{
		ProviderID:      pvd.ID,
		Alias:           testID("alias"),
		UpstreamTokenID: 42,
		UpstreamKeyEnc:  "enc-upstream",
		GroupName:       "default",
		Models:          []string{"claude-sonnet"},
		Markup:          testDecimal("1.5"),
		Enabled:         true,
	})
	got, err := GetNewAPIChannel(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderID != pvd.ID || got.UpstreamKeyEnc != "enc-upstream" || len(got.Models) != 1 {
		t.Fatalf("unexpected channel: %+v", got)
	}
	got.Alias = testID("alias")
	got.RemainQuota = 10
	if err := UpdateNewAPIChannel(ctx, got); err != nil {
		t.Fatal(err)
	}
	if err := UpdateNewAPIChannelQuota(ctx, ch.ID, 99, true, 1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	got, _ = GetNewAPIChannel(ctx, ch.ID)
	if got.RemainQuota != 99 || !got.UnlimitedQuota || got.LastSeenAt == nil {
		t.Fatalf("quota update failed: %+v", got)
	}
	if err := SoftDeleteNewAPIChannel(ctx, ch.ID); err != nil {
		t.Fatal(err)
	}
}
