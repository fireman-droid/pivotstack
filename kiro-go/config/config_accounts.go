package config

import (
	"fmt"
	"strings"
)

func GetAccounts() []Account {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	accounts := make([]Account, len(cfg.Accounts))
	copy(accounts, cfg.Accounts)
	return accounts
}

func GetEnabledAccounts() []Account {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	var accounts []Account
	for _, a := range cfg.Accounts {
		if a.Enabled {
			accounts = append(accounts, a)
		}
	}
	return accounts
}

// FindAccountByEmail returns the index of an account with matching email, or -1.
// Must be called with cfgLock held.
func findAccountByEmailLocked(email string) int {
	if email == "" {
		return -1
	}
	emailLower := strings.ToLower(strings.TrimSpace(email))
	for i, a := range cfg.Accounts {
		if strings.ToLower(strings.TrimSpace(a.Email)) == emailLower {
			return i
		}
	}
	return -1
}

func normalizeIdentityKey(email, authMethod, provider string) string {
	emailLower := strings.ToLower(strings.TrimSpace(email))
	if emailLower == "" {
		return ""
	}
	providerLower := strings.ToLower(strings.TrimSpace(provider))
	authLower := strings.ToLower(strings.TrimSpace(authMethod))
	if authLower == "" && providerLower != "" {
		authLower = "social"
	}
	if authLower == "social" || authLower == "google" || authLower == "github" {
		if providerLower != "" {
			return "social|" + emailLower + "|" + providerLower
		}
		return "social|" + emailLower
	}
	return "idc|" + emailLower
}

func findAccountByIdentityLocked(email, authMethod, provider string) int {
	key := normalizeIdentityKey(email, authMethod, provider)
	if key == "" {
		return -1
	}
	for i, a := range cfg.Accounts {
		if normalizeIdentityKey(a.Email, a.AuthMethod, a.Provider) == key {
			return i
		}
	}
	return -1
}

func FindAccountByEmail(email string) *Account {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	idx := findAccountByEmailLocked(email)
	if idx < 0 {
		return nil
	}
	a := cfg.Accounts[idx]
	return &a
}

func AddAccount(account Account) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if idx := findAccountByIdentityLocked(account.Email, account.AuthMethod, account.Provider); idx >= 0 {
		return fmt.Errorf("duplicate: account with same identity already exists (id: %s)", cfg.Accounts[idx].ID)
	}
	cfg.Accounts = append(cfg.Accounts, account)
	return Save()
}

// AddOrUpdateAccount adds a new account, or updates credentials if one with the same identity exists.
// Identity rule:
//   - social accounts: email + provider
//   - non-social accounts: email
//
// Returns (accountID, isNew, error).
func AddOrUpdateAccount(account Account) (string, bool, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if idx := findAccountByIdentityLocked(account.Email, account.AuthMethod, account.Provider); idx >= 0 {
		existing := &cfg.Accounts[idx]
		if account.AccessToken != "" {
			existing.AccessToken = account.AccessToken
		}
		if account.RefreshToken != "" {
			existing.RefreshToken = account.RefreshToken
		}
		if account.ClientID != "" {
			existing.ClientID = account.ClientID
		}
		if account.ClientSecret != "" {
			existing.ClientSecret = account.ClientSecret
		}
		if account.ExpiresAt > 0 {
			existing.ExpiresAt = account.ExpiresAt
		}
		if account.AuthMethod != "" {
			existing.AuthMethod = account.AuthMethod
		}
		if account.Provider != "" {
			existing.Provider = account.Provider
		}
		existing.Enabled = true
		if existing.BanStatus != "" && existing.BanStatus != "ACTIVE" {
			existing.BanStatus = "ACTIVE"
			existing.BanReason = ""
			existing.BanTime = 0
		}
		return existing.ID, false, Save()
	}
	cfg.Accounts = append(cfg.Accounts, account)
	return account.ID, true, Save()
}

func UpdateAccount(id string, account Account) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i] = account
			return Save()
		}
	}
	return nil
}

// UpdateAccountBanStatus 只更新封禁相关字段，不覆盖 token
// 避免用旧副本覆盖刚刷新的 refreshToken
func UpdateAccountBanStatus(id string, enabled bool, banStatus, banReason string, banTime int64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i].Enabled = enabled
			cfg.Accounts[i].BanStatus = banStatus
			cfg.Accounts[i].BanReason = banReason
			cfg.Accounts[i].BanTime = banTime
			return Save()
		}
	}
	return nil
}

func DeleteAccount(id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts = append(cfg.Accounts[:i], cfg.Accounts[i+1:]...)
			return Save()
		}
	}
	return nil
}

// ImportAccounts imports multiple accounts from a JSON array.
// This function is useful for batch importing accounts from external tools like KAM.
// Duplicate accounts (same ID) are skipped with a warning.
func ImportAccounts(accounts []Account) (int, int, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	imported := 0
	skipped := 0
	existingIDs := make(map[string]bool)
	existingIdentities := make(map[string]bool)

	// Build map of existing account IDs and identities
	for _, a := range cfg.Accounts {
		existingIDs[a.ID] = true
		if key := normalizeIdentityKey(a.Email, a.AuthMethod, a.Provider); key != "" {
			existingIdentities[key] = true
		}
	}

	// Import new accounts (skip duplicates by ID or identity)
	for _, account := range accounts {
		if existingIDs[account.ID] {
			skipped++
			continue
		}
		if key := normalizeIdentityKey(account.Email, account.AuthMethod, account.Provider); key != "" && existingIdentities[key] {
			skipped++
			continue
		}

		// Generate machine ID if not present
		if account.MachineId == "" {
			account.MachineId = GenerateMachineId()
		}

		cfg.Accounts = append(cfg.Accounts, account)
		existingIDs[account.ID] = true
		if key := normalizeIdentityKey(account.Email, account.AuthMethod, account.Provider); key != "" {
			existingIdentities[key] = true
		}
		imported++
	}

	if imported > 0 {
		if err := Save(); err != nil {
			return imported, skipped, err
		}
	}

	return imported, skipped, nil
}

func UpdateAccountToken(id, accessToken, refreshToken string, expiresAt int64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i].AccessToken = accessToken
			if refreshToken != "" {
				cfg.Accounts[i].RefreshToken = refreshToken
			}
			cfg.Accounts[i].ExpiresAt = expiresAt
			return Save()
		}
	}
	return nil
}

// UpdateAccountInfo updates an account's subscription and usage information.
// Called after refreshing account data from Kiro API.
func UpdateAccountInfo(id string, info AccountInfo) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			if info.Email != "" {
				cfg.Accounts[i].Email = info.Email
			}
			if info.UserId != "" {
				cfg.Accounts[i].UserId = info.UserId
			}
			cfg.Accounts[i].SubscriptionType = info.SubscriptionType
			cfg.Accounts[i].SubscriptionTitle = info.SubscriptionTitle
			cfg.Accounts[i].DaysRemaining = info.DaysRemaining
			cfg.Accounts[i].UsageCurrent = info.UsageCurrent
			cfg.Accounts[i].UsageLimit = info.UsageLimit
			cfg.Accounts[i].UsagePercent = info.UsagePercent
			cfg.Accounts[i].NextResetDate = info.NextResetDate
			cfg.Accounts[i].LastRefresh = info.LastRefresh
			cfg.Accounts[i].TrialUsageCurrent = info.TrialUsageCurrent
			cfg.Accounts[i].TrialUsageLimit = info.TrialUsageLimit
			cfg.Accounts[i].TrialUsagePercent = info.TrialUsagePercent
			cfg.Accounts[i].TrialStatus = info.TrialStatus
			cfg.Accounts[i].TrialExpiresAt = info.TrialExpiresAt
			return Save()
		}
	}
	return nil
}
