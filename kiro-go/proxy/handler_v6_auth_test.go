// proxy/handler_v6_auth_test.go
package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

var v6TestSeq uint64

func seedV6Test(t *testing.T) *Handler {
	t.Helper()

	newAPITestConfig(t)

	preexistingKeys := config.GetAllApiKeys()
	for _, k := range preexistingKeys {
		_ = config.DeleteApiKey(k.ID)
	}

	oldAllowSelfRegister := users.AllowSelfRegister
	oldRequireActivationCode := users.RequireActivationCode
	users.AllowSelfRegister = true
	users.RequireActivationCode = false

	t.Cleanup(func() {
		users.AllowSelfRegister = oldAllowSelfRegister
		users.RequireActivationCode = oldRequireActivationCode

		current := config.GetAllApiKeys()
		for _, k := range current {
			_ = config.DeleteApiKey(k.ID)
		}
		for _, k := range preexistingKeys {
			if err := config.AddApiKey(k); err != nil {
				t.Errorf("restore api key %s: %v", k.ID, err)
			}
		}
	})

	return tokenTestHandler()
}

func v6TestID(prefix string) string {
	clean := strings.NewReplacer("/", "-", "\\", "-", " ", "-", "_", "-").Replace(prefix)
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = "v6"
	}
	return fmt.Sprintf("%s-%d-%d", clean, time.Now().UnixNano(), atomic.AddUint64(&v6TestSeq, 1))
}

func v6TestEmail(prefix string) string {
	return v6TestID(prefix) + "@example.com"
}

func v6JSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()

	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func v6DecodeJSON[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response JSON: %v body=%s", err, rr.Body.String())
	}
	return out
}

func v6AssertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rr.Code != want {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, want, rr.Body.String())
	}
}

func v6AssertFloat(t *testing.T, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}

func v6AddApiKey(t *testing.T, prefix, note string) config.ApiKeyInfo {
	t.Helper()
	return v6AddApiKeyWithPlan(t, prefix, note, "credit")
}

func v6AddApiKeyWithPlan(t *testing.T, prefix, note, plan string) config.ApiKeyInfo {
	t.Helper()

	if plan == "" {
		plan = "credit"
	}
	id := v6TestID(prefix)
	key := config.ApiKeyInfo{
		ID:        id,
		Key:       "sk-" + id,
		Plan:      plan,
		Enabled:   true,
		Balance:   1,
		Note:      note,
		CreatedAt: time.Now().Unix(),
	}
	if err := config.AddApiKey(key); err != nil {
		t.Fatalf("AddApiKey: %v", err)
	}
	return key
}

func v6RegisterUser(t *testing.T, email, username, password string) (users.User, config.ApiKeyInfo) {
	t.Helper()

	u, key, err := users.Register(users.RegisterInput{
		Email:    email,
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("Register(%s): %v", email, err)
	}
	return u, key
}

func v6WriteRechargeRecords(t *testing.T, recs []RechargeRecord) {
	t.Helper()

	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("mkdir recharge dir: %v", err)
	}
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create recharge file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, rec := range recs {
		if err := enc.Encode(rec); err != nil {
			t.Fatalf("write recharge record: %v", err)
		}
	}
}

func v6RechargeRecord(ts int64, keyID, typ, keyNote, code, note string, amountCNY float64) RechargeRecord {
	return RechargeRecord{
		Time:      time.Unix(ts, 0).In(time.FixedZone("CST", 8*3600)).Format("01-02 15:04:05"),
		Timestamp: ts,
		KeyID:     keyID,
		KeyNote:   keyNote,
		Type:      typ,
		Code:      code,
		AmountCNY: amountCNY,
		AmountUSD: config.VirtualUSDFromCNY(amountCNY),
		Operator:  "test",
		Note:      note,
	}
}

func v6AuditSize(t *testing.T) int64 {
	t.Helper()

	path := filepath.Join(config.GetDataDir(), "audit.log")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("stat audit log: %v", err)
	}
	return info.Size()
}

func v6AuditTail(t *testing.T, before int64) string {
	t.Helper()

	path := filepath.Join(config.GetDataDir(), "audit.log")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	start := int(before)
	if start < 0 || start > len(data) {
		start = 0
	}
	return string(data[start:])
}

func v6AssertAuditContains(t *testing.T, before int64, needle string) {
	t.Helper()

	tail := v6AuditTail(t, before)
	if !strings.Contains(tail, needle) {
		t.Fatalf("audit tail does not contain %q: %s", needle, tail)
	}
}

func TestHandleUserPasswordLoginRejectsMissingFields(t *testing.T) {
	h := seedV6Test(t)

	tests := []struct {
		name string
		body string
	}{
		{name: "empty body fields", body: `{}`},
		{name: "missing password", body: `{"email":"user@example.com"}`},
		{name: "missing email and username", body: `{"password":"password-123"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/user/api/login", strings.NewReader(tt.body))
			rr := httptest.NewRecorder()

			h.handleUserPasswordLogin(rr, req)

			v6AssertStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestHandleUserPasswordLoginReturnsDefaultKey(t *testing.T) {
	h := seedV6Test(t)

	password := "correct-password"
	email := v6TestEmail("login")
	u, key := v6RegisterUser(t, email, "login-user", password)

	req := v6JSONRequest(t, http.MethodPost, "/user/api/login", map[string]any{
		"username": email,
		"password": password,
	})
	rr := httptest.NewRecorder()

	h.handleUserPasswordLogin(rr, req)

	v6AssertStatus(t, rr, http.StatusOK)
	resp := v6DecodeJSON[struct {
		Success bool   `json:"success"`
		APIKey  string `json:"apiKey"`
		User    struct {
			ID       string `json:"id"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"user"`
	}](t, rr)

	if !resp.Success || resp.APIKey != key.Key {
		t.Fatalf("login response mismatch: %+v key=%s", resp, key.Key)
	}
	// v7: username 不再由调用方指定，而是从 email 前缀派生。
	if resp.User.ID != u.ID || resp.User.Email != email || resp.User.Username == "" || resp.User.Username != u.Username {
		t.Fatalf("user response mismatch: %+v user=%+v", resp.User, u)
	}
}

func TestHandleUserRegisterRejectsInvalidInput(t *testing.T) {
	h := seedV6Test(t)

	tests := []struct {
		name string
		body map[string]any
	}{
		{name: "missing fields", body: map[string]any{}},
		{name: "invalid email", body: map[string]any{"email": "not-an-email", "password": "password-123"}},
		{name: "short password", body: map[string]any{"email": v6TestEmail("short-password"), "password": "short"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.handleUserRegister(rr, v6JSONRequest(t, http.MethodPost, "/user/api/register", tt.body))
			v6AssertStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestHandleUserRegisterRequiresActivationCodeWhenPolicyEnabled(t *testing.T) {
	h := seedV6Test(t)

	users.RequireActivationCode = true

	rr := httptest.NewRecorder()
	h.handleUserRegister(rr, v6JSONRequest(t, http.MethodPost, "/user/api/register", map[string]any{
		"email":    v6TestEmail("activation-required"),
		"password": "password-123",
	}))

	v6AssertStatus(t, rr, http.StatusBadRequest)
}

func TestHandleUserRegisterDuplicateEmailReturnsConflict(t *testing.T) {
	h := seedV6Test(t)

	email := v6TestEmail("duplicate-register")
	body := map[string]any{
		"email":    email,
		"password": "password-123",
	}

	rr := httptest.NewRecorder()
	h.handleUserRegister(rr, v6JSONRequest(t, http.MethodPost, "/user/api/register", body))
	v6AssertStatus(t, rr, http.StatusCreated)

	rr = httptest.NewRecorder()
	h.handleUserRegister(rr, v6JSONRequest(t, http.MethodPost, "/user/api/register", body))
	v6AssertStatus(t, rr, http.StatusConflict)
}

func TestHandleUserBindAccountRejectsInvalidRequest(t *testing.T) {
	h := seedV6Test(t)

	rr := httptest.NewRecorder()
	h.handleUserAPI(rr, v6JSONRequest(t, http.MethodPost, "/user/api/bind-account", map[string]any{
		"email":    v6TestEmail("missing-bearer"),
		"password": "password-123",
	}))
	v6AssertStatus(t, rr, http.StatusUnauthorized)

	key := v6AddApiKey(t, "bind-validation-key", "bind validation")

	tests := []struct {
		name string
		body map[string]any
	}{
		{name: "missing fields", body: map[string]any{}},
		{name: "invalid email", body: map[string]any{"email": "invalid", "password": "password-123"}},
		{name: "short password", body: map[string]any{"email": v6TestEmail("bind-short"), "password": "short"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.handleUserBindAccount(rr, v6JSONRequest(t, http.MethodPost, "/user/api/bind-account", tt.body), &key)
			v6AssertStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestHandleUserBindAccountConflicts(t *testing.T) {
	h := seedV6Test(t)

	key := v6AddApiKey(t, "bind-conflict-key", "bind conflict")

	rr := httptest.NewRecorder()
	h.handleUserBindAccount(rr, v6JSONRequest(t, http.MethodPost, "/user/api/bind-account", map[string]any{
		"email":    v6TestEmail("bind-first"),
		"password": "password-123",
	}), &key)
	v6AssertStatus(t, rr, http.StatusOK)

	rr = httptest.NewRecorder()
	h.handleUserBindAccount(rr, v6JSONRequest(t, http.MethodPost, "/user/api/bind-account", map[string]any{
		"email":    v6TestEmail("bind-second"),
		"password": "password-123",
	}), &key)
	v6AssertStatus(t, rr, http.StatusConflict)

	existingEmail := v6TestEmail("bind-existing-email")
	v6RegisterUser(t, existingEmail, "", "password-123")

	otherKey := v6AddApiKey(t, "bind-email-conflict-key", "email conflict")
	rr = httptest.NewRecorder()
	h.handleUserBindAccount(rr, v6JSONRequest(t, http.MethodPost, "/user/api/bind-account", map[string]any{
		"email":    existingEmail,
		"password": "password-123",
	}), &otherKey)
	v6AssertStatus(t, rr, http.StatusConflict)
}

func TestHandleUserMeReturnsUserIdentityOnlyWhenBound(t *testing.T) {
	h := seedV6Test(t)

	unbound := v6AddApiKey(t, "me-unbound-key", "unbound me")
	rr := httptest.NewRecorder()
	h.handleUserMe(rr, &unbound)
	v6AssertStatus(t, rr, http.StatusOK)

	unboundResp := v6DecodeJSON[map[string]any](t, rr)
	if _, ok := unboundResp["userId"]; ok {
		t.Fatalf("unbound /me should not include userId: %+v", unboundResp)
	}

	bound := v6AddApiKey(t, "me-bound-key", "bound me")
	email := v6TestEmail("me-bound")
	rr = httptest.NewRecorder()
	h.handleUserBindAccount(rr, v6JSONRequest(t, http.MethodPost, "/user/api/bind-account", map[string]any{
		"email":    email,
		"password": "password-123",
	}), &bound)
	v6AssertStatus(t, rr, http.StatusOK)

	rr = httptest.NewRecorder()
	h.handleUserMe(rr, &bound)
	v6AssertStatus(t, rr, http.StatusOK)

	boundResp := v6DecodeJSON[map[string]any](t, rr)
	if got, _ := boundResp["email"].(string); got != email {
		t.Fatalf("email = %q, want %q resp=%+v", got, email, boundResp)
	}
	if got, _ := boundResp["userId"].(string); got == "" {
		t.Fatalf("bound /me should include userId: %+v", boundResp)
	}
}
