package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SettingRow struct {
	Key       string
	Value     json.RawMessage
	UpdatedAt time.Time
	UpdatedBy string
}

func GetSetting(ctx context.Context, key string) (SettingRow, error) {
	p, err := requirePool()
	if err != nil {
		return SettingRow{}, err
	}
	row, err := scanSetting(p.QueryRow(ctx, `
		SELECT key, value, updated_at, updated_by
		FROM settings_kv
		WHERE key=$1
	`, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return SettingRow{}, ErrNotFound
	}
	return row, err
}

func SetSetting(ctx context.Context, key string, value any, updatedBy string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	raw, err := jsonParam(value)
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, `
		INSERT INTO settings_kv(key, value, updated_by)
		VALUES ($1,$2,$3)
		ON CONFLICT (key) DO UPDATE
		SET value=EXCLUDED.value, updated_at=now(), updated_by=EXCLUDED.updated_by
	`, key, raw, textFromString(updatedBy))
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

func DeleteSetting(ctx context.Context, key string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `DELETE FROM settings_kv WHERE key=$1`, key)
	if err != nil {
		return fmt.Errorf("delete setting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func ListSettings(ctx context.Context) ([]SettingRow, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT key, value, updated_at, updated_by
		FROM settings_kv
		ORDER BY key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	var out []SettingRow
	for rows.Next() {
		row, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return out, nil
}

func GetSettingJSON(ctx context.Context, key string, dst any) error {
	row, err := GetSetting(ctx, key)
	if err != nil {
		return err
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(row.Value, dst); err != nil {
		return fmt.Errorf("unmarshal setting %s: %w", key, err)
	}
	return nil
}

func SetSettingJSON(ctx context.Context, key string, value any, updatedBy string) error {
	return SetSetting(ctx, key, value, updatedBy)
}

type settingScanner interface {
	Scan(dest ...any) error
}

func scanSetting(row settingScanner) (SettingRow, error) {
	var s SettingRow
	var updatedBy pgtype.Text
	if err := row.Scan(&s.Key, &s.Value, &s.UpdatedAt, &updatedBy); err != nil {
		return SettingRow{}, fmt.Errorf("scan setting: %w", err)
	}
	s.UpdatedBy = stringFromText(updatedBy)
	if len(s.Value) == 0 {
		s.Value = json.RawMessage("{}")
	}
	return s, nil
}
