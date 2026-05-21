// kiro-migrate-to-pg 一次性把 PivotStack 的 JSON / JSONL 持久化层迁移到 PostgreSQL。
//
// 子命令：
//   - dry-run     仅读 JSON 不写 PG，输出每张表预期 count/sum
//   - apply       真正把 JSON 数据导入 PG（幂等：ON CONFLICT DO NOTHING）
//   - verify-only 只对比 JSON 与 PG 的 count/sum，输出 drift 报告
//
// 使用：
//
//	DATABASE_URL=postgres://... ./kiro-migrate-to-pg apply --source=./data
//	DATABASE_URL=postgres://... ./kiro-migrate-to-pg verify-only --source=./data
//
// 迁移顺序（按 plan §4）：
//
//	1) users + user_wallets + api_keys + user_api_keys
//	2) recharge_records (JSONL)
//	3) call_logs (JSONL + 自动建月分区)
//
// settings / accounts / channels / activation_codes 等其他表的迁移在后续 release 落地，
// 框架已经准备好（参考 importer.go 的 Importer 接口）。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"kiro-api-proxy/db"

	"github.com/spf13/cobra"
)

func main() {
	exitCode := 0
	root := newRootCommand(&exitCode)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func newRootCommand(exitCode *int) *cobra.Command {
	var sourceDir string
	root := &cobra.Command{
		Use:           "kiro-migrate-to-pg",
		Short:         "Migrate PivotStack JSON persistence to PostgreSQL",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&sourceDir, "source", "./data", "Source data directory (contains users.json/config.json/recharge_records.jsonl/call_logs.jsonl)")
	root.AddCommand(newDryRunCommand(&sourceDir))
	root.AddCommand(newApplyCommand(&sourceDir, exitCode))
	root.AddCommand(newVerifyCommand(&sourceDir, exitCode))
	return root
}

func newDryRunCommand(source *string) *cobra.Command {
	return &cobra.Command{
		Use:   "dry-run",
		Short: "Read source JSON only and print expected counts/sums (no DB writes)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rep, err := dryRunReport(*source)
			if err != nil {
				return err
			}
			printJSONReport(cmd, rep)
			return nil
		},
	}
}

func newApplyCommand(source *string, exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Import JSON data into PostgreSQL (idempotent via ON CONFLICT DO NOTHING)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := initDB(ctx); err != nil {
				return err
			}
			defer db.Close()

			stats, err := runApply(ctx, *source)
			if err != nil {
				return err
			}
			printStats(cmd, stats)
			verify, err := compareJSONvsPG(ctx, *source)
			if err != nil {
				return fmt.Errorf("post-apply verify: %w", err)
			}
			printVerify(cmd, verify)
			if !verify.OK() {
				*exitCode = 3
				return errors.New("post-apply verification reported drift")
			}
			return nil
		},
	}
}

func newVerifyCommand(source *string, exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "verify-only",
		Short: "Compare JSON source vs PostgreSQL state without writing",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := initDB(ctx); err != nil {
				return err
			}
			defer db.Close()

			verify, err := compareJSONvsPG(ctx, *source)
			if err != nil {
				return err
			}
			printVerify(cmd, verify)
			if !verify.OK() {
				*exitCode = 3
				return errors.New("verification reported drift")
			}
			return nil
		},
	}
}

func initDB(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	return db.InitPool(ctx, dsn)
}
