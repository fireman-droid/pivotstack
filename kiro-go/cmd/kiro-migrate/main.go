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
	root := &cobra.Command{
		Use:           "kiro-migrate",
		Short:         "Run PivotStack PostgreSQL schema migrations",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newUpCommand(exitCode))
	root.AddCommand(newStatusCommand())
	return root
}

func newUpCommand(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply pending schema migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := initDB(ctx); err != nil {
				return err
			}
			defer db.Close()

			if err := db.RunMigrations(ctx); err != nil {
				if errors.Is(err, db.ErrAlreadyAtLatest) {
					fmt.Fprintln(cmd.OutOrStdout(), "already at latest")
					*exitCode = 2
					return nil
				}
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "migrations applied")
			return nil
		},
	}
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print schema migration status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := initDB(ctx); err != nil {
				return err
			}
			defer db.Close()

			status, err := db.MigrationStatus(ctx)
			if err != nil {
				return err
			}
			pending := 0
			for _, m := range status.Migrations {
				if !m.Applied {
					pending++
				}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "current_version=%d\n", status.CurrentVersion)
			fmt.Fprintf(out, "pending=%d\n", pending)
			for _, m := range status.Migrations {
				state := "pending"
				if m.Applied {
					state = "applied"
				}
				fmt.Fprintf(out, "%04d %s %s checksum=%s\n", m.Version, m.Name, state, m.Checksum)
			}
			return nil
		},
	}
}

func initDB(ctx context.Context) error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	return db.InitPool(ctx, databaseURL)
}
