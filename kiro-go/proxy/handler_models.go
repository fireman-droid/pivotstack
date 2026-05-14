package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// handleModels 模型列表（根据账号类型返回对应模型）
func (h *Handler) handleModels(w http.ResponseWriter, _ *http.Request) {
	thinkingSuffix := config.GetThinkingConfig().Suffix

	// v3：channels 配置时从渠道列表构建（每个渠道贡献自己的 models）
	channels := config.GetChannels()
	if len(channels) > 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data":   buildModelsFromChannels(channels, thinkingSuffix),
		})
		return
	}

	// 判断当前可用账号的主要类型
	accounts := h.pool.GetAllAccounts()
	hasFree := false
	hasPro := false
	for _, acc := range accounts {
		switch acc.SubscriptionType {
		case "FREE", "":
			hasFree = true
		default: // PRO, PRO_PLUS, POWER
			hasPro = true
		}
	}

	var models []map[string]interface{}

	// 根据有哪些账号类型返回对应模型（注意：sonnet 只到 4.6，没有 4.7）
	if hasPro {
		models = append(models,
			buildModelInfo("claude-opus-4.7", "anthropic", true),
			buildModelInfo("claude-opus-4.7"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-opus-4.6", "anthropic", true),
			buildModelInfo("claude-opus-4.6"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-sonnet-4.6", "anthropic", true),
			buildModelInfo("claude-sonnet-4.6"+thinkingSuffix, "anthropic", true),
		)
	}
	if hasFree {
		models = append(models,
			buildModelInfo("claude-sonnet-4.5", "anthropic", true),
			buildModelInfo("claude-sonnet-4.5"+thinkingSuffix, "anthropic", true),
		)
	}

	// 如果没有任何账号，返回默认列表
	if len(models) == 0 {
		models = []map[string]interface{}{
			buildModelInfo("claude-opus-4.7", "anthropic", true),
			buildModelInfo("claude-opus-4.7"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-opus-4.6", "anthropic", true),
			buildModelInfo("claude-opus-4.6"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-sonnet-4.6", "anthropic", true),
			buildModelInfo("claude-sonnet-4.6"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-sonnet-4.5", "anthropic", true),
			buildModelInfo("claude-sonnet-4.5"+thinkingSuffix, "anthropic", true),
		}
	}

	// 添加别名模型
	models = append(models,
		buildModelInfo("auto", "kiro-proxy", true),
		buildModelInfo("gpt-4o", "kiro-proxy", true),
		buildModelInfo("gpt-4", "kiro-proxy", true),
	)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

func modelSupportsImage(inputTypes []string) bool {
	for _, t := range inputTypes {
		lt := strings.ToLower(t)
		if strings.Contains(lt, "image") || strings.Contains(lt, "vision") {
			return true
		}
	}
	return false
}

func buildModelInfo(id, ownedBy string, supportsImage bool) map[string]interface{} {
	modalities := []string{"text"}
	if supportsImage {
		modalities = append(modalities, "image")
	}
	modalitiesMap := map[string][]string{
		"input":  modalities,
		"output": []string{"text"},
	}

	return map[string]interface{}{
		"id":               id,
		"object":           "model",
		"owned_by":         ownedBy,
		"supports_image":   supportsImage,
		"input_modalities": modalities,
		"modalities":       modalitiesMap,
		"capabilities": map[string]bool{
			"vision":       supportsImage,
			"image":        supportsImage,
			"image_vision": supportsImage,
		},
		"info": map[string]interface{}{
			"meta": map[string]interface{}{
				"capabilities": map[string]bool{
					"vision":       supportsImage,
					"image_vision": supportsImage,
				},
			},
		},
	}
}

// buildModelsFromChannels v3：按 channels 配置构建 /v1/models 列表。
// 每个 enabled 渠道贡献其 Models 列表，多渠道支持同一 model 时去重（保留首次出现）。
// 同时给 Kiro 渠道的每个 model 额外添加 -thinking 变体（保持与旧行为一致）。
func buildModelsFromChannels(channels []config.ChannelConfig, thinkingSuffix string) []map[string]interface{} {
	seen := make(map[string]struct{})
	var models []map[string]interface{}
	for _, c := range channels {
		if !c.Enabled {
			continue
		}
		ownedBy := "kiro-proxy"
		if c.Type != "" {
			ownedBy = c.Type
		}
		for _, m := range c.Models {
			key := strings.ToLower(strings.TrimSpace(m))
			if key == "" {
				continue
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			models = append(models, buildModelInfo(m, ownedBy, true))
			if strings.ToLower(c.Type) == "kiro" && thinkingSuffix != "" {
				thinkID := m + thinkingSuffix
				thinkKey := strings.ToLower(thinkID)
				if _, dup := seen[thinkKey]; !dup {
					seen[thinkKey] = struct{}{}
					models = append(models, buildModelInfo(thinkID, ownedBy, true))
				}
			}
		}
	}
	return models
}

// refreshModelsCache 从 Kiro API 拉取模型列表并缓存
func (h *Handler) refreshModelsCache() {
	// Try a handful of accounts to find a kiro-backend one — relay accounts
	// can't enumerate Kiro's model list. Loop limit prevents starvation if
	// the pool happens to be all-relay.
	var account *config.Account
	for i := 0; i < 8; i++ {
		acc := h.pool.GetNext()
		if acc == nil {
			return
		}
		if true {
			account = acc
			break
		}
		// release relay account immediately so we don't pin its in-flight slot
		h.pool.ReleaseAccount(acc.ID)
	}
	if account == nil {
		return
	}

	// 确保 token 有效
	if err := h.ensureValidToken(account); err != nil {
		return
	}

	models, err := ListAvailableModels(account)
	if err != nil {
		fmt.Printf("[ModelsCache] Failed to refresh: %v\n", err)
		return
	}

	if len(models) > 0 {
		h.modelsCacheMu.Lock()
		h.cachedModels = models
		h.modelsCacheTime = time.Now().Unix()
		h.modelsCacheMu.Unlock()
		fmt.Printf("[ModelsCache] Cached %d models\n", len(models))
	}
}
