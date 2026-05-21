package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// GetNewAPIProviders 返回 provider 配置深拷贝，避免锁外修改污染内存配置。
func GetNewAPIProviders() []NewAPIProvider {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return deepCopyNewAPIProvidersLocked(cfg.NewAPIProviders)
}

// GetNewAPIProvider 按 ID 返回单个 provider。
func GetNewAPIProvider(id string) (NewAPIProvider, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	id = strings.TrimSpace(id)
	for _, p := range cfg.NewAPIProviders {
		if p.ID == id {
			return p, true
		}
	}
	return NewAPIProvider{}, false
}

// UpdateNewAPIProviders 写入 provider 列表（深拷贝）。
func UpdateNewAPIProviders(providers []NewAPIProvider) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.NewAPIProviders = deepCopyNewAPIProvidersLocked(providers)
	return Save()
}

// GetNewAPIChannels 返回 v5 channel 深拷贝。
func GetNewAPIChannels() []NewAPIChannel {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return deepCopyNewAPIChannelsLocked(cfg.NewAPIChannels)
}

// GetNewAPIChannel 按 ID 返回单个 NewAPI channel 深拷贝。
func GetNewAPIChannel(id string) (NewAPIChannel, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	id = strings.TrimSpace(id)
	for _, ch := range cfg.NewAPIChannels {
		if ch.ID == id {
			return cloneNewAPIChannel(ch), true
		}
	}
	return NewAPIChannel{}, false
}

// UpdateNewAPIChannels 写入 v5 channel 列表（深拷贝）。
func UpdateNewAPIChannels(channels []NewAPIChannel) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.NewAPIChannels = deepCopyNewAPIChannelsLocked(channels)
	return Save()
}

// AddNewAPIChannel 追加一个 PivotStack 主动创建的 NewAPI channel（v6 唯一入口）。
// 校验：ID 唯一 + alias 全局唯一 + 必要字段非空 + markup>0。
func AddNewAPIChannel(ch NewAPIChannel) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	cp := cloneNewAPIChannel(ch)
	now := time.Now().Unix()
	prepareNewAPIChannelForWrite(&cp, now)
	for _, existing := range cfg.NewAPIChannels {
		if existing.ID == cp.ID {
			return fmt.Errorf("newapi channel id already exists: %s", cp.ID)
		}
	}
	if err := validateNewAPIChannelForWriteLocked(cp); err != nil {
		return err
	}
	cfg.NewAPIChannels = append(cfg.NewAPIChannels, cp)
	return Save()
}

// SoftDeleteNewAPIChannel 软删除 channel，保留历史与对账关联（DeletedAt 时间戳 + Enabled=false）。
func SoftDeleteNewAPIChannel(id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	id = strings.TrimSpace(id)
	for i := range cfg.NewAPIChannels {
		if cfg.NewAPIChannels[i].ID != id {
			continue
		}
		now := time.Now().Unix()
		cfg.NewAPIChannels[i].Enabled = false
		cfg.NewAPIChannels[i].DeletedAt = now
		cfg.NewAPIChannels[i].UpdatedAt = now
		return Save()
	}
	return fmt.Errorf("newapi channel not found: %s", id)
}

func prepareNewAPIChannelForWrite(ch *NewAPIChannel, now int64) {
	ch.ID = strings.TrimSpace(ch.ID)
	ch.ProviderID = strings.TrimSpace(ch.ProviderID)
	ch.Alias = strings.TrimSpace(ch.Alias)
	ch.UpstreamTokenName = strings.TrimSpace(ch.UpstreamTokenName)
	ch.GroupName = strings.TrimSpace(ch.GroupName)
	ch.SeriesID = strings.TrimSpace(ch.SeriesID)
	ch.CreateMode = strings.TrimSpace(ch.CreateMode)
	if ch.CreatedAt == 0 {
		ch.CreatedAt = now
	}
	if ch.UpdatedAt == 0 {
		ch.UpdatedAt = now
	}
}

func validateNewAPIChannelForWriteLocked(ch NewAPIChannel) error {
	if strings.TrimSpace(ch.ID) == "" {
		return fmt.Errorf("newapi channel id is required")
	}
	if strings.TrimSpace(ch.ProviderID) == "" {
		return fmt.Errorf("newapi channel providerId is required")
	}
	if strings.TrimSpace(ch.Alias) == "" {
		return fmt.Errorf("newapi channel alias is required")
	}
	if ch.UpstreamTokenID <= 0 {
		return fmt.Errorf("newapi channel upstreamTokenId is required")
	}
	if ch.Markup <= 0 {
		return fmt.Errorf("newapi channel markup must be > 0")
	}
	return validateGroupAliasUniqueLocked(ch.ID, ch.Alias)
}

func cloneNewAPIChannel(ch NewAPIChannel) NewAPIChannel {
	cp := ch
	if len(ch.Models) > 0 {
		cp.Models = append([]string{}, ch.Models...)
	}
	return cp
}

// ensureSecretKeySaltLocked 确保 secret salt 稳定存在。
// 调用方必须持有 cfgLock.Lock()，因为首次调用会写 cfg.SecretKeySalt。
func ensureSecretKeySaltLocked() string {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.SecretKeySalt != "" {
		return cfg.SecretKeySalt
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// rand.Reader 极少失败；这里仍 fail-soft，避免加密路径 panic。
		cfg.SecretKeySalt = base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	} else {
		cfg.SecretKeySalt = base64.RawStdEncoding.EncodeToString(b)
	}
	if cfgPath != "" {
		if err := Save(); err != nil {
			fmt.Printf("[config] WARN: persist secret key salt failed: %v\n", err)
		}
	}
	return cfg.SecretKeySalt
}
