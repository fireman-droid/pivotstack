package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"kiro-api-proxy/db"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

// VerifyReport JSON 与 PG 端的对账结果。
type VerifyReport struct {
	Users     CountSumDelta `json:"users"`
	APIKeys   CountSumDelta `json:"api_keys"`
	Recharges CountSumDelta `json:"recharges"`
	CallLogs  CountDelta    `json:"call_logs"`
}

type CountSumDelta struct {
	JSONCount int     `json:"json_count"`
	PGCount   int     `json:"pg_count"`
	JSONSum   float64 `json:"json_sum,omitempty"`
	PGSum     float64 `json:"pg_sum,omitempty"`
	Drift     float64 `json:"drift,omitempty"`
}

type CountDelta struct {
	JSONCount int `json:"json_count"`
	PGCount   int `json:"pg_count"`
}

const moneyTolerance = 0.001

func (r VerifyReport) OK() bool {
	if r.Users.JSONCount != r.Users.PGCount {
		return false
	}
	if r.APIKeys.JSONCount != r.APIKeys.PGCount {
		return false
	}
	if r.Recharges.JSONCount != r.Recharges.PGCount {
		return false
	}
	if r.CallLogs.JSONCount != r.CallLogs.PGCount {
		return false
	}
	if math.Abs(r.Users.JSONSum-r.Users.PGSum) > moneyTolerance {
		return false
	}
	if math.Abs(r.APIKeys.JSONSum-r.APIKeys.PGSum) > moneyTolerance {
		return false
	}
	if math.Abs(r.Recharges.JSONSum-r.Recharges.PGSum) > moneyTolerance {
		return false
	}
	return true
}

func dryRunReport(sourceDir string) (VerifyReport, error) {
	rep := VerifyReport{}
	uf, _ := readUsersFile(filepath.Join(sourceDir, "users.json"))
	rep.Users.JSONCount = len(uf.Users)
	for _, u := range uf.Users {
		rep.Users.JSONSum += u.Balance + u.GiftBalance
	}
	cfg, _ := readConfigFile(filepath.Join(sourceDir, "config.json"))
	rep.APIKeys.JSONCount = len(cfg.ApiKeys)
	for _, k := range cfg.ApiKeys {
		// 只统计 orphan + reseller child 钱包总和（plan §4 校验）
		if k.ParentKeyID != "" {
			rep.APIKeys.JSONSum += k.Balance + k.GiftBalance
		}
	}
	rcount, rsum, err := scanRechargeJSONL(filepath.Join(sourceDir, "recharge_records.jsonl"))
	if err != nil {
		return rep, err
	}
	rep.Recharges.JSONCount = rcount
	rep.Recharges.JSONSum = rsum
	ccount, err := scanCallLogJSONL(filepath.Join(sourceDir, "call_logs.jsonl"))
	if err != nil {
		return rep, err
	}
	rep.CallLogs.JSONCount = ccount
	return rep, nil
}

func compareJSONvsPG(ctx context.Context, sourceDir string) (VerifyReport, error) {
	rep, err := dryRunReport(sourceDir)
	if err != nil {
		return rep, err
	}
	pool := db.Pool()
	if pool == nil {
		return rep, fmt.Errorf("postgres pool not initialized")
	}

	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&rep.Users.PGCount); err != nil {
		return rep, fmt.Errorf("count users: %w", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(sum(balance+gift_balance), 0) FROM user_wallets
	`).Scan(&rep.Users.PGSum); err != nil {
		return rep, fmt.Errorf("sum user wallets: %w", err)
	}
	rep.Users.Drift = math.Abs(rep.Users.JSONSum - rep.Users.PGSum)

	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE deleted_at IS NULL`).Scan(&rep.APIKeys.PGCount); err != nil {
		return rep, fmt.Errorf("count api keys: %w", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(sum(balance+gift_balance), 0)
		FROM api_keys
		WHERE deleted_at IS NULL AND parent_key_id IS NOT NULL
	`).Scan(&rep.APIKeys.PGSum); err != nil {
		return rep, fmt.Errorf("sum api keys: %w", err)
	}
	rep.APIKeys.Drift = math.Abs(rep.APIKeys.JSONSum - rep.APIKeys.PGSum)

	var rechargeSumNum pgScanFloat
	if err := pool.QueryRow(ctx, `SELECT count(*), COALESCE(sum(amount_usd), 0) FROM recharge_records`).
		Scan(&rep.Recharges.PGCount, &rechargeSumNum); err != nil {
		return rep, fmt.Errorf("recharges agg: %w", err)
	}
	rep.Recharges.PGSum = rechargeSumNum.value
	rep.Recharges.Drift = math.Abs(rep.Recharges.JSONSum - rep.Recharges.PGSum)

	if err := pool.QueryRow(ctx, `SELECT count(*) FROM call_logs`).Scan(&rep.CallLogs.PGCount); err != nil {
		return rep, fmt.Errorf("count call_logs: %w", err)
	}
	return rep, nil
}

func scanRechargeJSONL(path string) (int, float64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	count := 0
	sum := 0.0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec legacyRecharge
		if err := json.Unmarshal(line, &rec); err != nil {
			return count, sum, err
		}
		count++
		sum += rec.AmountUSD
	}
	return count, sum, scanner.Err()
}

func scanCallLogJSONL(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	count := 0
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

// pgScanFloat 包装 pgtype.Numeric → float64，方便 verify 走 float 路径（仅汇总用途）。
type pgScanFloat struct {
	value float64
}

func (p *pgScanFloat) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		p.value = 0
		return nil
	case float64:
		p.value = v
		return nil
	case []byte:
		d, err := decimal.NewFromString(string(v))
		if err != nil {
			return err
		}
		f, _ := d.Float64()
		p.value = f
		return nil
	case string:
		d, err := decimal.NewFromString(v)
		if err != nil {
			return err
		}
		f, _ := d.Float64()
		p.value = f
		return nil
	default:
		// pgx may pass pgtype.Numeric directly
		return fmt.Errorf("unexpected numeric src: %T", src)
	}
}

func printJSONReport(cmd *cobra.Command, rep VerifyReport) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
}

func printStats(cmd *cobra.Command, result ApplyResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "=== apply stats ===")
	for _, s := range result.Stats {
		fmt.Fprintf(out, "%-20s rows=%d inserted=%d duplicates=%d failed=%d\n",
			s.Table, s.Rows, s.Inserted, s.Duplicates, s.Failed)
	}
}

func printVerify(cmd *cobra.Command, rep VerifyReport) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "=== verify report ===")
	fmt.Fprintf(out, "users      json=%d pg=%d  json_sum=%.4f pg_sum=%.4f drift=%.6f\n",
		rep.Users.JSONCount, rep.Users.PGCount, rep.Users.JSONSum, rep.Users.PGSum, rep.Users.Drift)
	fmt.Fprintf(out, "api_keys   json=%d pg=%d  legacy_sum=%.4f pg_sum=%.4f drift=%.6f\n",
		rep.APIKeys.JSONCount, rep.APIKeys.PGCount, rep.APIKeys.JSONSum, rep.APIKeys.PGSum, rep.APIKeys.Drift)
	fmt.Fprintf(out, "recharges  json=%d pg=%d  json_sum=%.4f pg_sum=%.4f drift=%.6f\n",
		rep.Recharges.JSONCount, rep.Recharges.PGCount, rep.Recharges.JSONSum, rep.Recharges.PGSum, rep.Recharges.Drift)
	fmt.Fprintf(out, "call_logs  json=%d pg=%d\n",
		rep.CallLogs.JSONCount, rep.CallLogs.PGCount)
	if rep.OK() {
		fmt.Fprintln(out, "OK: counts equal, money drift within tolerance")
	} else {
		fmt.Fprintln(out, "DRIFT detected")
	}
}
