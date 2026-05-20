package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var ErrNotFound = errors.New("db: not found")

func requirePool() (*pgxpool.Pool, error) {
	p := Pool()
	if p == nil {
		return nil, errors.New("postgres pool is not initialized")
	}
	return p, nil
}

func pingPool(ctx context.Context) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	return p.Ping(ctx)
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func numericFromDecimal(d decimal.Decimal) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   new(big.Int).Set(d.Coefficient()),
		Exp:   d.Exponent(),
		Valid: true,
	}
}

func decimalFromNumeric(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	if n.NaN || n.InfinityModifier != 0 {
		return decimal.Zero, fmt.Errorf("unsupported numeric value")
	}
	if n.Int == nil {
		return decimal.Zero, nil
	}
	return decimal.NewFromBigInt(new(big.Int).Set(n.Int), n.Exp), nil
}

func textFromString(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func stringFromText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func timestamptzFromPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil || t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func ptrFromTimestamptz(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func jsonParam(v any) (string, error) {
	if v == nil {
		return "{}", nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal jsonb: %w", err)
	}
	return string(data), nil
}

func jsonObjectParam(v any) (string, error) {
	raw, err := jsonParam(v)
	if err != nil {
		return "", err
	}
	if raw == "null" {
		return "{}", nil
	}
	return raw, nil
}

func scanJSONMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal jsonb: %w", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func scanJSONInt64Map(raw []byte) (map[string]int64, error) {
	if len(raw) == 0 {
		return map[string]int64{}, nil
	}
	var out map[string]int64
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal int64 jsonb: %w", err)
	}
	if out == nil {
		out = map[string]int64{}
	}
	return out, nil
}
