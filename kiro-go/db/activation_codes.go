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

var ErrCodeAlreadyUsed = errors.New("db: activation code already used")

type ActivationCode struct {
	Code            string
	Type            string
	Amount          decimal.Decimal
	Tier            string
	CodeExpiresAt   *time.Time
	Used            bool
	UsedByKeyID     string
	UsedAt          *time.Time
	CreatedAt       time.Time
	Note            string
	RateLimitPerMin int
	SalePriceCNY    decimal.Decimal
}

type ActivationCodeFilter struct {
	IncludeUsed    bool
	IncludeExpired bool
	Type           string
	Limit          int
}

func InsertActivationCode(ctx context.Context, tx pgx.Tx, c ActivationCode) error {
	if tx == nil {
		return errors.New("insert activation code requires transaction")
	}
	prepareActivationCode(&c)
	_, err := tx.Exec(ctx, `
		INSERT INTO activation_codes (
			code, type, amount, tier, code_expires_at, used, used_by_key_id,
			used_at, created_at, note, rate_limit_per_min, sale_price_cny
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, c.Code, c.Type, numericFromDecimal(c.Amount), textFromString(c.Tier),
		timestamptzFromPtr(c.CodeExpiresAt), c.Used, textFromString(c.UsedByKeyID),
		timestamptzFromPtr(c.UsedAt), c.CreatedAt.UTC(), textFromString(c.Note),
		c.RateLimitPerMin, numericFromDecimal(c.SalePriceCNY))
	if err != nil {
		return fmt.Errorf("insert activation code: %w", err)
	}
	return nil
}

func GetActivationCode(ctx context.Context, code string) (ActivationCode, error) {
	p, err := requirePool()
	if err != nil {
		return ActivationCode{}, err
	}
	c, err := scanActivationCode(p.QueryRow(ctx, activationCodeSelectSQL(`code=$1`), strings.TrimSpace(code)))
	if errors.Is(err, pgx.ErrNoRows) {
		return ActivationCode{}, ErrNotFound
	}
	return c, err
}

func ListActivationCodes(ctx context.Context, filter ActivationCodeFilter) ([]ActivationCode, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := []string{"TRUE"}
	args := []any{}
	if !filter.IncludeUsed {
		where = append(where, "used=false")
	}
	if !filter.IncludeExpired {
		where = append(where, "(code_expires_at IS NULL OR code_expires_at > now())")
	}
	if strings.TrimSpace(filter.Type) != "" {
		args = append(args, strings.TrimSpace(filter.Type))
		where = append(where, fmt.Sprintf("type=$%d", len(args)))
	}
	query := activationCodeSelectSQL(strings.Join(where, " AND ")) + ` ORDER BY created_at DESC`
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	rows, err := p.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list activation codes: %w", err)
	}
	defer rows.Close()

	var out []ActivationCode
	for rows.Next() {
		c, err := scanActivationCode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activation codes: %w", err)
	}
	return out, nil
}

func LockActivationCodeForRedeem(ctx context.Context, tx pgx.Tx, code string) (ActivationCode, error) {
	if tx == nil {
		return ActivationCode{}, errors.New("lock activation code requires transaction")
	}
	c, err := scanActivationCode(tx.QueryRow(ctx, activationCodeSelectSQL(`code=$1`)+` FOR UPDATE`, strings.TrimSpace(code)))
	if errors.Is(err, pgx.ErrNoRows) {
		return ActivationCode{}, ErrNotFound
	}
	if err != nil {
		return ActivationCode{}, err
	}
	if c.Used {
		return ActivationCode{}, ErrCodeAlreadyUsed
	}
	return c, nil
}

func MarkActivationCodeUsed(ctx context.Context, tx pgx.Tx, code, byKeyID string, at time.Time) error {
	if tx == nil {
		return errors.New("mark activation code used requires transaction")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tag, err := tx.Exec(ctx, `
		UPDATE activation_codes
		SET used=true, used_by_key_id=$2, used_at=$3
		WHERE code=$1 AND used=false
	`, strings.TrimSpace(code), textFromString(byKeyID), at.UTC())
	if err != nil {
		return fmt.Errorf("mark activation code used: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var used bool
	if err := tx.QueryRow(ctx, `SELECT used FROM activation_codes WHERE code=$1`, strings.TrimSpace(code)).Scan(&used); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("check activation code used: %w", err)
	}
	if used {
		return ErrCodeAlreadyUsed
	}
	return ErrNotFound
}

func DeleteActivationCode(ctx context.Context, code string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `DELETE FROM activation_codes WHERE code=$1`, strings.TrimSpace(code))
	if err != nil {
		return fmt.Errorf("delete activation code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func prepareActivationCode(c *ActivationCode) {
	c.Code = strings.TrimSpace(c.Code)
	c.Type = strings.TrimSpace(c.Type)
	c.Tier = strings.TrimSpace(c.Tier)
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
}

func activationCodeSelectSQL(where string) string {
	return `
		SELECT code, type, amount, tier, code_expires_at, used,
			used_by_key_id, used_at, created_at, note, rate_limit_per_min,
			sale_price_cny
		FROM activation_codes
		WHERE ` + where
}

type activationCodeScanner interface {
	Scan(dest ...any) error
}

func scanActivationCode(row activationCodeScanner) (ActivationCode, error) {
	var c ActivationCode
	var tier, usedBy, note pgtype.Text
	var expiresAt, usedAt pgtype.Timestamptz
	var amount, salePrice pgtype.Numeric
	if err := row.Scan(
		&c.Code, &c.Type, &amount, &tier, &expiresAt, &c.Used,
		&usedBy, &usedAt, &c.CreatedAt, &note, &c.RateLimitPerMin,
		&salePrice,
	); err != nil {
		return ActivationCode{}, fmt.Errorf("scan activation code: %w", err)
	}
	var err error
	c.Tier = stringFromText(tier)
	c.CodeExpiresAt = ptrFromTimestamptz(expiresAt)
	c.UsedByKeyID = stringFromText(usedBy)
	c.UsedAt = ptrFromTimestamptz(usedAt)
	c.Note = stringFromText(note)
	if c.Amount, err = decimalFromNumeric(amount); err != nil {
		return ActivationCode{}, err
	}
	c.SalePriceCNY, err = decimalFromNumeric(salePrice)
	return c, err
}
