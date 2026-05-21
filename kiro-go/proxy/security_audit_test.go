package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func securityAuditHandler() *Handler {
	h := tokenTestHandler()
	h.adminSessions = newAdminSessionStore()
	return h
}

func securityAuditSessionCookie(t *testing.T, h *Handler) *http.Cookie {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://admin.example/admin", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if _, err := h.adminSessions.Create(rr, req); err != nil {
		t.Fatalf("Create admin session: %v", err)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == adminSessionCookieName {
			return c
		}
	}
	t.Fatalf("admin session cookie %q not set", adminSessionCookieName)
	return nil
}

func securityAuditResetRedeemLimiter(t *testing.T) {
	t.Helper()
	redeemAttempts = sync.Map{}
	t.Cleanup(func() { redeemAttempts = sync.Map{} })
}

func securityAuditFileSize(t *testing.T, name string) int64 {
	t.Helper()
	path := filepath.Join(config.GetDataDir(), name)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	return info.Size()
}

func securityAuditFileTail(t *testing.T, name string, before int64) string {
	t.Helper()
	path := filepath.Join(config.GetDataDir(), name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	start := int(before)
	if start < 0 || start > len(data) {
		start = 0
	}
	return string(data[start:])
}

func TestSecurityAudit_AdminUnsafeMethodsRequireCSRFAndRejectPasswordQuery(t *testing.T) {
	h := securityAuditHandler()
	cookie := securityAuditSessionCookie(t, h)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/settings", strings.NewReader(`{"apiKey":"","requireApiKey":false}`))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	h.handleAdminAPI(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("unsafe admin request without CSRF status=%d, want %d body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/settings?password=secret", nil)
	rr = httptest.NewRecorder()
	h.handleAdminAPI(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("password query status=%d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestSecurityAudit_AdminSessionCookieHasBrowserSecurityAttributes(t *testing.T) {
	h := securityAuditHandler()
	cookie := securityAuditSessionCookie(t, h)
	if !cookie.HttpOnly {
		t.Fatal("admin session cookie must be HttpOnly")
	}
	if !cookie.Secure {
		t.Fatal("admin session cookie must be Secure when request is HTTPS/forwarded HTTPS")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("admin session SameSite=%v, want Strict", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Fatalf("admin session path=%q, want /", cookie.Path)
	}
	if cookie.MaxAge <= 0 {
		t.Fatalf("admin session MaxAge=%d, want positive", cookie.MaxAge)
	}
}

func TestSecurityAudit_AdminLoginLimiterIgnoresSpoofedForwardedFor(t *testing.T) {
	h := securityAuditHandler()
	wrongPassword := "not-the-admin-password-" + tokenTestID("login")

	for i := 0; i < adminLoginMaxFailures; i++ {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(fmt.Sprintf(`{"password":%q}`, wrongPassword)))
		req.RemoteAddr = "203.0.113.44:55000"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		rr := httptest.NewRecorder()
		h.apiAdminLogin(rr, req)
		if i < adminLoginMaxFailures-1 {
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d status=%d, want %d body=%s", i+1, rr.Code, http.StatusUnauthorized, rr.Body.String())
			}
			continue
		}
		if rr.Code != http.StatusLocked {
			t.Fatalf("lockout attempt status=%d, want %d body=%s", rr.Code, http.StatusLocked, rr.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(fmt.Sprintf(`{"password":%q}`, wrongPassword)))
	req.RemoteAddr = "203.0.113.44:55000"
	req.Header.Set("X-Forwarded-For", "198.51.100.250")
	rr := httptest.NewRecorder()
	h.apiAdminLogin(rr, req)
	if rr.Code != http.StatusLocked {
		t.Fatalf("spoofed XFF after lockout status=%d, want %d body=%s", rr.Code, http.StatusLocked, rr.Body.String())
	}
}

func TestSecurityAudit_DisabledApiKeyCannotUseUserPortal(t *testing.T) {
	h := tokenTestHandler()
	id := tokenTestID("security-disabled-key")
	key := config.ApiKeyInfo{
		ID:        id,
		Key:       "sk-" + id,
		Plan:      "credit",
		Enabled:   false,
		Balance:   10,
		CreatedAt: time.Now().Unix(),
	}
	if err := config.AddApiKey(key); err != nil {
		t.Fatalf("AddApiKey: %v", err)
	}
	t.Cleanup(func() { _ = config.DeleteApiKey(id) })

	req := httptest.NewRequest(http.MethodGet, "/user/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rr := httptest.NewRecorder()
	h.handleUserAPI(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("disabled key user portal status=%d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestSecurityAudit_UserRedeemRateLimitUsesRemoteAddrWhenForwardedForIsUntrusted(t *testing.T) {
	securityAuditResetRedeemLimiter(t)
	h := tokenTestHandler()
	keyID := tokenTestAddKey(t, "credit", 1, 0, 0)
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		t.Fatalf("key %q not found", keyID)
	}

	missingCode := tokenTestID("missing-redeem")
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/user/api/redeem", strings.NewReader(fmt.Sprintf(`{"code":%q}`, missingCode)))
		req.RemoteAddr = "203.0.113.77:40000"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		rr := httptest.NewRecorder()
		h.handleUserRedeem(rr, req, info)
		if i < 5 {
			if rr.Code == http.StatusTooManyRequests {
				t.Fatalf("attempt %d was rate limited before threshold: body=%s", i+1, rr.Body.String())
			}
			continue
		}
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("sixth redeem attempt status=%d, want %d; X-Forwarded-For must not bypass RemoteAddr limit", rr.Code, http.StatusTooManyRequests)
		}
	}
}

func TestSecurityAudit_ChildKeyCannotRedeemActivationCode(t *testing.T) {
	securityAuditResetRedeemLimiter(t)
	h := tokenTestHandler()
	parentID := tokenTestAddKey(t, "credit", 5, 0, 0)
	childID := tokenTestID("security-child-key")
	child := config.ApiKeyInfo{
		ID:          childID,
		Key:         "sk-" + childID,
		Plan:        "credit",
		Enabled:     true,
		ParentKeyID: parentID,
		CreatedAt:   time.Now().Unix(),
	}
	if err := config.AddApiKey(child); err != nil {
		t.Fatalf("AddApiKey(child): %v", err)
	}
	t.Cleanup(func() { _ = config.DeleteApiKey(childID) })

	code := tokenTestID("SEC-CODE")
	if err := config.AddActivationCode(config.ActivationCode{
		Code:      code,
		Type:      "balance",
		Amount:    10,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	t.Cleanup(func() { _ = config.DeleteActivationCode(code) })

	req := httptest.NewRequest(http.MethodPost, "/user/api/redeem", strings.NewReader(fmt.Sprintf(`{"code":%q}`, code)))
	rr := httptest.NewRecorder()
	h.handleUserRedeem(rr, req, &child)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("child redeem status=%d, want %d body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}

	found := false
	for _, ac := range config.GetActivationCodes() {
		if ac.Code == code {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("child redeem consumed activation code %q", code)
	}
}

func TestSecurityAudit_UserLogsFilterByExactApiKeyAndSanitizeUpstreamFields(t *testing.T) {
	h := tokenTestHandler()
	parentID := tokenTestID("security-parent-log")
	childID := tokenTestID("security-child-log")
	h.callLogs = []CallLog{
		{
			ApiKeyID:        parentID,
			Timestamp:       time.Now().Unix(),
			OriginalModel:   "parent-model",
			ActualModel:     "upstream-parent",
			Account:         "parent-account@example.com",
			UpstreamCredits: 123,
			Status:          "success",
		},
		{
			ApiKeyID:        childID,
			Timestamp:       time.Now().Unix(),
			OriginalModel:   "child-model",
			ActualModel:     "upstream-child",
			Account:         "child-account@example.com",
			UpstreamCredits: 456,
			Status:          "success",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/user/api/logs?date=all", nil)
	rr := httptest.NewRecorder()
	h.handleUserLogs(rr, req, &config.ApiKeyInfo{ID: childID, Key: "sk-" + childID, Enabled: true})
	if rr.Code != http.StatusOK {
		t.Fatalf("user logs status=%d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Logs  []CallLog `json:"logs"`
		Total int       `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode user logs: %v", err)
	}
	if resp.Total != 1 || len(resp.Logs) != 1 || resp.Logs[0].ApiKeyID != childID {
		t.Fatalf("user logs leaked another key or missed child log: %+v", resp)
	}
	got := resp.Logs[0]
	if got.Account != "" || got.ActualModel != got.OriginalModel || got.UpstreamCredits != 0 {
		t.Fatalf("user log was not sanitized: %+v", got)
	}
}

func TestSecurityAudit_RechargeRecordConcurrentAppendKeepsEveryRecordValid(t *testing.T) {
	const writers = 25
	keyID := tokenTestID("security-recharge-key")
	codePrefix := tokenTestID("SEC-RECH")

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			appendRechargeRecord(RechargeRecord{
				Time:      time.Now().Format("01-02 15:04:05"),
				Timestamp: time.Now().Unix(),
				KeyID:     keyID,
				Type:      "admin_balance",
				Code:      fmt.Sprintf("%s-%02d", codePrefix, i),
				AmountUSD: 1,
				AmountCNY: config.CNYFromVirtualUSD(1.0),
				Operator:  "test",
			})
		}()
	}
	wg.Wait()

	records, total := readRechargeRecords(keyID, 1, 500)
	if total != writers || len(records) != writers {
		t.Fatalf("recharge records total=%d len=%d, want %d", total, len(records), writers)
	}
	seen := make(map[string]bool, writers)
	for _, rec := range records {
		if !strings.HasPrefix(rec.Code, codePrefix+"-") {
			t.Fatalf("unexpected recharge record mixed into filtered result: %+v", rec)
		}
		if seen[rec.Code] {
			t.Fatalf("duplicate recharge record code %q", rec.Code)
		}
		seen[rec.Code] = true
	}
}

func TestSecurityAudit_ApiKeyDeleteMustBeAudited(t *testing.T) {
	h := tokenTestHandler()
	keyID := tokenTestAddKey(t, "credit", 1, 0, 0)
	before := securityAuditFileSize(t, "audit.log")

	rr := httptest.NewRecorder()
	h.apiDeleteApiKey(rr, httptest.NewRequest(http.MethodDelete, "/admin/api/apikeys/"+keyID, nil), keyID)
	if rr.Code != http.StatusOK {
		t.Fatalf("apiDeleteApiKey status=%d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	tail := securityAuditFileTail(t, "audit.log", before)
	if !strings.Contains(tail, "apikey_delete") {
		t.Fatalf("api key delete must be audited; audit tail=%q", tail)
	}
}
