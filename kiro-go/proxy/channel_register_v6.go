package proxy

import (
	"fmt"
	"os"
	"strings"

	"kiro-api-proxy/config"
)

// registerNewAPIChannels 把启用且 provider 在线的 NewAPIChannel 注册到 router。
// 失败的单个 channel 跳过；不阻断 router 启动。
func registerNewAPIChannels(r *ChannelRouter, channels []config.NewAPIChannel, providers []config.NewAPIProvider) {
	if len(channels) == 0 {
		return
	}
	providersByID := make(map[string]config.NewAPIProvider, len(providers))
	for _, p := range providers {
		providersByID[p.ID] = p
	}
	for _, nc := range channels {
		if !nc.Enabled || nc.DeletedAt > 0 {
			continue
		}
		p, ok := providersByID[nc.ProviderID]
		if !ok || !p.Enabled {
			continue
		}
		ch := newNewAPIRuntimeChannel(nc, p)
		r.channels = append(r.channels, ch)
		r.byID[ch.ID()] = ch
	}
}

// registerDirectChannels 把 v6 自营渠道适配成 ChannelConfig，复用 newOpenAIChannel / newKiroChannel。
// APIKeyEnc 在 runtime decrypt；decrypt 失败的 openai channel 跳过并记 stderr。
// ID 加 "direct:" 前缀避免与 v4 ChannelConfig.ID 撞表。
func registerDirectChannels(r *ChannelRouter, channels []config.DirectChannel, exec KiroExecutor) {
	for _, dc := range channels {
		if !dc.Enabled || dc.DeletedAt > 0 {
			continue
		}
		cfg, ok := directRuntimeChannelConfig(dc)
		if !ok {
			continue
		}
		var ch Channel
		switch cfg.Type {
		case "openai":
			ch = newOpenAIChannel(cfg)
		case "kiro":
			ch = newKiroChannel(cfg, exec)
		default:
			continue
		}
		r.channels = append(r.channels, ch)
		r.byID[ch.ID()] = ch
	}
}

func directRuntimeChannelConfig(dc config.DirectChannel) (config.ChannelConfig, bool) {
	id := strings.TrimSpace(dc.ID)
	if id == "" {
		return config.ChannelConfig{}, false
	}
	typ := strings.ToLower(strings.TrimSpace(dc.Type))
	if typ != "openai" && typ != "kiro" {
		return config.ChannelConfig{}, false
	}
	cfg := config.ChannelConfig{
		ID:           "direct:" + id,
		Type:         typ,
		BaseURL:      strings.TrimSpace(dc.BaseURL),
		Models:       append([]string{}, dc.Models...),
		ModelAliases: dc.ModelMapping,
		ExtraHeaders: dc.ExtraHeaders,
		Enabled:      true,
	}
	if typ == "openai" {
		if cfg.BaseURL == "" {
			fmt.Fprintf(os.Stderr, "[router] direct openai channel %s baseUrl missing; skipping\n", cfg.ID)
			return config.ChannelConfig{}, false
		}
		cfg.APIKey = directChannelOpenAIAPIKey(dc.APIKeyEnc)
		if cfg.APIKey == "" {
			fmt.Fprintf(os.Stderr, "[router] direct openai channel %s apiKey unavailable; skipping\n", cfg.ID)
			return config.ChannelConfig{}, false
		}
	}
	return cfg, true
}

// directChannelOpenAIAPIKey 解密 DirectChannel 的 APIKeyEnc；空或失败返 ""。
// 调用方负责把"空 key + openai type"当作 skip 信号。
func directChannelOpenAIAPIKey(enc string) string {
	enc = strings.TrimSpace(enc)
	if enc == "" {
		return ""
	}
	plain, err := config.DecryptSecret(enc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[router] direct openai apiKey decrypt failed: %v\n", err)
		return ""
	}
	return strings.TrimSpace(plain)
}
