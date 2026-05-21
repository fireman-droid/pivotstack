package proxy

import (
	"bufio"
	"encoding/json"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxBusinessTo int64 = 1<<31 - 1

type businessBoardResponse struct {
	Period           string            `json:"period"`
	From             int64             `json:"from"`
	To               int64             `json:"to"`
	IncludeGift      bool              `json:"include_gift"`
	KPI              businessKPIs      `json:"kpi"`
	RevenueBreakdown RevenueBreakdown  `json:"revenue_breakdown"`
	Channels         []channelUsageRow `json:"channels"`
	Models           []modelUsageRow   `json:"models"`
	Trend            []TrendPoint      `json:"trend"`
	Warnings         []string          `json:"warnings,omitempty"`
}

type businessKPIs struct {
	RevenueCNY    float64 `json:"revenue_cny"`
	CostCNY       float64 `json:"cost_cny"`
	ProfitCNY     float64 `json:"profit_cny"`
	MarginPercent float64 `json:"margin_percent"`
}

type RevenueBreakdown struct {
	BalanceCNY   float64            `json:"balance_cny"`
	TimeCardsCNY float64            `json:"time_cards_cny"`
	GiftCNY      float64            `json:"gift_cny"`
	TotalCNY     float64            `json:"total_cny"`
	Daily        map[string]float64 `json:"-"`
}

type TrendPoint struct {
	Date       string  `json:"date"`
	RevenueCNY float64 `json:"revenue_cny"`
	CostCNY    float64 `json:"cost_cny"`
}

type channelUsageRow struct {
	ChannelID       string  `json:"channel_id"`
	Alias           string  `json:"alias"`
	ChannelType     string  `json:"channel_type"`
	Requests        int64   `json:"requests"`
	Errors          int64   `json:"errors"`
	InputTokens     int64   `json:"tokens_in"`
	OutputTokens    int64   `json:"tokens_out"`
	Tokens          int64   `json:"tokens"`
	ChargedCNY      float64 `json:"charged_cny"`
	CostCNY         float64 `json:"cost_cny"`
	RevenueShareCNY float64 `json:"revenue_share_cny"`
	ProfitCNY       float64 `json:"profit_cny"`
	MarginPercent   float64 `json:"margin_percent"`
}

type modelUsageRow struct {
	Model           string  `json:"model"`
	ChannelID       string  `json:"channel_id"`
	Requests        int64   `json:"requests"`
	InputTokens     int64   `json:"tokens_in"`
	OutputTokens    int64   `json:"tokens_out"`
	Tokens          int64   `json:"tokens"`
	ChargedCNY      float64 `json:"charged_cny"`
	CostCNY         float64 `json:"cost_cny"`
	RevenueShareCNY float64 `json:"revenue_share_cny"`
	ProfitCNY       float64 `json:"profit_cny"`
	MarginPercent   float64 `json:"margin_percent"`
}

// GET /admin/api/business-board?period=today|7d|30d|custom&from=&to=&include_gift=&channel=&top_n=
//
// 经营看板：现金入账口径收入 + 调用发生口径成本。
//   revenue = recharge_records.jsonl 真实充值（含/不含赠送，admin_adjust 不算）
//   cost    = call_logs.jsonl × 渠道 cost 单价（direct 走配置；newapi 走上游 ratio）
//   profit  = revenue - cost，margin = profit / revenue
func (h *Handler) apiBusinessBoard(w http.ResponseWriter, r *http.Request) {
	period, from, to := resolveBusinessPeriod(r)
	includeGift := strings.EqualFold(r.URL.Query().Get("include_gift"), "true")
	filter := strings.TrimSpace(r.URL.Query().Get("channel"))
	topN := parseTopN(r, 10)

	revenue := aggregateRevenueV2(from, to, includeGift)
	usage := h.aggregateChannelCost(from, to, filter)
	allocateRevenueShare(revenue.TotalCNY, usage)

	profit := revenue.TotalCNY - usage.TotalCostCNY
	margin := 0.0
	if revenue.TotalCNY > 0 {
		margin = profit / revenue.TotalCNY * 100
	}

	writeAdminJSON(w, http.StatusOK, businessBoardResponse{
		Period:      period,
		From:        from,
		To:          to,
		IncludeGift: includeGift,
		KPI: businessKPIs{
			RevenueCNY:    revenue.TotalCNY,
			CostCNY:       usage.TotalCostCNY,
			ProfitCNY:     profit,
			MarginPercent: margin,
		},
		RevenueBreakdown: revenue,
		Channels:         sortedChannelRows(usage),
		Models:           sortedModelRows(usage, topN),
		Trend:            buildDailyTrend(from, to, revenue.Daily, usage.DailyCost),
		Warnings:         usage.Warnings,
	})
}

func resolveBusinessPeriod(r *http.Request) (string, int64, int64) {
	q := r.URL.Query()
	period := strings.TrimSpace(q.Get("period"))
	if period == "" {
		period = "today"
	}
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)

	switch period {
	case "all":
		return period, 0, maxBusinessTo
	case "custom":
		from := parseBusinessUnix(q.Get("from"))
		to := parseBusinessUnix(q.Get("to"))
		if to <= 0 {
			to = now.Unix()
		}
		return period, from, to
	case "7d":
		return period, now.Add(-7 * 24 * time.Hour).Unix(), now.Unix()
	case "30d":
		return period, now.Add(-30 * 24 * time.Hour).Unix(), now.Unix()
	case "this_month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst)
		return period, start.Unix(), now.Unix()
	case "today":
		fallthrough
	default:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, cst)
		return "today", start.Unix(), now.Unix()
	}
}

func parseTopN(r *http.Request, fallback int) int {
	if fallback <= 0 {
		fallback = 10
	}
	raw := r.URL.Query().Get("top_n")
	if raw == "" {
		raw = r.URL.Query().Get("topN")
	}
	n := fallback
	if raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			n = v
		}
	}
	if n < 1 {
		return 1
	}
	if n > 50 {
		return 50
	}
	return n
}

func parseBusinessUnix(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// aggregateRevenueV2 扫 recharge_records.jsonl，按 type 分桶累加 amountCNY。
// admin_adjust 视为校正不计入收入；admin_balance 算真实充值（之前漏算）。
func aggregateRevenueV2(from, to int64, includeGift bool) RevenueBreakdown {
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	out := RevenueBreakdown{Daily: map[string]float64{}}
	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		return out
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var rec RechargeRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec.Timestamp < from || rec.Timestamp > to {
			continue
		}
		addRevenueRecord(&out, rec, includeGift)
	}
	return out
}

func addRevenueRecord(out *RevenueBreakdown, rec RechargeRecord, includeGift bool) {
	addDaily := func(amount float64) {
		out.TotalCNY += amount
		if amount != 0 {
			out.Daily[businessDayKey(rec.Timestamp)] += amount
		}
	}
	switch rec.Type {
	case "code_redeem", "admin_balance":
		out.BalanceCNY += rec.AmountCNY
		addDaily(rec.AmountCNY)
	case "code_redeem_days":
		out.TimeCardsCNY += rec.AmountCNY
		addDaily(rec.AmountCNY)
	case "admin_gift":
		if includeGift {
			out.GiftCNY += rec.AmountCNY
			addDaily(rec.AmountCNY)
		}
	}
}
