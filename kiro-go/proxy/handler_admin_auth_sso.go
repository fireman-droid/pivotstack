package proxy

import (
	"encoding/json"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"time"
)

func (h *Handler) apiStartIamSso(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartUrl string `json:"startUrl"`
		Region   string `json:"region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.StartUrl == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "startUrl is required"})
		return
	}
	sessionID, authorizeUrl, expiresIn, err := auth.StartIamSsoLogin(req.StartUrl, req.Region)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId": sessionID, "authorizeUrl": authorizeUrl, "expiresIn": expiresIn,
	})
}

func (h *Handler) apiCompleteIamSso(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID   string `json:"sessionId"`
		CallbackUrl string `json:"callbackUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	accessToken, refreshToken, clientID, clientSecret, region, expiresIn, err := auth.CompleteIamSsoLogin(req.SessionID, req.CallbackUrl)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	email, _, _ := auth.GetUserInfo(accessToken)
	account := config.Account{
		ID: auth.GenerateAccountID(), Email: email,
		AccessToken: accessToken, RefreshToken: refreshToken,
		ClientID: clientID, ClientSecret: clientSecret,
		AuthMethod: "idc", Region: region,
		ExpiresAt: time.Now().Unix() + int64(expiresIn),
		Enabled:   true, MachineId: config.GenerateMachineId(),
	}
	id, isNew, err := config.AddOrUpdateAccount(account)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "isNew": isNew,
		"account": map[string]interface{}{"id": id, "email": account.Email},
	})
}
