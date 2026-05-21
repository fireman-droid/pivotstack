package db

import (
	"testing"
	"time"
)

func TestAuditRepository_InsertList(t *testing.T) {
	ctx := testDB(t)
	action := testID("action")
	actor := testID("actor")
	if err := InsertAuditLog(ctx, action, actor, map[string]any{"target": "x"}); err != nil {
		t.Fatal(err)
	}
	rows, err := ListAuditLogs(ctx, AuditFilter{Action: action, Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Action != action || rows[0].Actor != actor || rows[0].Payload["target"] != "x" {
		t.Fatalf("unexpected audit rows: %+v", rows)
	}
}

func TestAuditRepository_FilterByActionActor(t *testing.T) {
	ctx := testDB(t)
	action := testID("action")
	actor := testID("actor")
	if err := InsertAuditLog(ctx, action, actor, map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	if err := InsertAuditLog(ctx, action, testID("actor"), map[string]any{"ok": false}); err != nil {
		t.Fatal(err)
	}
	rows, err := ListAuditLogs(ctx, AuditFilter{Action: action, Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Actor != actor {
		t.Fatalf("actor filter failed: %+v", rows)
	}
}

func TestAuditRepository_TimeWindowLimitOffset(t *testing.T) {
	ctx := testDB(t)
	action := testID("action")
	actor := testID("actor")
	from := time.Now().UTC().Add(-time.Second)
	for i := 0; i < 3; i++ {
		if err := InsertAuditLog(ctx, action, actor, map[string]any{"i": i}); err != nil {
			t.Fatal(err)
		}
	}
	to := time.Now().UTC().Add(time.Second)
	rows, err := ListAuditLogs(ctx, AuditFilter{
		Action: action,
		Actor:  actor,
		From:   &from,
		To:     &to,
		Limit:  2,
		Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("limit/offset failed: got %d rows", len(rows))
	}
	for _, row := range rows {
		if row.OccurredAt.Before(from) || row.OccurredAt.After(to) {
			t.Fatalf("row outside window: %+v", row)
		}
	}
}
