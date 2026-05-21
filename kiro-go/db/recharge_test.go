package db

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func cleanupRecharges(t *testing.T, rows []RechargeRecordRow) {
	t.Helper()
	p, err := requirePool()
	if err != nil {
		return
	}
	ctx := context.Background()
	for _, row := range rows {
		if _, err := p.Exec(ctx, `DELETE FROM recharge_records WHERE id=$1`, row.ID); err != nil {
			t.Logf("cleanup recharge %s: %v", row.ID, err)
		}
	}
}

func testRechargeRow(id, typ string, at time.Time, amountCNY string) RechargeRecordRow {
	return RechargeRecordRow{
		ID:            id,
		TimeLabel:     at.Format(time.RFC3339),
		TimestampUnix: at.Unix(),
		OccurredAt:    at,
		DayCST:        ComputeDayCST(at),
		APIKeyID:      testID("key"),
		UserID:        "",
		KeyNote:       "test key",
		Type:          typ,
		Code:          testID("code"),
		AmountUSD:     decimal.RequireFromString("1.25"),
		AmountCNY:     decimal.RequireFromString(amountCNY),
		BalanceBefore: decimal.RequireFromString("10"),
		BalanceAfter:  decimal.RequireFromString("11.25"),
		GiftBefore:    decimal.RequireFromString("2"),
		GiftAfter:     decimal.RequireFromString("2"),
		Operator:      "test",
		Note:          "stage4",
		IP:            "127.0.0.1",
		RawPayload:    map[string]any{"id": id},
	}
}

func TestComputeDayCST(t *testing.T) {
	got := ComputeDayCST(time.Date(2026, 5, 21, 0, 30, 0, 0, time.UTC))
	if got.Year() != 2026 || got.Month() != time.May || got.Day() != 21 || got.Hour() != 0 {
		t.Fatalf("day cst = %s", got)
	}
	if got.Location().String() != "CST" {
		t.Fatalf("location = %s", got.Location())
	}
}

func TestInsertRechargeAndList(t *testing.T) {
	ctx := testDB(t)
	now := time.Now().UTC()
	row := testRechargeRow(testID("recharge"), "admin_balance", now, "12.50")
	ok, err := InsertRecharge(ctx, row)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("first insert returned duplicate")
	}
	ok, err = InsertRecharge(ctx, row)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("duplicate insert returned ok")
	}

	rows, err := ListRecharges(ctx, RechargeFilter{
		UserID:   row.UserID,
		APIKeyID: row.APIKeyID,
		Type:     row.Type,
		Limit:    10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != row.ID || !rows[0].AmountCNY.Equal(decimal.RequireFromString("12.50")) {
		t.Fatalf("unexpected recharge rows: %+v", rows)
	}
	if rows[0].RawPayload["id"] != row.ID {
		t.Fatalf("raw payload = %+v", rows[0].RawPayload)
	}
}

func TestRechargeBoardQuery(t *testing.T) {
	ctx := testDB(t)
	// 用未来时间窗口隔离当前批数据；t.Cleanup 删除避免连续跑同一测试互相污染聚合 SUM。
	base := time.Now().UTC().AddDate(100, 0, 0)
	day1 := base
	day2 := base.AddDate(0, 0, -1)
	rows := []RechargeRecordRow{
		testRechargeRow(testID("recharge"), "code_redeem", day1, "10.25"),
		testRechargeRow(testID("recharge"), "code_redeem", day1.Add(time.Minute), "2.75"),
		testRechargeRow(testID("recharge"), "admin_gift", day2, "5.00"),
	}
	for _, row := range rows {
		if ok, err := InsertRecharge(ctx, row); err != nil || !ok {
			t.Fatalf("insert recharge ok=%v err=%v", ok, err)
		}
	}
	t.Cleanup(func() { cleanupRecharges(t, rows) })
	buckets, err := RechargeBoardQuery(ctx, day2.Add(-time.Hour), day1.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var codeTotal, giftTotal decimal.Decimal
	for _, b := range buckets {
		switch b.Type {
		case "code_redeem":
			codeTotal = codeTotal.Add(b.AmountCNY)
		case "admin_gift":
			giftTotal = giftTotal.Add(b.AmountCNY)
		}
	}
	if !codeTotal.Equal(decimal.RequireFromString("13.00")) || !giftTotal.Equal(decimal.RequireFromString("5.00")) {
		t.Fatalf("bucket totals code=%s gift=%s buckets=%+v", codeTotal, giftTotal, buckets)
	}
}
