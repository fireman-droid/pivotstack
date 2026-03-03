package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// handleModels 模型列表
func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	// 尝试用缓存的真实模型列表
	h.modelsCacheMu.RLock()
	cached := h.cachedModels
	h.modelsCacheMu.RUnlock()

	thinkingSuffix := config.GetThinkingConfig().Suffix

	var models []map[string]interface{}
	if len(cached) > 0 {
		for _, m := range cached {
			supportsImage := modelSupportsImage(m.InputTypes)
			models = append(models, buildModelInfo(m.ModelId, "anthropic", supportsImage))
			// 自动生成 thinking 变体
			models = append(models, buildModelInfo(m.ModelId+thinkingSuffix, "anthropic", supportsImage))
		}
	} else {
		// fallback 静态列表
		models = []map[string]interface{}{
			buildModelInfo("claude-sonnet-4.6", "anthropic", true),
			buildModelInfo("claude-sonnet-4.6"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-opus-4.6", "anthropic", true),
			buildModelInfo("claude-opus-4.6"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-sonnet-4.5", "anthropic", true),
			buildModelInfo("claude-sonnet-4.5"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-sonnet-4", "anthropic", true),
			buildModelInfo("claude-sonnet-4"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-haiku-4.5", "anthropic", true),
			buildModelInfo("claude-haiku-4.5"+thinkingSuffix, "anthropic", true),
			buildModelInfo("claude-opus-4.5", "anthropic", true),
			buildModelInfo("claude-opus-4.5"+thinkingSuffix, "anthropic", true),
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

// refreshModelsCache 从 Kiro API 拉取模型列表并缓存
func (h *Handler) refreshModelsCache() {
	account := h.pool.GetNext()
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
