package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type ApiKey struct {
	ID                 string
	KeyHash            []byte
	KeyCiphertext      string
	Tier               string
	Plan               string
	ExpiresAt          *time.Time
	Enabled            bool
	Balance            decimal.Decimal
	GiftBalance        decimal.Decimal
	TotalRecharged     decimal.Decimal
	TotalGifted        decimal.Decimal
	Note               string
	CreatedAt          time.Time
	LastUsed           *time.Time
	Requests           int64
	Errors             int64
	Tokens             int64
	Credits            decimal.Decimal
	Models             map[string]int64
	ParentKeyID        string
	IsReseller         bool
	MaxChildKeys       int
	ResellerDiscount   decimal.Decimal
	SoldToChildren     decimal.Decimal
	RateLimitPerMin    int
	SeriesPreferences  map[string]any
	ChannelPreferences map[string]any
	DebtUSD            decimal.Decimal
	DeletedAt          *time.Time
	Metadata           map[string]any
}

func InsertApiKey(ctx context.Context, tx pgx.Tx, k ApiKey) error {
	if tx == nil {
		return errors.New("insert api key requires transaction")
	}
	if k.Plan == "" {
		k.Plan = "credit"
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now().UTC()
	}
	models, seriesPrefs, channelPrefs, meta, err := apiKeyJSONParams(k)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_keys (
			id, key_hash, key_ciphertext, tier, plan, expires_at, enabled,
			balance, gift_balance, total_recharged, total_gifted, note,
			created_at, last_used, requests, errors, tokens, credits, models,
			parent_key_id, is_reseller, max_child_keys, reseller_discount,
			sold_to_children, rate_limit_per_min, series_preferences,
			channel_preferences, debt_usd, deleted_at, metadata
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,
			$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30
		)
	`, k.ID, k.KeyHash, k.KeyCiphertext, textFromString(k.Tier), k.Plan,
		timestamptzFromPtr(k.ExpiresAt), k.Enabled, numericFromDecimal(k.Balance),
		numericFromDecimal(k.GiftBalance), numericFromDecimal(k.TotalRecharged),
		numericFromDecimal(k.TotalGifted), textFromString(k.Note), k.CreatedAt.UTC(),
		timestamptzFromPtr(k.LastUsed), k.Requests, k.Errors, k.Tokens,
		numericFromDecimal(k.Credits), models, textFromString(k.ParentKeyID),
		k.IsReseller, k.MaxChildKeys, numericFromDecimal(k.ResellerDiscount),
		numericFromDecimal(k.SoldToChildren), k.RateLimitPerMin, seriesPrefs,
		channelPrefs, numericFromDecimal(k.DebtUSD), timestamptzFromPtr(k.DeletedAt), meta)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	return nil
}

func GetApiKey(ctx context.Context, id string) (ApiKey, error) {
	p, err := requirePool()
	if err != nil {
		return ApiKey{}, err
	}
	k, err := scanApiKey(p.QueryRow(ctx, apiKeySelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ApiKey{}, ErrNotFound
	}
	return k, err
}

func GetApiKeyByHash(ctx context.Context, hash []byte) (ApiKey, error) {
	p, err := requirePool()
	if err != nil {
		return ApiKey{}, err
	}
	k, err := scanApiKey(p.QueryRow(ctx, apiKeySelectSQL(`key_hash=$1 AND deleted_at IS NULL`), hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return ApiKey{}, ErrNotFound
	}
	return k, err
}

func ListApiKeys(ctx context.Context, includeDeleted bool) ([]ApiKey, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := `deleted_at IS NULL`
	if includeDeleted {
		where = `TRUE`
	}
	rows, err := p.Query(ctx, apiKeySelectSQL(where)+` ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	return scanApiKeys(rows)
}

func ListChildKeys(ctx context.Context, parentID string) ([]ApiKey, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, apiKeySelectSQL(`parent_key_id=$1 AND deleted_at IS NULL`)+` ORDER BY created_at ASC`, parentID)
	if err != nil {
		return nil, fmt.Errorf("list child keys: %w", err)
	}
	defer rows.Close()
	return scanApiKeys(rows)
}

func UpdateApiKey(ctx context.Context, k ApiKey) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	models, seriesPrefs, channelPrefs, meta, err := apiKeyJSONParams(k)
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE api_keys
		SET key_hash=$2, key_ciphertext=$3, tier=$4, plan=$5, expires_at=$6,
			enabled=$7, balance=$8, gift_balance=$9, total_recharged=$10,
			total_gifted=$11, note=$12, created_at=$13, last_used=$14,
			requests=$15, errors=$16, tokens=$17, credits=$18, models=$19,
			parent_key_id=$20, is_reseller=$21, max_child_keys=$22,
			reseller_discount=$23, sold_to_children=$24, rate_limit_per_min=$25,
			series_preferences=$26, channel_preferences=$27, debt_usd=$28,
			deleted_at=$29, metadata=$30
		WHERE id=$1
	`, k.ID, k.KeyHash, k.KeyCiphertext, textFromString(k.Tier), k.Plan,
		timestamptzFromPtr(k.ExpiresAt), k.Enabled, numericFromDecimal(k.Balance),
		numericFromDecimal(k.GiftBalance), numericFromDecimal(k.TotalRecharged),
		numericFromDecimal(k.TotalGifted), textFromString(k.Note), k.CreatedAt.UTC(),
		timestamptzFromPtr(k.LastUsed), k.Requests, k.Errors, k.Tokens,
		numericFromDecimal(k.Credits), models, textFromString(k.ParentKeyID),
		k.IsReseller, k.MaxChildKeys, numericFromDecimal(k.ResellerDiscount),
		numericFromDecimal(k.SoldToChildren), k.RateLimitPerMin, seriesPrefs,
		channelPrefs, numericFromDecimal(k.DebtUSD), timestamptzFromPtr(k.DeletedAt), meta)
	if err != nil {
		return fmt.Errorf("update api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SetApiKeyEnabled(ctx context.Context, id string, enabled bool) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE api_keys SET enabled=$2 WHERE id=$1`, id, enabled)
	if err != nil {
		return fmt.Errorf("set api key enabled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SetApiKeyExpiry(ctx context.Context, id string, expiresAt *time.Time) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE api_keys SET expires_at=$2 WHERE id=$1`, id, timestamptzFromPtr(expiresAt))
	if err != nil {
		return fmt.Errorf("set api key expiry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SoftDeleteApiKey(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE api_keys SET deleted_at=now(), enabled=false WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func IncrementApiKeyUsage(ctx context.Context, id string, requests, keyErrors, tokens int64, credits decimal.Decimal, models map[string]int64) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin api key usage: %w", err)
	}
	defer tx.Rollback(ctx)

	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT models FROM api_keys WHERE id=$1 FOR UPDATE`, id).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock api key usage: %w", err)
	}
	merged, err := scanJSONInt64Map(raw)
	if err != nil {
		return err
	}
	for model, count := range models {
		merged[model] += count
	}
	modelsJSON, err := jsonObjectParam(merged)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE api_keys
		SET requests=requests+$2, errors=errors+$3, tokens=tokens+$4,
			credits=credits+$5, models=$6, last_used=now()
		WHERE id=$1
	`, id, requests, keyErrors, tokens, numericFromDecimal(credits), modelsJSON); err != nil {
		return fmt.Errorf("increment api key usage: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit api key usage: %w", err)
	}
	return nil
}

func UpdateApiKeySoldToChildren(ctx context.Context, tx pgx.Tx, id string, delta decimal.Decimal) error {
	if tx == nil {
		return errors.New("update sold_to_children requires transaction")
	}
	tag, err := tx.Exec(ctx,
		`UPDATE api_keys SET sold_to_children=sold_to_children+$2 WHERE id=$1`,
		id, numericFromDecimal(delta),
	)
	if err != nil {
		return fmt.Errorf("update sold_to_children: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func apiKeyJSONParams(k ApiKey) (models, seriesPrefs, channelPrefs, meta string, err error) {
	if models, err = jsonObjectParam(k.Models); err != nil {
		return
	}
	if seriesPrefs, err = jsonObjectParam(k.SeriesPreferences); err != nil {
		return
	}
	if channelPrefs, err = jsonObjectParam(k.ChannelPreferences); err != nil {
		return
	}
	meta, err = jsonObjectParam(k.Metadata)
	return
}

func apiKeySelectSQL(where string) string {
	return `
		SELECT
			id, key_hash, key_ciphertext, tier, plan, expires_at, enabled,
			balance, gift_balance, total_recharged, total_gifted, note,
			created_at, last_used, requests, errors, tokens, credits, models,
			parent_key_id, is_reseller, max_child_keys, reseller_discount,
			sold_to_children, rate_limit_per_min, series_preferences,
			channel_preferences, debt_usd, deleted_at, metadata
		FROM api_keys
		WHERE ` + where
}

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func scanApiKeys(rows pgx.Rows) ([]ApiKey, error) {
	var out []ApiKey
	for rows.Next() {
		k, err := scanApiKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return out, nil
}

func scanApiKey(row apiKeyScanner) (ApiKey, error) {
	var k ApiKey
	var tier, note, parentKeyID pgtype.Text
	var expiresAt, lastUsed, deletedAt pgtype.Timestamptz
	var balance, gift, totalRecharged, totalGifted pgtype.Numeric
	var credits, resellerDiscount, soldToChildren, debt pgtype.Numeric
	var modelsRaw, seriesRaw, channelRaw, metaRaw []byte
	if err := row.Scan(
		&k.ID, &k.KeyHash, &k.KeyCiphertext, &tier, &k.Plan, &expiresAt, &k.Enabled,
		&balance, &gift, &totalRecharged, &totalGifted, &note,
		&k.CreatedAt, &lastUsed, &k.Requests, &k.Errors, &k.Tokens, &credits, &modelsRaw,
		&parentKeyID, &k.IsReseller, &k.MaxChildKeys, &resellerDiscount,
		&soldToChildren, &k.RateLimitPerMin, &seriesRaw, &channelRaw,
		&debt, &deletedAt, &metaRaw,
	); err != nil {
		return ApiKey{}, fmt.Errorf("scan api key: %w", err)
	}
	var err error
	k.Tier = stringFromText(tier)
	k.Note = stringFromText(note)
	k.ParentKeyID = stringFromText(parentKeyID)
	k.ExpiresAt = ptrFromTimestamptz(expiresAt)
	k.LastUsed = ptrFromTimestamptz(lastUsed)
	k.DeletedAt = ptrFromTimestamptz(deletedAt)
	if k.Balance, err = decimalFromNumeric(balance); err != nil {
		return ApiKey{}, err
	}
	if k.GiftBalance, err = decimalFromNumeric(gift); err != nil {
		return ApiKey{}, err
	}
	if k.TotalRecharged, err = decimalFromNumeric(totalRecharged); err != nil {
		return ApiKey{}, err
	}
	if k.TotalGifted, err = decimalFromNumeric(totalGifted); err != nil {
		return ApiKey{}, err
	}
	if k.Credits, err = decimalFromNumeric(credits); err != nil {
		return ApiKey{}, err
	}
	if k.ResellerDiscount, err = decimalFromNumeric(resellerDiscount); err != nil {
		return ApiKey{}, err
	}
	if k.SoldToChildren, err = decimalFromNumeric(soldToChildren); err != nil {
		return ApiKey{}, err
	}
	if k.DebtUSD, err = decimalFromNumeric(debt); err != nil {
		return ApiKey{}, err
	}
	if k.Models, err = scanJSONInt64Map(modelsRaw); err != nil {
		return ApiKey{}, err
	}
	if k.SeriesPreferences, err = scanJSONMap(seriesRaw); err != nil {
		return ApiKey{}, err
	}
	if k.ChannelPreferences, err = scanJSONMap(channelRaw); err != nil {
		return ApiKey{}, err
	}
	if k.Metadata, err = scanJSONMap(metaRaw); err != nil {
		return ApiKey{}, err
	}
	return k, nil
}
