package config

import (
	"fmt"
	"strings"
)

// copyStringMap 深拷贝 string→string map（v5 用于 ApiKeyInfo.SeriesPreferences）。
func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// SetApiKeySeriesPreferences replaces a user's per-series NewAPI channel preferences.
// 校验放在 config 层，确保所有入口（user API / admin） 都遵守 series→channel 绑定约束。
func SetApiKeySeriesPreferences(keyID string, prefs map[string]string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	channelByID := make(map[string]NewAPIChannel, len(cfg.NewAPIChannels))
	for _, ch := range cfg.NewAPIChannels {
		channelByID[ch.ID] = ch
	}

	normalized := make(map[string]string, len(prefs))
	for rawSeriesID, rawChannelID := range prefs {
		seriesID := strings.TrimSpace(rawSeriesID)
		channelID := strings.TrimSpace(rawChannelID)
		if seriesID == "" || channelID == "" {
			continue
		}
		ch, ok := channelByID[channelID]
		if !ok {
			return fmt.Errorf("series preference %q: channel %q not found", seriesID, channelID)
		}
		if !ch.Enabled {
			return fmt.Errorf("series preference %q: channel %q is disabled", seriesID, channelID)
		}
		if ch.DeletedAt > 0 {
			return fmt.Errorf("series preference %q: channel %q is deleted", seriesID, channelID)
		}
		if ch.SeriesID != seriesID {
			return fmt.Errorf("series preference %q: channel %q belongs to series %q", seriesID, channelID, ch.SeriesID)
		}
		normalized[seriesID] = channelID
	}
	if len(normalized) == 0 {
		normalized = nil
	}

	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID != keyID {
			continue
		}
		cp := copyApiKeyInfoLocked(cfg.ApiKeys[i])
		cp.SeriesPreferences = normalized
		cfg.ApiKeys[i] = cp
		appendConfigAuditLog("user_preference_changed",
			fmt.Sprintf("keyId=%s preferences=%d", keyID, len(normalized)))
		return Save()
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// SetApiKeyChannelPreferences 替换 user 的 per-ChannelGroup 偏好（v6）。
// 输入 map：groupID → runtime channel id。校验：
//   1. group 存在且未软删 + enabled
//   2. 给定的 runtime channel id 是 group 当前 enabled 成员之一
//   3. 对应 channel 仍 enabled 且未删
// 不命中规则的条目直接丢弃（不报错，避免前端旧偏好阻塞），仅 strict 模式才返回错误。
func SetApiKeyChannelPreferences(keyID string, prefs map[string]string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	groupByID := make(map[string]ChannelGroup, len(cfg.ChannelGroups))
	for _, g := range cfg.ChannelGroups {
		if g.DeletedAt == 0 {
			groupByID[g.ID] = g
		}
	}
	enabledNewAPIChannels := make(map[string]struct{}, len(cfg.NewAPIChannels))
	for _, ch := range cfg.NewAPIChannels {
		if ch.Enabled && ch.DeletedAt == 0 {
			enabledNewAPIChannels[ch.ID] = struct{}{}
		}
	}
	enabledDirectChannels := make(map[string]struct{}, len(cfg.DirectChannels))
	for _, ch := range cfg.DirectChannels {
		if ch.Enabled && ch.DeletedAt == 0 {
			enabledDirectChannels["direct:"+ch.ID] = struct{}{}
		}
	}

	normalized := make(map[string]string, len(prefs))
	for rawGroupID, rawRuntimeID := range prefs {
		groupID := strings.TrimSpace(rawGroupID)
		runtimeID := strings.TrimSpace(rawRuntimeID)
		if groupID == "" || runtimeID == "" {
			continue
		}
		group, ok := groupByID[groupID]
		if !ok || !group.Enabled {
			return fmt.Errorf("channel preference %q: group not found or disabled", groupID)
		}
		memberOK := false
		for _, ref := range group.Channels {
			if RuntimeChannelIDFor(ref) == runtimeID {
				memberOK = true
				break
			}
		}
		if !memberOK {
			return fmt.Errorf("channel preference %q: channel %q is not a member of the group", groupID, runtimeID)
		}
		if _, ok := enabledNewAPIChannels[runtimeID]; !ok {
			if _, ok := enabledDirectChannels[runtimeID]; !ok {
				return fmt.Errorf("channel preference %q: channel %q is disabled or removed", groupID, runtimeID)
			}
		}
		normalized[groupID] = runtimeID
	}
	if len(normalized) == 0 {
		normalized = nil
	}

	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID != keyID {
			continue
		}
		cp := copyApiKeyInfoLocked(cfg.ApiKeys[i])
		cp.ChannelPreferences = normalized
		cfg.ApiKeys[i] = cp
		appendConfigAuditLog("user_channel_preference_changed",
			fmt.Sprintf("keyId=%s preferences=%d", keyID, len(normalized)))
		return Save()
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// PruneApiKeyChannelPreferences 清除所有 ApiKey 上指向给定 group id 的偏好。
// 调用时机：admin 软删 ChannelGroup 后；批量替换 group 成员且某 channel 被移出时调用方应自行调用。
func PruneApiKeyChannelPreferences(removedGroupIDs []string) error {
	if len(removedGroupIDs) == 0 {
		return nil
	}
	removed := make(map[string]struct{}, len(removedGroupIDs))
	for _, id := range removedGroupIDs {
		removed[id] = struct{}{}
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	dirty := false
	for i := range cfg.ApiKeys {
		if len(cfg.ApiKeys[i].ChannelPreferences) == 0 {
			continue
		}
		hasMatch := false
		for groupID := range cfg.ApiKeys[i].ChannelPreferences {
			if _, ok := removed[groupID]; ok {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			continue
		}
		cp := copyApiKeyInfoLocked(cfg.ApiKeys[i])
		next := make(map[string]string, len(cp.ChannelPreferences))
		for groupID, runtimeID := range cp.ChannelPreferences {
			if _, ok := removed[groupID]; ok {
				continue
			}
			next[groupID] = runtimeID
		}
		if len(next) == 0 {
			next = nil
		}
		cp.ChannelPreferences = next
		cfg.ApiKeys[i] = cp
		dirty = true
	}
	if !dirty {
		return nil
	}
	return Save()
}
