package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrChecksumDrift = errors.New("db: checksum drift")

func RecordImport(ctx context.Context, tx pgx.Tx, source, legacyID, payloadSHA256 string) (bool, error) {
	if tx == nil {
		return false, errors.New("record import requires transaction")
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO migration_imports(source_name, legacy_id, payload_sha256)
		VALUES ($1,$2,$3)
		ON CONFLICT (source_name, legacy_id) DO NOTHING
	`, source, legacyID, payloadSHA256)
	if err != nil {
		return false, fmt.Errorf("record migration import: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func HasImport(ctx context.Context, source, legacyID string) (bool, error) {
	p, err := requirePool()
	if err != nil {
		return false, err
	}
	var exists bool
	if err := p.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM migration_imports
			WHERE source_name=$1 AND legacy_id=$2
		)
	`, source, legacyID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check migration import: %w", err)
	}
	return exists, nil
}

func DriftCheck(ctx context.Context, source, legacyID, payloadSHA256 string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	var stored string
	if err := p.QueryRow(ctx, `
		SELECT payload_sha256
		FROM migration_imports
		WHERE source_name=$1 AND legacy_id=$2
	`, source, legacyID).Scan(&stored); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read migration import checksum: %w", err)
	}
	if stored != payloadSHA256 {
		return ErrChecksumDrift
	}
	return nil
}
