package db

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPoolPing(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := InitPool(ctx, databaseURL); err != nil {
		t.Fatalf("InitPool() error = %v", err)
	}
	defer Close()

	p := Pool()
	if p == nil {
		t.Fatal("Pool() returned nil")
	}

	var one int
	if err := p.QueryRow(ctx, `SELECT 1`).Scan(&one); err != nil {
		t.Fatalf("query postgres: %v", err)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 = %d", one)
	}
}
