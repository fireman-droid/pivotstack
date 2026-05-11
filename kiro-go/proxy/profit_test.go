package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"math"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ============== 工具 ==============

// writeRechargeRecords 把给定 records 写到 GetDataDir()/recharge_records.jsonl，
// 覆盖已有内容（保证测试隔离）。
func writeRechargeRecords(t *testing.T, recs []RechargeRecord) {
	t.Helper()
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()
	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create recharge file: %v", err)
	}
	defer f.Close()
	for _, r := range recs {
		data, _ := json.Marshal(r)
		f.Write(data)
		f.WriteString("\n")
	}
}

// resetCostEntries 清空 PricingConfig 里的 CostEntries（只在测试内用，TestMain 留下的脏数据要清）。
func resetCostEntries(t *testing.T) {
	t.Helper()
	p := config.GetPricing()
	p.ProCostEntries = nil
	p.FreeCostEntries = nil
	if err := config.UpdatePricing(p); err != nil {
		t.Fatalf("UpdatePricing: %v", err)
	}
}

// callProfitAPI 直接调 apiGetProfit，返回解析后的 map。
func callProfitAPI(t *testing.T, query string) map[string]interface{} {
	t.Helper()
	h := &Handler{}
	url := "/admin/api/profit"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	h.apiGetProfit(w, req)
	if w.Code != 200 {
		t.Fatalf("apiGetProfit: status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	return resp
}

func mkRecord(ts int64, t string, amountCNY float64) RechargeRecord {
	return RechargeRecord{
		Time:      "test",
		Timestamp: ts,
		KeyID:     "tk-test",
		Type:      t,
		AmountCNY: amountCNY,
		AmountUSD: amountCNY / config.CNYPerUSDFace,
	}
}

func nowUnix() int64 { return time.Now().Unix() }

// ============== 测试用例 ==============

// TestProfit_OnlyBalanceRedeems: 只有 balance 卡兑换 → revenue = sum(amountCNY)
func TestProfit_OnlyBalanceRedeems(t *testing.T) {
	resetCostEntries(t)
	writeRechargeRecords(t, []RechargeRecord{
		mkRecord(nowUnix()-10, "code_redeem", 50),
		mkRecord(nowUnix()-20, "code_redeem", 30),
	})
	resp := callProfitAPI(t, "period=this_month&include_gift=false")

	if rev := resp["revenue_cny"].(float64); math.Abs(rev-80) > 1e-6 {
		t.Errorf("revenue: got %.4f, want 80", rev)
	}
	bd := resp["revenue_breakdown"].(map[string]interface{})
	if bal := bd["balance_cards"].(float64); math.Abs(bal-80) > 1e-6 {
		t.Errorf("breakdown.balance_cards: got %.4f, want 80", bal)
	}
	if tc := bd["time_cards"].(float64); tc != 0 {
		t.Errorf("breakdown.time_cards: expect 0, got %.4f", tc)
	}
	if g := bd["gift"].(float64); g != 0 {
		t.Errorf("breakdown.gift: expect 0 (include_gift=false), got %.4f", g)
	}
}

// TestProfit_TimeCardWithSalePrice: SalePrice=30 兑换 → time_cards revenue +30
func TestProfit_TimeCardWithSalePrice(t *testing.T) {
	resetCostEntries(t)
	writeRechargeRecords(t, []RechargeRecord{
		mkRecord(nowUnix()-5, "code_redeem_days", 30),
		mkRecord(nowUnix()-3, "code_redeem", 10),
	})
	resp := callProfitAPI(t, "period=this_month&include_gift=false")
	bd := resp["revenue_breakdown"].(map[string]interface{})
	if bal := bd["balance_cards"].(float64); math.Abs(bal-10) > 1e-6 {
		t.Errorf("balance_cards: got %.4f, want 10", bal)
	}
	if tc := bd["time_cards"].(float64); math.Abs(tc-30) > 1e-6 {
		t.Errorf("time_cards: got %.4f, want 30", tc)
	}
	if rev := resp["revenue_cny"].(float64); math.Abs(rev-40) > 1e-6 {
		t.Errorf("revenue: got %.4f, want 40", rev)
	}
}

// TestProfit_TimeCardWithoutSalePrice: amountCNY=0（旧卡兼容）→ 不计入 revenue
func TestProfit_TimeCardWithoutSalePrice(t *testing.T) {
	resetCostEntries(t)
	writeRechargeRecords(t, []RechargeRecord{
		mkRecord(nowUnix()-5, "code_redeem_days", 0),
	})
	resp := callProfitAPI(t, "period=this_month&include_gift=false")
	if rev := resp["revenue_cny"].(float64); rev != 0 {
		t.Errorf("revenue: got %.4f, want 0 (legacy time card)", rev)
	}
}

// TestProfit_PeriodFilter: 上月流水不计入 this_month
func TestProfit_PeriodFilter(t *testing.T) {
	resetCostEntries(t)
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst).Unix()
	lastMonth := thisMonthStart - 86400 // 上月最后一秒前后

	writeRechargeRecords(t, []RechargeRecord{
		mkRecord(lastMonth, "code_redeem", 100),    // 上月
		mkRecord(now.Unix()-10, "code_redeem", 20), // 这月
	})
	resp := callProfitAPI(t, "period=this_month&include_gift=false")
	if rev := resp["revenue_cny"].(float64); math.Abs(rev-20) > 1e-6 {
		t.Errorf("this_month: got %.4f, want 20", rev)
	}
	resp2 := callProfitAPI(t, "period=last_month&include_gift=false")
	if rev := resp2["revenue_cny"].(float64); math.Abs(rev-100) > 1e-6 {
		t.Errorf("last_month: got %.4f, want 100", rev)
	}
}

// TestProfit_IncludeGift_Off: include_gift=false → admin_gift 不计入
func TestProfit_IncludeGift_Off(t *testing.T) {
	resetCostEntries(t)
	writeRechargeRecords(t, []RechargeRecord{
		mkRecord(nowUnix()-5, "code_redeem", 10),
		mkRecord(nowUnix()-3, "admin_gift", 50),
	})
	resp := callProfitAPI(t, "period=this_month&include_gift=false")
	if rev := resp["revenue_cny"].(float64); math.Abs(rev-10) > 1e-6 {
		t.Errorf("include_gift=false revenue: got %.4f, want 10", rev)
	}
}

// TestProfit_IncludeGift_On: include_gift=true → admin_gift 计入
func TestProfit_IncludeGift_On(t *testing.T) {
	resetCostEntries(t)
	writeRechargeRecords(t, []RechargeRecord{
		mkRecord(nowUnix()-5, "code_redeem", 10),
		mkRecord(nowUnix()-3, "admin_gift", 50),
	})
	resp := callProfitAPI(t, "period=this_month&include_gift=true")
	if rev := resp["revenue_cny"].(float64); math.Abs(rev-60) > 1e-6 {
		t.Errorf("include_gift=true revenue: got %.4f, want 60", rev)
	}
	bd := resp["revenue_breakdown"].(map[string]interface{})
	if g := bd["gift"].(float64); math.Abs(g-50) > 1e-6 {
		t.Errorf("breakdown.gift: got %.4f, want 50", g)
	}
}

// TestProfit_CostFromCostEntries: PRO 500 + FREE 100 → cost=600 按 pool 分桶
func TestProfit_CostFromCostEntries(t *testing.T) {
	resetCostEntries(t)
	now := nowUnix()
	if err := config.AddCostEntry("pro", config.CostEntry{
		ID: "p1", Count: 5, CostCNY: 500, Credits: 1500, CreatedAt: now - 100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.AddCostEntry("free", config.CostEntry{
		ID: "f1", Count: 50, CostCNY: 100, CreatedAt: now - 100,
	}); err != nil {
		t.Fatal(err)
	}
	writeRechargeRecords(t, nil) // 空流水
	resp := callProfitAPI(t, "period=this_month&include_gift=false")
	if c := resp["cost_cny"].(float64); math.Abs(c-600) > 1e-6 {
		t.Errorf("cost_cny: got %.4f, want 600", c)
	}
	bd := resp["cost_breakdown"].(map[string]interface{})
	if pro := bd["pro"].(float64); math.Abs(pro-500) > 1e-6 {
		t.Errorf("cost.pro: got %.4f, want 500", pro)
	}
	if free := bd["free"].(float64); math.Abs(free-100) > 1e-6 {
		t.Errorf("cost.free: got %.4f, want 100", free)
	}
}

// TestProfit_CostEntry_PeriodFilter: AddedAt 在 period 外的 entry 不计入
func TestProfit_CostEntry_PeriodFilter(t *testing.T) {
	resetCostEntries(t)
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst).Unix()

	// 一条上月的，一条这月的
	if err := config.AddCostEntry("pro", config.CostEntry{
		ID: "p_old", Count: 1, CostCNY: 999, Credits: 100, CreatedAt: thisMonthStart - 86400,
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.AddCostEntry("pro", config.CostEntry{
		ID: "p_new", Count: 1, CostCNY: 200, Credits: 100, CreatedAt: now.Unix() - 10,
	}); err != nil {
		t.Fatal(err)
	}
	writeRechargeRecords(t, nil)

	resp := callProfitAPI(t, "period=this_month&include_gift=false")
	if c := resp["cost_cny"].(float64); math.Abs(c-200) > 1e-6 {
		t.Errorf("this_month cost: got %.4f, want 200 (上月 999 不应计入)", c)
	}
}

// TestProfit_ZeroRevenue_MarginIsZero: revenue=0 → margin 为 0（前端展示 "—"）
func TestProfit_ZeroRevenue_MarginIsZero(t *testing.T) {
	resetCostEntries(t)
	if err := config.AddCostEntry("pro", config.CostEntry{
		ID: "p1", Count: 1, CostCNY: 100, Credits: 100, CreatedAt: nowUnix() - 10,
	}); err != nil {
		t.Fatal(err)
	}
	writeRechargeRecords(t, nil)
	resp := callProfitAPI(t, "period=this_month")
	if m := resp["margin_percent"].(float64); m != 0 {
		t.Errorf("margin: got %.4f, want 0 when revenue=0", m)
	}
	if p := resp["profit_cny"].(float64); math.Abs(p-(-100)) > 1e-6 {
		t.Errorf("profit: got %.4f, want -100 (亏损)", p)
	}
}

// TestProfit_PersistedIncludeGift: include_gift query 不传时回退 Settings 持久化值
func TestProfit_PersistedIncludeGift(t *testing.T) {
	resetCostEntries(t)
	writeRechargeRecords(t, []RechargeRecord{
		mkRecord(nowUnix()-3, "admin_gift", 77),
	})
	prev := config.GetProfitIncludeGift()
	defer config.UpdateProfitIncludeGift(prev)

	_ = config.UpdateProfitIncludeGift(true)
	resp := callProfitAPI(t, "period=this_month") // 不传 include_gift
	if rev := resp["revenue_cny"].(float64); math.Abs(rev-77) > 1e-6 {
		t.Errorf("settings=true: revenue got %.4f, want 77", rev)
	}

	_ = config.UpdateProfitIncludeGift(false)
	resp2 := callProfitAPI(t, "period=this_month")
	if rev := resp2["revenue_cny"].(float64); rev != 0 {
		t.Errorf("settings=false: revenue got %.4f, want 0", rev)
	}
}

// TestResolveProfitPeriod: 各 period 字符串解析正确
func TestResolveProfitPeriod(t *testing.T) {
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst).Unix()
	lastMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, cst).Unix()

	cases := []struct {
		period string
		from   int64
		toMin  int64 // to 用 >= 比较（now 在变）
	}{
		{"this_month", thisMonthStart, now.Unix() - 60},
		{"last_month", lastMonthStart, thisMonthStart - 1},
	}
	for _, c := range cases {
		from, to := resolveProfitPeriod(c.period, "", "")
		if from != c.from {
			t.Errorf("%s: from got %d, want %d", c.period, from, c.from)
		}
		if to < c.toMin {
			t.Errorf("%s: to got %d, want >= %d", c.period, to, c.toMin)
		}
	}

	// custom
	from, to := resolveProfitPeriod("custom", "100", "200")
	if from != 100 || to != 200 {
		t.Errorf("custom: got [%d, %d], want [100, 200]", from, to)
	}

	// all
	from, to = resolveProfitPeriod("all", "", "")
	if from != 0 || to != math.MaxInt32 {
		t.Errorf("all: got [%d, %d], want [0, maxint32]", from, to)
	}
}

// TestEntryInWindow: 边界
func TestEntryInWindow(t *testing.T) {
	cases := []struct {
		name             string
		createdAt, from, to int64
		want             bool
	}{
		{"zero CreatedAt always in", 0, 100, 200, true},
		{"in range", 150, 100, 200, true},
		{"on lower bound", 100, 100, 200, true},
		{"on upper bound", 200, 100, 200, true},
		{"before window", 50, 100, 200, false},
		{"after window", 250, 100, 200, false},
	}
	for _, c := range cases {
		got := entryInWindow(c.createdAt, c.from, c.to)
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

var _ = fmt.Sprintf // keep fmt import if unused
