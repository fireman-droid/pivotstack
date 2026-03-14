package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kiro-api-proxy/config"
	"net/http"
	"time"
)

// RefreshToken 刷新 access token
func RefreshToken(account *config.Account) (string, string, int64, error) {
	if account.AuthMethod == "social" {
		return refreshSocialToken(account.RefreshToken)
	}
	return refreshOIDCToken(account.RefreshToken, account.ClientID, account.ClientSecret, account.Region)
}

// refreshOIDCToken IdC/Builder ID token 刷新
func refreshOIDCToken(refreshToken, clientID, clientSecret, region string) (string, string, int64, error) {
	if clientID == "" || clientSecret == "" {
		return "", "", 0, fmt.Errorf("OIDC refresh requires clientId and clientSecret")
	}
	if region == "" {
		region = "us-east-1"
	}

	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", region)

	payload := map[string]string{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("refresh failed: %d %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, err
	}

	expiresAt := time.Now().Unix() + int64(result.ExpiresIn)
	return result.AccessToken, result.RefreshToken, expiresAt, nil
}

// refreshSocialToken Social (GitHub/Google) token 刷新
func refreshSocialToken(refreshToken string) (string, string, int64, error) {
	apiURL := "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken"

	fmt.Printf("[SocialRefresh] Calling %s with refreshToken prefix=%s... (len=%d)\n",
		apiURL, refreshToken[:min(20, len(refreshToken))], len(refreshToken))

	payload := map[string]string{
		"refreshToken": refreshToken,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "KiroBatchLoginCLI/1.0.0")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Printf("[SocialRefresh] HTTP request failed: %v\n", err)
		return "", "", 0, fmt.Errorf("social refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		fmt.Printf("[SocialRefresh] HTTP %d: %s\n", resp.StatusCode, string(respBody))
		return "", "", 0, fmt.Errorf("social refresh failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", 0, fmt.Errorf("social refresh parse failed: %v (body: %s)", err, string(respBody))
	}

	// 详细诊断日志
	newRTSame := result.RefreshToken == refreshToken
	fmt.Printf("[SocialRefresh] Success! accessToken len=%d, newRefreshToken len=%d, expiresIn=%d, refreshToken_changed=%v\n",
		len(result.AccessToken), len(result.RefreshToken), result.ExpiresIn, !newRTSame)
	if result.RefreshToken == "" {
		fmt.Printf("[SocialRefresh] WARNING: Server returned EMPTY refreshToken!\n")
	}

	expiresAt := time.Now().Unix() + int64(result.ExpiresIn)
	return result.AccessToken, result.RefreshToken, expiresAt, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
