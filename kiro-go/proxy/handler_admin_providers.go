package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kiro-api-proxy/config"
)

const newAPIProviderBodyLimit = 64 << 10

type publicProvider struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	BaseURL               string  `json:"baseUrl"`
	Username              string  `json:"username"`
	HasPassword           bool    `json:"hasPassword"`
	HasAccessToken        bool    `json:"hasAccessToken"`
	AccessTokenExpiresAt  int64   `json:"accessTokenExpiresAt,omitempty"`
	UserID                int     `json:"userId,omitempty"`
	QuotaPerUnitDollar    float64 `json:"quotaPerUnitDollar"`
	YuanPerUpstreamDollar float64 `json:"yuanPerUpstreamDollar"`
	LastSyncAt            int64   `json:"lastSyncAt,omitempty"`
	LastSyncError         string  `json:"lastSyncError,omitempty"`
	SyncIntervalSec       int     `json:"syncIntervalSec"`
	Enabled               bool    `json:"enabled"`
	ChannelCount          int     `json:"channelCount"`
	TokenCount            int     `json:"tokenCount"`
	GroupCount            int     `json:"groupCount"`
	ModelCount            int     `json:"modelCount"`
}

type newAPIProviderRequest struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	BaseURL               string  `json:"baseUrl"`
	Username              string  `json:"username"`
	Password              string  `json:"password"`
	QuotaPerUnitDollar    float64 `json:"quotaPerUnitDollar"`
	YuanPerUpstreamDollar float64 `json:"yuanPerUpstreamDollar"`
	SyncIntervalSec       int     `json:"syncIntervalSec"`
	Enabled               bool    `json:"enabled"`
}

type newAPIProviderUpdateRequest struct {
	Name                  *string  `json:"name"`
	BaseURL               *string  `json:"baseUrl"`
	Username              *string  `json:"username"`
	Password              *string  `json:"password"`
	QuotaPerUnitDollar    *float64 `json:"quotaPerUnitDollar"`
	YuanPerUpstreamDollar *float64 `json:"yuanPerUpstreamDollar"`
	SyncIntervalSec       *int     `json:"syncIntervalSec"`
	Enabled               *bool    `json:"enabled"`
}

func (h *Handler) apiListProviders(w http.ResponseWriter, r *http.Request) {
	providers := config.GetNewAPIProviders()
	channels := config.GetNewAPIChannels()
	out := make([]publicProvider, 0, len(providers))
	for _, p := range providers {
		var cache *providerCache
		if h.ensureNewAPIManager() != nil {
			cache, _ = h.newapiManager.Cache(p.ID)
		}
		out = append(out, makePublicProvider(p, cache, channels))
	}
	writeAdminJSON(w, http.StatusOK, out)
}

func (h *Handler) apiCreateProvider(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, newAPIProviderBodyLimit)
	var req newAPIProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Username = strings.TrimSpace(req.Username)
	if !adminIDPattern.MatchString(req.ID) {
		writeAdminJSONError(w, http.StatusBadRequest, "provider id must match ^[a-zA-Z0-9_-]{1,32}$")
		return
	}
	if _, exists := config.GetNewAPIProvider(req.ID); exists {
		writeAdminJSONError(w, http.StatusConflict, "provider already exists")
		return
	}
	if _, err := validateBaseURL(req.BaseURL); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "baseUrl must be a valid http(s) URL")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.QuotaPerUnitDollar <= 0 || req.YuanPerUpstreamDollar <= 0 {
		writeAdminJSONError(w, http.StatusBadRequest, "quotaPerUnitDollar and yuanPerUpstreamDollar must be positive")
		return
	}
	if req.SyncIntervalSec <= 0 {
		req.SyncIntervalSec = defaultNewAPISyncSec
	}
	if req.Name == "" {
		req.Name = req.ID
	}

	manager := h.ensureNewAPIManager()
	loginCtx, cancel := context.WithTimeout(r.Context(), newAPISyncTimeout)
	defer cancel()
	login, err := manager.client.Login(loginCtx, req.BaseURL, req.Username, req.Password)
	if err != nil {
		writeAdminJSONError(w, http.StatusUnauthorized, fmt.Sprintf("provider login failed: %v", err))
		return
	}
	passwordEnc, err := config.EncryptSecret(req.Password)
	if err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to encrypt provider password")
		return
	}
	accessTokenEnc, err := config.EncryptSecret(login.AccessToken)
	if err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to encrypt provider access token")
		return
	}

	providers := config.GetNewAPIProviders()
	providers = append(providers, config.NewAPIProvider{
		ID:                    req.ID,
		Name:                  req.Name,
		BaseURL:               req.BaseURL,
		Username:              req.Username,
		PasswordEnc:           passwordEnc,
		QuotaPerUnitDollar:    req.QuotaPerUnitDollar,
		YuanPerUpstreamDollar: req.YuanPerUpstreamDollar,
		AccessTokenEnc:        accessTokenEnc,
		AccessTokenExpiresAt:  login.ExpiresAt,
		UserID:                login.UserID,
		SyncIntervalSec:       req.SyncIntervalSec,
		Enabled:               req.Enabled,
	})
	if err := config.UpdateNewAPIProviders(providers); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to save provider")
		return
	}
	AuditLog("provider_create", adminAuditActor(r), fmt.Sprintf("id=%s baseUrl=%s enabled=%t", req.ID, req.BaseURL, req.Enabled))
	if req.Enabled {
		manager.StartScheduler(req.ID)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), newAPISyncTimeout)
		defer cancel()
		_ = manager.SyncProviderMetadata(ctx, req.ID)
	}()

	p, _ := config.GetNewAPIProvider(req.ID)
	writeAdminJSON(w, http.StatusCreated, makePublicProvider(p, nil, config.GetNewAPIChannels()))
}

func (h *Handler) apiGetProvider(w http.ResponseWriter, r *http.Request, id string) {
	p, ok := config.GetNewAPIProvider(id)
	if !ok {
		writeAdminJSONError(w, http.StatusNotFound, "provider not found")
		return
	}
	var cache *providerCache
	if h.ensureNewAPIManager() != nil {
		cache, _ = h.newapiManager.Cache(id)
	}
	writeAdminJSON(w, http.StatusOK, makePublicProvider(p, cache, config.GetNewAPIChannels()))
}

func (h *Handler) apiUpdateProvider(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, newAPIProviderBodyLimit)
	var req newAPIProviderUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	old, ok := config.GetNewAPIProvider(id)
	if !ok {
		writeAdminJSONError(w, http.StatusNotFound, "provider not found")
		return
	}

	next := old
	baseURLChanged := false
	loginChanged := false
	if req.Name != nil {
		next.Name = strings.TrimSpace(*req.Name)
		if next.Name == "" {
			next.Name = next.ID
		}
	}
	if req.BaseURL != nil {
		baseURL := strings.TrimRight(strings.TrimSpace(*req.BaseURL), "/")
		if _, err := validateBaseURL(baseURL); err != nil {
			writeAdminJSONError(w, http.StatusBadRequest, "baseUrl must be a valid http(s) URL")
			return
		}
		if baseURL != old.BaseURL {
			baseURLChanged = true
			loginChanged = true
			next.BaseURL = baseURL
		}
	}
	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if username == "" {
			writeAdminJSONError(w, http.StatusBadRequest, "username cannot be empty")
			return
		}
		if username != old.Username {
			loginChanged = true
			next.Username = username
		}
	}
	passwordChanged := req.Password != nil && !isMaskedOrEmpty(*req.Password)
	if passwordChanged {
		loginChanged = true
	}
	if req.QuotaPerUnitDollar != nil {
		if *req.QuotaPerUnitDollar <= 0 {
			writeAdminJSONError(w, http.StatusBadRequest, "quotaPerUnitDollar must be positive")
			return
		}
		next.QuotaPerUnitDollar = *req.QuotaPerUnitDollar
	}
	if req.YuanPerUpstreamDollar != nil {
		if *req.YuanPerUpstreamDollar <= 0 {
			writeAdminJSONError(w, http.StatusBadRequest, "yuanPerUpstreamDollar must be positive")
			return
		}
		next.YuanPerUpstreamDollar = *req.YuanPerUpstreamDollar
	}
	if req.SyncIntervalSec != nil {
		if *req.SyncIntervalSec <= 0 {
			writeAdminJSONError(w, http.StatusBadRequest, "syncIntervalSec must be positive")
			return
		}
		next.SyncIntervalSec = *req.SyncIntervalSec
	}
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}

	if loginChanged {
		password := ""
		if passwordChanged {
			password = *req.Password
		} else {
			var err error
			password, err = config.DecryptSecret(old.PasswordEnc)
			if err != nil {
				writeAdminJSONError(w, http.StatusInternalServerError, "failed to decrypt existing provider password")
				return
			}
		}
		loginCtx, cancel := context.WithTimeout(r.Context(), newAPISyncTimeout)
		login, err := h.ensureNewAPIManager().client.Login(loginCtx, next.BaseURL, next.Username, password)
		cancel()
		if err != nil {
			writeAdminJSONError(w, http.StatusUnauthorized, fmt.Sprintf("provider login failed: %v", err))
			return
		}
		if passwordChanged {
			passwordEnc, err := config.EncryptSecret(password)
			if err != nil {
				writeAdminJSONError(w, http.StatusInternalServerError, "failed to encrypt provider password")
				return
			}
			next.PasswordEnc = passwordEnc
		}
		tokenEnc, err := config.EncryptSecret(login.AccessToken)
		if err != nil {
			writeAdminJSONError(w, http.StatusInternalServerError, "failed to encrypt provider access token")
			return
		}
		next.AccessTokenEnc = tokenEnc
		next.AccessTokenExpiresAt = login.ExpiresAt
		next.UserID = login.UserID
		next.LastSyncError = ""
	}

	providers := config.GetNewAPIProviders()
	for i := range providers {
		if providers[i].ID == id {
			providers[i] = next
			break
		}
	}
	if err := config.UpdateNewAPIProviders(providers); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to save provider")
		return
	}
	manager := h.ensureNewAPIManager()
	if baseURLChanged {
		manager.caches.Delete(id)
	}
	if next.Enabled {
		manager.StartScheduler(id)
	} else {
		manager.StopScheduler(id)
	}
	if next.Enabled && (baseURLChanged || loginChanged) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), newAPISyncTimeout)
			defer cancel()
			_ = manager.SyncProviderMetadata(ctx, id)
		}()
	}
	AuditLog("provider_update", adminAuditActor(r), fmt.Sprintf("id=%s enabled=%t", id, next.Enabled))
	writeAdminJSON(w, http.StatusOK, makePublicProvider(next, nil, config.GetNewAPIChannels()))
}

func (h *Handler) apiDeleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	_, ok := config.GetNewAPIProvider(id)
	if !ok {
		writeAdminJSONError(w, http.StatusNotFound, "provider not found")
		return
	}
	purge := strings.EqualFold(r.URL.Query().Get("purge"), "true")
	channels := config.GetNewAPIChannels()
	if purge {
		for _, ch := range channels {
			if ch.ProviderID == id && ch.DeletedAt == 0 {
				writeAdminJSONError(w, http.StatusConflict, "provider still has active channel references")
				return
			}
		}
		providers := config.GetNewAPIProviders()
		nextProviders := providers[:0]
		for _, p := range providers {
			if p.ID != id {
				nextProviders = append(nextProviders, p)
			}
		}
		nextChannels := channels[:0]
		for _, ch := range channels {
			if ch.ProviderID != id {
				nextChannels = append(nextChannels, ch)
			}
		}
		if err := config.UpdateNewAPIProviders(nextProviders); err != nil {
			writeAdminJSONError(w, http.StatusInternalServerError, "failed to delete provider")
			return
		}
		if err := config.UpdateNewAPIChannels(nextChannels); err != nil {
			writeAdminJSONError(w, http.StatusInternalServerError, "failed to delete provider channels")
			return
		}
		h.ensureNewAPIManager().StopScheduler(id)
		h.newapiManager.caches.Delete(id)
		AuditLog("provider_delete", adminAuditActor(r), fmt.Sprintf("id=%s purge=true", id))
		writeAdminJSON(w, http.StatusOK, map[string]any{"success": true, "purged": true})
		return
	}

	disabledChannels := 0
	for i := range channels {
		if channels[i].ProviderID == id && channels[i].Enabled {
			channels[i].Enabled = false
			disabledChannels++
		}
	}
	if err := updateNewAPIProvider(id, func(item *config.NewAPIProvider) {
		item.Enabled = false
	}); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to disable provider")
		return
	}
	if err := config.UpdateNewAPIChannels(channels); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to disable provider channels")
		return
	}
	h.ensureNewAPIManager().StopScheduler(id)
	AuditLog("provider_delete", adminAuditActor(r), fmt.Sprintf("id=%s purge=false disabledChannels=%d", id, disabledChannels))
	writeAdminJSON(w, http.StatusOK, map[string]any{"success": true, "purged": false, "disabledChannels": disabledChannels})
}

func (h *Handler) apiSyncProvider(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := config.GetNewAPIProvider(id); !ok {
		writeAdminJSONError(w, http.StatusNotFound, "provider not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), newAPISyncTimeout)
	defer cancel()
	if err := h.ensureNewAPIManager().SyncProviderMetadata(ctx, id); err != nil {
		writeAdminJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	cache, _ := h.newapiManager.Cache(id)
	writeAdminJSON(w, http.StatusOK, providerSyncSummary(cache))
}

func (h *Handler) apiGetProviderMetadata(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := config.GetNewAPIProvider(id); !ok {
		writeAdminJSONError(w, http.StatusNotFound, "provider not found")
		return
	}
	cache, ok := h.ensureNewAPIManager().Cache(id)
	if !ok {
		writeAdminJSONError(w, http.StatusNotFound, "provider cache not found; sync provider first")
		return
	}
	groups, models, tokens, updatedAt := snapshotProviderCache(cache)
	for i := range tokens {
		tokens[i].Key = maskAPIKey(tokens[i].Key)
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{
		"updatedAt": updatedAt,
		"groups":    groups,
		"models":    models,
		"tokens":    tokens,
	})
}

func (h *Handler) ensureNewAPIManager() *NewAPIManager {
	if h.newapiManager == nil {
		h.newapiManager = NewNewAPIManager(h)
	}
	return h.newapiManager
}

func makePublicProvider(p config.NewAPIProvider, cache *providerCache, channels []config.NewAPIChannel) publicProvider {
	out := publicProvider{
		ID:                    p.ID,
		Name:                  p.Name,
		BaseURL:               p.BaseURL,
		Username:              p.Username,
		HasPassword:           p.PasswordEnc != "",
		HasAccessToken:        p.AccessTokenEnc != "",
		AccessTokenExpiresAt:  p.AccessTokenExpiresAt,
		UserID:                p.UserID,
		QuotaPerUnitDollar:    p.QuotaPerUnitDollar,
		YuanPerUpstreamDollar: p.YuanPerUpstreamDollar,
		LastSyncAt:            p.LastSyncAt,
		LastSyncError:         p.LastSyncError,
		SyncIntervalSec:       p.SyncIntervalSec,
		Enabled:               p.Enabled,
	}
	for _, ch := range channels {
		if ch.ProviderID == p.ID {
			out.ChannelCount++
		}
	}
	if cache != nil {
		groups, models, tokens, _ := snapshotProviderCache(cache)
		out.GroupCount = len(groups)
		out.ModelCount = len(models)
		out.TokenCount = len(tokens)
	}
	return out
}

func snapshotProviderCache(cache *providerCache) ([]config.NewAPIGroup, []config.NewAPIModel, []config.NewAPIToken, int64) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return copyNewAPIGroups(cache.Groups), copyNewAPIModels(cache.Models), copyNewAPITokens(cache.Tokens), cache.UpdatedAt
}

func providerSyncSummary(cache *providerCache) map[string]any {
	if cache == nil {
		return map[string]any{"success": true, "groupCount": 0, "modelCount": 0, "tokenCount": 0}
	}
	groups, models, tokens, updatedAt := snapshotProviderCache(cache)
	return map[string]any{
		"success":    true,
		"updatedAt":  updatedAt,
		"groupCount": len(groups),
		"modelCount": len(models),
		"tokenCount": len(tokens),
	}
}

var _ = time.Second // silence unused import on bare builds
