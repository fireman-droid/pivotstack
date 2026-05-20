package db

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

var callLogsPartitionNameRE = regexp.MustCompile(`^call_logs_\d{4}_\d{2}$`)

type CallLogRow struct {
	ID              string
	OccurredAt      time.Time
	TimestampUnix   int64
	DayCST          time.Time
	TimeLabel       string
	RequestID       string
	APIType         string
	OriginalModel   string
	ActualModel     string
	Account         string
	APIKeyID        string
	InputTokens     int
	OutputTokens    int
	TotalTokens     int
	Credits         decimal.Decimal
	UpstreamCredits decimal.Decimal
	PaidCredits     decimal.Decimal
	GiftedCredits   decimal.Decimal
	CostUSD         decimal.Decimal
	ChargedUSD      decimal.Decimal
	CostUSDLegacy   decimal.Decimal
	PriceModel      string
	Stream          bool
	Error           string
	PayloadKB       int
	Status          string
	StopReason      string
	DurationMS      int64
	Attempt         int
	Subscription    string
	RequestSummary  string
	ResponseSummary string
	ChannelID       string
	ChannelType     string
	BillingMode     string
	BillingStatus   string
	UsageEstimated  bool
	RawPayload      map[string]any
}

type CallLogFilter struct {
	APIKeyID     string
	ChannelID    string
	Status       string
	ErrorOnly    bool
	RequestID    string
	OccurredFrom *time.Time
	OccurredTo   *time.Time
	Limit        int
	Offset       int
}

func PartitionTableName(month time.Time) string {
	m := normalizePartitionMonth(month)
	return fmt.Sprintf("call_logs_%04d_%02d", m.Year(), int(m.Month()))
}

func EnsureCallLogsPartition(ctx context.Context, month time.Time) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	start := normalizePartitionMonth(month)
	end := start.AddDate(0, 1, 0)
	name := PartitionTableName(start)
	if !callLogsPartitionNameRE.MatchString(name) {
		return fmt.Errorf("invalid call_logs partition name: %s", name)
	}
	sql := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s PARTITION OF call_logs FOR VALUES FROM ('%s') TO ('%s')",
		name,
		start.Format("2006-01-02 15:04:05-07"),
		end.Format("2006-01-02 15:04:05-07"),
	)
	if _, err := p.Exec(ctx, sql); err != nil {
		return fmt.Errorf("ensure call_logs partition: %w", err)
	}
	return nil
}

func InsertCallLog(ctx context.Context, row CallLogRow) (bool, error) {
	p, err := requirePool()
	if err != nil {
		return false, err
	}
	raw, err := jsonObjectParam(row.RawPayload)
	if err != nil {
		return false, err
	}
	tag, err := p.Exec(ctx, `
		INSERT INTO call_logs (
			id, occurred_at, timestamp_unix, day_cst, time_label,
			request_id, api_type, original_model, actual_model, account,
			api_key_id, input_tokens, output_tokens, total_tokens, credits,
			upstream_credits, paid_credits, gifted_credits, cost_usd,
			charged_usd, cost_usd_legacy, price_model, stream, error,
			payload_kb, status, stop_reason, duration_ms, attempt,
			subscription, request_summary, response_summary, channel_id,
			channel_type, billing_mode, billing_status, usage_estimated,
			raw_payload
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,
			$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38
		)
		ON CONFLICT (id, occurred_at) DO NOTHING
	`, row.ID, row.OccurredAt.UTC(), row.TimestampUnix, dateFromTime(row.DayCST),
		row.TimeLabel, textFromString(row.RequestID), row.APIType,
		textFromString(row.OriginalModel), textFromString(row.ActualModel),
		textFromString(row.Account), textFromString(row.APIKeyID), row.InputTokens,
		row.OutputTokens, row.TotalTokens, numericFromDecimal(row.Credits),
		numericFromDecimal(row.UpstreamCredits), numericFromDecimal(row.PaidCredits),
		numericFromDecimal(row.GiftedCredits), numericFromDecimal(row.CostUSD),
		numericFromDecimal(row.ChargedUSD), numericFromDecimal(row.CostUSDLegacy),
		textFromString(row.PriceModel), row.Stream, textFromString(row.Error),
		row.PayloadKB, row.Status, textFromString(row.StopReason), row.DurationMS,
		row.Attempt, textFromString(row.Subscription), textFromString(row.RequestSummary),
		textFromString(row.ResponseSummary), textFromString(row.ChannelID),
		textFromString(row.ChannelType), textFromString(row.BillingMode),
		textFromString(row.BillingStatus), row.UsageEstimated, raw)
	if err != nil {
		return false, fmt.Errorf("insert call log: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func ListCallLogs(ctx context.Context, f CallLogFilter) ([]CallLogRow, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := []string{"TRUE"}
	args := []any{}
	if strings.TrimSpace(f.APIKeyID) != "" {
		args = append(args, strings.TrimSpace(f.APIKeyID))
		where = append(where, fmt.Sprintf("api_key_id=$%d", len(args)))
	}
	if strings.TrimSpace(f.ChannelID) != "" {
		args = append(args, strings.TrimSpace(f.ChannelID))
		where = append(where, fmt.Sprintf("channel_id=$%d", len(args)))
	}
	if strings.TrimSpace(f.Status) != "" {
		args = append(args, strings.TrimSpace(f.Status))
		where = append(where, fmt.Sprintf("status=$%d", len(args)))
	}
	if f.ErrorOnly {
		where = append(where, "(status='error' OR (error IS NOT NULL AND error <> ''))")
	}
	if strings.TrimSpace(f.RequestID) != "" {
		args = append(args, strings.TrimSpace(f.RequestID))
		where = append(where, fmt.Sprintf("request_id=$%d", len(args)))
	}
	if f.OccurredFrom != nil {
		args = append(args, f.OccurredFrom.UTC())
		where = append(where, fmt.Sprintf("occurred_at >= $%d", len(args)))
	}
	if f.OccurredTo != nil {
		args = append(args, f.OccurredTo.UTC())
		where = append(where, fmt.Sprintf("occurred_at <= $%d", len(args)))
	}
	limit := normalizeListLimit(f.Limit)
	args = append(args, limit)
	limitParam := len(args)
	args = append(args, normalizeListOffset(f.Offset))
	offsetParam := len(args)

	rows, err := p.Query(ctx, callLogSelectSQL(strings.Join(where, " AND "))+`
		ORDER BY occurred_at DESC, id DESC
		LIMIT $`+fmt.Sprint(limitParam)+` OFFSET $`+fmt.Sprint(offsetParam), args...)
	if err != nil {
		return nil, fmt.Errorf("list call logs: %w", err)
	}
	defer rows.Close()

	var out []CallLogRow
	for rows.Next() {
		row, err := scanCallLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate call logs: %w", err)
	}
	return out, nil
}

func normalizePartitionMonth(month time.Time) time.Time {
	utc := month.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func callLogSelectSQL(where string) string {
	return `
		SELECT id, occurred_at, timestamp_unix, day_cst, time_label,
			request_id, api_type, original_model, actual_model, account,
			api_key_id, input_tokens, output_tokens, total_tokens, credits,
			upstream_credits, paid_credits, gifted_credits, cost_usd,
			charged_usd, cost_usd_legacy, price_model, stream, error,
			payload_kb, status, stop_reason, duration_ms, attempt,
			subscription, request_summary, response_summary, channel_id,
			channel_type, billing_mode, billing_status, usage_estimated,
			raw_payload
		FROM call_logs
		WHERE ` + where
}

type callLogScanner interface {
	Scan(dest ...any) error
}

func scanCallLog(scanner callLogScanner) (CallLogRow, error) {
	var row CallLogRow
	var day pgtype.Date
	var requestID, originalModel, actualModel, account, apiKeyID pgtype.Text
	var priceModel, errText, stopReason, subscription pgtype.Text
	var requestSummary, responseSummary, channelID, channelType pgtype.Text
	var billingMode, billingStatus pgtype.Text
	var credits, upstreamCredits, paidCredits, giftedCredits pgtype.Numeric
	var costUSD, chargedUSD, costUSDLegacy pgtype.Numeric
	var raw []byte
	if err := scanner.Scan(
		&row.ID, &row.OccurredAt, &row.TimestampUnix, &day, &row.TimeLabel,
		&requestID, &row.APIType, &originalModel, &actualModel, &account,
		&apiKeyID, &row.InputTokens, &row.OutputTokens, &row.TotalTokens,
		&credits, &upstreamCredits, &paidCredits, &giftedCredits, &costUSD,
		&chargedUSD, &costUSDLegacy, &priceModel, &row.Stream, &errText,
		&row.PayloadKB, &row.Status, &stopReason, &row.DurationMS, &row.Attempt,
		&subscription, &requestSummary, &responseSummary, &channelID,
		&channelType, &billingMode, &billingStatus, &row.UsageEstimated, &raw,
	); err != nil {
		return CallLogRow{}, fmt.Errorf("scan call log: %w", err)
	}
	var err error
	row.DayCST = timeFromDate(day)
	row.RequestID = stringFromText(requestID)
	row.OriginalModel = stringFromText(originalModel)
	row.ActualModel = stringFromText(actualModel)
	row.Account = stringFromText(account)
	row.APIKeyID = stringFromText(apiKeyID)
	row.PriceModel = stringFromText(priceModel)
	row.Error = stringFromText(errText)
	row.StopReason = stringFromText(stopReason)
	row.Subscription = stringFromText(subscription)
	row.RequestSummary = stringFromText(requestSummary)
	row.ResponseSummary = stringFromText(responseSummary)
	row.ChannelID = stringFromText(channelID)
	row.ChannelType = stringFromText(channelType)
	row.BillingMode = stringFromText(billingMode)
	row.BillingStatus = stringFromText(billingStatus)
	if row.Credits, err = decimalFromNumeric(credits); err != nil {
		return CallLogRow{}, err
	}
	if row.UpstreamCredits, err = decimalFromNumeric(upstreamCredits); err != nil {
		return CallLogRow{}, err
	}
	if row.PaidCredits, err = decimalFromNumeric(paidCredits); err != nil {
		return CallLogRow{}, err
	}
	if row.GiftedCredits, err = decimalFromNumeric(giftedCredits); err != nil {
		return CallLogRow{}, err
	}
	if row.CostUSD, err = decimalFromNumeric(costUSD); err != nil {
		return CallLogRow{}, err
	}
	if row.ChargedUSD, err = decimalFromNumeric(chargedUSD); err != nil {
		return CallLogRow{}, err
	}
	if row.CostUSDLegacy, err = decimalFromNumeric(costUSDLegacy); err != nil {
		return CallLogRow{}, err
	}
	row.RawPayload, err = scanJSONMap(raw)
	return row, err
}
