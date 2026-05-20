// Package main provides the entry point for Kiro API Proxy.
//
// Kiro API Proxy is a reverse proxy service that translates Kiro API requests
// into OpenAI and Anthropic (Claude) compatible formats. Key features include:
//   - Multi-account pool with round-robin load balancing
//   - Automatic OAuth token refresh
//   - Streaming response support for real-time AI interactions
//   - Admin panel for account and configuration management
//
// The service exposes the following endpoints:
//   - /v1/messages - Claude API compatible endpoint
//   - /v1/chat/completions - OpenAI API compatible endpoint
//   - /admin - Web-based administration panel
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/db"
	"kiro-api-proxy/pool"
	"kiro-api-proxy/proxy"
	"kiro-api-proxy/users"
)

func main() {
	// 配置文件路径，支持环境变量覆盖
	configPath := "data/config.json"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	// 确保数据目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// 加载配置
	if err := config.Init(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// v10 Stage 1: 仅初始化 pgx pool（如果 DATABASE_URL 存在）。
	// JSON 仍然是当前唯一持久化后端，DAL repository 在 Stage 2-3 接入。
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		dbCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := db.InitPool(dbCtx, databaseURL); err != nil {
			cancel()
			log.Fatalf("Failed to initialize postgres pool: %v", err)
		}
		cancel()
		defer db.Close()
	}

	// v8: 主动初始化 users store 触发 schemaVersion 迁移（v2→v3 把 key 钱包搬到 user）。
	// 如果不在这里 init，懒加载要等首个 user-portal 请求才触发，可能错过早期 admin/billing 路径。
	_ = users.Default()

	// 环境变量覆盖密码（容器化部署常用；不写盘，仅存内存 hash）
	if envPassword := os.Getenv("ADMIN_PASSWORD"); envPassword != "" {
		if err := config.SetPassword(envPassword); err != nil {
			log.Fatalf("invalid ADMIN_PASSWORD: %v", err)
		}
	}

	// 生产环境强制独立加密密钥（防 config.json 泄露 + admin password hash 泄露 → 上游密钥可解）
	if err := config.RequireProductionEncryptionKey(); err != nil {
		log.Fatalf("invalid encryption configuration: %v", err)
	}

	// 注入 supportedModels 表（迁移函数和 fallback 路由要用），必须在 MaybeMigratePricing 之前
	config.SetSupportedModels(proxy.SupportedModels())

	// 启动时检查并执行 pricing/promotion v1→v2 迁移（一次性）
	if migrated, err := config.MaybeMigratePricing(); err != nil {
		log.Fatalf("pricing migration failed: %v", err)
	} else if migrated {
		log.Printf("[Migrate] pricing/promotion migrated from v1 (PoolPrice × Multiplier) to v2 (ModelPrices)")
	}

	// 初始化账号池
	pool.GetPool()

	// 创建 HTTP 处理器（包含后台刷新任务）
	handler := proxy.NewHandler()

	// 启动服务器
	host := config.GetHost()
	port := config.GetPort()
	addr := fmt.Sprintf("%s:%d", host, port)
	// 回写运行时监听地址，admin settings 页用它显示"实际监听 Port"
	config.SetRuntimeHTTPAddress(host, strconv.Itoa(port))

	// 显示地址：0.0.0.0 转换为 localhost
	displayHost := host
	if host == "0.0.0.0" {
		displayHost = "localhost"
	}
	displayAddr := fmt.Sprintf("%s:%d", displayHost, port)

	log.Printf("PivotStack starting on http://%s", displayAddr)
	log.Printf("Admin panel: http://%s/admin", displayAddr)
	log.Printf("Claude API: http://%s/v1/messages", displayAddr)
	log.Printf("OpenAI API: http://%s/v1/chat/completions", displayAddr)

	// 创建自定义 HTTP 服务器，增加请求大小限制
	server := &http.Server{
		Addr:           addr,
		Handler:        handler,
		MaxHeaderBytes: 10 << 20, // 10MB header limit
		ReadTimeout:    300 * time.Second,
		WriteTimeout:   300 * time.Second,
	}

	// SIGTERM / SIGINT graceful shutdown：让 handler.Stop() 跑完，
	// log persist worker drain 内存队列 + newapi reconciler 收尾。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer handler.Stop()

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received, draining...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			_ = server.Close()
		}
		cancel()
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}
}
