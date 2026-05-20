package db

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func testCallLogRow(id string, at time.Time, channelID, model, status string) CallLogRow {
	return CallLogRow{
		ID:              id,
		OccurredAt:      at,
		TimestampUnix:   at.Unix(),
		DayCST:          ComputeDayCST(at),
		TimeLabel:       at.Format(time.RFC3339),
		RequestID:       testID("req"),
		APIType:         "chat",
		OriginalModel:   model,
		ActualModel:     model + "-actual",
		Account:         "account@example.com",
		APIKeyID:        testID("key"),
		InputTokens:     10,
		OutputTokens:    20,
		TotalTokens:     30,
		Credits:         decimal.RequireFromString("0.30"),
		UpstreamCredits: decimal.RequireFromString("0.10"),
		PaidCredits:     decimal.RequireFromString("0.20"),
		GiftedCredits:   decimal.RequireFromString("0.10"),
		CostUSD:         decimal.RequireFromString("0.05"),
		ChargedUSD:      decimal.RequireFromString("0.30"),
		CostUSDLegacy:   decimal.RequireFromString("0.04"),
		PriceModel:      model,
		Stream:          false,
		Error:           "",
		PayloadKB:       3,
		Status:          status,
		StopReason:      "stop",
		DurationMS:      123,
		Attempt:         1,
		Subscription:    "pro",
		RequestSummary:  "request",
		ResponseSummary: "response",
		ChannelID:       channelID,
		ChannelType:     "direct",
		BillingMode:     "token",
		BillingStatus:   "settled",
		UsageEstimated:  false,
		RawPayload:      map[string]any{"id": id},
	}
}

func TestPartitionTableName(t *testing.T) {
	got := PartitionTableName(time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC))
	if got != "call_logs_2026_05" {
		t.Fatalf("partition name = %s", got)
	}
}

func TestEnsureCallLogsPartition_Idempotent(t *testing.T) {
	ctx := testDB(t)
	month := time.Now().UTC()
	if err := EnsureCallLogsPartition(ctx, month); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCallLogsPartition(ctx, month); err != nil {
		t.Fatal(err)
	}
}

func TestInsertCallLogAndBoardQuery(t *testing.T) {
	ctx := testDB(t)
	now := time.Now().UTC()
	if err := EnsureCallLogsPartition(ctx, now); err != nil {
		t.Fatal(err)
	}
	success := testCallLogRow(testID("call"), now, testID("channel"), "claude-sonnet", "success")
	errRow := testCallLogRow(testID("call"), now.Add(time.Minute), testID("channel"), "gpt-4.1", "error")
	errRow.Error = "upstream failed"
	for _, row := range []CallLogRow{success, errRow} {
		ok, err := InsertCallLog(ctx, row)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("first insert returned duplicate")
		}
	}
	ok, err := InsertCallLog(ctx, success)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("duplicate call log insert returned ok")
	}

	buckets, err := CallBoardQuery(ctx, now.Add(-time.Hour), now.Add(time.Hour), "")
	if err != nil {
		t.Fatal(err)
	}
	foundSuccess, foundError := false, false
	for _, b := range buckets {
		switch b.ChannelID {
		case success.ChannelID:
			if b.Model == success.PriceModel {
				foundSuccess = true
				if b.Requests != 1 || b.Errors != 0 || b.Tokens != 30 || !b.ChargedUSD.Equal(decimal.RequireFromString("0.30")) {
					t.Fatalf("success bucket = %+v", b)
				}
			}
		case errRow.ChannelID:
			if b.Model == errRow.PriceModel {
				foundError = true
				if b.Requests != 1 || b.Errors != 1 || b.TokensIn != 10 || !b.PaidRevenueUSD.Equal(decimal.RequireFromString("0.05")) {
					t.Fatalf("error bucket = %+v", b)
				}
			}
		}
	}
	if !foundSuccess || !foundError {
		t.Fatalf("missing buckets success=%v error=%v all=%+v", foundSuccess, foundError, buckets)
	}
}

func TestListCallLogsFilters(t *testing.T) {
	ctx := testDB(t)
	now := time.Now().UTC()
	if err := EnsureCallLogsPartition(ctx, now); err != nil {
		t.Fatal(err)
	}
	target := testCallLogRow(testID("call"), now, testID("channel"), "claude", "success")
	other := testCallLogRow(testID("call"), now.Add(time.Minute), testID("channel"), "gpt", "error")
	other.Error = "boom"
	for _, row := range []CallLogRow{target, other} {
		if ok, err := InsertCallLog(ctx, row); err != nil || !ok {
			t.Fatalf("insert call ok=%v err=%v", ok, err)
		}
	}
	byChannel, err := ListCallLogs(ctx, CallLogFilter{ChannelID: target.ChannelID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byChannel) != 1 || byChannel[0].ID != target.ID {
		t.Fatalf("by channel = %+v", byChannel)
	}
	byRequest, err := ListCallLogs(ctx, CallLogFilter{RequestID: target.RequestID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byRequest) != 1 || byRequest[0].ID != target.ID {
		t.Fatalf("by request = %+v", byRequest)
	}
	byKey, err := ListCallLogs(ctx, CallLogFilter{APIKeyID: target.APIKeyID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byKey) != 1 || byKey[0].ID != target.ID {
		t.Fatalf("by api key = %+v", byKey)
	}
	errorsOnly, err := ListCallLogs(ctx, CallLogFilter{ErrorOnly: true, RequestID: other.RequestID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(errorsOnly) != 1 || errorsOnly[0].ID != other.ID {
		t.Fatalf("error only = %+v", errorsOnly)
	}
}
