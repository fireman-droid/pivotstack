package proxy

import (
	"net/http"
	"strings"

	"kiro-api-proxy/config"
)

// globalDebtWarningThresholdUSD：globalDebtUsd 超过此阈值前端显示红色警告。
// $10 已经能买 ~50 万 token；超过它意味着系统化漏扣，admin 需要排查。
const globalDebtWarningThresholdUSD = 10.0

type newAPIReconcileStatusResponse struct {
	Providers         []reconcileProviderStatus `json:"providers"`
	GlobalDebtUSD     float64                   `json:"globalDebtUsd"`
	GlobalDebtWarning bool                      `json:"globalDebtWarning"`
}

func (h *Handler) apiGetNewAPIReconcileStatus(w http.ResponseWriter, _ *http.Request) {
	providers := []reconcileProviderStatus{}
	if h != nil && h.newapiReconciler != nil {
		providers = h.newapiReconciler.SnapshotStatus()
	}
	debt := totalApiKeyDebtUSD()
	writeJSONStatus(w, http.StatusOK, newAPIReconcileStatusResponse{
		Providers:         providers,
		GlobalDebtUSD:     debt,
		GlobalDebtWarning: debt >= globalDebtWarningThresholdUSD,
	})
}

func (h *Handler) apiRetryNewAPIReconcile(w http.ResponseWriter, _ *http.Request, requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || h == nil || h.newapiReconciler == nil || !h.newapiReconciler.Retry(requestID) {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "request not found"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"requestId": requestID,
	})
}

func totalApiKeyDebtUSD() float64 {
	keys := config.GetAllApiKeys()
	var total float64
	for _, k := range keys {
		total += k.DebtUSD
	}
	return total
}
