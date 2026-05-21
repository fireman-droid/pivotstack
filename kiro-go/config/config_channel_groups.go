package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ChannelGroup 持久化与查询（v6）。
// 设计准则：
//   - 软删（DeletedAt != 0）保留记录，不重用 ID
//   - runtime channel id：NewAPI 用原 channel id；direct 用 "direct:<id>" 前缀
//   - 偏好（ApiKeyInfo.ChannelPreferences）以 groupID → runtime channel id 为单位
//   - 删 group / 删 member channel 时调用方负责清理偏好（清理函数 PruneApiKeyChannelPreferences）

var channelGroupIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// GetChannelGroups 返回当前 ChannelGroup 列表的深拷贝（包含软删）。
func GetChannelGroups() []ChannelGroup {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return deepCopyChannelGroupsLocked(cfg.ChannelGroups)
}

// GetActiveChannelGroups 仅返回未软删的 group。
func GetActiveChannelGroups() []ChannelGroup {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	out := make([]ChannelGroup, 0, len(cfg.ChannelGroups))
	for _, g := range cfg.ChannelGroups {
		if g.DeletedAt == 0 {
			out = append(out, g)
		}
	}
	return deepCopyChannelGroupsLocked(out)
}

// GetChannelGroup 单条查找（含软删）。
func GetChannelGroup(id string) (ChannelGroup, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	for _, g := range cfg.ChannelGroups {
		if g.ID == id {
			return deepCopyChannelGroupsLocked([]ChannelGroup{g})[0], true
		}
	}
	return ChannelGroup{}, false
}

// UpdateChannelGroups 替换整个 group 列表（调用方负责持锁外做校验，本函数只是持久化）。
func UpdateChannelGroups(groups []ChannelGroup) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ChannelGroups = groups
	return Save()
}

// AddChannelGroup 插入新分组；ID 必须匹配正则且不与现有 ID（含软删）冲突。
func AddChannelGroup(g ChannelGroup) error {
	if err := validateChannelGroup(g); err != nil {
		return err
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for _, existing := range cfg.ChannelGroups {
		if existing.ID == g.ID {
			return fmt.Errorf("channel group id %q already exists", g.ID)
		}
	}
	now := time.Now().Unix()
	g.CreatedAt = now
	g.UpdatedAt = now
	cfg.ChannelGroups = append(cfg.ChannelGroups, g)
	return Save()
}

// UpdateChannelGroupByID 应用 mutate 闭包到指定 group（仅作用于未软删项）。
// validateChannelGroup 失败时回滚到 mutate 前快照，不持久化。
func UpdateChannelGroupByID(id string, mutate func(*ChannelGroup)) error {
	if mutate == nil {
		return errors.New("mutate function is required")
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i := range cfg.ChannelGroups {
		if cfg.ChannelGroups[i].ID == id && cfg.ChannelGroups[i].DeletedAt == 0 {
			snapshot := deepCopyChannelGroupsLocked([]ChannelGroup{cfg.ChannelGroups[i]})[0]
			mutate(&cfg.ChannelGroups[i])
			if err := validateChannelGroup(cfg.ChannelGroups[i]); err != nil {
				cfg.ChannelGroups[i] = snapshot
				return err
			}
			cfg.ChannelGroups[i].UpdatedAt = time.Now().Unix()
			return Save()
		}
	}
	return fmt.Errorf("channel group %q not found", id)
}

// SoftDeleteChannelGroup 软删指定 group；同时返回需要清理偏好的 group id（供调用方调 PruneApiKeyChannelPreferences）。
func SoftDeleteChannelGroup(id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i := range cfg.ChannelGroups {
		if cfg.ChannelGroups[i].ID == id && cfg.ChannelGroups[i].DeletedAt == 0 {
			now := time.Now().Unix()
			cfg.ChannelGroups[i].DeletedAt = now
			cfg.ChannelGroups[i].Enabled = false
			cfg.ChannelGroups[i].UpdatedAt = now
			return Save()
		}
	}
	return fmt.Errorf("channel group %q not found", id)
}

// ReplaceChannelGroupMembers 替换 group 的 Channels 列表 + DefaultRuntimeChannelID（用 PUT /groups/:id/channels 调用）。
// channels 中的 runtime channel id 由调用方先转换好（newapi: 原 id；direct: "direct:<id>"）。
// 调用方应在调用前校验所有 channelId 都存在且 enabled。
func ReplaceChannelGroupMembers(id string, channels []ChannelGroupChannelRef, defaultRuntimeID string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i := range cfg.ChannelGroups {
		if cfg.ChannelGroups[i].ID == id && cfg.ChannelGroups[i].DeletedAt == 0 {
			snapshot := deepCopyChannelGroupsLocked([]ChannelGroup{cfg.ChannelGroups[i]})[0]
			cfg.ChannelGroups[i].Channels = append([]ChannelGroupChannelRef{}, channels...)
			cfg.ChannelGroups[i].DefaultRuntimeChannelID = defaultRuntimeID
			if err := validateChannelGroup(cfg.ChannelGroups[i]); err != nil {
				cfg.ChannelGroups[i] = snapshot
				return err
			}
			cfg.ChannelGroups[i].UpdatedAt = time.Now().Unix()
			return Save()
		}
	}
	return fmt.Errorf("channel group %q not found", id)
}

// RuntimeChannelIDFor 把 ChannelGroupChannelRef 转成 router 用的 runtime channel id。
func RuntimeChannelIDFor(ref ChannelGroupChannelRef) string {
	switch strings.ToLower(strings.TrimSpace(ref.SourceType)) {
	case "direct":
		return "direct:" + ref.ChannelID
	case "newapi":
		return ref.ChannelID
	default:
		return ref.ChannelID
	}
}

// FindChannelGroupByRuntimeID 找出哪个 group 包含指定 runtime channel id（仅未软删）。
// 用于 user 偏好校验和 router 反向查询。返回 group 深拷贝。
func FindChannelGroupByRuntimeID(runtimeID string) (ChannelGroup, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	for _, g := range cfg.ChannelGroups {
		if g.DeletedAt != 0 {
			continue
		}
		for _, ref := range g.Channels {
			if RuntimeChannelIDFor(ref) == runtimeID {
				return deepCopyChannelGroupsLocked([]ChannelGroup{g})[0], true
			}
		}
	}
	return ChannelGroup{}, false
}

func validateChannelGroup(g ChannelGroup) error {
	if !channelGroupIDPattern.MatchString(g.ID) {
		return fmt.Errorf("channel group id must match %s", channelGroupIDPattern.String())
	}
	if strings.TrimSpace(g.Name) == "" {
		return errors.New("channel group name is required")
	}
	// 允许空分组创建：admin 通常先建分组、之后慢慢挂 channel。
	// router resolveViaGroup 对空分组会自然跳过，不会路由到它。
	seen := make(map[string]struct{}, len(g.Channels))
	for _, ref := range g.Channels {
		typ := strings.ToLower(strings.TrimSpace(ref.SourceType))
		if typ != "newapi" && typ != "direct" {
			return fmt.Errorf("invalid channel source type %q", ref.SourceType)
		}
		if strings.TrimSpace(ref.ChannelID) == "" {
			return errors.New("channel id is required for each member")
		}
		key := typ + ":" + ref.ChannelID
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate channel member %s", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}
