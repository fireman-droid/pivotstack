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

type PromotionState struct {
	Payload   json.RawMessage
	Enabled   bool
	StartAt   *time.Time
	EndAt     *time.Time
	UpdatedAt time.Time
	UpdatedBy string
}

func GetPricingConfig(ctx context.Context, dst any) (version int, updatedAt time.Time, err error) {
	p, err := requirePool()
	if err != nil {
		return 0, time.Time{}, err
	}
	var raw []byte
	var updatedBy pgtype.Text
	err = p.QueryRow(ctx, `
		SELECT payload, version, updated_at, updated_by
		FROM pricing_config
		WHERE singleton_id=true
	`).Scan(&raw, &version, &updatedAt, &updatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, time.Time{}, ErrNotFound
	}
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("get pricing config: %w", err)
	}
	if dst != nil {
		if err := json.Unmarshal(raw, dst); err != nil {
			return 0, time.Time{}, fmt.Errorf("unmarshal pricing config: %w", err)
		}
	}
	return version, updatedAt, nil
}

func UpdatePricingConfig(ctx context.Context, payload any, version int, updatedBy string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	if version <= 0 {
		version = 1
	}
	raw, err := jsonObjectParam(payload)
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, `
		INSERT INTO pricing_config(singleton_id, payload, version, updated_by)
		VALUES (true, $1, $2, $3)
		ON CONFLICT (singleton_id) DO UPDATE
		SET payload=EXCLUDED.payload,
			version=GREATEST(pricing_config.version + 1, EXCLUDED.version),
			updated_at=now(),
			updated_by=EXCLUDED.updated_by
	`, raw, version, textFromString(updatedBy))
	if err != nil {
		return fmt.Errorf("update pricing config: %w", err)
	}
	return nil
}

func GetStealthConfig(ctx context.Context, dst any) (updatedAt time.Time, err error) {
	p, err := requirePool()
	if err != nil {
		return time.Time{}, err
	}
	var raw []byte
	var updatedBy pgtype.Text
	err = p.QueryRow(ctx, `
		SELECT payload, updated_at, updated_by
		FROM stealth_config
		WHERE singleton_id=true
	`).Scan(&raw, &updatedAt, &updatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get stealth config: %w", err)
	}
	if dst != nil {
		if err := json.Unmarshal(raw, dst); err != nil {
			return time.Time{}, fmt.Errorf("unmarshal stealth config: %w", err)
		}
	}
	return updatedAt, nil
}

func UpdateStealthConfig(ctx context.Context, payload any, updatedBy string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	raw, err := jsonObjectParam(payload)
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, `
		INSERT INTO stealth_config(singleton_id, payload, updated_by)
		VALUES (true, $1, $2)
		ON CONFLICT (singleton_id) DO UPDATE
		SET payload=EXCLUDED.payload, updated_at=now(), updated_by=EXCLUDED.updated_by
	`, raw, textFromString(updatedBy))
	if err != nil {
		return fmt.Errorf("update stealth config: %w", err)
	}
	return nil
}

func GetPromotionConfig(ctx context.Context) (PromotionState, error) {
	p, err := requirePool()
	if err != nil {
		return PromotionState{}, err
	}
	var st PromotionState
	var startAt, endAt pgtype.Timestamptz
	var updatedBy pgtype.Text
	err = p.QueryRow(ctx, `
		SELECT payload, enabled, start_at, end_at, updated_at, updated_by
		FROM promotion_config
		WHERE singleton_id=true
	`).Scan(&st.Payload, &st.Enabled, &startAt, &endAt, &st.UpdatedAt, &updatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return PromotionState{}, ErrNotFound
	}
	if err != nil {
		return PromotionState{}, fmt.Errorf("get promotion config: %w", err)
	}
	st.StartAt = ptrFromTimestamptz(startAt)
	st.EndAt = ptrFromTimestamptz(endAt)
	st.UpdatedBy = stringFromText(updatedBy)
	if len(st.Payload) == 0 {
		st.Payload = json.RawMessage("{}")
	}
	return st, nil
}

func UpdatePromotionConfig(ctx context.Context, payload any, enabled bool, startAt, endAt *time.Time, updatedBy string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	raw, err := jsonObjectParam(payload)
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, `
		INSERT INTO promotion_config(singleton_id, payload, enabled, start_at, end_at, updated_by)
		VALUES (true, $1, $2, $3, $4, $5)
		ON CONFLICT (singleton_id) DO UPDATE
		SET payload=EXCLUDED.payload,
			enabled=EXCLUDED.enabled,
			start_at=EXCLUDED.start_at,
			end_at=EXCLUDED.end_at,
			updated_at=now(),
			updated_by=EXCLUDED.updated_by
	`, raw, enabled, timestamptzFromPtr(startAt), timestamptzFromPtr(endAt), textFromString(updatedBy))
	if err != nil {
		return fmt.Errorf("update promotion config: %w", err)
	}
	return nil
}
