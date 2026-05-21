package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestChannelGroupRepository_CRUDAndMembers(t *testing.T) {
	ctx := testDB(t)
	direct := insertDirectChannelForTest(t, DirectChannel{})
	p, err := requirePool()
	if err != nil {
		t.Fatal(err)
	}
	g := ChannelGroup{
		ID:            testID("grp"),
		Name:          "group",
		Enabled:       true,
		ModelPatterns: []string{"^claude-"},
		SortOrder:     10,
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertChannelGroup(ctx, tx, g); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := JoinGroup(ctx, g.ID, "direct", direct.ID, 1); err != nil {
		t.Fatal(err)
	}
	got, err := GetChannelGroup(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Members) != 1 || got.Members[0].ChannelID != direct.ID {
		t.Fatalf("members not loaded: %+v", got)
	}
	got.Name = "updated"
	got.DefaultRuntimeChannelID = "direct:" + direct.ID
	if err := UpdateChannelGroup(ctx, got); err != nil {
		t.Fatal(err)
	}
	if err := LeaveGroup(ctx, g.ID, "direct", direct.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = GetChannelGroup(ctx, g.ID)
	if len(got.Members) != 0 || got.Name != "updated" {
		t.Fatalf("update/leave failed: %+v", got)
	}
	if err := SoftDeleteChannelGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	groups, err := ListGroupsWithMembers(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range groups {
		if row.ID == g.ID && row.DeletedAt != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("soft deleted group missing")
	}
}
