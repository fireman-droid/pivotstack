package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type RechargeBucket struct {
	Type      string
	DayCST    time.Time
	AmountCNY decimal.Decimal
}

type CallBucket struct {
	ChannelID      string
	Model          string
	DayCST         time.Time
	Requests       int64
	Errors         int64
	TokensIn       int64
	TokensOut      int64
	Tokens         int64
	ChargedUSD     decimal.Decimal
	PaidRevenueUSD decimal.Decimal
}

func RechargeBoardQuery(ctx context.Context, from, to time.Time) ([]RechargeBucket, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT type, day_cst, sum(amount_cny) AS amount_cny
		FROM recharge_records
		WHERE occurred_at BETWEEN $1 AND $2
			AND type IN ('code_redeem','code_redeem_days','admin_balance','admin_gift')
		GROUP BY type, day_cst
		ORDER BY day_cst ASC, type ASC
	`, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query recharge board: %w", err)
	}
	defer rows.Close()

	var out []RechargeBucket
	for rows.Next() {
		var b RechargeBucket
		var day pgtype.Date
		var amount pgtype.Numeric
		if err := rows.Scan(&b.Type, &day, &amount); err != nil {
			return nil, fmt.Errorf("scan recharge bucket: %w", err)
		}
		b.DayCST = timeFromDate(day)
		b.AmountCNY, err = decimalFromNumeric(amount)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recharge buckets: %w", err)
	}
	return out, nil
}

func CallBoardQuery(ctx context.Context, from, to time.Time, channelID string) ([]CallBucket, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT
			coalesce(nullif(channel_id,''),'unknown') AS channel_id,
			coalesce(nullif(price_model,''), nullif(original_model,''), nullif(actual_model,''),'unknown') AS model,
			day_cst,
			count(*) AS requests,
			sum(CASE WHEN status='error' THEN 1 ELSE 0 END) AS errors,
			sum(input_tokens) AS tokens_in,
			sum(output_tokens) AS tokens_out,
			sum(total_tokens) AS tokens,
			sum(charged_usd) AS charged_usd,
			sum(cost_usd) AS paid_revenue_usd
		FROM call_logs
		WHERE occurred_at BETWEEN $1 AND $2
			AND ($3 = '' OR channel_id = $3)
		GROUP BY
			coalesce(nullif(channel_id,''),'unknown'),
			coalesce(nullif(price_model,''), nullif(original_model,''), nullif(actual_model,''),'unknown'),
			day_cst
		ORDER BY day_cst ASC, channel_id ASC, model ASC
	`, from.UTC(), to.UTC(), channelID)
	if err != nil {
		return nil, fmt.Errorf("query call board: %w", err)
	}
	defer rows.Close()

	var out []CallBucket
	for rows.Next() {
		var b CallBucket
		var day pgtype.Date
		var charged, revenue pgtype.Numeric
		if err := rows.Scan(
			&b.ChannelID, &b.Model, &day, &b.Requests, &b.Errors,
			&b.TokensIn, &b.TokensOut, &b.Tokens, &charged, &revenue,
		); err != nil {
			return nil, fmt.Errorf("scan call bucket: %w", err)
		}
		b.DayCST = timeFromDate(day)
		if b.ChargedUSD, err = decimalFromNumeric(charged); err != nil {
			return nil, err
		}
		if b.PaidRevenueUSD, err = decimalFromNumeric(revenue); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate call buckets: %w", err)
	}
	return out, nil
}
