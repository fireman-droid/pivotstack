package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ChannelGroupMember struct {
	SourceType string
	ChannelID  string
	SortOrder  int
}

type ChannelGroup struct {
	ID                      string
	Name                    string
	Description             string
	Enabled                 bool
	ModelPatterns           []string
	DefaultRuntimeChannelID string
	SortOrder               int
	CreatedAt               time.Time
	UpdatedAt               time.Time
	DeletedAt               *time.Time
	Members                 []ChannelGroupMember
}

func InsertChannelGroup(ctx context.Context, tx pgx.Tx, g ChannelGroup) error {
	if tx == nil {
		return errors.New("insert channel group requires transaction")
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = g.CreatedAt
	}
	patterns, err := jsonArrayParam(g.ModelPatterns)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO channel_groups (
			id, name, description, enabled, model_patterns,
			default_runtime_channel_id, sort_order, created_at, updated_at,
			deleted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, g.ID, g.Name, textFromString(g.Description), g.Enabled, patterns,
		textFromString(g.DefaultRuntimeChannelID), g.SortOrder, g.CreatedAt.UTC(),
		g.UpdatedAt.UTC(), timestamptzFromPtr(g.DeletedAt))
	if err != nil {
		return fmt.Errorf("insert channel group: %w", err)
	}
	for _, m := range g.Members {
		if err := joinGroupTx(ctx, tx, g.ID, m.SourceType, m.ChannelID, m.SortOrder); err != nil {
			return err
		}
	}
	return nil
}

func GetChannelGroup(ctx context.Context, id string) (ChannelGroup, error) {
	groups, err := listChannelGroups(ctx, `g.id=$1`, id)
	if err != nil {
		return ChannelGroup{}, err
	}
	if len(groups) == 0 {
		return ChannelGroup{}, ErrNotFound
	}
	return groups[0], nil
}

func ListGroupsWithMembers(ctx context.Context, includeDeleted bool) ([]ChannelGroup, error) {
	where := `g.deleted_at IS NULL`
	if includeDeleted {
		where = `TRUE`
	}
	return listChannelGroups(ctx, where)
}

func UpdateChannelGroup(ctx context.Context, g ChannelGroup) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	patterns, err := jsonArrayParam(g.ModelPatterns)
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE channel_groups
		SET name=$2, description=$3, enabled=$4, model_patterns=$5,
			default_runtime_channel_id=$6, sort_order=$7, updated_at=now(),
			deleted_at=$8
		WHERE id=$1
	`, g.ID, g.Name, textFromString(g.Description), g.Enabled, patterns,
		textFromString(g.DefaultRuntimeChannelID), g.SortOrder, timestamptzFromPtr(g.DeletedAt))
	if err != nil {
		return fmt.Errorf("update channel group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SoftDeleteChannelGroup(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE channel_groups SET enabled=false, deleted_at=now(), updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete channel group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func JoinGroup(ctx context.Context, groupID, sourceType, channelID string, sortOrder int) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin join group: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := joinGroupTx(ctx, tx, groupID, sourceType, channelID, sortOrder); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE channel_groups SET updated_at=now() WHERE id=$1`, groupID); err != nil {
		return fmt.Errorf("touch channel group: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit join group: %w", err)
	}
	return nil
}

func LeaveGroup(ctx context.Context, groupID, sourceType, channelID string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		DELETE FROM channel_group_members
		WHERE group_id=$1 AND source_type=$2 AND channel_id=$3
	`, groupID, sourceType, channelID)
	if err != nil {
		return fmt.Errorf("leave channel group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func joinGroupTx(ctx context.Context, tx pgx.Tx, groupID, sourceType, channelID string, sortOrder int) error {
	if sourceType != "newapi" && sourceType != "direct" {
		return fmt.Errorf("invalid channel source type: %s", sourceType)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO channel_group_members(group_id, source_type, channel_id, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (group_id, source_type, channel_id)
		DO UPDATE SET sort_order=EXCLUDED.sort_order
	`, groupID, sourceType, channelID, sortOrder)
	if err != nil {
		return fmt.Errorf("join channel group: %w", err)
	}
	return nil
}

func listChannelGroups(ctx context.Context, where string, args ...any) ([]ChannelGroup, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT g.id, g.name, g.description, g.enabled, g.model_patterns,
			g.default_runtime_channel_id, g.sort_order, g.created_at,
			g.updated_at, g.deleted_at, m.source_type, m.channel_id,
			m.sort_order
		FROM channel_groups g
		LEFT JOIN channel_group_members m ON m.group_id=g.id
		WHERE `+where+`
		ORDER BY g.sort_order ASC, g.created_at ASC, m.sort_order ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list channel groups: %w", err)
	}
	defer rows.Close()

	byID := map[string]*ChannelGroup{}
	var order []string
	for rows.Next() {
		var g ChannelGroup
		var desc, defaultRuntime pgtype.Text
		var deletedAt pgtype.Timestamptz
		var patternsRaw []byte
		var sourceType, channelID pgtype.Text
		var memberSort pgtype.Int4
		if err := rows.Scan(&g.ID, &g.Name, &desc, &g.Enabled, &patternsRaw,
			&defaultRuntime, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt,
			&deletedAt, &sourceType, &channelID, &memberSort); err != nil {
			return nil, fmt.Errorf("scan channel group: %w", err)
		}
		existing := byID[g.ID]
		if existing == nil {
			patterns, err := scanJSONStringSlice(patternsRaw)
			if err != nil {
				return nil, err
			}
			g.Description = stringFromText(desc)
			g.DefaultRuntimeChannelID = stringFromText(defaultRuntime)
			g.DeletedAt = ptrFromTimestamptz(deletedAt)
			g.ModelPatterns = patterns
			byID[g.ID] = &g
			order = append(order, g.ID)
			existing = &g
		}
		if sourceType.Valid && channelID.Valid {
			existing.Members = append(existing.Members, ChannelGroupMember{
				SourceType: sourceType.String,
				ChannelID:  channelID.String,
				SortOrder:  int(memberSort.Int32),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel groups: %w", err)
	}
	out := make([]ChannelGroup, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}
