package client

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &Client{
		BaseURL:  server.URL,
		Password: "admin-secret",
		HTTP:     server.Client(),
	}
}

func TestNewSetsDefaults(t *testing.T) {
	c := New("http://localhost:8088", "pwd")

	if c.BaseURL != "http://localhost:8088" {
		t.Fatalf("expected base url to be set, got %q", c.BaseURL)
	}
	if c.Password != "pwd" {
		t.Fatalf("expected password to be set, got %q", c.Password)
	}
	if c.HTTP == nil {
		t.Fatal("expected http client to be initialized")
	}
	if c.HTTP.Timeout != 30*time.Second {
		t.Fatalf("expected default timeout to be 30s, got %s", c.HTTP.Timeout)
	}
}

func TestRequestSetsHeadersAndBody(t *testing.T) {
	cases := []struct {
		name            string
		method          string
		body            interface{}
		wantContentType string
		wantBody        string
	}{
		{
			name:            "get without body",
			method:          http.MethodGet,
			body:            nil,
			wantContentType: "",
			wantBody:        "",
		},
		{
			name:            "put with json body",
			method:          http.MethodPut,
			body:            map[string]interface{}{"enabled": true},
			wantContentType: "application/json",
			wantBody:        "{\"enabled\":true}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody string

			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method {
					t.Fatalf("expected method %s, got %s", tc.method, r.Method)
				}
				if r.Header.Get("X-Admin-Password") != "admin-secret" {
					t.Fatalf("expected X-Admin-Password header to be set")
				}
				if r.Header.Get("Content-Type") != tc.wantContentType {
					t.Fatalf("expected content type %q, got %q", tc.wantContentType, r.Header.Get("Content-Type"))
				}

				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				gotBody = string(bodyBytes)

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{}"))
			})

			resp, err := c.request(tc.method, "/admin/api/accounts/demo", tc.body)
			if err != nil {
				t.Fatalf("request returned error: %v", err)
			}
			resp.Body.Close()

			if gotBody != tc.wantBody {
				t.Fatalf("expected request body %q, got %q", tc.wantBody, gotBody)
			}
		})
	}
}

func TestRequestReturnsHTTPError(t *testing.T) {
	cases := []struct {
		name           string
		statusCode     int
		responseBody   string
		wantErrSnippet string
	}{
		{
			name:           "400 bad request",
			statusCode:     http.StatusBadRequest,
			responseBody:   "invalid payload",
			wantErrSnippet: "HTTP 400: invalid payload",
		},
		{
			name:           "500 internal error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   "server exploded",
			wantErrSnippet: "HTTP 500: server exploded",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.responseBody))
			})

			resp, err := c.request(http.MethodGet, "/admin/api/status", nil)
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			if resp != nil {
				t.Fatal("expected nil response on HTTP error")
			}
			if !strings.Contains(err.Error(), tc.wantErrSnippet) {
				t.Fatalf("expected error to contain %q, got %q", tc.wantErrSnippet, err.Error())
			}
		})
	}
}

func TestRequestTimeout(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	c.HTTP.Timeout = 20 * time.Millisecond

	_, err := c.request(http.MethodGet, "/admin/api/status", nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("expected wrapped request error, got %q", err.Error())
	}

	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected timeout net.Error, got %v", err)
	}
}

func TestRequestInvalidBaseURL(t *testing.T) {
	c := &Client{
		BaseURL:  "://bad-url",
		Password: "pwd",
		HTTP:     &http.Client{},
	}

	_, err := c.request(http.MethodGet, "/admin/api/status", nil)
	if err == nil {
		t.Fatal("expected new request error, got nil")
	}
}

func TestGetAccountsScenarios(t *testing.T) {
	cases := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantErr        bool
	}{
		{
			name:           "success",
			responseStatus: http.StatusOK,
			responseBody:   `[{"id":"acc-1","email":"a@example.com"}]`,
			wantErr:        false,
		},
		{
			name:           "invalid json",
			responseStatus: http.StatusOK,
			responseBody:   `{"not":"array"`,
			wantErr:        true,
		},
		{
			name:           "http error",
			responseStatus: http.StatusBadRequest,
			responseBody:   `bad request`,
			wantErr:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", r.Method)
				}
				if r.URL.Path != "/admin/api/accounts" {
					t.Fatalf("expected path /admin/api/accounts, got %s", r.URL.Path)
				}
				w.WriteHeader(tc.responseStatus)
				_, _ = w.Write([]byte(tc.responseBody))
			})

			accounts, err := c.GetAccounts()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (accounts=%v)", accounts)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if len(accounts) != 1 {
				t.Fatalf("expected 1 account, got %d", len(accounts))
			}
			if accounts[0]["id"] != "acc-1" {
				t.Fatalf("expected id acc-1, got %v", accounts[0]["id"])
			}
		})
	}
}

func TestMutatingMethodsSendExpectedRequest(t *testing.T) {
	cases := []struct {
		name       string
		call       func(c *Client) error
		wantMethod string
		wantPath   string
		wantBody   map[string]interface{}
	}{
		{
			name:       "refresh account",
			call:       func(c *Client) error { return c.RefreshAccount("acc-1") },
			wantMethod: http.MethodPost,
			wantPath:   "/admin/api/accounts/acc-1/refresh",
			wantBody:   nil,
		},
		{
			name:       "delete account",
			call:       func(c *Client) error { return c.DeleteAccount("acc-2") },
			wantMethod: http.MethodDelete,
			wantPath:   "/admin/api/accounts/acc-2",
			wantBody:   nil,
		},
		{
			name:       "update account",
			call:       func(c *Client) error { return c.UpdateAccount("acc-3", map[string]interface{}{"enabled": false}) },
			wantMethod: http.MethodPut,
			wantPath:   "/admin/api/accounts/acc-3",
			wantBody:   map[string]interface{}{"enabled": false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.wantMethod {
					t.Fatalf("expected method %s, got %s", tc.wantMethod, r.Method)
				}
				if r.URL.Path != tc.wantPath {
					t.Fatalf("expected path %s, got %s", tc.wantPath, r.URL.Path)
				}

				if tc.wantBody != nil {
					var got map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
						t.Fatalf("failed to decode request body: %v", err)
					}
					if got["enabled"] != tc.wantBody["enabled"] {
						t.Fatalf("expected enabled=%v, got %v", tc.wantBody["enabled"], got["enabled"])
					}
				} else {
					bodyBytes, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("failed to read request body: %v", err)
					}
					if len(bodyBytes) != 0 {
						t.Fatalf("expected empty request body, got %q", string(bodyBytes))
					}
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{}"))
			})

			if err := tc.call(c); err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

func TestGetStatusScenarios(t *testing.T) {
	cases := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantErr        bool
	}{
		{
			name:           "success",
			responseStatus: http.StatusOK,
			responseBody:   `{"totalRequests":10,"activeAccounts":3}`,
			wantErr:        false,
		},
		{
			name:           "invalid json",
			responseStatus: http.StatusOK,
			responseBody:   `{"totalRequests":`,
			wantErr:        true,
		},
		{
			name:           "http error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `boom`,
			wantErr:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", r.Method)
				}
				if r.URL.Path != "/admin/api/status" {
					t.Fatalf("expected path /admin/api/status, got %s", r.URL.Path)
				}
				w.WriteHeader(tc.responseStatus)
				_, _ = w.Write([]byte(tc.responseBody))
			})

			status, err := c.GetStatus()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (status=%v)", status)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if status["activeAccounts"] != float64(3) {
				t.Fatalf("expected activeAccounts=3, got %v", status["activeAccounts"])
			}
		})
	}
}

func TestGetLogsBuildsExpectedPath(t *testing.T) {
	cases := []struct {
		name        string
		limit       int
		wantRequest string
	}{
		{
			name:        "without limit",
			limit:       0,
			wantRequest: "/admin/api/logs",
		},
		{
			name:        "with limit",
			limit:       25,
			wantRequest: "/admin/api/logs?limit=25",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", r.Method)
				}
				if got := r.URL.RequestURI(); got != tc.wantRequest {
					t.Fatalf("expected request uri %s, got %s", tc.wantRequest, got)
				}
				_, _ = w.Write([]byte(`[{"level":"INFO","message":"ok"}]`))
			})

			logs, err := c.GetLogs(tc.limit)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if len(logs) != 1 {
				t.Fatalf("expected 1 log entry, got %d", len(logs))
			}
			if logs[0]["message"] != "ok" {
				t.Fatalf("expected message ok, got %v", logs[0]["message"])
			}
		})
	}
}

func TestGetLogsDecodeError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{`))
	})

	logs, err := c.GetLogs(1)
	if err == nil {
		t.Fatalf("expected decode error, got nil (logs=%v)", logs)
	}
	if !strings.Contains(err.Error(), "invalid character") && !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("expected JSON decode error, got %v", err)
	}
}
