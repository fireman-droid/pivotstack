// Package db owns PostgreSQL connection pooling and schema migrations.
package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConns        int32 = 20
	defaultMinConns        int32 = 2
	defaultMaxConnLifetime       = time.Hour
	defaultMaxConnIdleTime       = 30 * time.Minute
	defaultPingAttempts          = 5
)

var (
	poolOnce sync.Once
	poolMu   sync.RWMutex
	pool     *pgxpool.Pool
	poolErr  error
)

// InitPool initializes the process-wide pgx pool.
func InitPool(ctx context.Context, databaseURL string) error {
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	poolOnce.Do(func() {
		poolErr = initPool(ctx, databaseURL)
	})
	return poolErr
}

func initPool(ctx context.Context, databaseURL string) error {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse postgres config: %w", err)
	}
	cfg.MaxConns = defaultMaxConns
	cfg.MinConns = defaultMinConns
	cfg.MaxConnLifetime = defaultMaxConnLifetime
	cfg.MaxConnIdleTime = defaultMaxConnIdleTime
	cfg.HealthCheckPeriod = time.Minute

	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pingWithRetry(ctx, p); err != nil {
		p.Close()
		return err
	}

	poolMu.Lock()
	pool = p
	poolMu.Unlock()
	return nil
}

func pingWithRetry(ctx context.Context, p *pgxpool.Pool) error {
	var lastErr error
	for attempt := 1; attempt <= defaultPingAttempts; attempt++ {
		if err := p.Ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == defaultPingAttempts {
			break
		}
		timer := time.NewTimer(time.Duration(attempt) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("ping postgres canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("ping postgres: %w", lastErr)
}

// Pool returns the process-wide PostgreSQL pool.
func Pool() *pgxpool.Pool {
	poolMu.RLock()
	defer poolMu.RUnlock()
	return pool
}

// Close gracefully closes the process-wide PostgreSQL pool.
func Close() {
	poolMu.Lock()
	defer poolMu.Unlock()
	if pool != nil {
		pool.Close()
		pool = nil
	}
}
