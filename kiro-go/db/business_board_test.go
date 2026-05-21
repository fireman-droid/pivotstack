package db

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBusinessBoard_CallQueryChannelFilter(t *testing.T) {
	ctx := testDB(t)
	now := time.Now().UTC()
	if err := EnsureCallLogsPartition(ctx, now); err != nil {
		t.Fatal(err)
	}
	channel := testID("channel")
	target := testCallLogRow(testID("call"), now, channel, "claude-opus", "success")
	other := testCallLogRow(testID("call"), now.Add(time.Minute), testID("channel"), "claude-opus", "success")
	for _, row := range []CallLogRow{target, other} {
		if ok, err := InsertCallLog(ctx, row); err != nil || !ok {
			t.Fatalf("insert call ok=%v err=%v", ok, err)
		}
	}
	buckets, err := CallBoardQuery(ctx, now.Add(-time.Hour), now.Add(time.Hour), channel)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 {
		t.Fatalf("buckets = %+v", buckets)
	}
	if buckets[0].ChannelID != channel || buckets[0].Requests != 1 || !buckets[0].ChargedUSD.Equal(decimal.RequireFromString("0.30")) {
		t.Fatalf("filtered bucket = %+v", buckets[0])
	}
}

func TestBusinessBoard_RechargeQueryTypeAllowList(t *testing.T) {
	ctx := testDB(t)
	// 用未来时间窗口隔离，确保 included bucket 的 sum 严格等于本测试插入值；t.Cleanup 防止连续跑互污染。
	now := time.Now().UTC().AddDate(101, 0, 0)
	included := testRechargeRow(testID("recharge"), "admin_balance", now, "21.00")
	excluded := testRechargeRow(testID("recharge"), "admin_adjust", now, "99.00")
	rows := []RechargeRecordRow{included, excluded}
	for _, row := range rows {
		if ok, err := InsertRecharge(ctx, row); err != nil || !ok {
			t.Fatalf("insert recharge ok=%v err=%v", ok, err)
		}
	}
	t.Cleanup(func() { cleanupRecharges(t, rows) })
	buckets, err := RechargeBoardQuery(ctx, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	foundIncluded := false
	for _, b := range buckets {
		if b.Type == "admin_adjust" {
			t.Fatalf("excluded type returned: %+v", b)
		}
		if b.Type == "admin_balance" && b.AmountCNY.Equal(decimal.RequireFromString("21.00")) {
			foundIncluded = true
		}
	}
	if !foundIncluded {
		t.Fatalf("included bucket missing: %+v", buckets)
	}
}
