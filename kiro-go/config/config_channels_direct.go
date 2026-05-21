package config

import (
	"fmt"
	"strings"
	"time"
)

func GetDirectChannels() []DirectChannel {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return deepCopyDirectChannelsLocked(cfg.DirectChannels)
}

func GetDirectChannel(id string) (DirectChannel, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	id = strings.TrimSpace(id)
	for _, ch := range cfg.DirectChannels {
		if ch.ID == id {
			return cloneDirectChannel(ch), true
		}
	}
	return DirectChannel{}, false
}

func UpdateDirectChannels(channels []DirectChannel) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	cp := deepCopyDirectChannelsLocked(channels)
	now := time.Now().Unix()
	for i := range cp {
		prepareDirectChannelForWrite(&cp[i], now)
	}
	if err := validateDirectChannelsLocked(cp); err != nil {
		return err
	}
	cfg.DirectChannels = cp
	return Save()
}

func AddDirectChannel(ch DirectChannel) (DirectChannel, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	cp := cloneDirectChannel(ch)
	now := time.Now().Unix()
	prepareDirectChannelForWrite(&cp, now)
	if err := validateDirectChannelLocked(cp); err != nil {
		return DirectChannel{}, err
	}
	cfg.DirectChannels = append(cfg.DirectChannels, cp)
	if err := Save(); err != nil {
		return DirectChannel{}, err
	}
	return cloneDirectChannel(cp), nil
}

// UpdateDirectChannel 合并 patch 到现有 channel — 不是全量替换。
// 字段语义：
//   - 非 zero 标量字段（Type/Alias/BaseURL/Enabled/Status 等）：值改写
//   - 切片/map（Models/ModelMapping/ExtraHeaders）：nil → 保留旧值；非 nil → 覆盖
//   - APIKeyEnc：空字符串 → 保留旧值（用 SetDirectChannelAPIKey 显式清空）
//   - SellPrice：Default 全 0 + Models 为 nil → 保留旧；其它情况整体覆盖
// 这是 codex 审计意见 #2 的修复 — admin UI 只想改 alias 不应清掉 APIKeyEnc。
func UpdateDirectChannel(id string, patch DirectChannel) (DirectChannel, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	id = strings.TrimSpace(id)
	for i := range cfg.DirectChannels {
		if cfg.DirectChannels[i].ID != id {
			continue
		}
		merged := mergeDirectChannelPatch(cfg.DirectChannels[i], patch, id)
		merged.UpdatedAt = time.Now().Unix()
		if err := validateDirectChannelLocked(merged); err != nil {
			return DirectChannel{}, err
		}
		cfg.DirectChannels[i] = merged
		if err := Save(); err != nil {
			return DirectChannel{}, err
		}
		return cloneDirectChannel(merged), nil
	}
	return DirectChannel{}, fmt.Errorf("direct channel not found: %s", id)
}

// mergeDirectChannelPatch 把 patch 合并进 existing，返回新副本。
// 标量字段用 patch 值；nil 切片/map 保留 existing；空字符串 APIKeyEnc 保留 existing；
// SellPrice 全空时保留 existing。
func mergeDirectChannelPatch(existing, patch DirectChannel, id string) DirectChannel {
	out := cloneDirectChannel(existing)
	out.ID = id
	if patch.Type != "" {
		out.Type = patch.Type
	}
	if patch.Alias != "" {
		out.Alias = patch.Alias
	}
	if patch.BaseURL != "" {
		out.BaseURL = patch.BaseURL
	}
	// APIKeyEnc：空保留旧；显式清空走 SetDirectChannelAPIKey
	if patch.APIKeyEnc != "" {
		out.APIKeyEnc = patch.APIKeyEnc
	}
	if patch.Models != nil {
		out.Models = append([]string(nil), patch.Models...)
	}
	if patch.ModelMapping != nil {
		out.ModelMapping = copyStringMap(patch.ModelMapping)
	}
	if patch.ExtraHeaders != nil {
		out.ExtraHeaders = copyStringMap(patch.ExtraHeaders)
	}
	if !isZeroSellPrice(patch.SellPrice) {
		out.SellPrice = cloneDirectSellPrice(patch.SellPrice)
	}
	// 布尔字段直接用 patch（admin 总是带完整状态发过来）
	out.Enabled = patch.Enabled
	if patch.Status != "" {
		out.Status = patch.Status
	}
	return out
}

func isZeroSellPrice(p DirectSellPrice) bool {
	return p.Default == (DirectSellPriceRow{}) && len(p.Models) == 0
}

func DeleteDirectChannel(id string, hard bool) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	id = strings.TrimSpace(id)
	for i := range cfg.DirectChannels {
		if cfg.DirectChannels[i].ID != id {
			continue
		}
		if hard {
			cfg.DirectChannels = append(cfg.DirectChannels[:i], cfg.DirectChannels[i+1:]...)
			return Save()
		}
		now := time.Now().Unix()
		cfg.DirectChannels[i].Enabled = false
		cfg.DirectChannels[i].DeletedAt = now
		cfg.DirectChannels[i].UpdatedAt = now
		return Save()
	}
	return fmt.Errorf("direct channel not found: %s", id)
}

func SetDirectChannelAPIKey(id, enc string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	id = strings.TrimSpace(id)
	for i := range cfg.DirectChannels {
		if cfg.DirectChannels[i].ID != id {
			continue
		}
		cfg.DirectChannels[i].APIKeyEnc = strings.TrimSpace(enc)
		cfg.DirectChannels[i].UpdatedAt = time.Now().Unix()
		return Save()
	}
	return fmt.Errorf("direct channel not found: %s", id)
}

func validateDirectChannelLocked(ch DirectChannel) error {
	if err := validateDirectChannelShape(ch); err != nil {
		return err
	}
	return validateGroupAliasUniqueLocked(ch.ID, ch.Alias)
}

func validateDirectChannelsLocked(channels []DirectChannel) error {
	seenIDs := make(map[string]bool, len(channels))
	seenAliases := make(map[string]string, len(channels))
	for _, ch := range channels {
		if err := validateDirectChannelShape(ch); err != nil {
			return err
		}
		if seenIDs[ch.ID] {
			return fmt.Errorf("duplicate direct channel id: %s", ch.ID)
		}
		seenIDs[ch.ID] = true
		alias := normalizeGroupAlias(ch.Alias)
		if ch.DeletedAt == 0 && alias != "" {
			if other := seenAliases[alias]; other != "" {
				return fmt.Errorf("direct channel alias %q conflicts with %s", ch.Alias, other)
			}
			seenAliases[alias] = ch.ID
		}
		// codex 审计意见 #3：删除中的渠道不参与跨域 alias 冲突检查
		if ch.DeletedAt > 0 {
			continue
		}
		if err := validateNewAPIAliasUniqueLocked(ch.ID, ch.Alias); err != nil {
			return err
		}
	}
	return nil
}

func validateDirectChannelShape(ch DirectChannel) error {
	if strings.TrimSpace(ch.ID) == "" {
		return fmt.Errorf("direct channel id is required")
	}
	switch strings.ToLower(strings.TrimSpace(ch.Type)) {
	case "openai", "kiro":
	default:
		return fmt.Errorf("direct channel type must be openai or kiro")
	}
	if strings.TrimSpace(ch.Alias) == "" {
		return fmt.Errorf("direct channel alias is required")
	}
	return validateDirectSellPrice(ch.ID, ch.SellPrice)
}

func validateDirectSellPrice(id string, price DirectSellPrice) error {
	if err := validateDirectSellPriceRow(id, "default", price.Default); err != nil {
		return err
	}
	for model, row := range price.Models {
		if err := validateDirectSellPriceRow(id, model, row); err != nil {
			return err
		}
	}
	return nil
}

func validateDirectSellPriceRow(id, label string, row DirectSellPriceRow) error {
	if row.InputPerM < 0 {
		return fmt.Errorf("direct channel %s sellPrice.%s.inputPerM is negative", id, label)
	}
	if row.OutputPerM < 0 {
		return fmt.Errorf("direct channel %s sellPrice.%s.outputPerM is negative", id, label)
	}
	if row.CostInputPerM < 0 {
		return fmt.Errorf("direct channel %s sellPrice.%s.costInputPerM is negative", id, label)
	}
	if row.CostOutputPerM < 0 {
		return fmt.Errorf("direct channel %s sellPrice.%s.costOutputPerM is negative", id, label)
	}
	return nil
}

func prepareDirectChannelForWrite(ch *DirectChannel, now int64) {
	ch.ID = strings.TrimSpace(ch.ID)
	if ch.ID == "" {
		ch.ID = GenerateMachineId()
	}
	ch.Type = strings.ToLower(strings.TrimSpace(ch.Type))
	ch.Alias = strings.TrimSpace(ch.Alias)
	ch.BaseURL = strings.TrimSpace(ch.BaseURL)
	ch.APIKeyEnc = strings.TrimSpace(ch.APIKeyEnc)
	if ch.CreatedAt == 0 {
		ch.CreatedAt = now
	}
	if ch.UpdatedAt == 0 {
		ch.UpdatedAt = now
	}
}

func deepCopyDirectChannelsLocked(in []DirectChannel) []DirectChannel {
	if len(in) == 0 {
		return nil
	}
	out := make([]DirectChannel, len(in))
	for i, ch := range in {
		out[i] = cloneDirectChannel(ch)
	}
	return out
}

func cloneDirectChannel(ch DirectChannel) DirectChannel {
	cp := ch
	if len(ch.Models) > 0 {
		cp.Models = append([]string{}, ch.Models...)
	}
	cp.ModelMapping = copyStringMap(ch.ModelMapping)
	cp.ExtraHeaders = copyStringMap(ch.ExtraHeaders)
	cp.SellPrice = cloneDirectSellPrice(ch.SellPrice)
	return cp
}

func cloneDirectSellPrice(p DirectSellPrice) DirectSellPrice {
	out := DirectSellPrice{Default: p.Default}
	if len(p.Models) > 0 {
		out.Models = make(map[string]DirectSellPriceRow, len(p.Models))
		for k, v := range p.Models {
			out.Models[k] = v
		}
	}
	return out
}
