package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// MigrateConfigToV6 幂等：SchemaVersion>=6 时不做任何事并返回 (false, nil)。
//
// 调用方约定（codex 审计意见 #1 修正）：调用者**必须**持有 cfgLock（写锁），
// 因为本函数会 mutate cfg.NewAPIChannels[i].* + cfg.SchemaVersion +
// cfg.LastV6MigrationAt。Load() 路径里我们在 unlock 前把 needV6 标记捕获后
// 再 unlock；如果需要迁移，重新 Lock 后调本函数。
func MigrateConfigToV6(c *Config) (bool, []string) {
	if c == nil {
		return false, []string{"config is nil"}
	}
	if c.SchemaVersion >= 6 {
		return false, nil
	}

	warnings := make([]string, 0)
	if err := backupConfigV5(); err != nil {
		warnings = append(warnings, err.Error())
	}

	now := time.Now().Unix()
	for i := range c.NewAPIChannels {
		_, warning := migrateNewAPIChannelToV6(&c.NewAPIChannels[i], now)
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	c.SchemaVersion = 6
	c.LastV6MigrationAt = now
	// schema 版本升级永远算 changed（要持久化）。
	return true, warnings
}

func migrateNewAPIChannelToV6(ch *NewAPIChannel, now int64) (bool, string) {
	changed := false
	if ch.CreateMode == "" {
		ch.CreateMode = "legacy_import"
		changed = true
	}
	if ch.CreatedAt == 0 {
		ch.CreatedAt = now
		changed = true
	}
	if ch.UpdatedAt == 0 {
		ch.UpdatedAt = now
		changed = true
	}
	if reason := invalidNewAPIUpstreamKey(*ch); reason != "" {
		if ch.Enabled || ch.Status != 0 {
			ch.Enabled = false
			ch.Status = 0
			ch.UpdatedAt = now
			changed = true
		}
		return changed, fmt.Sprintf("new-api channel %s disabled: %s", ch.ID, reason)
	}
	return changed, ""
}

func invalidNewAPIUpstreamKey(ch NewAPIChannel) string {
	enc := strings.TrimSpace(ch.UpstreamKeyEnc)
	if enc == "" {
		return "empty upstream key"
	}
	// migration 在 cfgLock 写锁内运行 — 必须用 locked 版避免重入死锁。
	plain, err := DecryptSecretLocked(enc)
	if err != nil {
		if isMaskedTokenKey(enc) {
			return "masked upstream key"
		}
		return ""
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "empty upstream key"
	}
	if isMaskedTokenKey(plain) {
		return "masked upstream key"
	}
	return ""
}

func isMaskedTokenKey(s string) bool {
	return strings.Contains(s, "****")
}

func backupConfigV5() error {
	if strings.TrimSpace(cfgPath) == "" {
		return nil
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("backup v5 config read failed: %w", err)
	}
	backupPath := fmt.Sprintf("%s.v5.bak.%d", cfgPath, time.Now().Unix())
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return fmt.Errorf("backup v5 config write failed: %w", err)
	}
	return nil
}
