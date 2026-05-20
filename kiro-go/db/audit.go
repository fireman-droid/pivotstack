package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuditLog struct {
	ID         string
	OccurredAt time.Time
	Action     string
	Actor      string
	Payload    map[string]any
}

type AuditFilter struct {
	Action string
	Actor  string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

func InsertAuditLog(ctx context.Context, action, actor string, payload map[string]any) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	raw, err := jsonObjectParam(payload)
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, `
		INSERT INTO audit_logs(id, occurred_at, action, operator, fields)
		VALUES ($1, now(), $2, $3, $4)
	`, uuid.NewString(), textFromString(action), textFromString(actor), raw)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func ListAuditLogs(ctx context.Context, f AuditFilter) ([]AuditLog, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := []string{"TRUE"}
	args := []any{}
	if strings.TrimSpace(f.Action) != "" {
		args = append(args, strings.TrimSpace(f.Action))
		where = append(where, fmt.Sprintf("action=$%d", len(args)))
	}
	if strings.TrimSpace(f.Actor) != "" {
		args = append(args, strings.TrimSpace(f.Actor))
		where = append(where, fmt.Sprintf("operator=$%d", len(args)))
	}
	if f.From != nil {
		args = append(args, f.From.UTC())
		where = append(where, fmt.Sprintf("occurred_at >= $%d", len(args)))
	}
	if f.To != nil {
		args = append(args, f.To.UTC())
		where = append(where, fmt.Sprintf("occurred_at <= $%d", len(args)))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	args = append(args, limit)
	limitParam := len(args)
	args = append(args, f.Offset)
	offsetParam := len(args)

	rows, err := p.Query(ctx, `
		SELECT id, occurred_at, action, operator, fields
		FROM audit_logs
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY occurred_at DESC
		LIMIT $`+fmt.Sprint(limitParam)+` OFFSET $`+fmt.Sprint(offsetParam), args...)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var out []AuditLog
	for rows.Next() {
		var log AuditLog
		var action, actor pgtype.Text
		var raw []byte
		if err := rows.Scan(&log.ID, &log.OccurredAt, &action, &actor, &raw); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		log.Action = stringFromText(action)
		log.Actor = stringFromText(actor)
		log.Payload, err = scanJSONMap(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}
	return out, nil
}
