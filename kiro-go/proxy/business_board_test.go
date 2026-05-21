package proxy

import (
	"encoding/json"
	"kiro-api-proxy/config"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// bbReset 给每个测试隔离的 config/jsonl 环境，重置 unit rate 到默认 20。
func bbReset(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := config.Init(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("config.Init: %v", err)
	}
	if err := config.UpdatePivotStackDollarsPerYuan(20, false); err != nil {
		t.Fatalf("UpdatePivotStackDollarsPerYuan: %v", err)
	}
	if err := config.UpdateDirectChannels(nil); err != nil {
		t.Fatalf("UpdateDirectChannels(nil): %v", err)
	}
	if err := config.UpdateNewAPIProviders(nil); err != nil {
		t.Fatalf("UpdateNewAPIProviders(nil): %v", err)
	}
	if err := config.UpdateNewAPIChannels(nil); err != nil {
		t.Fatalf("UpdateNewAPIChannels(nil): %v", err)
	}
	bbWriteCallLogs(t, nil)
	bbWriteRechargeRecords(t, nil)
}

func bbWriteCallLogs(t *testing.T, logs []CallLog) {
	t.Helper()
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if err := os.MkdirAll(config.GetDataDir(), 0755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	f, err := os.Create(filepath.Join(config.GetDataDir(), "call_logs.jsonl"))
	if err != nil {
		t.Fatalf("create call logs: %v", err)
	}
	defer f.Close()
	for _, log := range logs {
		data, _ := json.Marshal(log)
		_, _ = f.Write(append(data, '\n'))
	}
}

func bbWriteRechargeRecords(t *testing.T, records []RechargeRecord) {
	t.Helper()
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	if err := os.MkdirAll(config.GetDataDir(), 0755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	f, err := os.Create(filepath.Join(config.GetDataDir(), "recharge_records.jsonl"))
	if err != nil {
		t.Fatalf("create recharge records: %v", err)
	}
	defer f.Close()
	for _, rec := range records {
		data, _ := json.Marshal(rec)
		_, _ = f.Write(append(data, '\n'))
	}
}

func bbCallBusinessBoard(t *testing.T, query string) businessBoardResponse {
	t.Helper()
	h := &Handler{}
	target := "/admin/api/business-board"
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	h.apiBusinessBoard(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("apiBusinessBoard status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out businessBoardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
	}
	return out
}

func bbDirectChannel(id string, inCost, outCost float64) config.DirectChannel {
	return config.DirectChannel{
		ID:      id,
		Type:    "openai",
		Alias:   id,
		Enabled: true,
		SellPrice: config.DirectSellPrice{Default: config.DirectSellPriceRow{
			InputPerM:      100,
			OutputPerM:     200,
			CostInputPerM:  inCost,
			CostOutputPerM: outCost,
		}},
	}
}

func bbSuccessLog(ts int64, channelID, model string, in, out int) CallLog {
	return CallLog{
		Timestamp:     ts,
		Status:        "success",
		ChannelID:     channelID,
		OriginalModel: model,
		ActualModel:   model,
		PriceModel:    model,
		InputTokens:   in,
		OutputTokens:  out,
		TotalTokens:   in + out,
		ChargedUSD:    10,
		CostUSD:       10,
	}
}

func TestBusinessBoardEmptyJSONLAllZeros(t *testing.T) {
	bbReset(t)

	resp := bbCallBusinessBoard(t, "period=custom&from=100&to=200")
	if resp.KPI.RevenueCNY != 0 || resp.KPI.CostCNY != 0 || resp.KPI.ProfitCNY != 0 || resp.KPI.MarginPercent != 0 {
		t.Fatalf("kpi = %+v, want all zeros", resp.KPI)
	}
	if len(resp.Channels) != 0 || len(resp.Models) != 0 {
		t.Fatalf("expected no rows, got channels=%d models=%d", len(resp.Channels), len(resp.Models))
	}
}

func TestBusinessBoardDirectChannelConfiguredCost(t *testing.T) {
	bbReset(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{bbDirectChannel("d1", 20, 40)}); err != nil {
		t.Fatalf("UpdateDirectChannels: %v", err)
	}
	bbWriteCallLogs(t, []CallLog{bbSuccessLog(150, "direct:d1", "gpt-test", 1000, 2000)})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "")
	got := agg.TotalCostCNY
	// 1000 * 20/1M + 2000 * 40/1M = 0.02 + 0.08 = 0.10 (virtual$)
	want := config.CNYFromVirtualUSD(0.1)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("cost = %.12f, want %.12f", got, want)
	}
}

func TestBusinessBoardDirectChannelZeroCostNoFallback(t *testing.T) {
	bbReset(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{bbDirectChannel("d1", 0, 0)}); err != nil {
		t.Fatalf("UpdateDirectChannels: %v", err)
	}
	bbWriteCallLogs(t, []CallLog{bbSuccessLog(150, "direct:d1", "gpt-test", 1000, 2000)})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "")
	// CostInputPerM/CostOutputPerM = 0 → 成本 = 0（不 fallback 别处；零是合法配置）。
	if agg.TotalCostCNY != 0 {
		t.Fatalf("cost = %.12f, want 0", agg.TotalCostCNY)
	}
}

func TestBusinessBoardMissingChannelIDUnknownBucket(t *testing.T) {
	bbReset(t)
	log := bbSuccessLog(150, "", "gpt-test", 1000, 2000)
	bbWriteCallLogs(t, []CallLog{log})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "")
	row := agg.Channels["unknown"]
	if row == nil {
		t.Fatal("missing unknown bucket")
	}
	if row.Requests != 1 || row.CostCNY != 0 {
		t.Fatalf("unknown row = %+v, want one zero-cost request", row)
	}
}

func TestBusinessBoardNewAPICacheMissWarning(t *testing.T) {
	bbReset(t)
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{
		ID: "p1", Name: "P1", QuotaPerUnitDollar: 500000, YuanPerUpstreamDollar: 1, Enabled: true,
	}}); err != nil {
		t.Fatalf("UpdateNewAPIProviders: %v", err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{{
		ID: "p1:tok-1", ProviderID: "p1", Alias: "N1", GroupName: "default", Markup: 2, Enabled: true,
	}}); err != nil {
		t.Fatalf("UpdateNewAPIChannels: %v", err)
	}
	bbWriteCallLogs(t, []CallLog{bbSuccessLog(150, "p1:tok-1", "gpt-test", 1000, 2000)})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "")
	if agg.TotalCostCNY != 0 {
		t.Fatalf("cost = %.12f, want 0 on cache miss", agg.TotalCostCNY)
	}
	if len(agg.Warnings) == 0 {
		t.Fatal("expected cache miss warning")
	}
}

func TestBusinessBoardPeriodBoundariesInclusive(t *testing.T) {
	bbReset(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{bbDirectChannel("d1", 10, 10)}); err != nil {
		t.Fatalf("UpdateDirectChannels: %v", err)
	}
	bbWriteCallLogs(t, []CallLog{
		bbSuccessLog(99, "direct:d1", "m1", 1000, 0),
		bbSuccessLog(100, "direct:d1", "m1", 1000, 0),
		bbSuccessLog(200, "direct:d1", "m1", 1000, 0),
		bbSuccessLog(201, "direct:d1", "m1", 1000, 0),
	})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "")
	if row := agg.Channels["direct:d1"]; row == nil || row.Requests != 2 {
		t.Fatalf("row = %+v, want exactly two boundary-inclusive requests", row)
	}
}

func TestBusinessBoardChannelFilter(t *testing.T) {
	bbReset(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{
		bbDirectChannel("d1", 10, 10),
		bbDirectChannel("d2", 10, 10),
	}); err != nil {
		t.Fatalf("UpdateDirectChannels: %v", err)
	}
	bbWriteCallLogs(t, []CallLog{
		bbSuccessLog(150, "direct:d1", "m1", 1000, 0),
		bbSuccessLog(150, "direct:d2", "m1", 1000, 0),
	})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "direct:d1")
	if len(agg.Channels) != 1 || agg.Channels["direct:d1"] == nil {
		t.Fatalf("channels = %+v, want only direct:d1", agg.Channels)
	}
}

func TestBusinessBoardTopNCap(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/api/business-board?top_n=999", nil)
	if got := parseTopN(req, 10); got != 50 {
		t.Fatalf("parseTopN high cap = %d, want 50", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/admin/api/business-board?top_n=0", nil)
	if got := parseTopN(req, 10); got != 1 {
		t.Fatalf("parseTopN low cap = %d, want 1", got)
	}
}

func TestBusinessBoardChannelFilterUnknown(t *testing.T) {
	bbReset(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{bbDirectChannel("d1", 10, 10)}); err != nil {
		t.Fatalf("UpdateDirectChannels: %v", err)
	}
	bbWriteCallLogs(t, []CallLog{
		bbSuccessLog(150, "", "m1", 1000, 0),
		bbSuccessLog(150, "direct:d1", "m1", 1000, 0),
	})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "unknown")
	if len(agg.Channels) != 1 || agg.Channels["unknown"] == nil {
		t.Fatalf("channels = %+v, want only unknown bucket", agg.Channels)
	}
}

func TestBusinessBoardDirectModelZeroCostOverridesDefault(t *testing.T) {
	bbReset(t)
	ch := bbDirectChannel("d1", 20, 40)
	ch.SellPrice.Models = map[string]config.DirectSellPriceRow{
		"gpt-free-cost": {},
	}
	if err := config.UpdateDirectChannels([]config.DirectChannel{ch}); err != nil {
		t.Fatalf("UpdateDirectChannels: %v", err)
	}
	bbWriteCallLogs(t, []CallLog{
		bbSuccessLog(150, "direct:d1", "gpt-free-cost", 1000, 2000),
	})

	agg := (&Handler{}).aggregateChannelCost(100, 200, "")
	if agg.TotalCostCNY != 0 {
		t.Fatalf("cost = %.12f, want explicit per-model zero cost", agg.TotalCostCNY)
	}
}

func TestBusinessBoardRevenueAdminBalanceIncluded(t *testing.T) {
	bbReset(t)
	bbWriteRechargeRecords(t, []RechargeRecord{
		{Timestamp: 150, Type: "admin_balance", AmountCNY: 100},
		{Timestamp: 150, Type: "admin_adjust", AmountCNY: 50},
		{Timestamp: 150, Type: "admin_gift", AmountCNY: 30},
	})
	// includeGift=false: 只算 admin_balance (100); admin_adjust skip; admin_gift skip
	got := aggregateRevenueV2(100, 200, false)
	if math.Abs(got.TotalCNY-100) > 1e-9 {
		t.Fatalf("revenue (no gift) = %.4f, want 100", got.TotalCNY)
	}
	if math.Abs(got.BalanceCNY-100) > 1e-9 {
		t.Fatalf("revenue.BalanceCNY = %.4f, want 100", got.BalanceCNY)
	}
	// includeGift=true: 100 + 30 = 130
	got = aggregateRevenueV2(100, 200, true)
	if math.Abs(got.TotalCNY-130) > 1e-9 {
		t.Fatalf("revenue (with gift) = %.4f, want 130", got.TotalCNY)
	}
}
