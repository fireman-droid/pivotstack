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

type Account struct {
	ID                 string
	Email              string
	EmailNorm          string
	UserID             string
	Nickname           string
	AccessTokenEnc     string
	RefreshTokenEnc    string
	ClientID           string
	ClientSecretEnc    string
	AuthMethod         string
	Provider           string
	Region             string
	StartURL           string
	ExpiresAt          *time.Time
	MachineID          string
	Weight             int
	Enabled            bool
	AllowOverQuota     bool
	BanStatus          string
	BanReason          string
	BanTime            *time.Time
	SubscriptionType   string
	SubscriptionTitle  string
	DaysRemaining      int
	UsageCurrent       decimal.Decimal
	UsageLimit         decimal.Decimal
	UsagePercent       decimal.Decimal
	NextResetDate      string
	LastRefresh        *time.Time
	TrialUsageCurrent  decimal.Decimal
	TrialUsageLimit    decimal.Decimal
	TrialUsagePercent  decimal.Decimal
	TrialStatus        string
	TrialExpiresAt     *time.Time
	RequestCount       int64
	ErrorCount         int64
	LastUsed           *time.Time
	TotalTokens        int64
	TotalCredits       decimal.Decimal
	DeletedAt          *time.Time
	Metadata           map[string]any
}

func InsertAccount(ctx context.Context, tx pgx.Tx, a Account) error {
	if tx == nil {
		return errors.New("insert account requires transaction")
	}
	prepareAccount(&a)
	meta, err := jsonObjectParam(a.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO accounts (
			id, email, email_norm, user_id, nickname, access_token_enc,
			refresh_token_enc, client_id, client_secret_enc, auth_method,
			provider, region, start_url, expires_at, machine_id, weight,
			enabled, allow_over_quota, ban_status, ban_reason, ban_time,
			subscription_type, subscription_title, days_remaining,
			usage_current, usage_limit, usage_percent, next_reset_date,
			last_refresh, trial_usage_current, trial_usage_limit,
			trial_usage_percent, trial_status, trial_expires_at,
			request_count, error_count, last_used, total_tokens,
			total_credits, deleted_at, metadata
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,
			$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,
			$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,$41
		)
	`, a.ID, textFromString(a.Email), textFromString(a.EmailNorm), textFromString(a.UserID),
		textFromString(a.Nickname), textFromString(a.AccessTokenEnc), textFromString(a.RefreshTokenEnc),
		textFromString(a.ClientID), textFromString(a.ClientSecretEnc), a.AuthMethod,
		textFromString(a.Provider), textFromString(a.Region), textFromString(a.StartURL),
		timestamptzFromPtr(a.ExpiresAt), textFromString(a.MachineID), a.Weight, a.Enabled,
		a.AllowOverQuota, textFromString(a.BanStatus), textFromString(a.BanReason),
		timestamptzFromPtr(a.BanTime), textFromString(a.SubscriptionType),
		textFromString(a.SubscriptionTitle), a.DaysRemaining, numericFromDecimal(a.UsageCurrent),
		numericFromDecimal(a.UsageLimit), numericFromDecimal(a.UsagePercent),
		textFromString(a.NextResetDate), timestamptzFromPtr(a.LastRefresh),
		numericFromDecimal(a.TrialUsageCurrent), numericFromDecimal(a.TrialUsageLimit),
		numericFromDecimal(a.TrialUsagePercent), textFromString(a.TrialStatus),
		timestamptzFromPtr(a.TrialExpiresAt), a.RequestCount, a.ErrorCount,
		timestamptzFromPtr(a.LastUsed), a.TotalTokens, numericFromDecimal(a.TotalCredits),
		timestamptzFromPtr(a.DeletedAt), meta)
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

func UpsertAccountByIdentity(ctx context.Context, a Account) (string, bool, error) {
	p, err := requirePool()
	if err != nil {
		return "", false, err
	}
	prepareAccount(&a)
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", false, fmt.Errorf("begin account upsert: %w", err)
	}
	defer tx.Rollback(ctx)

	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM accounts
		WHERE deleted_at IS NULL
			AND email_norm=$1
			AND coalesce(auth_method, '')=$2
			AND coalesce(provider, '')=coalesce($3, '')
		FOR UPDATE
	`, a.EmailNorm, a.AuthMethod, textFromString(a.Provider)).Scan(&existingID)
	if err == nil {
		a.ID = existingID
		if err := updateAccountTx(ctx, tx, a); err != nil {
			return "", false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", false, fmt.Errorf("commit account upsert: %w", err)
		}
		return existingID, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("lookup account identity: %w", err)
	}
	if err := InsertAccount(ctx, tx, a); err != nil {
		return "", false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false, fmt.Errorf("commit account upsert: %w", err)
	}
	return a.ID, true, nil
}

func GetAccount(ctx context.Context, id string) (Account, error) {
	p, err := requirePool()
	if err != nil {
		return Account{}, err
	}
	a, err := scanAccount(p.QueryRow(ctx, accountSelectSQL(`id=$1`), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return a, err
}

func ListAccounts(ctx context.Context, includeDeleted bool) ([]Account, error) {
	p, err := requirePool()
	if err != nil {
		return nil, err
	}
	where := `deleted_at IS NULL`
	if includeDeleted {
		where = `TRUE`
	}
	rows, err := p.Query(ctx, accountSelectSQL(where)+` ORDER BY weight DESC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}
	return out, nil
}

func UpdateAccount(ctx context.Context, a Account) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	prepareAccount(&a)
	tag, err := p.Exec(ctx, accountUpdateSQL(), accountArgs(a)...)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SoftDeleteAccount(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `UPDATE accounts SET deleted_at=now(), enabled=false WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func IncrementAccountUsage(ctx context.Context, id string, requests, accountErrors, tokens int64, credits decimal.Decimal) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE accounts
		SET request_count=request_count+$2, error_count=error_count+$3,
			total_tokens=total_tokens+$4, total_credits=total_credits+$5,
			last_used=now()
		WHERE id=$1 AND deleted_at IS NULL
	`, id, requests, accountErrors, tokens, numericFromDecimal(credits))
	if err != nil {
		return fmt.Errorf("increment account usage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func SetAccountBan(ctx context.Context, id, status, reason string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE accounts
		SET enabled=false, ban_status=$2, ban_reason=$3, ban_time=now()
		WHERE id=$1 AND deleted_at IS NULL
	`, id, textFromString(status), textFromString(reason))
	if err != nil {
		return fmt.Errorf("set account ban: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func ClearAccountBan(ctx context.Context, id string) error {
	p, err := requirePool()
	if err != nil {
		return err
	}
	tag, err := p.Exec(ctx, `
		UPDATE accounts
		SET enabled=true, ban_status='ACTIVE', ban_reason=NULL, ban_time=NULL
		WHERE id=$1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("clear account ban: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func updateAccountTx(ctx context.Context, tx pgx.Tx, a Account) error {
	tag, err := tx.Exec(ctx, accountUpdateSQL(), accountArgs(a)...)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func prepareAccount(a *Account) {
	a.Email = strings.TrimSpace(a.Email)
	a.EmailNorm = normalizeEmail(a.EmailNorm)
	if a.EmailNorm == "" {
		a.EmailNorm = normalizeEmail(a.Email)
	}
	a.AuthMethod = strings.ToLower(strings.TrimSpace(a.AuthMethod))
	if a.AuthMethod == "" {
		a.AuthMethod = "idc"
	}
	a.Provider = strings.ToLower(strings.TrimSpace(a.Provider))
	a.Region = strings.TrimSpace(a.Region)
	a.StartURL = strings.TrimSpace(a.StartURL)
}

func accountUpdateSQL() string {
	return `
		UPDATE accounts
		SET email=$2, email_norm=$3, user_id=$4, nickname=$5,
			access_token_enc=$6, refresh_token_enc=$7, client_id=$8,
			client_secret_enc=$9, auth_method=$10, provider=$11, region=$12,
			start_url=$13, expires_at=$14, machine_id=$15, weight=$16,
			enabled=$17, allow_over_quota=$18, ban_status=$19, ban_reason=$20,
			ban_time=$21, subscription_type=$22, subscription_title=$23,
			days_remaining=$24, usage_current=$25, usage_limit=$26,
			usage_percent=$27, next_reset_date=$28, last_refresh=$29,
			trial_usage_current=$30, trial_usage_limit=$31,
			trial_usage_percent=$32, trial_status=$33, trial_expires_at=$34,
			request_count=$35, error_count=$36, last_used=$37,
			total_tokens=$38, total_credits=$39, deleted_at=$40, metadata=$41
		WHERE id=$1`
}

func accountArgs(a Account) []any {
	meta, _ := jsonObjectParam(a.Metadata)
	return []any{
		a.ID, textFromString(a.Email), textFromString(a.EmailNorm), textFromString(a.UserID),
		textFromString(a.Nickname), textFromString(a.AccessTokenEnc), textFromString(a.RefreshTokenEnc),
		textFromString(a.ClientID), textFromString(a.ClientSecretEnc), a.AuthMethod,
		textFromString(a.Provider), textFromString(a.Region), textFromString(a.StartURL),
		timestamptzFromPtr(a.ExpiresAt), textFromString(a.MachineID), a.Weight, a.Enabled,
		a.AllowOverQuota, textFromString(a.BanStatus), textFromString(a.BanReason),
		timestamptzFromPtr(a.BanTime), textFromString(a.SubscriptionType),
		textFromString(a.SubscriptionTitle), a.DaysRemaining, numericFromDecimal(a.UsageCurrent),
		numericFromDecimal(a.UsageLimit), numericFromDecimal(a.UsagePercent),
		textFromString(a.NextResetDate), timestamptzFromPtr(a.LastRefresh),
		numericFromDecimal(a.TrialUsageCurrent), numericFromDecimal(a.TrialUsageLimit),
		numericFromDecimal(a.TrialUsagePercent), textFromString(a.TrialStatus),
		timestamptzFromPtr(a.TrialExpiresAt), a.RequestCount, a.ErrorCount,
		timestamptzFromPtr(a.LastUsed), a.TotalTokens, numericFromDecimal(a.TotalCredits),
		timestamptzFromPtr(a.DeletedAt), meta,
	}
}

func accountSelectSQL(where string) string {
	return `
		SELECT id, email, email_norm, user_id, nickname, access_token_enc,
			refresh_token_enc, client_id, client_secret_enc, auth_method,
			provider, region, start_url, expires_at, machine_id, weight,
			enabled, allow_over_quota, ban_status, ban_reason, ban_time,
			subscription_type, subscription_title, days_remaining,
			usage_current, usage_limit, usage_percent, next_reset_date,
			last_refresh, trial_usage_current, trial_usage_limit,
			trial_usage_percent, trial_status, trial_expires_at,
			request_count, error_count, last_used, total_tokens,
			total_credits, deleted_at, metadata
		FROM accounts
		WHERE ` + where
}

type accountScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row accountScanner) (Account, error) {
	var a Account
	var email, emailNorm, userID, nickname, accessToken, refreshToken pgtype.Text
	var clientID, clientSecret, provider, region, startURL, machineID pgtype.Text
	var banStatus, banReason, subType, subTitle, nextResetDate, trialStatus pgtype.Text
	var expiresAt, banTime, lastRefresh, trialExpiresAt, lastUsed, deletedAt pgtype.Timestamptz
	var usageCurrent, usageLimit, usagePercent, trialUsageCurrent, trialUsageLimit pgtype.Numeric
	var trialUsagePercent, totalCredits pgtype.Numeric
	var metadata []byte
	if err := row.Scan(
		&a.ID, &email, &emailNorm, &userID, &nickname, &accessToken,
		&refreshToken, &clientID, &clientSecret, &a.AuthMethod, &provider,
		&region, &startURL, &expiresAt, &machineID, &a.Weight, &a.Enabled,
		&a.AllowOverQuota, &banStatus, &banReason, &banTime, &subType,
		&subTitle, &a.DaysRemaining, &usageCurrent, &usageLimit,
		&usagePercent, &nextResetDate, &lastRefresh, &trialUsageCurrent,
		&trialUsageLimit, &trialUsagePercent, &trialStatus, &trialExpiresAt,
		&a.RequestCount, &a.ErrorCount, &lastUsed, &a.TotalTokens,
		&totalCredits, &deletedAt, &metadata,
	); err != nil {
		return Account{}, fmt.Errorf("scan account: %w", err)
	}
	var err error
	a.Email = stringFromText(email)
	a.EmailNorm = stringFromText(emailNorm)
	a.UserID = stringFromText(userID)
	a.Nickname = stringFromText(nickname)
	a.AccessTokenEnc = stringFromText(accessToken)
	a.RefreshTokenEnc = stringFromText(refreshToken)
	a.ClientID = stringFromText(clientID)
	a.ClientSecretEnc = stringFromText(clientSecret)
	a.Provider = stringFromText(provider)
	a.Region = stringFromText(region)
	a.StartURL = stringFromText(startURL)
	a.ExpiresAt = ptrFromTimestamptz(expiresAt)
	a.MachineID = stringFromText(machineID)
	a.BanStatus = stringFromText(banStatus)
	a.BanReason = stringFromText(banReason)
	a.BanTime = ptrFromTimestamptz(banTime)
	a.SubscriptionType = stringFromText(subType)
	a.SubscriptionTitle = stringFromText(subTitle)
	a.NextResetDate = stringFromText(nextResetDate)
	a.LastRefresh = ptrFromTimestamptz(lastRefresh)
	a.TrialStatus = stringFromText(trialStatus)
	a.TrialExpiresAt = ptrFromTimestamptz(trialExpiresAt)
	a.LastUsed = ptrFromTimestamptz(lastUsed)
	a.DeletedAt = ptrFromTimestamptz(deletedAt)
	if a.UsageCurrent, err = decimalFromNumeric(usageCurrent); err != nil {
		return Account{}, err
	}
	if a.UsageLimit, err = decimalFromNumeric(usageLimit); err != nil {
		return Account{}, err
	}
	if a.UsagePercent, err = decimalFromNumeric(usagePercent); err != nil {
		return Account{}, err
	}
	if a.TrialUsageCurrent, err = decimalFromNumeric(trialUsageCurrent); err != nil {
		return Account{}, err
	}
	if a.TrialUsageLimit, err = decimalFromNumeric(trialUsageLimit); err != nil {
		return Account{}, err
	}
	if a.TrialUsagePercent, err = decimalFromNumeric(trialUsagePercent); err != nil {
		return Account{}, err
	}
	if a.TotalCredits, err = decimalFromNumeric(totalCredits); err != nil {
		return Account{}, err
	}
	a.Metadata, err = scanJSONMap(metadata)
	return a, err
}
