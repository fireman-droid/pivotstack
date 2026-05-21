package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func newReconcileAdminTestHandler(t *testing.T) *Handler {
	t.Helper()
	newAPITestConfig(t)
	preexistingKeys := config.GetAllApiKeys()
	for _, k := range preexistingKeys {
		_ = config.DeleteApiKey(k.ID)
	}
	t.Cleanup(func() {
		current := config.GetAllApiKeys()
		for _, k := range current {
			_ = config.DeleteApiKey(k.ID)
		}
		for _, k := range preexistingKeys {
			_ = config.AddApiKey(k)
		}
	})
	h := tokenTestHandler()
	h.newapiReconciler = NewNewAPIReconciler(nil)
	return h
}

func TestGetReconcileStatusReturnsPendingAndGlobalDebt(t *testing.T) {
	h := newReconcileAdminTestHandler(t)
	if err := config.AddApiKey(config.ApiKeyInfo{ID: "k1", Key: "sk-k1", Enabled: true, Plan: "credit", DebtUSD: 0.01, CreatedAt: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	if err := config.AddApiKey(config.ApiKeyInfo{ID: "k2", Key: "sk-k2", Enabled: true, Plan: "credit", DebtUSD: 0.003, CreatedAt: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	if !h.newapiReconciler.Enqueue("apijing", newReconcilePending("k1", "req-1", 2, 0)) {
		t.Fatal("enqueue returned false")
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/newapi/reconcile-status", nil)
	rr := httptest.NewRecorder()
	h.apiGetNewAPIReconcileStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp newAPIReconcileStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	tokenTestAssertFloat(t, resp.GlobalDebtUSD, 0.013)
	if len(resp.Providers) != 1 {
		t.Fatalf("providers len = %d", len(resp.Providers))
	}
	if resp.Providers[0].ProviderID != "apijing" || resp.Providers[0].PendingCount != 1 {
		t.Fatalf("provider status wrong: %+v", resp.Providers[0])
	}
}

func TestPostReconcileRetryResetsAttempt(t *testing.T) {
	h := newReconcileAdminTestHandler(t)
	p := newReconcilePending("k1", "req-1", 2, 0)
	if !h.newapiReconciler.Enqueue("apijing", p) {
		t.Fatal("enqueue returned false")
	}
	q, _ := h.newapiReconciler.queue("apijing")
	q.mu.Lock()
	q.pending["req-1"].Attempt = 4
	q.pending["req-1"].LastErr = "no upstream log match"
	q.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/reconcile-status/retry/req-1", nil)
	rr := httptest.NewRecorder()
	h.apiRetryNewAPIReconcile(rr, req, "req-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	q.mu.Lock()
	got := q.pending["req-1"]
	q.mu.Unlock()
	if got == nil {
		t.Fatal("pending entry missing")
	}
	if got.Attempt != 0 || got.LastErr != "" {
		t.Fatalf("retry did not reset attempt/error: %+v", got)
	}
}

func TestPostReconcileRetryRequestIDNotFound404(t *testing.T) {
	h := newReconcileAdminTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/reconcile-status/retry/missing", nil)
	rr := httptest.NewRecorder()
	h.apiRetryNewAPIReconcile(rr, req, "missing")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}
