package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type WalletLedgerEntry struct {
	ID            string
	OccurredAt    time.Time
	APIKeyID      string
	OwnerType     string
	OwnerID       string
	Operation     string
	ReservationID string
	RequestID     string
	PaidDelta     decimal.Decimal
	GiftDelta     decimal.Decimal
	PaidAfter     decimal.Decimal
	GiftAfter     decimal.Decimal
	Metadata      map[string]any
}

type WalletLedgerFilter struct {
	APIKeyID      string
	OwnerType     string
	OwnerID       string
	Operation     string
	ReservationID string
	RequestID     string
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
	Limit         int
	Offset        int
}

type WalletLedgerAggregate struct {
	Operation string
	PaidSum   decimal.Decimal
	GiftSum   decimal.Decimal
	Count     int64
}

func ListWalletLedger(ctx context.Context, f WalletLedgerFilter) ([]WalletLedgerEntry, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := []string{"TRUE"}
	args := []any{}
	if s := strings.TrimSpace(f.APIKeyID); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("api_key_id=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.OwnerType); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("owner_type=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.OwnerID); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("owner_id=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.Operation); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("operation=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.ReservationID); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("reservation_id=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.RequestID); s != "" {
		args = append(args, s)
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
	args = append(args, normalizeListLimit(f.Limit))
	limitParam := len(args)
	args = append(args, normalizeListOffset(f.Offset))
	offsetParam := len(args)

	rows, err := p.Query(ctx, `
		SELECT id, occurred_at, api_key_id, owner_type, owner_id, operation,
			reservation_id, request_id, paid_delta, gift_delta, paid_after, gift_after, metadata
		FROM wallet_ledger
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY occurred_at DESC, id DESC
		LIMIT $`+fmt.Sprint(limitParam)+` OFFSET $`+fmt.Sprint(offsetParam), args...)
	if err != nil {
		return nil, fmt.Errorf("list wallet ledger: %w", err)
	}
	defer rows.Close()
	var out []WalletLedgerEntry
	for rows.Next() {
		entry, err := scanWalletLedger(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wallet ledger: %w", err)
	}
	return out, nil
}

// AggregateWalletLedgerByOperation 返回指定 owner 在时间窗口里按 operation 分桶的总流水。
// 用于钱包账户的对账与日志总览。
func AggregateWalletLedgerByOperation(ctx context.Context, ownerType, ownerID string, from, to time.Time) ([]WalletLedgerAggregate, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT operation, COALESCE(sum(paid_delta), 0), COALESCE(sum(gift_delta), 0), count(*)
		FROM wallet_ledger
		WHERE owner_type=$1 AND owner_id=$2 AND occurred_at BETWEEN $3 AND $4
		GROUP BY operation
		ORDER BY operation ASC
	`, ownerType, ownerID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("aggregate wallet ledger: %w", err)
	}
	defer rows.Close()
	var out []WalletLedgerAggregate
	for rows.Next() {
		var agg WalletLedgerAggregate
		var paid, gift pgtype.Numeric
		if err := rows.Scan(&agg.Operation, &paid, &gift, &agg.Count); err != nil {
			return nil, fmt.Errorf("scan wallet ledger aggregate: %w", err)
		}
		if agg.PaidSum, err = decimalFromNumeric(paid); err != nil {
			return nil, err
		}
		if agg.GiftSum, err = decimalFromNumeric(gift); err != nil {
			return nil, err
		}
		out = append(out, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wallet ledger aggregate: %w", err)
	}
	return out, nil
}

type walletLedgerScanner interface {
	Scan(dest ...any) error
}

func scanWalletLedger(scanner walletLedgerScanner) (WalletLedgerEntry, error) {
	var entry WalletLedgerEntry
	var reservationID, requestID pgtype.Text
	var paidDelta, giftDelta, paidAfter, giftAfter pgtype.Numeric
	var meta []byte
	if err := scanner.Scan(
		&entry.ID, &entry.OccurredAt, &entry.APIKeyID, &entry.OwnerType, &entry.OwnerID,
		&entry.Operation, &reservationID, &requestID,
		&paidDelta, &giftDelta, &paidAfter, &giftAfter, &meta,
	); err != nil {
		return WalletLedgerEntry{}, fmt.Errorf("scan wallet ledger entry: %w", err)
	}
	entry.ReservationID = stringFromText(reservationID)
	entry.RequestID = stringFromText(requestID)
	var err error
	if entry.PaidDelta, err = decimalFromNumeric(paidDelta); err != nil {
		return WalletLedgerEntry{}, err
	}
	if entry.GiftDelta, err = decimalFromNumeric(giftDelta); err != nil {
		return WalletLedgerEntry{}, err
	}
	if entry.PaidAfter, err = decimalFromNumeric(paidAfter); err != nil {
		return WalletLedgerEntry{}, err
	}
	if entry.GiftAfter, err = decimalFromNumeric(giftAfter); err != nil {
		return WalletLedgerEntry{}, err
	}
	entry.Metadata, err = scanJSONMap(meta)
	return entry, err
}
