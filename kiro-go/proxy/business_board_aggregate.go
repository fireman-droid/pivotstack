package proxy

import (
	"bufio"
	"encoding/json"
	"kiro-api-proxy/config"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// channelCostAggregate 是 aggregateChannelCost 的结果：
// 渠道 / 模型 维度的 token + cost 聚合，外加 warnings（缺 cache、缺配置等运维信号）。
type channelCostAggregate struct {
	Channels        map[string]*channelUsageRow
	Models          map[string]*modelUsageRow
	DailyCost       map[string]float64
	TotalChargedCNY float64
	TotalCostCNY    float64
	Warnings        []string
}

func newChannelCostAggregate() *channelCostAggregate {
	return &channelCostAggregate{
		Channels:  map[string]*channelUsageRow{},
		Models:    map[string]*modelUsageRow{},
		DailyCost: map[string]float64{},
	}
}

// aggregateChannelCost 扫 call_logs.jsonl，按 ChannelID + Model 聚合 token 用量 / 真实成本 / 用户支出。
//   - 只算 Status=="success" 的请求
//   - 时间窗严格 [from, to] 闭区间
//   - direct 渠道：走渠道配置 CostInputPerM/CostOutputPerM
//   - newapi 渠道：走上游 model_ratio cache
//   - 其他 / 缺 channel：cost=0（仍计入 unknown bucket，不丢请求量）
func (h *Handler) aggregateChannelCost(from, to int64, filter string) *channelCostAggregate {
	agg := newChannelCostAggregate()
	// filter 语义：
	//   空字符串 → 不过滤
	//   "unknown" → 只看缺 ChannelID 的旧日志 bucket（保留前端选中"unknown" 时的下钻能力）
	//   其他      → 渠道 runtime ID（normalize 去前后空白；支持带或不带 "direct:" 前缀）
	filter = strings.TrimSpace(filter)
	if filter != "" {
		filter = normalizeChannelID(filter)
	}

	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		return agg
	}
	defer f.Close()

	warningSeen := map[string]bool{}
	addWarning := func(w string) {
		if w != "" && !warningSeen[w] {
			warningSeen[w] = true
			agg.Warnings = append(agg.Warnings, w)
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var log CallLog
		if err := json.Unmarshal(scanner.Bytes(), &log); err != nil {
			continue
		}
		if !businessLogInScope(log, from, to, filter) {
			continue
		}
		model := firstNonEmpty(log.PriceModel, log.OriginalModel, log.ActualModel)
		costCNY, warning := h.logCostCNY(log, model)
		addWarning(warning)
		agg.addLog(log, model, chargedCNYFromLog(log), costCNY)
	}
	return agg
}

func businessLogInScope(log CallLog, from, to int64, filter string) bool {
	if log.Status != "success" {
		return false
	}
	if log.Timestamp < from || log.Timestamp > to {
		return false
	}
	if filter == "" {
		return true
	}
	chID := normalizeChannelID(log.ChannelID)
	return chID == filter || strings.TrimPrefix(chID, "direct:") == filter
}

func (h *Handler) logCostCNY(log CallLog, model string) (float64, string) {
	chID := normalizeChannelID(log.ChannelID)
	switch {
	case strings.HasPrefix(chID, "direct:"):
		return directLogCostCNY(chID, model, log.InputTokens, log.OutputTokens), ""
	default:
		if _, ok := config.GetNewAPIChannel(chID); ok {
			return h.newAPILogCostCNY(chID, model, log.InputTokens, log.OutputTokens)
		}
	}
	return 0, ""
}

func directLogCostCNY(chID, model string, in, out int) float64 {
	id := strings.TrimSpace(strings.TrimPrefix(chID, "direct:"))
	if id == "" {
		return 0
	}
	ch, ok := config.GetDirectChannel(id)
	if !ok {
		return 0
	}
	row, ok := directCostRowForModel(ch.SellPrice, model)
	if !ok || row.CostInputPerM < 0 || row.CostOutputPerM < 0 {
		return 0
	}
	usd := float64(nonNegativeInt(in))*row.CostInputPerM/1_000_000.0 +
		float64(nonNegativeInt(out))*row.CostOutputPerM/1_000_000.0
	return config.CNYFromVirtualUSD(usd)
}

// directCostRowForModel：模型 key 命中即返回该 row（即使 row 全 0），不要 fallback Default —
// admin 显式给某模型配置 0 成本是合法语义，不能被 Default 静默覆盖。
func directCostRowForModel(price config.DirectSellPrice, model string) (config.DirectSellPriceRow, bool) {
	target := normalizeChannelModelKey(model)
	for k, row := range price.Models {
		if normalizeChannelModelKey(k) == target {
			return row, true
		}
	}
	return price.Default, true
}

func (h *Handler) newAPILogCostCNY(chID, model string, in, out int) (float64, string) {
	ch, ok := config.GetNewAPIChannel(chID)
	if !ok {
		return 0, ""
	}
	provider, ok := config.GetNewAPIProvider(ch.ProviderID)
	if !ok || provider.QuotaPerUnitDollar <= 0 || provider.YuanPerUpstreamDollar <= 0 {
		return 0, "newapi provider cost config missing: " + chID
	}
	manager := h.ensureNewAPIManager()
	cache, ok := manager.Cache(ch.ProviderID)
	if !ok {
		return 0, "newapi pricing cache miss: " + chID
	}
	quota, _, err := estimateNewAPIQuotaWithRatios(cache, model, ch.GroupName, nonNegativeInt(in), nonNegativeInt(out))
	if err != nil {
		return 0, "newapi pricing lookup failed: " + chID + " model=" + model
	}
	return float64(quota) / provider.QuotaPerUnitDollar * provider.YuanPerUpstreamDollar, ""
}

func allocateRevenueShare(totalRevenueCNY float64, agg *channelCostAggregate) {
	if agg == nil {
		return
	}
	for _, row := range agg.Channels {
		applyRevenueShare(totalRevenueCNY, agg.TotalChargedCNY, &row.RevenueShareCNY, &row.ProfitCNY, &row.MarginPercent, row.ChargedCNY, row.CostCNY)
	}
	for _, row := range agg.Models {
		applyRevenueShare(totalRevenueCNY, agg.TotalChargedCNY, &row.RevenueShareCNY, &row.ProfitCNY, &row.MarginPercent, row.ChargedCNY, row.CostCNY)
	}
}

func applyRevenueShare(total, denom float64, share, profit, margin *float64, weight, cost float64) {
	if denom > 0 {
		*share = total * weight / denom
	}
	*profit = *share - cost
	if *share > 0 {
		*margin = *profit / *share * 100
	}
}

func buildDailyTrend(from, to int64, revenueDaily, costDaily map[string]float64) []TrendPoint {
	cst := time.FixedZone("CST", 8*3600)
	if to <= 0 {
		to = time.Now().Unix()
	}
	if from <= 0 {
		from = to
	}
	start := businessDayStart(time.Unix(from, 0).In(cst))
	end := businessDayStart(time.Unix(to, 0).In(cst))
	if end.Before(start) {
		start = end
	}
	if end.Sub(start) > 369*24*time.Hour {
		start = end.AddDate(0, 0, -369)
	}

	out := make([]TrendPoint, 0, int(end.Sub(start).Hours()/24)+1)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		out = append(out, TrendPoint{
			Date:       key,
			RevenueCNY: revenueDaily[key],
			CostCNY:    costDaily[key],
		})
	}
	return out
}

func (a *channelCostAggregate) addLog(log CallLog, model string, chargedCNY, costCNY float64) {
	chID := normalizeChannelID(log.ChannelID)
	row := a.channelRow(chID)
	addUsageToChannel(row, log, chargedCNY, costCNY)

	if strings.TrimSpace(model) == "" {
		model = "unknown"
	}
	modelKey := model + "::" + chID
	m := a.modelRow(modelKey, model, chID)
	addUsageToModel(m, log, chargedCNY, costCNY)

	a.TotalChargedCNY += chargedCNY
	a.TotalCostCNY += costCNY
	a.DailyCost[businessDayKey(log.Timestamp)] += costCNY
}

func (a *channelCostAggregate) channelRow(chID string) *channelUsageRow {
	if row := a.Channels[chID]; row != nil {
		return row
	}
	alias, typ := channelBusinessMeta(chID)
	row := &channelUsageRow{ChannelID: chID, Alias: alias, ChannelType: typ}
	a.Channels[chID] = row
	return row
}

func (a *channelCostAggregate) modelRow(key, model, chID string) *modelUsageRow {
	if row := a.Models[key]; row != nil {
		return row
	}
	row := &modelUsageRow{Model: model, ChannelID: chID}
	a.Models[key] = row
	return row
}

func addUsageToChannel(row *channelUsageRow, log CallLog, chargedCNY, costCNY float64) {
	row.Requests++
	if log.Error != "" {
		row.Errors++
	}
	row.InputTokens += int64(nonNegativeInt(log.InputTokens))
	row.OutputTokens += int64(nonNegativeInt(log.OutputTokens))
	if log.TotalTokens > 0 {
		row.Tokens += int64(nonNegativeInt(log.TotalTokens))
	} else {
		row.Tokens += int64(nonNegativeInt(log.InputTokens + log.OutputTokens))
	}
	row.ChargedCNY += chargedCNY
	row.CostCNY += costCNY
}

func addUsageToModel(row *modelUsageRow, log CallLog, chargedCNY, costCNY float64) {
	row.Requests++
	row.InputTokens += int64(nonNegativeInt(log.InputTokens))
	row.OutputTokens += int64(nonNegativeInt(log.OutputTokens))
	if log.TotalTokens > 0 {
		row.Tokens += int64(nonNegativeInt(log.TotalTokens))
	} else {
		row.Tokens += int64(nonNegativeInt(log.InputTokens + log.OutputTokens))
	}
	row.ChargedCNY += chargedCNY
	row.CostCNY += costCNY
}

func chargedCNYFromLog(log CallLog) float64 {
	charged := log.ChargedUSD
	if charged == 0 {
		charged = log.CostUSD
	}
	return config.CNYFromVirtualUSD(charged)
}

func channelBusinessMeta(chID string) (string, string) {
	if chID == "unknown" {
		return "(unknown)", "unknown"
	}
	if strings.HasPrefix(chID, "direct:") {
		id := strings.TrimPrefix(chID, "direct:")
		if ch, ok := config.GetDirectChannel(id); ok {
			return firstNonEmpty(ch.Alias, ch.ID), "direct:" + ch.Type
		}
		return chID, "direct"
	}
	if ch, ok := config.GetNewAPIChannel(chID); ok {
		return firstNonEmpty(ch.Alias, ch.ID), "newapi"
	}
	return chID, "unknown"
}

func sortedChannelRows(agg *channelCostAggregate) []channelUsageRow {
	rows := make([]channelUsageRow, 0, len(agg.Channels))
	for _, row := range agg.Channels {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CostCNY == rows[j].CostCNY {
			return rows[i].Requests > rows[j].Requests
		}
		return rows[i].CostCNY > rows[j].CostCNY
	})
	return rows
}

func sortedModelRows(agg *channelCostAggregate, n int) []modelUsageRow {
	rows := make([]modelUsageRow, 0, len(agg.Models))
	for _, row := range agg.Models {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CostCNY == rows[j].CostCNY {
			return rows[i].ChargedCNY > rows[j].ChargedCNY
		}
		return rows[i].CostCNY > rows[j].CostCNY
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

func businessDayKey(ts int64) string {
	cst := time.FixedZone("CST", 8*3600)
	return time.Unix(ts, 0).In(cst).Format("2006-01-02")
}

func businessDayStart(t time.Time) time.Time {
	cst := time.FixedZone("CST", 8*3600)
	ti := t.In(cst)
	return time.Date(ti.Year(), ti.Month(), ti.Day(), 0, 0, 0, 0, cst)
}

func nonNegativeInt(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func normalizeChannelID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "unknown"
	}
	return id
}
