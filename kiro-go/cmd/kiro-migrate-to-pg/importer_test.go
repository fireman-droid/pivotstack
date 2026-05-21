package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/db"
	"kiro-api-proxy/users"

	"github.com/google/uuid"
)

func TestRunApply_Smoke(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := db.InitPool(ctx, dsn); err != nil {
		t.Fatalf("init pool: %v", err)
	}
	defer db.Close()

	// 必须 require encryption key 否则 EncryptSecret 会失败
	if err := os.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "test-key-for-migrate-tool-32bytes!!!"); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	keyID := "key_" + uuid.NewString()
	userID := "usr_" + uuid.NewString()

	cfg := config.Config{
		ApiKeys: []config.ApiKeyInfo{
			{
				ID: keyID, Key: "sk-test-" + uuid.NewString(),
				Plan: "credit", Enabled: true, CreatedAt: time.Now().Unix(),
				Balance: 1.5, GiftBalance: 0.5,
			},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(tmp, "config.json"), cfgJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	uf := users.UsersFile{
		SchemaVersion: 3,
		Users: []users.User{
			{
				ID: userID, Email: "test+" + userID + "@example.com",
				Username: "testuser_" + userID[:8],
				PasswordHash: "$2a$10$dummyhashfortest1234567890123456",
				ApiKeyIDs: []string{keyID}, DefaultKeyID: keyID,
				CreatedAt: time.Now().Unix(),
				Balance:   10.0, GiftBalance: 2.0,
			},
		},
	}
	ufJSON, _ := json.Marshal(uf)
	if err := os.WriteFile(filepath.Join(tmp, "users.json"), ufJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	rechargeLine := map[string]any{
		"time": "05-21 00:00:00", "timestamp": time.Now().Unix(),
		"key_id": keyID, "type": "code_redeem", "code": "TESTCODE",
		"amount_usd": 5.0, "amount_cny": 100.0,
		"balance_before": 0.0, "balance_after": 5.0,
		"gift_before": 0.0, "gift_after": 0.0,
		"operator": "test",
	}
	rechargeJSON, _ := json.Marshal(rechargeLine)
	if err := os.WriteFile(filepath.Join(tmp, "recharge_records.jsonl"), append(rechargeJSON, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	callLogLine := map[string]any{
		"time": "05-21 00:00:00", "timestamp": time.Now().Unix(),
		"request_id": "req_" + uuid.NewString(),
		"api_type":   "chat", "original_model": "claude", "actual_model": "claude",
		"account": "acc1", "api_key_id": keyID,
		"input_tokens": 100, "output_tokens": 200, "total_tokens": 300,
		"credits": 0.5, "paid_credits": 0.4, "gifted_credits": 0.1,
		"cost_usd": 0.05, "charged_usd": 0.5,
		"status": "success", "stream": false,
	}
	callLogJSON, _ := json.Marshal(callLogLine)
	if err := os.WriteFile(filepath.Join(tmp, "call_logs.jsonl"), append(callLogJSON, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := runApply(ctx, tmp)
	if err != nil {
		t.Fatalf("runApply: %v", err)
	}
	stats := map[string]ImportStats{}
	for _, s := range result.Stats {
		stats[s.Table] = s
	}
	if stats["api_keys"].Inserted != 1 {
		t.Fatalf("api_keys inserted = %d", stats["api_keys"].Inserted)
	}
	if stats["users"].Inserted != 1 {
		t.Fatalf("users inserted = %d", stats["users"].Inserted)
	}
	if stats["recharge_records"].Inserted != 1 {
		t.Fatalf("recharges inserted = %d", stats["recharge_records"].Inserted)
	}
	if stats["call_logs"].Inserted != 1 {
		t.Fatalf("call_logs inserted = %d", stats["call_logs"].Inserted)
	}

	// 第二次 apply：应当全部 duplicates
	result2, err := runApply(ctx, tmp)
	if err != nil {
		t.Fatalf("runApply 2: %v", err)
	}
	statsByTable := map[string]ImportStats{}
	for _, s := range result2.Stats {
		statsByTable[s.Table] = s
	}
	if statsByTable["api_keys"].Inserted != 0 || statsByTable["api_keys"].Duplicates != 1 {
		t.Fatalf("api_keys idempotency: %+v", statsByTable["api_keys"])
	}
	if statsByTable["recharge_records"].Inserted != 0 {
		t.Fatalf("recharges not idempotent: %+v", statsByTable["recharge_records"])
	}

	// 清理
	pool := db.Pool()
	_, _ = pool.Exec(ctx, `DELETE FROM call_logs WHERE api_key_id=$1`, keyID)
	_, _ = pool.Exec(ctx, `DELETE FROM recharge_records WHERE api_key_id=$1`, keyID)
	_, _ = pool.Exec(ctx, `DELETE FROM user_api_keys WHERE api_key_id=$1`, keyID)
	_, _ = pool.Exec(ctx, `DELETE FROM user_wallets WHERE user_id=$1`, userID)
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	_, _ = pool.Exec(ctx, `DELETE FROM api_keys WHERE id=$1`, keyID)
	_, _ = pool.Exec(ctx, `DELETE FROM migration_imports WHERE legacy_id IN ($1, $2)`, keyID, userID)
}
