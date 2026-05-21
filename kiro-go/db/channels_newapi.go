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

type NewAPIProvider struct {
	ID                      string
	Name                    string
	BaseURL                 string
	Username                string
	PasswordEnc             string
	AccessTokenEnc          string
	AccessTokenExpiresAt    *time.Time
	UpstreamUserID          int
	QuotaPerUnitDollar      decimal.Decimal
	YuanPerUpstreamDollar   decimal.Decimal
	LastSyncAt              *time.Time
	LastSyncError           string
	SyncIntervalSec         int
	Enabled                 bool
	Metadata                map[string]any
}

type NewAPIChannel struct {
	ID                string
	ProviderID        string
	Alias             string
	AliasNorm         string
	UpstreamTokenID   int
	UpstreamKeyEnc    string
	UpstreamTokenName string
	GroupName         string
	Models            []string
	Markup            decimal.Decimal
	SeriesID          string
	CreateMode        string
	Enabled           bool
	RemainQuota       int64
	UnlimitedQuota    bool
	Status            int
	CreatedAt         *time.Time
	UpdatedAt         *time.Time
	LastSeenAt        *time.Time
	DeletedAt         *time.Time
}

func InsertNewAPIProvider(ctx context.Context, tx pgx.Tx, pvd NewAPIProvider) error {
	if tx == nil {
		return errors.New("insert newapi provider requires transaction")
	}
	meta, err := jsonObjectParam(pvd.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO newapi_providers (
			id, name, base_url, username, password_enc, access_token_enc,
			access_token_expires_at, upstream_user_id, quota_per_unit_dollar,
			yuan_per_upstream_dollar, last_sync_at, last_sync_error,
			sync_interval_sec, enabled, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, strings.TrimSpace(pvd.ID), pvd.Name, pvd.BaseURL, pvd.Username,
		textFromString(pvd.PasswordEnc), textFromString(pvd.AccessTokenEnc),
		timestamptzFromPtr(pvd.AccessTokenExpiresAt), pvd.UpstreamUserID,
		numericFromDecimal(pvd.QuotaPerUnitDollar), numericFromDecimal(pvd.YuanPerUpstreamDollar),
		timestamptzFromPtr(pvd.LastSyncAt), textFromString(pvd.LastSyncError),
		pvd.SyncIntervalSec, pvd.Enabled, meta)
	if err != nil {
		return fmt.Errorf("insert newapi provider: %w", err)
	}
	return nil
}

func GetNewAPIProvider(ctx context.Context, id string) (NewAPIProvider, error) {
	p, err := requirePool()
	if err != nil {
		return NewAPIProvider{}, err
	}
	pvd, err := scanNewAPIProvider(p.QueryRow(ctx, newAPIProviderSelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return NewAPIProvider{}, ErrNotFound
	}
	return pvd, err
}

func ListNewAPIProviders(ctx context.Context) ([]NewAPIProvider, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, newAPIProviderSelectSQL(`TRUE`)+` ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list newapi providers: %w", err)
	}
	defer rows.Close()
	var out []NewAPIProvider
	for rows.Next() {
		pvd, err := scanNewAPIProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pvd)
	}
	return out, rows.Err()
}

func UpdateNewAPIProvider(ctx context.Context, pvd NewAPIProvider) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	meta, err := jsonObjectParam(pvd.Metadata)
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE newapi_providers
		SET name=$2, base_url=$3, username=$4, password_enc=$5,
			access_token_enc=$6, access_token_expires_at=$7,
			upstream_user_id=$8, quota_per_unit_dollar=$9,
			yuan_per_upstream_dollar=$10, last_sync_at=$11,
			last_sync_error=$12, sync_interval_sec=$13, enabled=$14,
			metadata=$15
		WHERE id=$1
	`, pvd.ID, pvd.Name, pvd.BaseURL, pvd.Username, textFromString(pvd.PasswordEnc),
		textFromString(pvd.AccessTokenEnc), timestamptzFromPtr(pvd.AccessTokenExpiresAt),
		pvd.UpstreamUserID, numericFromDecimal(pvd.QuotaPerUnitDollar),
		numericFromDecimal(pvd.YuanPerUpstreamDollar), timestamptzFromPtr(pvd.LastSyncAt),
		textFromString(pvd.LastSyncError), pvd.SyncIntervalSec, pvd.Enabled, meta)
	if err != nil {
		return fmt.Errorf("update newapi provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func UpdateNewAPIProviderSync(ctx context.Context, id string, at time.Time, syncErr string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE newapi_providers SET last_sync_at=$2, last_sync_error=$3 WHERE id=$1`, id, at.UTC(), textFromString(syncErr))
	if err != nil {
		return fmt.Errorf("update newapi provider sync: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func RotateNewAPIProviderToken(ctx context.Context, id, tokenEnc string, expiresAt *time.Time) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE newapi_providers SET access_token_enc=$2, access_token_expires_at=$3 WHERE id=$1`, id, textFromString(tokenEnc), timestamptzFromPtr(expiresAt))
	if err != nil {
		return fmt.Errorf("rotate newapi provider token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func InsertNewAPIChannel(ctx context.Context, tx pgx.Tx, ch NewAPIChannel) error {
	if tx == nil {
		return errors.New("insert newapi channel requires transaction")
	}
	prepareNewAPIChannel(&ch)
	if err := ensureChannelAliasAvailable(ctx, tx, "newapi", ch.ID, ch.AliasNorm); err != nil {
		return err
	}
	models, err := jsonArrayParam(ch.Models)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO newapi_channels (
			id, provider_id, alias, alias_norm, upstream_token_id,
			upstream_key_enc, upstream_token_name, group_name, models,
			markup, series_id, create_mode, enabled, remain_quota,
			unlimited_quota, status, created_at, updated_at, last_seen_at,
			deleted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	`, ch.ID, ch.ProviderID, ch.Alias, ch.AliasNorm, ch.UpstreamTokenID,
		textFromString(ch.UpstreamKeyEnc), textFromString(ch.UpstreamTokenName),
		ch.GroupName, models, numericFromDecimal(ch.Markup), textFromString(ch.SeriesID),
		textFromString(ch.CreateMode), ch.Enabled, ch.RemainQuota, ch.UnlimitedQuota,
		ch.Status, timestamptzFromPtr(ch.CreatedAt), timestamptzFromPtr(ch.UpdatedAt),
		timestamptzFromPtr(ch.LastSeenAt), timestamptzFromPtr(ch.DeletedAt))
	if err != nil {
		return fmt.Errorf("insert newapi channel: %w", err)
	}
	return nil
}

func GetNewAPIChannel(ctx context.Context, id string) (NewAPIChannel, error) {
	p, err := requirePool()
	if err != nil {
		return NewAPIChannel{}, err
	}
	ch, err := scanNewAPIChannel(p.QueryRow(ctx, newAPIChannelSelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return NewAPIChannel{}, ErrNotFound
	}
	return ch, err
}

func ListNewAPIChannels(ctx context.Context, includeDeleted bool) ([]NewAPIChannel, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := `deleted_at IS NULL`
	if includeDeleted {
		where = `TRUE`
	}
	rows, err := p.Query(ctx, newAPIChannelSelectSQL(where)+` ORDER BY provider_id ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list newapi channels: %w", err)
	}
	defer rows.Close()
	var out []NewAPIChannel
	for rows.Next() {
		ch, err := scanNewAPIChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func UpdateNewAPIChannel(ctx context.Context, ch NewAPIChannel) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	prepareNewAPIChannel(&ch)
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin newapi channel update: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := ensureChannelAliasAvailable(ctx, tx, "newapi", ch.ID, ch.AliasNorm); err != nil {
		return err
	}
	models, err := jsonArrayParam(ch.Models)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE newapi_channels
		SET provider_id=$2, alias=$3, alias_norm=$4, upstream_token_id=$5,
			upstream_key_enc=$6, upstream_token_name=$7, group_name=$8,
			models=$9, markup=$10, series_id=$11, create_mode=$12,
			enabled=$13, remain_quota=$14, unlimited_quota=$15, status=$16,
			updated_at=$17, last_seen_at=$18, deleted_at=$19
		WHERE id=$1
	`, ch.ID, ch.ProviderID, ch.Alias, ch.AliasNorm, ch.UpstreamTokenID,
		textFromString(ch.UpstreamKeyEnc), textFromString(ch.UpstreamTokenName),
		ch.GroupName, models, numericFromDecimal(ch.Markup), textFromString(ch.SeriesID),
		textFromString(ch.CreateMode), ch.Enabled, ch.RemainQuota, ch.UnlimitedQuota,
		ch.Status, time.Now().UTC(), timestamptzFromPtr(ch.LastSeenAt),
		timestamptzFromPtr(ch.DeletedAt))
	if err != nil {
		return fmt.Errorf("update newapi channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit newapi channel update: %w", err)
	}
	return nil
}

func SoftDeleteNewAPIChannel(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE newapi_channels SET enabled=false, deleted_at=now(), updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete newapi channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func UpdateNewAPIChannelQuota(ctx context.Context, id string, remainQuota int64, unlimited bool, status int, seenAt time.Time) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE newapi_channels
		SET remain_quota=$2, unlimited_quota=$3, status=$4,
			last_seen_at=$5, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL
	`, id, remainQuota, unlimited, status, seenAt.UTC())
	if err != nil {
		return fmt.Errorf("update newapi channel quota: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func prepareNewAPIChannel(ch *NewAPIChannel) {
	now := time.Now().UTC()
	ch.ID = strings.TrimSpace(ch.ID)
	ch.ProviderID = strings.TrimSpace(ch.ProviderID)
	ch.Alias = strings.TrimSpace(ch.Alias)
	ch.AliasNorm = normalizeChannelAlias(ch.Alias)
	ch.UpstreamTokenName = strings.TrimSpace(ch.UpstreamTokenName)
	ch.GroupName = strings.TrimSpace(ch.GroupName)
	ch.SeriesID = strings.TrimSpace(ch.SeriesID)
	ch.CreateMode = strings.TrimSpace(ch.CreateMode)
	if ch.CreatedAt == nil {
		ch.CreatedAt = &now
	}
	if ch.UpdatedAt == nil {
		ch.UpdatedAt = &now
	}
}

func newAPIProviderSelectSQL(where string) string {
	return `
		SELECT id, name, base_url, username, password_enc, access_token_enc,
			access_token_expires_at, upstream_user_id, quota_per_unit_dollar,
			yuan_per_upstream_dollar, last_sync_at, last_sync_error,
			sync_interval_sec, enabled, metadata
		FROM newapi_providers
		WHERE ` + where
}

func newAPIChannelSelectSQL(where string) string {
	return `
		SELECT id, provider_id, alias, alias_norm, upstream_token_id,
			upstream_key_enc, upstream_token_name, group_name, models,
			markup, series_id, create_mode, enabled, remain_quota,
			unlimited_quota, status, created_at, updated_at, last_seen_at,
			deleted_at
		FROM newapi_channels
		WHERE ` + where
}

type newAPIProviderScanner interface {
	Scan(dest ...any) error
}

func scanNewAPIProvider(row newAPIProviderScanner) (NewAPIProvider, error) {
	var pvd NewAPIProvider
	var password, token, lastErr pgtype.Text
	var tokenExp, lastSync pgtype.Timestamptz
	var quota, yuan pgtype.Numeric
	var metadata []byte
	if err := row.Scan(&pvd.ID, &pvd.Name, &pvd.BaseURL, &pvd.Username,
		&password, &token, &tokenExp, &pvd.UpstreamUserID, &quota, &yuan,
		&lastSync, &lastErr, &pvd.SyncIntervalSec, &pvd.Enabled, &metadata); err != nil {
		return NewAPIProvider{}, fmt.Errorf("scan newapi provider: %w", err)
	}
	var err error
	pvd.PasswordEnc = stringFromText(password)
	pvd.AccessTokenEnc = stringFromText(token)
	pvd.AccessTokenExpiresAt = ptrFromTimestamptz(tokenExp)
	pvd.LastSyncAt = ptrFromTimestamptz(lastSync)
	pvd.LastSyncError = stringFromText(lastErr)
	if pvd.QuotaPerUnitDollar, err = decimalFromNumeric(quota); err != nil {
		return NewAPIProvider{}, err
	}
	if pvd.YuanPerUpstreamDollar, err = decimalFromNumeric(yuan); err != nil {
		return NewAPIProvider{}, err
	}
	pvd.Metadata, err = scanJSONMap(metadata)
	return pvd, err
}

type newAPIChannelScanner interface {
	Scan(dest ...any) error
}

func scanNewAPIChannel(row newAPIChannelScanner) (NewAPIChannel, error) {
	var ch NewAPIChannel
	var upstreamKey, tokenName, seriesID, createMode pgtype.Text
	var createdAt, updatedAt, lastSeenAt, deletedAt pgtype.Timestamptz
	var markup pgtype.Numeric
	var modelsRaw []byte
	if err := row.Scan(&ch.ID, &ch.ProviderID, &ch.Alias, &ch.AliasNorm,
		&ch.UpstreamTokenID, &upstreamKey, &tokenName, &ch.GroupName,
		&modelsRaw, &markup, &seriesID, &createMode, &ch.Enabled,
		&ch.RemainQuota, &ch.UnlimitedQuota, &ch.Status, &createdAt,
		&updatedAt, &lastSeenAt, &deletedAt); err != nil {
		return NewAPIChannel{}, fmt.Errorf("scan newapi channel: %w", err)
	}
	var err error
	ch.UpstreamKeyEnc = stringFromText(upstreamKey)
	ch.UpstreamTokenName = stringFromText(tokenName)
	ch.SeriesID = stringFromText(seriesID)
	ch.CreateMode = stringFromText(createMode)
	ch.CreatedAt = ptrFromTimestamptz(createdAt)
	ch.UpdatedAt = ptrFromTimestamptz(updatedAt)
	ch.LastSeenAt = ptrFromTimestamptz(lastSeenAt)
	ch.DeletedAt = ptrFromTimestamptz(deletedAt)
	ch.Markup, err = decimalFromNumeric(markup)
	if err != nil {
		return NewAPIChannel{}, err
	}
	ch.Models, err = scanJSONStringSlice(modelsRaw)
	return ch, err
}
