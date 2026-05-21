// Package config provides configuration management for Kiro API Proxy.
//
// This package handles persistent storage and retrieval of:
//   - Account credentials and authentication tokens
//   - Server settings (port, host, API keys)
//   - Usage statistics and metrics
//   - Thinking mode configuration for AI responses
//
// All configuration is stored in a JSON file with thread-safe access
// via read-write mutex protection.
package config

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	cfg     *Config
	cfgLock sync.RWMutex
	cfgPath string
)

// GenerateMachineId generates a UUID v4 format machine identifier.
// This ID is used to uniquely identify the proxy instance in Kiro API requests,
// helping with request tracking and rate limiting on the server side.
func GenerateMachineId() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // 版本 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // 变体
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// Init initializes the configuration system with the specified file path.
// If the file doesn't exist, a default configuration is created.
func Init(path string) error {
	cfgPath = path
	return Load()
}

// GetDataDir returns the directory containing the config file (used for log persistence)
func GetDataDir() string {
	if cfgPath == "" {
		return "."
	}
	dir := cfgPath
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' || dir[i] == '\\' {
			return dir[:i]
		}
	}
	return "."
}

// BackupCurrentConfig copies the current config.json to config.json.<suffix>.<unix>.
// Used by migration paths that need a rollback snapshot.
func BackupCurrentConfig(suffix string) error {
	if cfgPath == "" {
		return fmt.Errorf("config path is empty")
	}
	if suffix == "" {
		suffix = "backup"
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	backupPath := fmt.Sprintf("%s.%s.%d", cfgPath, suffix, time.Now().Unix())
	return os.WriteFile(backupPath, data, 0o600)
}

func Load() error {
	cfgLock.Lock()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 默认密码也写 hash，不写明文 "changeme"
			defaultHash, hashErr := HashAdminPassword("changeme")
			if hashErr != nil {
				cfgLock.Unlock()
				return hashErr
			}
			cfg = &Config{
				Password:      defaultHash,
				Port:          8080,
				Host:          "0.0.0.0",
				RequireApiKey: false,
				Accounts:      []Account{},
				SchemaVersion: 6,
			}
			err := Save()
			cfgLock.Unlock()
			return err
		}
		cfgLock.Unlock()
		return err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		cfgLock.Unlock()
		return err
	}
	// Backward-compatible migration: single ApiKey → ApiKeys[]
	migrated := false
	if len(c.ApiKeys) == 0 && c.ApiKey != "" {
		c.ApiKeys = []ApiKeyInfo{{
			ID: GenerateMachineId(), Key: c.ApiKey, Plan: "timed",
			ExpiresAt: 0, Enabled: true, Note: "migrated", CreatedAt: time.Now().Unix(),
		}}
		c.ApiKey = ""
		migrated = true
	}
	cfg = &c
	// 密码迁移失败不阻止启动 —— 否则备份/写盘失败会让管理员被锁在后台外面，
	// 反而比"暂时还是明文"更糟。verifyAdminPasswordHash 在迁移期支持明文兜底。
	if err := migrateAdminPasswordLocked(); err != nil {
		fmt.Printf("[config] WARN: admin password migration skipped: %v\n", err)
	}
	if migrated {
		if err := Save(); err != nil {
			cfgLock.Unlock()
			return err
		}
	}
	// codex 审计意见 #1 修正：MigrateConfigToV6 mutate cfg，必须在锁内调，避免 race。
	var v6Warnings []string
	var v6Changed bool
	if cfg.SchemaVersion < 6 {
		v6Changed, v6Warnings = MigrateConfigToV6(cfg)
		if v6Changed {
			if err := Save(); err != nil {
				cfgLock.Unlock()
				for _, w := range v6Warnings {
					fmt.Printf("[config] WARN: v6 migration: %s\n", w)
				}
				return err
			}
		}
	}
	cfgLock.Unlock()
	for _, w := range v6Warnings {
		fmt.Printf("[config] WARN: v6 migration: %s\n", w)
	}
	return nil
}

// Save persists the current configuration to the JSON file.
// 用 atomicWriteFile 写入：tmp → fsync → rename，防止 crash / 磁盘满留下截断 JSON。
func Save() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(cfgPath, data, 0600)
}

func Get() *Config {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg
}
