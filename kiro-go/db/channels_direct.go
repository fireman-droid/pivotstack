package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DirectSellPriceRow struct {
	InputPerM      float64 `json:"inputPerM,omitempty"`
	OutputPerM     float64 `json:"outputPerM,omitempty"`
	CostInputPerM  float64 `json:"costInputPerM,omitempty"`
	CostOutputPerM float64 `json:"costOutputPerM,omitempty"`
}

type DirectSellPrice struct {
	Default DirectSellPriceRow            `json:"default,omitempty"`
	Models  map[string]DirectSellPriceRow `json:"models,omitempty"`
}

type DirectChannel struct {
	ID           string
	Type         string
	Alias        string
	AliasNorm    string
	BaseURL      string
	APIKeyEnc    string
	Models       []string
	SellPrice    DirectSellPrice
	ModelMapping map[string]string
	ExtraHeaders map[string]string
	Enabled      bool
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

func InsertDirectChannel(ctx context.Context, tx pgx.Tx, ch DirectChannel) error {
	if tx == nil {
		return errors.New("insert direct channel requires transaction")
	}
	prepareDirectChannel(&ch)
	if err := ensureChannelAliasAvailable(ctx, tx, "direct", ch.ID, ch.AliasNorm); err != nil {
		return err
	}
	models, err := jsonArrayParam(ch.Models)
	if err != nil {
		return err
	}
	sellPrice, err := jsonObjectParam(ch.SellPrice)
	if err != nil {
		return err
	}
	mapping, err := jsonObjectParam(ch.ModelMapping)
	if err != nil {
		return err
	}
	headers, err := jsonObjectParam(ch.ExtraHeaders)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO direct_channels (
			id, type, alias, alias_norm, base_url, api_key_enc, models,
			sell_price, model_mapping, extra_headers, enabled, status,
			created_at, updated_at, deleted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, ch.ID, ch.Type, ch.Alias, ch.AliasNorm, textFromString(ch.BaseURL),
		textFromString(ch.APIKeyEnc), models, sellPrice, mapping, headers,
		ch.Enabled, textFromString(ch.Status), ch.CreatedAt.UTC(), ch.UpdatedAt.UTC(),
		timestamptzFromPtr(ch.DeletedAt))
	if err != nil {
		return fmt.Errorf("insert direct channel: %w", err)
	}
	return nil
}

func GetDirectChannel(ctx context.Context, id string) (DirectChannel, error) {
	p, err := requirePool()
	if err != nil {
		return DirectChannel{}, err
	}
	ch, err := scanDirectChannel(p.QueryRow(ctx, directChannelSelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return DirectChannel{}, ErrNotFound
	}
	return ch, err
}

func ListDirectChannels(ctx context.Context, includeDeleted bool) ([]DirectChannel, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := `deleted_at IS NULL`
	if includeDeleted {
		where = `TRUE`
	}
	rows, err := p.Query(ctx, directChannelSelectSQL(where)+` ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list direct channels: %w", err)
	}
	defer rows.Close()
	var out []DirectChannel
	for rows.Next() {
		ch, err := scanDirectChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate direct channels: %w", err)
	}
	return out, nil
}

func UpdateDirectChannel(ctx context.Context, ch DirectChannel) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	prepareDirectChannel(&ch)
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin direct channel update: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := ensureChannelAliasAvailable(ctx, tx, "direct", ch.ID, ch.AliasNorm); err != nil {
		return err
	}
	models, err := jsonArrayParam(ch.Models)
	if err != nil {
		return err
	}
	sellPrice, err := jsonObjectParam(ch.SellPrice)
	if err != nil {
		return err
	}
	mapping, err := jsonObjectParam(ch.ModelMapping)
	if err != nil {
		return err
	}
	headers, err := jsonObjectParam(ch.ExtraHeaders)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE direct_channels
		SET type=$2, alias=$3, alias_norm=$4, base_url=$5, api_key_enc=$6,
			models=$7, sell_price=$8, model_mapping=$9, extra_headers=$10,
			enabled=$11, status=$12, updated_at=$13, deleted_at=$14
		WHERE id=$1
	`, ch.ID, ch.Type, ch.Alias, ch.AliasNorm, textFromString(ch.BaseURL),
		textFromString(ch.APIKeyEnc), models, sellPrice, mapping, headers,
		ch.Enabled, textFromString(ch.Status), time.Now().UTC(), timestamptzFromPtr(ch.DeletedAt))
	if err != nil {
		return fmt.Errorf("update direct channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit direct channel update: %w", err)
	}
	return nil
}

func SoftDeleteDirectChannel(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE direct_channels SET enabled=false, deleted_at=now(), updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete direct channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func HardDeleteDirectChannel(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `DELETE FROM direct_channels WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("hard delete direct channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SetDirectChannelAPIKey(ctx context.Context, id, enc string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE direct_channels SET api_key_enc=$2, updated_at=now() WHERE id=$1`, id, textFromString(enc))
	if err != nil {
		return fmt.Errorf("set direct channel api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func prepareDirectChannel(ch *DirectChannel) {
	ch.ID = strings.TrimSpace(ch.ID)
	ch.Type = strings.ToLower(strings.TrimSpace(ch.Type))
	ch.Alias = strings.TrimSpace(ch.Alias)
	ch.AliasNorm = normalizeChannelAlias(ch.Alias)
	ch.BaseURL = strings.TrimSpace(ch.BaseURL)
	if ch.CreatedAt.IsZero() {
		ch.CreatedAt = time.Now().UTC()
	}
	if ch.UpdatedAt.IsZero() {
		ch.UpdatedAt = ch.CreatedAt
	}
}

func normalizeChannelAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

func ensureChannelAliasAvailable(ctx context.Context, tx pgx.Tx, ownerType, ownerID, aliasNorm string) error {
	if aliasNorm == "" {
		return errors.New("channel alias is required")
	}
	if tx == nil {
		return errors.New("check channel alias requires transaction")
	}
	// 跨表 alias 唯一性在 app 层校验（direct_channels 和 newapi_channels 都有自己的
	// UNIQUE(alias_norm)，但跨表没有 DB 约束）。这里用 transaction-scoped advisory
	// lock 序列化 SELECT-then-INSERT，否则两个并发 tx 可以同时通过 SELECT 检查后各自
	// 写入不同表导致跨表 alias 冲突。
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, aliasNorm); err != nil {
		return fmt.Errorf("lock channel alias: %w", err)
	}
	var conflictID string
	err := tx.QueryRow(ctx, `
		SELECT id FROM direct_channels
		WHERE alias_norm=$1 AND deleted_at IS NULL AND ($2 <> 'direct' OR id <> $3)
		UNION ALL
		SELECT id FROM newapi_channels
		WHERE alias_norm=$1 AND deleted_at IS NULL AND ($2 <> 'newapi' OR id <> $3)
		LIMIT 1
	`, aliasNorm, ownerType, ownerID).Scan(&conflictID)
	if err == nil {
		return fmt.Errorf("%w: %s", ErrAliasConflict, aliasNorm)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return fmt.Errorf("check channel alias: %w", err)
}

func directChannelSelectSQL(where string) string {
	return `
		SELECT id, type, alias, alias_norm, base_url, api_key_enc, models,
			sell_price, model_mapping, extra_headers, enabled, status,
			created_at, updated_at, deleted_at
		FROM direct_channels
		WHERE ` + where
}

type directChannelScanner interface {
	Scan(dest ...any) error
}

func scanDirectChannel(row directChannelScanner) (DirectChannel, error) {
	var ch DirectChannel
	var baseURL, apiKey, status pgtype.Text
	var deletedAt pgtype.Timestamptz
	var modelsRaw, sellRaw, mappingRaw, headersRaw []byte
	if err := row.Scan(
		&ch.ID, &ch.Type, &ch.Alias, &ch.AliasNorm, &baseURL, &apiKey,
		&modelsRaw, &sellRaw, &mappingRaw, &headersRaw, &ch.Enabled,
		&status, &ch.CreatedAt, &ch.UpdatedAt, &deletedAt,
	); err != nil {
		return DirectChannel{}, fmt.Errorf("scan direct channel: %w", err)
	}
	var err error
	ch.BaseURL = stringFromText(baseURL)
	ch.APIKeyEnc = stringFromText(apiKey)
	ch.Status = stringFromText(status)
	ch.DeletedAt = ptrFromTimestamptz(deletedAt)
	ch.Models, err = scanJSONStringSlice(modelsRaw)
	if err != nil {
		return DirectChannel{}, err
	}
	if err := jsonUnmarshalObject(sellRaw, &ch.SellPrice); err != nil {
		return DirectChannel{}, err
	}
	ch.ModelMapping, err = scanJSONStringMap(mappingRaw)
	if err != nil {
		return DirectChannel{}, err
	}
	ch.ExtraHeaders, err = scanJSONStringMap(headersRaw)
	return ch, err
}
