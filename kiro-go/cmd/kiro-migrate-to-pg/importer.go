package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/db"
	"kiro-api-proxy/users"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type ImportStats struct {
	Table      string
	Rows       int
	Inserted   int
	Duplicates int
	Failed     int
}

type ApplyResult struct {
	Stats []ImportStats
}

func runApply(ctx context.Context, sourceDir string) (ApplyResult, error) {
	res := ApplyResult{}

	// 1) users + api_keys
	userStats, keyStats, err := importUsersAndKeys(ctx, sourceDir)
	if err != nil {
		return res, fmt.Errorf("import users+keys: %w", err)
	}
	res.Stats = append(res.Stats, keyStats, userStats)

	// 2) recharges
	rechargeStats, err := importRecharges(ctx, sourceDir)
	if err != nil {
		return res, fmt.Errorf("import recharges: %w", err)
	}
	res.Stats = append(res.Stats, rechargeStats)

	// 3) call_logs
	callStats, err := importCallLogs(ctx, sourceDir)
	if err != nil {
		return res, fmt.Errorf("import call_logs: %w", err)
	}
	res.Stats = append(res.Stats, callStats)

	return res, nil
}

func importUsersAndKeys(ctx context.Context, sourceDir string) (userStats, keyStats ImportStats, err error) {
	userStats = ImportStats{Table: "users"}
	keyStats = ImportStats{Table: "api_keys"}

	// Read users.json
	usersPath := filepath.Join(sourceDir, "users.json")
	usersFile, err := readUsersFile(usersPath)
	if err != nil && !os.IsNotExist(err) {
		return userStats, keyStats, err
	}

	// Read config.json apiKeys
	cfg, err := readConfigFile(filepath.Join(sourceDir, "config.json"))
	if err != nil && !os.IsNotExist(err) {
		return userStats, keyStats, err
	}

	keyByID := map[string]config.ApiKeyInfo{}
	for _, k := range cfg.ApiKeys {
		keyByID[k.ID] = k
	}

	pool, err := requirePool()
	if err != nil {
		return userStats, keyStats, err
	}

	// 先全部 api_keys，再全部 users（保证 user_api_keys FK 不缺）
	for _, k := range cfg.ApiKeys {
		keyStats.Rows++
		ok, e := insertLegacyApiKey(ctx, pool, k)
		if e != nil {
			keyStats.Failed++
			return userStats, keyStats, fmt.Errorf("api_key %s: %w", k.ID, e)
		}
		if ok {
			keyStats.Inserted++
		} else {
			keyStats.Duplicates++
		}
	}

	for _, u := range usersFile.Users {
		userStats.Rows++
		ok, e := insertLegacyUser(ctx, pool, u)
		if e != nil {
			userStats.Failed++
			return userStats, keyStats, fmt.Errorf("user %s: %w", u.ID, e)
		}
		if ok {
			userStats.Inserted++
		} else {
			userStats.Duplicates++
		}
	}

	return userStats, keyStats, nil
}

func insertLegacyApiKey(ctx context.Context, pool pgxPool, k config.ApiKeyInfo) (bool, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	// 幂等检查：已有 row 则跳过。
	if exists, e := apiKeyExists(ctx, tx, k.ID); e != nil {
		return false, e
	} else if exists {
		return false, nil
	}

	// 加密原 key + 计算 key_hash。
	cipherText, e := config.EncryptSecret(k.Key)
	if e != nil {
		return false, fmt.Errorf("encrypt api key: %w", e)
	}
	hash := sha256.Sum256([]byte(k.Key))

	expires := timestampPtr(k.ExpiresAt)
	lastUsed := timestampPtr(k.LastUsed)

	row := db.ApiKey{
		ID:                 k.ID,
		KeyHash:            hash[:],
		KeyCiphertext:      cipherText,
		Tier:               k.Tier,
		Plan:               valueOrDefault(k.Plan, "credit"),
		ExpiresAt:          expires,
		Enabled:            k.Enabled,
		Balance:            decimal.NewFromFloat(k.Balance),
		GiftBalance:        decimal.NewFromFloat(k.GiftBalance),
		TotalRecharged:     decimal.NewFromFloat(k.TotalRecharged),
		TotalGifted:        decimal.NewFromFloat(k.TotalGifted),
		Note:               k.Note,
		CreatedAt:          unixToTime(k.CreatedAt),
		LastUsed:           lastUsed,
		Requests:           k.Requests,
		Errors:             k.Errors,
		Tokens:             k.Tokens,
		Credits:            decimal.NewFromFloat(k.Credits),
		Models:             k.Models,
		ParentKeyID:        k.ParentKeyID,
		IsReseller:         k.IsReseller,
		MaxChildKeys:       k.MaxChildKeys,
		ResellerDiscount:   decimal.NewFromFloat(k.ResellerDiscount),
		SoldToChildren:     decimal.NewFromFloat(k.SoldToChildren),
		RateLimitPerMin:    k.RateLimitPerMin,
		SeriesPreferences:  map[string]any{},
		ChannelPreferences: map[string]any{},
		DebtUSD:            decimal.Zero,
		Metadata:           map[string]any{"imported_from": "legacy_config_json"},
	}
	if err := db.InsertApiKey(ctx, tx, row); err != nil {
		return false, fmt.Errorf("db.InsertApiKey: %w", err)
	}
	if _, err := db.RecordImport(ctx, tx, "config.apiKeys", k.ID, hashPayload(k)); err != nil {
		return false, fmt.Errorf("record migration: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func insertLegacyUser(ctx context.Context, pool pgxPool, u users.User) (bool, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	if exists, e := userExists(ctx, tx, u.ID); e != nil {
		return false, e
	} else if exists {
		return false, nil
	}

	row := db.User{
		ID:             u.ID,
		Email:          u.Email,
		EmailNorm:      users.NormalizeEmail(u.Email),
		Username:       valueOrDefault(u.Username, u.ID),
		PasswordHash:   u.PasswordHash,
		DefaultKeyID:   u.DefaultKeyID,
		InvitedBy:      u.InvitedBy,
		InviterUserID:  u.InviterUserID,
		CreatedAt:      unixToTime(u.CreatedAt),
		LastLoginAt:    timestampPtr(u.LastLoginAt),
		Disabled:       u.Disabled,
		SchemaVersion:  users.CurrentSchemaVersion,
		Metadata:       map[string]any{"imported_from": "legacy_users_json"},
		APIKeyIDs:      u.ApiKeyIDs,
		Balance:        decimal.NewFromFloat(u.Balance),
		GiftBalance:    decimal.NewFromFloat(u.GiftBalance),
		TotalRecharged: decimal.NewFromFloat(u.TotalRecharged),
		TotalGifted:    decimal.NewFromFloat(u.TotalGifted),
	}
	if err := db.InsertUser(ctx, tx, row); err != nil {
		return false, fmt.Errorf("db.InsertUser: %w", err)
	}
	if _, err := db.RecordImport(ctx, tx, "users.users", u.ID, hashPayload(u)); err != nil {
		return false, fmt.Errorf("record migration: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func importRecharges(ctx context.Context, sourceDir string) (ImportStats, error) {
	stats := ImportStats{Table: "recharge_records"}
	path := filepath.Join(sourceDir, "recharge_records.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		stats.Rows++
		var rec legacyRecharge
		if err := json.Unmarshal(line, &rec); err != nil {
			stats.Failed++
			return stats, fmt.Errorf("recharge line %d: %w", lineNo, err)
		}
		legacyID := legacyJSONLID("recharge_records.jsonl", lineNo, line)
		row := rechargeRowFromLegacy(rec, legacyID)
		ok, e := db.InsertRecharge(ctx, row)
		if e != nil {
			stats.Failed++
			return stats, fmt.Errorf("recharge line %d insert: %w", lineNo, e)
		}
		if ok {
			stats.Inserted++
		} else {
			stats.Duplicates++
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, err
	}
	return stats, nil
}

func importCallLogs(ctx context.Context, sourceDir string) (ImportStats, error) {
	stats := ImportStats{Table: "call_logs"}
	path := filepath.Join(sourceDir, "call_logs.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, err
	}
	defer f.Close()

	ensuredMonths := map[string]bool{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		stats.Rows++
		var rec legacyCallLog
		if err := json.Unmarshal(line, &rec); err != nil {
			stats.Failed++
			return stats, fmt.Errorf("call_log line %d: %w", lineNo, err)
		}
		legacyID := legacyJSONLID("call_logs.jsonl", lineNo, line)
		row := callLogRowFromLegacy(rec, legacyID)
		monthKey := row.OccurredAt.UTC().Format("2006-01")
		if !ensuredMonths[monthKey] {
			if err := db.EnsureCallLogsPartition(ctx, row.OccurredAt); err != nil {
				stats.Failed++
				return stats, fmt.Errorf("ensure partition %s: %w", monthKey, err)
			}
			ensuredMonths[monthKey] = true
		}
		ok, e := db.InsertCallLog(ctx, row)
		if e != nil {
			stats.Failed++
			return stats, fmt.Errorf("call_log line %d insert: %w", lineNo, e)
		}
		if ok {
			stats.Inserted++
		} else {
			stats.Duplicates++
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, err
	}
	return stats, nil
}

// ----- helpers -----

type legacyRecharge struct {
	Time          string  `json:"time"`
	Timestamp     int64   `json:"timestamp"`
	KeyID         string  `json:"key_id"`
	KeyNote       string  `json:"key_note,omitempty"`
	Type          string  `json:"type"`
	Code          string  `json:"code,omitempty"`
	AmountUSD     float64 `json:"amount_usd"`
	AmountCNY     float64 `json:"amount_cny"`
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	GiftBefore    float64 `json:"gift_before"`
	GiftAfter     float64 `json:"gift_after"`
	Operator      string  `json:"operator"`
	Note          string  `json:"note,omitempty"`
	IP            string  `json:"ip,omitempty"`
}

type legacyCallLog struct {
	Time            string  `json:"time"`
	Timestamp       int64   `json:"timestamp"`
	RequestID       string  `json:"request_id,omitempty"`
	APIType         string  `json:"api_type"`
	OriginalModel   string  `json:"original_model"`
	ActualModel     string  `json:"actual_model"`
	Account         string  `json:"account"`
	ApiKeyID        string  `json:"api_key_id,omitempty"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	Credits         float64 `json:"credits,omitempty"`
	UpstreamCredits float64 `json:"upstream_credits,omitempty"`
	PaidCredits     float64 `json:"paid_credits"`
	GiftedCredits   float64 `json:"gifted_credits"`
	CostUSD         float64 `json:"cost_usd"`
	ChargedUSD      float64 `json:"charged_usd,omitempty"`
	CostUSDLegacy   float64 `json:"cost_usd_legacy,omitempty"`
	PriceModel      string  `json:"price_model,omitempty"`
	Stream          bool    `json:"stream"`
	Error           string  `json:"error,omitempty"`
	PayloadKB       int     `json:"payload_kb,omitempty"`
	Status          string  `json:"status"`
	StopReason      string  `json:"stop_reason,omitempty"`
	DurationMs      int64   `json:"duration_ms,omitempty"`
	Attempt         int     `json:"attempt,omitempty"`
	Subscription    string  `json:"subscription,omitempty"`
	RequestSummary  string  `json:"request_summary,omitempty"`
	ResponseSummary string  `json:"response_summary,omitempty"`
	ChannelID       string  `json:"channel_id,omitempty"`
	ChannelType     string  `json:"channel_type,omitempty"`
	BillingMode     string  `json:"billing_mode,omitempty"`
	BillingStatus   string  `json:"billing_status,omitempty"`
	UsageEstimated  bool    `json:"usage_estimated,omitempty"`
}

func rechargeRowFromLegacy(r legacyRecharge, legacyID string) db.RechargeRecordRow {
	ts := unixToTime(r.Timestamp)
	return db.RechargeRecordRow{
		ID:            legacyID,
		TimeLabel:     r.Time,
		TimestampUnix: r.Timestamp,
		OccurredAt:    ts,
		DayCST:        db.ComputeDayCST(ts),
		APIKeyID:      r.KeyID,
		UserID:        "",
		KeyNote:       r.KeyNote,
		Type:          r.Type,
		Code:          r.Code,
		AmountUSD:     decimal.NewFromFloat(r.AmountUSD),
		AmountCNY:     decimal.NewFromFloat(r.AmountCNY),
		BalanceBefore: decimal.NewFromFloat(r.BalanceBefore),
		BalanceAfter:  decimal.NewFromFloat(r.BalanceAfter),
		GiftBefore:    decimal.NewFromFloat(r.GiftBefore),
		GiftAfter:     decimal.NewFromFloat(r.GiftAfter),
		Operator:      r.Operator,
		Note:          r.Note,
		IP:            r.IP,
		RawPayload:    map[string]any{"legacy_id": legacyID},
	}
}

func callLogRowFromLegacy(r legacyCallLog, legacyID string) db.CallLogRow {
	ts := unixToTime(r.Timestamp)
	return db.CallLogRow{
		ID:              legacyID,
		OccurredAt:      ts,
		TimestampUnix:   r.Timestamp,
		DayCST:          db.ComputeDayCST(ts),
		TimeLabel:       r.Time,
		RequestID:       r.RequestID,
		APIType:         valueOrDefault(r.APIType, "unknown"),
		OriginalModel:   r.OriginalModel,
		ActualModel:     r.ActualModel,
		Account:         r.Account,
		APIKeyID:        r.ApiKeyID,
		InputTokens:     r.InputTokens,
		OutputTokens:    r.OutputTokens,
		TotalTokens:     r.TotalTokens,
		Credits:         decimal.NewFromFloat(r.Credits),
		UpstreamCredits: decimal.NewFromFloat(r.UpstreamCredits),
		PaidCredits:     decimal.NewFromFloat(r.PaidCredits),
		GiftedCredits:   decimal.NewFromFloat(r.GiftedCredits),
		CostUSD:         decimal.NewFromFloat(r.CostUSD),
		ChargedUSD:      decimal.NewFromFloat(r.ChargedUSD),
		CostUSDLegacy:   decimal.NewFromFloat(r.CostUSDLegacy),
		PriceModel:      r.PriceModel,
		Stream:          r.Stream,
		Error:           r.Error,
		PayloadKB:       r.PayloadKB,
		Status:          valueOrDefault(r.Status, "unknown"),
		StopReason:      r.StopReason,
		DurationMS:      r.DurationMs,
		Attempt:         r.Attempt,
		Subscription:    r.Subscription,
		RequestSummary:  r.RequestSummary,
		ResponseSummary: r.ResponseSummary,
		ChannelID:       r.ChannelID,
		ChannelType:     r.ChannelType,
		BillingMode:     r.BillingMode,
		BillingStatus:   r.BillingStatus,
		UsageEstimated:  r.UsageEstimated,
		RawPayload:      map[string]any{"legacy_id": legacyID},
	}
}

func legacyJSONLID(file string, lineNo int, payload []byte) string {
	sum := sha256.Sum256(payload)
	prefix := hex.EncodeToString(sum[:6])
	return "legacy:" + file + ":" + strconv.Itoa(lineNo) + ":" + prefix
}

func hashPayload(v any) string {
	data, _ := json.Marshal(v)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func unixToTime(unix int64) time.Time {
	if unix == 0 {
		return time.Time{}
	}
	return time.Unix(unix, 0).UTC()
}

func timestampPtr(unix int64) *time.Time {
	if unix == 0 {
		return nil
	}
	t := time.Unix(unix, 0).UTC()
	return &t
}

func valueOrDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}

func readUsersFile(path string) (users.UsersFile, error) {
	var f users.UsersFile
	data, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	if len(data) == 0 {
		return f, errors.New("empty users.json")
	}
	err = json.Unmarshal(data, &f)
	return f, err
}

func readConfigFile(path string) (config.Config, error) {
	var c config.Config
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if len(data) == 0 {
		return c, errors.New("empty config.json")
	}
	err = json.Unmarshal(data, &c)
	return c, err
}

// pgx pool interface narrowed to what we use to make tests easier later.
type pgxPool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func requirePool() (pgxPool, error) {
	p := db.Pool()
	if p == nil {
		return nil, errors.New("postgres pool is not initialized")
	}
	return p, nil
}

func apiKeyExists(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	var v int
	err := tx.QueryRow(ctx, `SELECT 1 FROM api_keys WHERE id=$1`, id).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func userExists(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	var v int
	err := tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id=$1`, id).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// avoid unused-import error.
var _ = io.Discard
