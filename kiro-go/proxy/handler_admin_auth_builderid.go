package proxy

import (
	"encoding/json"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"time"
)

func (h *Handler) apiStartBuilderIdLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Region string `json:"region"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	session, err := auth.StartBuilderIdLogin(req.Region)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId": session.ID, "userCode": session.UserCode,
		"verificationUri": session.VerificationUri, "interval": session.Interval,
	})
}

func (h *Handler) apiPollBuilderIdAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	accessToken, refreshToken, clientID, clientSecret, region, expiresIn, status, err := auth.PollBuilderIdAuth(req.SessionID)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	if status == "pending" || status == "slow_down" {
		interval := 5
		if session := auth.GetBuilderIdSession(req.SessionID); session != nil {
			interval = session.Interval
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true, "completed": false, "status": status, "interval": interval,
		})
		return
	}
	email, _, _ := auth.GetUserInfo(accessToken)
	account := config.Account{
		ID: auth.GenerateAccountID(), Email: email,
		AccessToken: accessToken, RefreshToken: refreshToken,
		ClientID: clientID, ClientSecret: clientSecret,
		AuthMethod: "idc", Provider: "BuilderId", Region: region,
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
		"success": true, "completed": true, "isNew": isNew,
		"account": map[string]interface{}{"id": id, "email": account.Email},
	})
}
