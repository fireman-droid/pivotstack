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

type User struct {
	ID             string
	Email          string
	EmailNorm      string
	Username       string
	PasswordHash   string
	DefaultKeyID   string
	InvitedBy      string
	InviterUserID  string
	CreatedAt      time.Time
	LastLoginAt    *time.Time
	Disabled       bool
	SchemaVersion  int
	Metadata       map[string]any
	APIKeyIDs      []string
	Balance        decimal.Decimal
	GiftBalance    decimal.Decimal
	TotalRecharged decimal.Decimal
	TotalGifted    decimal.Decimal
	WalletVersion  int64
}

func InsertUser(ctx context.Context, tx pgx.Tx, u User) error {
	if tx == nil {
		return errors.New("insert user requires transaction")
	}
	if u.EmailNorm == "" {
		u.EmailNorm = normalizeEmail(u.Email)
	}
	if u.SchemaVersion == 0 {
		u.SchemaVersion = 3
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	meta, err := jsonObjectParam(u.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO users (
			id, email, email_norm, username, password_hash, default_key_id,
			invited_by, inviter_user_id, created_at, last_login_at, disabled,
			schema_version, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, u.ID, u.Email, u.EmailNorm, u.Username, u.PasswordHash, textFromString(u.DefaultKeyID),
		textFromString(u.InvitedBy), textFromString(u.InviterUserID), u.CreatedAt.UTC(),
		timestamptzFromPtr(u.LastLoginAt), u.Disabled, u.SchemaVersion, meta)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO user_wallets (
			user_id, balance, gift_balance, total_recharged, total_gifted, version
		) VALUES ($1,$2,$3,$4,$5,$6)
	`, u.ID, numericFromDecimal(u.Balance), numericFromDecimal(u.GiftBalance),
		numericFromDecimal(u.TotalRecharged), numericFromDecimal(u.TotalGifted), u.WalletVersion)
	if err != nil {
		return fmt.Errorf("insert user wallet: %w", err)
	}
	for _, keyID := range u.APIKeyIDs {
		if keyID == "" {
			continue
		}
		if err := lockApiKeyForBinding(ctx, tx, keyID); err != nil {
			return fmt.Errorf("lock user key %s: %w", keyID, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_api_keys(user_id, api_key_id) VALUES ($1,$2)`,
			u.ID, keyID,
		); err != nil {
			return fmt.Errorf("bind user key %s: %w", keyID, err)
		}
	}
	return nil
}

func GetUser(ctx context.Context, id string) (User, error) {
	return getUserByQuery(ctx, `u.id = $1`, id)
}

func GetUserByEmail(ctx context.Context, emailNorm string) (User, error) {
	return getUserByQuery(ctx, `u.email_norm = $1`, normalizeEmail(emailNorm))
}

func GetUserByUsername(ctx context.Context, usernameNorm string) (User, error) {
	return getUserByQuery(ctx, `u.username = $1`, usernameNorm)
}

func ListUsers(ctx context.Context) ([]User, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, userSelectSQL(`TRUE`)+" ORDER BY u.created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	return scanUsers(rows)
}

func UpdateUser(ctx context.Context, u User) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	if u.EmailNorm == "" {
		u.EmailNorm = normalizeEmail(u.Email)
	}
	if u.SchemaVersion == 0 {
		u.SchemaVersion = 3
	}
	meta, err := jsonObjectParam(u.Metadata)
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE users
		SET email=$2, email_norm=$3, username=$4, password_hash=$5,
			default_key_id=$6, invited_by=$7, inviter_user_id=$8,
			created_at=$9, last_login_at=$10, disabled=$11,
			schema_version=$12, metadata=$13
		WHERE id=$1
	`, u.ID, u.Email, u.EmailNorm, u.Username, u.PasswordHash,
		textFromString(u.DefaultKeyID), textFromString(u.InvitedBy), textFromString(u.InviterUserID),
		u.CreatedAt.UTC(), timestamptzFromPtr(u.LastLoginAt), u.Disabled, u.SchemaVersion, meta)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func UpdateUserPassword(ctx context.Context, id, newHash string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, id, newHash)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func MarkUserLogin(ctx context.Context, id string, at time.Time) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE users SET last_login_at=$2 WHERE id=$1`, id, at.UTC())
	if err != nil {
		return fmt.Errorf("mark user login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SetUserDisabled(ctx context.Context, id string, disabled bool) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE users SET disabled=$2 WHERE id=$1`, id, disabled)
	if err != nil {
		return fmt.Errorf("set user disabled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func DeleteUser(ctx context.Context, id string) error {
	return SetUserDisabled(ctx, id, true)
}

func BindKeyToUser(ctx context.Context, userID, apiKeyID string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin bind key: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockApiKeyForBinding(ctx, tx, apiKeyID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_api_keys(user_id, api_key_id) VALUES ($1,$2)`,
		userID, apiKeyID,
	); err != nil {
		return fmt.Errorf("bind key to user: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET default_key_id=COALESCE(default_key_id, $2) WHERE id=$1`,
		userID, apiKeyID,
	); err != nil {
		return fmt.Errorf("update default key: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bind key: %w", err)
	}
	return nil
}

func UnbindKeyFromUser(ctx context.Context, userID, apiKeyID string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin unbind key: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockApiKeyForBinding(ctx, tx, apiKeyID); err != nil {
		return err
	}
	var lockedKeyID string
	if err := tx.QueryRow(ctx, `
		SELECT api_key_id
		FROM user_api_keys
		WHERE user_id=$1 AND api_key_id=$2
		FOR UPDATE
	`, userID, apiKeyID).Scan(&lockedKeyID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock user key binding: %w", err)
	}
	tag, err := tx.Exec(ctx,
		`DELETE FROM user_api_keys WHERE user_id=$1 AND api_key_id=$2`,
		userID, apiKeyID,
	)
	if err != nil {
		return fmt.Errorf("unbind key from user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET default_key_id = (
			SELECT api_key_id
			FROM user_api_keys
			WHERE user_id=$1
			ORDER BY bound_at ASC
			LIMIT 1
		)
		WHERE id=$1 AND default_key_id=$2
	`, userID, apiKeyID); err != nil {
		return fmt.Errorf("repair default key: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit unbind key: %w", err)
	}
	return nil
}

func getUserByQuery(ctx context.Context, where string, args ...any) (User, error) {
	p, err := requirePool()
	if err != nil {
		return User{}, err
	}
	row := p.QueryRow(ctx, userSelectSQL(where), args...)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func userSelectSQL(where string) string {
	return `
		SELECT
			u.id, u.email, u.email_norm, u.username, u.password_hash,
			u.default_key_id, u.invited_by, u.inviter_user_id,
			u.created_at, u.last_login_at, u.disabled, u.schema_version,
			u.metadata, COALESCE(w.balance, 0), COALESCE(w.gift_balance, 0),
			COALESCE(w.total_recharged, 0), COALESCE(w.total_gifted, 0),
			COALESCE(w.version, 0),
			COALESCE(
				array_agg(uak.api_key_id ORDER BY uak.bound_at)
					FILTER (WHERE uak.api_key_id IS NOT NULL),
				ARRAY[]::text[]
			)
		FROM users u
		LEFT JOIN user_wallets w ON w.user_id = u.id
		LEFT JOIN user_api_keys uak ON uak.user_id = u.id
		WHERE ` + where + `
		GROUP BY
			u.id, u.email, u.email_norm, u.username, u.password_hash,
			u.default_key_id, u.invited_by, u.inviter_user_id,
			u.created_at, u.last_login_at, u.disabled, u.schema_version,
			u.metadata, w.balance, w.gift_balance, w.total_recharged,
			w.total_gifted, w.version`
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUsers(rows pgx.Rows) ([]User, error) {
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return out, nil
}

func scanUser(row userScanner) (User, error) {
	var u User
	var defaultKey, invitedBy, inviterUserID pgtype.Text
	var lastLogin pgtype.Timestamptz
	var metadata []byte
	var balance, gift, totalRecharged, totalGifted pgtype.Numeric
	if err := row.Scan(
		&u.ID, &u.Email, &u.EmailNorm, &u.Username, &u.PasswordHash,
		&defaultKey, &invitedBy, &inviterUserID, &u.CreatedAt, &lastLogin,
		&u.Disabled, &u.SchemaVersion, &metadata, &balance, &gift,
		&totalRecharged, &totalGifted, &u.WalletVersion, &u.APIKeyIDs,
	); err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	var err error
	u.DefaultKeyID = stringFromText(defaultKey)
	u.InvitedBy = stringFromText(invitedBy)
	u.InviterUserID = stringFromText(inviterUserID)
	u.LastLoginAt = ptrFromTimestamptz(lastLogin)
	u.Metadata, err = scanJSONMap(metadata)
	if err != nil {
		return User{}, err
	}
	if u.Balance, err = decimalFromNumeric(balance); err != nil {
		return User{}, err
	}
	if u.GiftBalance, err = decimalFromNumeric(gift); err != nil {
		return User{}, err
	}
	if u.TotalRecharged, err = decimalFromNumeric(totalRecharged); err != nil {
		return User{}, err
	}
	if u.TotalGifted, err = decimalFromNumeric(totalGifted); err != nil {
		return User{}, err
	}
	return u, nil
}

// lockApiKeyForBinding 取 api_keys row 的 row lock。
// 所有 bind/unbind 路径都必须先经此锁，保证与 lockWalletState 的锁顺序一致，
// 避免扣费事务与绑定变更交叉导致钱包打错账户。
func lockApiKeyForBinding(ctx context.Context, tx pgx.Tx, apiKeyID string) error {
	var id string
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM api_keys
		WHERE id=$1 AND deleted_at IS NULL
		FOR UPDATE
	`, apiKeyID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock api key for binding: %w", err)
	}
	return nil
}
