package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

var cstZone = time.FixedZone("CST", 8*3600)

type RechargeRecordRow struct {
	ID            string
	TimeLabel     string
	TimestampUnix int64
	OccurredAt    time.Time
	DayCST        time.Time
	APIKeyID      string
	UserID        string
	KeyNote       string
	Type          string
	Code          string
	AmountUSD     decimal.Decimal
	AmountCNY     decimal.Decimal
	BalanceBefore decimal.Decimal
	BalanceAfter  decimal.Decimal
	GiftBefore    decimal.Decimal
	GiftAfter     decimal.Decimal
	Operator      string
	Note          string
	IP            string
	RawPayload    map[string]any
}

type RechargeFilter struct {
	UserID       string
	APIKeyID     string
	Type         string
	DayCSTFrom   *time.Time
	DayCSTTo     *time.Time
	OccurredFrom *time.Time
	OccurredTo   *time.Time
	Limit        int
	Offset       int
}

func ComputeDayCST(t time.Time) time.Time {
	local := t.In(cstZone)
	y, m, d := local.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, cstZone)
}

func InsertRecharge(ctx context.Context, row RechargeRecordRow) (bool, error) {
	p, err := requirePool()
	if err != nil {
		return false, err
	}
	raw, err := jsonObjectParam(row.RawPayload)
	if err != nil {
		return false, err
	}
	tag, err := p.Exec(ctx, `
		INSERT INTO recharge_records (
			id, time_label, timestamp_unix, occurred_at, day_cst,
			api_key_id, user_id, key_note, type, code, amount_usd,
			amount_cny, balance_before, balance_after, gift_before,
			gift_after, operator, note, ip, raw_payload
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
		)
		ON CONFLICT (id) DO NOTHING
	`, row.ID, row.TimeLabel, row.TimestampUnix, row.OccurredAt.UTC(),
		dateFromTime(row.DayCST), row.APIKeyID, textFromString(row.UserID),
		textFromString(row.KeyNote), row.Type, textFromString(row.Code),
		numericFromDecimal(row.AmountUSD), numericFromDecimal(row.AmountCNY),
		numericFromDecimal(row.BalanceBefore), numericFromDecimal(row.BalanceAfter),
		numericFromDecimal(row.GiftBefore), numericFromDecimal(row.GiftAfter),
		row.Operator, textFromString(row.Note), textFromString(row.IP), raw)
	if err != nil {
		return false, fmt.Errorf("insert recharge record: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func ListRecharges(ctx context.Context, f RechargeFilter) ([]RechargeRecordRow, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := []string{"TRUE"}
	args := []any{}
	if strings.TrimSpace(f.UserID) != "" {
		args = append(args, strings.TrimSpace(f.UserID))
		where = append(where, fmt.Sprintf("user_id=$%d", len(args)))
	}
	if strings.TrimSpace(f.APIKeyID) != "" {
		args = append(args, strings.TrimSpace(f.APIKeyID))
		where = append(where, fmt.Sprintf("api_key_id=$%d", len(args)))
	}
	if strings.TrimSpace(f.Type) != "" {
		args = append(args, strings.TrimSpace(f.Type))
		where = append(where, fmt.Sprintf("type=$%d", len(args)))
	}
	if f.DayCSTFrom != nil {
		args = append(args, dateFromTime(*f.DayCSTFrom))
		where = append(where, fmt.Sprintf("day_cst >= $%d", len(args)))
	}
	if f.DayCSTTo != nil {
		args = append(args, dateFromTime(*f.DayCSTTo))
		where = append(where, fmt.Sprintf("day_cst <= $%d", len(args)))
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

	rows, err := p.Query(ctx, rechargeSelectSQL(strings.Join(where, " AND "))+`
		ORDER BY occurred_at DESC, id DESC
		LIMIT $`+fmt.Sprint(limitParam)+` OFFSET $`+fmt.Sprint(offsetParam), args...)
	if err != nil {
		return nil, fmt.Errorf("list recharge records: %w", err)
	}
	defer rows.Close()

	var out []RechargeRecordRow
	for rows.Next() {
		row, err := scanRechargeRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recharge records: %w", err)
	}
	return out, nil
}

func normalizeListLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeListOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func dateFromTime(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	local := t.In(cstZone)
	y, m, d := local.Date()
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

func timeFromDate(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	y, m, day := d.Time.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, cstZone)
}

func rechargeSelectSQL(where string) string {
	return `
		SELECT id, time_label, timestamp_unix, occurred_at, day_cst,
			api_key_id, user_id, key_note, type, code, amount_usd,
			amount_cny, balance_before, balance_after, gift_before,
			gift_after, operator, note, ip, raw_payload
		FROM recharge_records
		WHERE ` + where
}

type rechargeRecordScanner interface {
	Scan(dest ...any) error
}

func scanRechargeRecord(scanner rechargeRecordScanner) (RechargeRecordRow, error) {
	var row RechargeRecordRow
	var day pgtype.Date
	var userID, keyNote, code, note, ip pgtype.Text
	var amountUSD, amountCNY, balanceBefore, balanceAfter pgtype.Numeric
	var giftBefore, giftAfter pgtype.Numeric
	var raw []byte
	if err := scanner.Scan(
		&row.ID, &row.TimeLabel, &row.TimestampUnix, &row.OccurredAt, &day,
		&row.APIKeyID, &userID, &keyNote, &row.Type, &code, &amountUSD,
		&amountCNY, &balanceBefore, &balanceAfter, &giftBefore, &giftAfter,
		&row.Operator, &note, &ip, &raw,
	); err != nil {
		return RechargeRecordRow{}, fmt.Errorf("scan recharge record: %w", err)
	}
	var err error
	row.DayCST = timeFromDate(day)
	row.UserID = stringFromText(userID)
	row.KeyNote = stringFromText(keyNote)
	row.Code = stringFromText(code)
	row.Note = stringFromText(note)
	row.IP = stringFromText(ip)
	if row.AmountUSD, err = decimalFromNumeric(amountUSD); err != nil {
		return RechargeRecordRow{}, err
	}
	if row.AmountCNY, err = decimalFromNumeric(amountCNY); err != nil {
		return RechargeRecordRow{}, err
	}
	if row.BalanceBefore, err = decimalFromNumeric(balanceBefore); err != nil {
		return RechargeRecordRow{}, err
	}
	if row.BalanceAfter, err = decimalFromNumeric(balanceAfter); err != nil {
		return RechargeRecordRow{}, err
	}
	if row.GiftBefore, err = decimalFromNumeric(giftBefore); err != nil {
		return RechargeRecordRow{}, err
	}
	if row.GiftAfter, err = decimalFromNumeric(giftAfter); err != nil {
		return RechargeRecordRow{}, err
	}
	row.RawPayload, err = scanJSONMap(raw)
	return row, err
}
