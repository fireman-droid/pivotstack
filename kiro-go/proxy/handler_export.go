package proxy

import (
	"encoding/json"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// apiExportAccounts 导出账号凭证
func (h *Handler) apiExportAccounts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.IDs = nil
	}

	accounts := config.GetAccounts()

	if len(req.IDs) > 0 {
		idSet := make(map[string]bool)
		for _, id := range req.IDs {
			idSet[id] = true
		}
		var filtered []config.Account
		for _, a := range accounts {
			if idSet[a.ID] {
				filtered = append(filtered, a)
			}
		}
		accounts = filtered
	}

	type ExportCredentials struct {
		AccessToken  string `json:"accessToken"`
		CsrfToken    string `json:"csrfToken"`
		RefreshToken string `json:"refreshToken"`
		ClientID     string `json:"clientId,omitempty"`
		ClientSecret string `json:"clientSecret,omitempty"`
		Region       string `json:"region,omitempty"`
		ExpiresAt    int64  `json:"expiresAt"`
		AuthMethod   string `json:"authMethod,omitempty"`
		Provider     string `json:"provider,omitempty"`
	}
	type ExportSubscription struct {
		Type  string `json:"type"`
		Title string `json:"title,omitempty"`
	}
	type ExportUsage struct {
		Current     float64 `json:"current"`
		Limit       float64 `json:"limit"`
		PercentUsed float64 `json:"percentUsed"`
		LastUpdated int64   `json:"lastUpdated"`
	}
	type ExportAccount struct {
		ID           string             `json:"id"`
		Email        string             `json:"email"`
		Nickname     string             `json:"nickname,omitempty"`
		Idp          string             `json:"idp"`
		UserId       string             `json:"userId,omitempty"`
		MachineId    string             `json:"machineId,omitempty"`
		Credentials  ExportCredentials  `json:"credentials"`
		Subscription ExportSubscription `json:"subscription"`
		Usage        ExportUsage        `json:"usage"`
		Tags         []string           `json:"tags"`
		Status       string             `json:"status"`
		CreatedAt    int64              `json:"createdAt"`
		LastUsedAt   int64              `json:"lastUsedAt"`
	}
	type ExportData struct {
		Version    string          `json:"version"`
		ExportedAt int64           `json:"exportedAt"`
		Accounts   []ExportAccount `json:"accounts"`
		Groups     []interface{}   `json:"groups"`
		Tags       []interface{}   `json:"tags"`
	}

	exportAccounts := make([]ExportAccount, 0, len(accounts))
	for _, a := range accounts {
		idp := a.Provider
		if idp == "" {
			if a.AuthMethod == "social" {
				idp = "Google"
			} else {
				idp = "BuilderId"
			}
		}
		authMethod := a.AuthMethod
		if authMethod == "idc" {
			authMethod = "IdC"
		}
		subType := "Free"
		rawType := strings.ToUpper(a.SubscriptionType)
		if strings.Contains(rawType, "PRO_PLUS") || strings.Contains(rawType, "PROPLUS") {
			subType = "Pro_Plus"
		} else if strings.Contains(rawType, "PRO") {
			subType = "Pro"
		} else if strings.Contains(rawType, "POWER") {
			subType = "Pro_Plus"
		}

		exportAccounts = append(exportAccounts, ExportAccount{
			ID: a.ID, Email: a.Email, Nickname: a.Nickname,
			Idp: idp, UserId: a.UserId, MachineId: a.MachineId,
			Credentials: ExportCredentials{
				AccessToken: a.AccessToken, CsrfToken: "", RefreshToken: a.RefreshToken,
				ClientID: a.ClientID, ClientSecret: a.ClientSecret, Region: a.Region,
				ExpiresAt: a.ExpiresAt * 1000, AuthMethod: authMethod, Provider: a.Provider,
			},
			Subscription: ExportSubscription{Type: subType, Title: a.SubscriptionTitle},
			Usage: ExportUsage{
				Current: a.UsageCurrent, Limit: a.UsageLimit,
				PercentUsed: a.UsagePercent, LastUpdated: time.Now().UnixMilli(),
			},
			Tags: []string{}, Status: "active",
			CreatedAt: time.Now().UnixMilli(), LastUsedAt: time.Now().UnixMilli(),
		})
	}

	json.NewEncoder(w).Encode(ExportData{
		Version: config.Version, ExportedAt: time.Now().UnixMilli(),
		Accounts: exportAccounts, Groups: []interface{}{}, Tags: []interface{}{},
	})
}
