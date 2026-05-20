package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type BillingReservation struct {
	ID            string
	RequestID     string
	APIKeyID      string
	OwnerType     string
	OwnerID       string
	ChannelID     string
	Model         string
	Status        string
	Action        string
	EstCostUSD    decimal.Decimal
	PrePaidUSD    decimal.Decimal
	PreGiftUSD    decimal.Decimal
	ActualCostUSD *decimal.Decimal
	PriceSnapshot map[string]any
	CreatedAt     time.Time
	SettledAt     *time.Time
}

type BillingReservationFilter struct {
	Status      string
	OwnerType   string
	OwnerID     string
	APIKeyID    string
	ChannelID   string
	RequestID   string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Limit       int
	Offset      int
}

func GetBillingReservation(ctx context.Context, id string) (BillingReservation, error) {
	p, err := requirePool()
	if err != nil {
		return BillingReservation{}, err
	}
	row, err := scanBillingReservation(p.QueryRow(ctx, billingReservationSelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return BillingReservation{}, ErrNotFound
	}
	return row, err
}

func ListBillingReservations(ctx context.Context, f BillingReservationFilter) ([]BillingReservation, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := []string{"TRUE"}
	args := []any{}
	if s := strings.TrimSpace(f.Status); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("status=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.OwnerType); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("owner_type=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.OwnerID); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("owner_id=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.APIKeyID); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("api_key_id=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.ChannelID); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("channel_id=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.RequestID); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("request_id=$%d", len(args)))
	}
	if f.CreatedFrom != nil {
		args = append(args, f.CreatedFrom.UTC())
		where = append(where, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if f.CreatedTo != nil {
		args = append(args, f.CreatedTo.UTC())
		where = append(where, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	args = append(args, normalizeListLimit(f.Limit))
	limitParam := len(args)
	args = append(args, normalizeListOffset(f.Offset))
	offsetParam := len(args)

	rows, err := p.Query(ctx, billingReservationSelectSQL(strings.Join(where, " AND "))+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+fmt.Sprint(limitParam)+` OFFSET $`+fmt.Sprint(offsetParam), args...)
	if err != nil {
		return nil, fmt.Errorf("list billing reservations: %w", err)
	}
	defer rows.Close()
	var out []BillingReservation
	for rows.Next() {
		r, err := scanBillingReservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate billing reservations: %w", err)
	}
	return out, nil
}

// ExpireStaleReservations 把超过 before 时刻仍为 pending 的预扣标记为 expired，
// 返回受影响行数。这是定时任务调用的入口，调用方需要自行触发 refund/对账。
func ExpireStaleReservations(ctx context.Context, before time.Time) (int64, error) {
	p, err := requirePool()
	if err != nil {
		return 0, err
	}
	tag, err := p.Exec(ctx, `
		UPDATE billing_reservations
		SET status='expired', settled_at=now()
		WHERE status='pending' AND created_at < $1
	`, before.UTC())
	if err != nil {
		return 0, fmt.Errorf("expire stale reservations: %w", err)
	}
	return tag.RowsAffected(), nil
}

func billingReservationSelectSQL(where string) string {
	return `
		SELECT id, request_id, api_key_id, owner_type, owner_id,
			channel_id, model, status, action,
			est_cost_usd, pre_paid_usd, pre_gift_usd, actual_cost_usd,
			price_snapshot, created_at, settled_at
		FROM billing_reservations
		WHERE ` + where
}

type billingReservationScanner interface {
	Scan(dest ...any) error
}

func scanBillingReservation(scanner billingReservationScanner) (BillingReservation, error) {
	var r BillingReservation
	var requestID, channelID, model pgtype.Text
	var estCost, prePaid, preGift, actualCost pgtype.Numeric
	var settledAt pgtype.Timestamptz
	var snap []byte
	if err := scanner.Scan(
		&r.ID, &requestID, &r.APIKeyID, &r.OwnerType, &r.OwnerID,
		&channelID, &model, &r.Status, &r.Action,
		&estCost, &prePaid, &preGift, &actualCost,
		&snap, &r.CreatedAt, &settledAt,
	); err != nil {
		return BillingReservation{}, fmt.Errorf("scan billing reservation: %w", err)
	}
	var err error
	r.RequestID = stringFromText(requestID)
	r.ChannelID = stringFromText(channelID)
	r.Model = stringFromText(model)
	r.SettledAt = ptrFromTimestamptz(settledAt)
	if r.EstCostUSD, err = decimalFromNumeric(estCost); err != nil {
		return BillingReservation{}, err
	}
	if r.PrePaidUSD, err = decimalFromNumeric(prePaid); err != nil {
		return BillingReservation{}, err
	}
	if r.PreGiftUSD, err = decimalFromNumeric(preGift); err != nil {
		return BillingReservation{}, err
	}
	if actualCost.Valid {
		v, err := decimalFromNumeric(actualCost)
		if err != nil {
			return BillingReservation{}, err
		}
		r.ActualCostUSD = &v
	}
	r.PriceSnapshot, err = scanJSONMap(snap)
	return r, err
}
