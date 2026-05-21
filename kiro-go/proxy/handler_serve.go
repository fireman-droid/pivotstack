package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

// resolveApiKey resolves the API key from the request and returns a UserContext.
// When API key validation is disabled, still tries to extract KeyID for log association.
func (h *Handler) resolveApiKey(r *http.Request) (*UserContext, error) {
	authHeader := r.Header.Get("Authorization")
	apiKeyHeader := r.Header.Get("X-Api-Key")

	var providedKey string
	if strings.HasPrefix(authHeader, "Bearer ") {
		providedKey = strings.TrimPrefix(authHeader, "Bearer ")
	} else if apiKeyHeader != "" {
		providedKey = apiKeyHeader
	}

	if !config.IsApiKeyRequired() {
		// API key validation disabled, but still try to associate logs with user
		uc := &UserContext{KeyTier: "pro"}
		if providedKey != "" {
			if info := config.FindApiKey(providedKey); info != nil {
				uc.KeyID = info.ID
				uc.KeyTier = info.Tier
			}
		}
		return uc, nil
	}

	if providedKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	info := config.FindApiKey(providedKey)
	if info == nil {
		return nil, fmt.Errorf("invalid api key")
	}

	// v8: 绑了 user 且 user 被禁 → 拒绝（孤儿 key 不属于任何 user，仍通）
	if u, ok := users.Default().FindByApiKeyID(info.ID); ok && u.Disabled {
		return nil, fmt.Errorf("account disabled")
	}

	// v8: overlay user wallet onto key view (bound non-child keys read from User.Balance via overlay)
	info = users.OverlayWalletOnKey(info)

	// Unified plan validation (timed / credit / hybrid)
	if errType, err := config.ValidateKeyAccess(info); err != nil {
		return nil, fmt.Errorf("%s: %s", errType, err.Error())
	}

	return &UserContext{KeyID: info.ID, KeyTier: info.Tier}, nil
}

// ServeHTTP 路由分发
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 生成 request_id
	requestID := generateRequestID()
	r.Header.Set("X-Request-ID", requestID)
	w.Header().Set("X-Request-ID", requestID)

	// CORS：模型 API（/v1/* /chat/completions /messages）保留 `*` 给 IDE；
	// admin/user API 默认同源 + ENV allowlist（PIVOTSTACK_ALLOWED_ORIGINS）。
	applyCORS(w, r, path)

	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}

	switch {
	case path == "/v1/messages" || path == "/messages" || path == "/anthropic/v1/messages":
		uc, err := h.resolveApiKey(r)
		if err != nil {
			h.sendClaudeError(w, 401, "authentication_error", err.Error())
			return
		}
		h.handleClaudeMessages(w, withUserContext(r, uc))
	case path == "/v1/messages/count_tokens" || path == "/messages/count_tokens":
		uc, err := h.resolveApiKey(r)
		if err != nil {
			h.sendClaudeError(w, 401, "authentication_error", err.Error())
			return
		}
		h.handleCountTokens(w, withUserContext(r, uc))
	case path == "/v1/chat/completions" || path == "/chat/completions":
		uc, err := h.resolveApiKey(r)
		if err != nil {
			h.sendOpenAIError(w, 401, "authentication_error", err.Error())
			return
		}
		h.handleOpenAIChat(w, withUserContext(r, uc))
	case path == "/v1/models" || path == "/models":
		h.handleModels(w, r)
	case path == "/api/event_logging/batch":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"status":"ok"}`))

	case strings.HasPrefix(path, "/admin/api/sse/"):
		h.handleAdminAPI(w, r) // SSE endpoints handled inside admin API router

	case strings.HasPrefix(path, "/admin/api/"):
		h.handleAdminAPI(w, r)

	case strings.HasPrefix(path, "/user/api/"):
		h.handleUserAPI(w, r)

	case path == "/admin" || path == "/admin/":
		// 老 URL 兼容：直接 redirect 到根，让前端 history router 接管
		http.Redirect(w, r, "/", http.StatusMovedPermanently)

	case strings.HasPrefix(path, "/assets/"):
		// Serve Vue static assets (JS, CSS)
		distDir := "web-vue/dist"
		if _, err := os.Stat(distDir); os.IsNotExist(err) {
			distDir = "/app/web-vue/dist"
		}
		http.StripPrefix("/", http.FileServer(http.Dir(distDir))).ServeHTTP(w, r)

	case path == "/health":
		h.handleHealth(w, r)

	case path == "/v1/stats":
		if _, err := h.resolveApiKey(r); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or missing API key"})
			return
		}
		h.handleStats(w, r)

	default:
		// v6 stage 8：旧 admin 前端 URL 301 到新路径（GET/HEAD only）。
		// 必须放在 SPA fallback 之前，避免 index.html 把老 URL 当成新路由渲染。
		if tryLegacyAdminRedirect(w, r) {
			return
		}
		// SPA fallback：任何非 API 非 assets 路径都返回 index.html，
		// 让前端 history mode router 接管（/login / /dashboard / /user/dashboard 等）。
		distDir := "web-vue/dist"
		if _, err := os.Stat(distDir); os.IsNotExist(err) {
			distDir = "/app/web-vue/dist"
		}
		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
	}
}
