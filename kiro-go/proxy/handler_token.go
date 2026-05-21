package proxy

import (
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"time"
)

// ensureValidToken 确保 token 有效，过期前 30 分钟自动刷新
func (h *Handler) ensureValidToken(account *config.Account) error {
	return h.refreshAccountToken(account, false)
}

// forceRefreshToken 无视到期时间强制刷新（用于上游返回 INVALID_MODEL_ID 等"token 看着没过期但其实已废"的情形）
func (h *Handler) forceRefreshToken(account *config.Account) error {
	return h.refreshAccountToken(account, true)
}

func (h *Handler) refreshAccountToken(account *config.Account, force bool) error {
	if !force && (account.ExpiresAt == 0 || time.Now().Unix() < account.ExpiresAt-tokenRefreshLeadSec) {
		return nil
	}

	tag := "ensureValidToken"
	if force {
		tag = "forceRefreshToken"
	}
	fmt.Printf("[%s] Refreshing token for %s (expiresAt=%d, now=%d)\n",
		tag, account.Email, account.ExpiresAt, time.Now().Unix())

	accessToken, refreshToken, expiresAt, err := auth.RefreshToken(account)
	if err != nil {
		fmt.Printf("[%s] Token refresh FAILED for %s: %v\n", tag, account.Email, err)
		return err
	}

	h.pool.UpdateToken(account.ID, accessToken, refreshToken, expiresAt)
	account.AccessToken = accessToken
	if refreshToken != "" {
		account.RefreshToken = refreshToken
	}
	account.ExpiresAt = expiresAt

	config.UpdateAccountToken(account.ID, accessToken, refreshToken, expiresAt)
	fmt.Printf("[%s] Token refreshed OK for %s, new expiresAt=%d\n", tag, account.Email, expiresAt)

	return nil
}
