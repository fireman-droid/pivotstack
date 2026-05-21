package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Series struct {
	ID               string
	Name             string
	DefaultChannelID string
	ModelPatterns    []string
	SortOrder        int
}

func InsertSeries(ctx context.Context, tx pgx.Tx, s Series) error {
	if tx == nil {
		return errors.New("insert series requires transaction")
	}
	patterns, err := jsonArrayParam(s.ModelPatterns)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO series(id, name, default_channel_id, model_patterns, sort_order)
		VALUES ($1,$2,$3,$4,$5)
	`, s.ID, s.Name, textFromString(s.DefaultChannelID), patterns, s.SortOrder)
	if err != nil {
		return fmt.Errorf("insert series: %w", err)
	}
	return nil
}

func GetSeries(ctx context.Context, id string) (Series, error) {
	p, err := requirePool()
	if err != nil {
		return Series{}, err
	}
	s, err := scanSeries(p.QueryRow(ctx, seriesSelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Series{}, ErrNotFound
	}
	return s, err
}

func ListSeries(ctx context.Context) ([]Series, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, seriesSelectSQL(`TRUE`)+` ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	defer rows.Close()
	var out []Series
	for rows.Next() {
		s, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func UpdateSeries(ctx context.Context, s Series) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	patterns, err := jsonArrayParam(s.ModelPatterns)
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE series
		SET name=$2, default_channel_id=$3, model_patterns=$4, sort_order=$5
		WHERE id=$1
	`, s.ID, s.Name, textFromString(s.DefaultChannelID), patterns, s.SortOrder)
	if err != nil {
		return fmt.Errorf("update series: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func DeleteSeries(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `DELETE FROM series WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete series: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func seriesSelectSQL(where string) string {
	return `
		SELECT id, name, default_channel_id, model_patterns, sort_order
		FROM series
		WHERE ` + where
}

type seriesScanner interface {
	Scan(dest ...any) error
}

func scanSeries(row seriesScanner) (Series, error) {
	var s Series
	var defaultChannelID pgtype.Text
	var patternsRaw []byte
	if err := row.Scan(&s.ID, &s.Name, &defaultChannelID, &patternsRaw, &s.SortOrder); err != nil {
		return Series{}, fmt.Errorf("scan series: %w", err)
	}
	var err error
	s.DefaultChannelID = stringFromText(defaultChannelID)
	s.ModelPatterns, err = scanJSONStringSlice(patternsRaw)
	return s, err
}
