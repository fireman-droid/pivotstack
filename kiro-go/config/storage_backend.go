package config

import (
	"os"
	"strings"
	"sync"
)

// StorageBackend 决定 PivotStack 在运行时使用哪种持久化层。
//   - "json" (default)：legacy 路径，所有写入落 data/*.json 与 data/*.jsonl。
//   - "pg"：PostgreSQL 后端，由 kiro-go/db 包提供 DAL。
//
// 当 STORAGE_BACKEND=pg 时，应用要求 DATABASE_URL 必须配置；DAL 模块在
// main.go 启动期通过 db.InitPool 初始化连接池，并在切换前用 kiro-migrate-to-pg
// 工具完成 JSON → PG 一次性数据导入和 verify-only 对账。
type StorageBackend string

const (
	StorageBackendJSON StorageBackend = "json"
	StorageBackendPG   StorageBackend = "pg"
)

const (
	envStorageBackend         = "STORAGE_BACKEND"
	envDatabaseURL            = "DATABASE_URL"
	envStorageJSONFallback    = "STORAGE_JSON_FALLBACK_READ"
	envStorageDualWriteJSON   = "STORAGE_DUAL_WRITE_JSON"
	envStorageRollbackEnabled = "STORAGE_ROLLBACK_READY"
)

var (
	storageBackendOnce sync.Once
	storageBackendVal  StorageBackend
)

// GetStorageBackend 解析 STORAGE_BACKEND 环境变量。
//   - 空 / 任意非 "pg" 字符串 → StorageBackendJSON
//   - 大小写不敏感
//
// 结果会在进程内 memoize，避免每次调用重读 env；进程重启即可切换。
func GetStorageBackend() StorageBackend {
	storageBackendOnce.Do(func() {
		raw := strings.TrimSpace(strings.ToLower(os.Getenv(envStorageBackend)))
		switch raw {
		case "pg", "postgres", "postgresql":
			storageBackendVal = StorageBackendPG
		default:
			storageBackendVal = StorageBackendJSON
		}
	})
	return storageBackendVal
}

// DatabaseURL 返回 DATABASE_URL；调用方在 PG 模式下应校验非空。
func DatabaseURL() string {
	return strings.TrimSpace(os.Getenv(envDatabaseURL))
}

// IsJSONFallbackReadEnabled Stage C dual-write 期间读 JSON 兜底。
// 默认 false，等到 Stage D canary 时打开。
func IsJSONFallbackReadEnabled() bool {
	return parseBoolEnv(envStorageJSONFallback, false)
}

// IsDualWriteJSONEnabled 写 PG 成功后同步写 JSON，作为 rollback 资产。
// 默认 false；Stage C 切 PG 后打开，Stage E 关闭。
func IsDualWriteJSONEnabled() bool {
	return parseBoolEnv(envStorageDualWriteJSON, false)
}

// IsRollbackReady 由后台 verify 任务每天写入；当 PG 与 JSON 比较 drift=0 时为 true。
// 这个开关由 ops 工具维护，应用本身只读不写。
func IsRollbackReady() bool {
	return parseBoolEnv(envStorageRollbackEnabled, false)
}

// ResetStorageBackendForTest 仅用于测试场景：清理 memoize 结果以便切换 env 后重新读。
func ResetStorageBackendForTest() {
	storageBackendOnce = sync.Once{}
	storageBackendVal = ""
}

func parseBoolEnv(name string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on", "y":
		return true
	default:
		return false
	}
}
