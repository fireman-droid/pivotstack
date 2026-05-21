package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type LegacyChannel struct {
	ID           string
	Type         string
	SeriesID     string
	BaseURL      string
	APIKeyEnc    string
	Models       []string
	ModelPrices  map[string]any
	ModelAliases map[string]string
	ExtraHeaders map[string]string
	Enabled      bool
}

func InsertLegacyChannel(ctx context.Context, tx pgx.Tx, ch LegacyChannel) error {
	if tx == nil {
		return errors.New("insert legacy channel requires transaction")
	}
	models, prices, aliases, headers, err := legacyChannelJSONParams(ch)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO legacy_channels (
			id, type, series_id, base_url, api_key_enc, models,
			model_prices, model_aliases, extra_headers, enabled
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, ch.ID, ch.Type, textFromString(ch.SeriesID), textFromString(ch.BaseURL),
		textFromString(ch.APIKeyEnc), models, prices, aliases, headers, ch.Enabled)
	if err != nil {
		return fmt.Errorf("insert legacy channel: %w", err)
	}
	return nil
}

func GetLegacyChannel(ctx context.Context, id string) (LegacyChannel, error) {
	p, err := requirePool()
	if err != nil {
		return LegacyChannel{}, err
	}
	ch, err := scanLegacyChannel(p.QueryRow(ctx, legacyChannelSelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return LegacyChannel{}, ErrNotFound
	}
	return ch, err
}

func ListLegacyChannels(ctx context.Context) ([]LegacyChannel, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, legacyChannelSelectSQL(`TRUE`)+` ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list legacy channels: %w", err)
	}
	defer rows.Close()
	var out []LegacyChannel
	for rows.Next() {
		ch, err := scanLegacyChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func UpdateLegacyChannel(ctx context.Context, ch LegacyChannel) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	models, prices, aliases, headers, err := legacyChannelJSONParams(ch)
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE legacy_channels
		SET type=$2, series_id=$3, base_url=$4, api_key_enc=$5,
			models=$6, model_prices=$7, model_aliases=$8,
			extra_headers=$9, enabled=$10
		WHERE id=$1
	`, ch.ID, ch.Type, textFromString(ch.SeriesID), textFromString(ch.BaseURL),
		textFromString(ch.APIKeyEnc), models, prices, aliases, headers, ch.Enabled)
	if err != nil {
		return fmt.Errorf("update legacy channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func DeleteLegacyChannel(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `DELETE FROM legacy_channels WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete legacy channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func legacyChannelJSONParams(ch LegacyChannel) (models, prices, aliases, headers string, err error) {
	if models, err = jsonArrayParam(ch.Models); err != nil {
		return
	}
	if prices, err = jsonObjectParam(ch.ModelPrices); err != nil {
		return
	}
	if aliases, err = jsonObjectParam(ch.ModelAliases); err != nil {
		return
	}
	headers, err = jsonObjectParam(ch.ExtraHeaders)
	return
}

func legacyChannelSelectSQL(where string) string {
	return `
		SELECT id, type, series_id, base_url, api_key_enc, models,
			model_prices, model_aliases, extra_headers, enabled
		FROM legacy_channels
		WHERE ` + where
}

type legacyChannelScanner interface {
	Scan(dest ...any) error
}

func scanLegacyChannel(row legacyChannelScanner) (LegacyChannel, error) {
	var ch LegacyChannel
	var seriesID, baseURL, apiKey pgtype.Text
	var modelsRaw, pricesRaw, aliasesRaw, headersRaw []byte
	if err := row.Scan(&ch.ID, &ch.Type, &seriesID, &baseURL, &apiKey,
		&modelsRaw, &pricesRaw, &aliasesRaw, &headersRaw, &ch.Enabled); err != nil {
		return LegacyChannel{}, fmt.Errorf("scan legacy channel: %w", err)
	}
	var err error
	ch.SeriesID = stringFromText(seriesID)
	ch.BaseURL = stringFromText(baseURL)
	ch.APIKeyEnc = stringFromText(apiKey)
	ch.Models, err = scanJSONStringSlice(modelsRaw)
	if err != nil {
		return LegacyChannel{}, err
	}
	ch.ModelPrices, err = scanJSONMap(pricesRaw)
	if err != nil {
		return LegacyChannel{}, err
	}
	ch.ModelAliases, err = scanJSONStringMap(aliasesRaw)
	if err != nil {
		return LegacyChannel{}, err
	}
	ch.ExtraHeaders, err = scanJSONStringMap(headersRaw)
	return ch, err
}
