package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestAccountCommandArgsValidation(t *testing.T) {
	cases := []struct {
		name    string
		command *cobra.Command
		args    []string
		wantErr bool
	}{
		{name: "refresh requires one arg", command: accountRefreshCmd, args: []string{}, wantErr: true},
		{name: "refresh accepts one arg", command: accountRefreshCmd, args: []string{"acc-1"}, wantErr: false},
		{name: "refresh rejects extra arg", command: accountRefreshCmd, args: []string{"acc-1", "extra"}, wantErr: true},
		{name: "enable requires one arg", command: accountEnableCmd, args: []string{}, wantErr: true},
		{name: "enable accepts one arg", command: accountEnableCmd, args: []string{"acc-2"}, wantErr: false},
		{name: "disable requires one arg", command: accountDisableCmd, args: []string{}, wantErr: true},
		{name: "disable accepts one arg", command: accountDisableCmd, args: []string{"acc-3"}, wantErr: false},
		{name: "delete requires one arg", command: accountDeleteCmd, args: []string{}, wantErr: true},
		{name: "delete accepts one arg", command: accountDeleteCmd, args: []string{"acc-4"}, wantErr: false},
		{name: "delete rejects extra arg", command: accountDeleteCmd, args: []string{"acc-4", "extra"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.command.Args(tc.command, tc.args)
			if tc.wantErr && err == nil {
				t.Fatal("expected args validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected args validation to pass, got %v", err)
			}
		})
	}
}

func TestAccountListCommandOutputFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/admin/api/accounts" {
			t.Fatalf("expected /admin/api/accounts, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Admin-Password") != "secret" {
			t.Fatalf("expected password header to be set")
		}

		_, _ = io.WriteString(w, `[
			{
				"id":"1234567890abcdef",
				"email":"running@example.com",
				"enabled":true,
				"banStatus":"ACTIVE",
				"usageCurrent":10,
				"usageLimit":100,
				"usagePercent":0.1,
				"trialUsageCurrent":1,
				"trialUsageLimit":5,
				"subscriptionType":"pro"
			},
			{
				"id":"2234567890abcdef",
				"email":"disabled@example.com",
				"enabled":false,
				"banStatus":"ACTIVE",
				"usageCurrent":20,
				"usageLimit":100,
				"usagePercent":0.2,
				"trialUsageCurrent":2,
				"trialUsageLimit":5,
				"subscriptionType":"free"
			},
			{
				"id":"3234567890abcdef",
				"email":"banned@example.com",
				"enabled":true,
				"banStatus":"BANNED",
				"usageCurrent":30,
				"usageLimit":100,
				"usagePercent":0.3,
				"trialUsageCurrent":3,
				"trialUsageLimit":5,
				"subscriptionType":"pro"
			}
		]`)
	}))
	defer server.Close()

	cases := []struct {
		name   string
		output string
		verify func(t *testing.T, out string)
	}{
		{
			name:   "json output",
			output: "json",
			verify: func(t *testing.T, out string) {
				t.Helper()

				var accounts []map[string]interface{}
				if err := json.Unmarshal([]byte(out), &accounts); err != nil {
					t.Fatalf("expected valid json output, got error: %v", err)
				}
				if len(accounts) != 3 {
					t.Fatalf("expected 3 accounts, got %d", len(accounts))
				}
				if accounts[0]["email"] != "running@example.com" {
					t.Fatalf("unexpected first account email: %v", accounts[0]["email"])
				}
			},
		},
		{
			name:   "table output",
			output: "table",
			verify: func(t *testing.T, out string) {
				t.Helper()

				checks := []string{
					"running@example.com",
					"disabled@example.com",
					"banned@example.com",
					"12345678...",
					"✓ 运行中",
					"✗ 已禁用",
					"⚠ 已封禁",
					"共 3 个账号",
				}
				for _, want := range checks {
					if !strings.Contains(out, want) {
						t.Fatalf("expected output to contain %q, got: %s", want, out)
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetCLIState(t)
			viper.Set("api_url", server.URL)
			viper.Set("password", "secret")
			viper.Set("output", tc.output)

			out := captureStdout(t, func() {
				accountListCmd.Run(accountListCmd, nil)
			})

			tc.verify(t, out)
		})
	}
}

func TestAccountRefreshCommandSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/admin/api/accounts/acc-1/refresh" {
			t.Fatalf("expected refresh path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resetCLIState(t)
	viper.Set("api_url", server.URL)
	viper.Set("password", "secret")

	out := captureStdout(t, func() {
		accountRefreshCmd.Run(accountRefreshCmd, []string{"acc-1"})
	})

	if !strings.Contains(out, "刷新成功") {
		t.Fatalf("expected success message, got: %s", out)
	}
}

func TestAccountToggleCommandsSendExpectedPayload(t *testing.T) {
	cases := []struct {
		name        string
		command     *cobra.Command
		wantEnabled bool
		wantMessage string
	}{
		{name: "enable", command: accountEnableCmd, wantEnabled: true, wantMessage: "已启用"},
		{name: "disable", command: accountDisableCmd, wantEnabled: false, wantMessage: "已禁用"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var gotEnabled bool

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path

				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}
				enabled, ok := body["enabled"].(bool)
				if !ok {
					t.Fatalf("expected bool enabled field, got %#v", body["enabled"])
				}
				gotEnabled = enabled

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			resetCLIState(t)
			viper.Set("api_url", server.URL)
			viper.Set("password", "secret")

			out := captureStdout(t, func() {
				tc.command.Run(tc.command, []string{"acc-1"})
			})

			if gotMethod != http.MethodPut {
				t.Fatalf("expected PUT, got %s", gotMethod)
			}
			if gotPath != "/admin/api/accounts/acc-1" {
				t.Fatalf("expected account update path, got %s", gotPath)
			}
			if gotEnabled != tc.wantEnabled {
				t.Fatalf("expected enabled=%v, got %v", tc.wantEnabled, gotEnabled)
			}
			if !strings.Contains(out, tc.wantMessage) {
				t.Fatalf("expected output to contain %q, got: %s", tc.wantMessage, out)
			}
		})
	}
}

func TestAccountDeleteCommandConfirmed(t *testing.T) {
	var gotMethod, gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resetCLIState(t)
	viper.Set("api_url", server.URL)
	viper.Set("password", "secret")

	out := captureStdout(t, func() {
		withStdin(t, "y\n", func() {
			accountDeleteCmd.Run(accountDeleteCmd, []string{"acc-9"})
		})
	})

	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/admin/api/accounts/acc-9" {
		t.Fatalf("expected delete path /admin/api/accounts/acc-9, got %s", gotPath)
	}
	if !strings.Contains(out, "已删除") {
		t.Fatalf("expected delete success message, got: %s", out)
	}
}

func TestAccountDeleteCommandCancel(t *testing.T) {
	resetCLIState(t)
	viper.Set("api_url", "http://127.0.0.1:65535")
	viper.Set("password", "secret")

	out := captureStdout(t, func() {
		withStdin(t, "n\n", func() {
			accountDeleteCmd.Run(accountDeleteCmd, []string{"acc-9"})
		})
	})

	if !strings.Contains(out, "已取消") {
		t.Fatalf("expected cancel message, got: %s", out)
	}
	if strings.Contains(out, "已删除") {
		t.Fatalf("did not expect delete success message after cancel, got: %s", out)
	}
}

func TestAccountListCommandMissingPasswordShowsError(t *testing.T) {
	out, exitCode := runHelperProcess(t, "account-list-missing-password")

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d (output=%s)", exitCode, out)
	}
	if !strings.Contains(out, "错误:") {
		t.Fatalf("expected generic error prefix, got: %s", out)
	}
	if !strings.Contains(out, "密码未设置") {
		t.Fatalf("expected missing password hint, got: %s", out)
	}
}

func TestAccountRefreshCommandAPIFailureShowsError(t *testing.T) {
	out, exitCode := runHelperProcess(t, "account-refresh-api-error")

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d (output=%s)", exitCode, out)
	}
	if !strings.Contains(out, "刷新失败") {
		t.Fatalf("expected refresh failed message, got: %s", out)
	}
	if !strings.Contains(out, "HTTP 500") {
		t.Fatalf("expected HTTP 500 detail, got: %s", out)
	}
}

func TestAccountValueHelpers(t *testing.T) {
	t.Run("getString", func(t *testing.T) {
		cases := []struct {
			name string
			data map[string]interface{}
			key  string
			want string
		}{
			{name: "existing string", data: map[string]interface{}{"email": "a@example.com"}, key: "email", want: "a@example.com"},
			{name: "missing key", data: map[string]interface{}{}, key: "email", want: ""},
			{name: "wrong type", data: map[string]interface{}{"email": 123}, key: "email", want: ""},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := getString(tc.data, tc.key); got != tc.want {
					t.Fatalf("expected %q, got %q", tc.want, got)
				}
			})
		}
	})

	t.Run("getBool", func(t *testing.T) {
		cases := []struct {
			name string
			data map[string]interface{}
			key  string
			want bool
		}{
			{name: "existing true", data: map[string]interface{}{"enabled": true}, key: "enabled", want: true},
			{name: "existing false", data: map[string]interface{}{"enabled": false}, key: "enabled", want: false},
			{name: "missing key", data: map[string]interface{}{}, key: "enabled", want: false},
			{name: "wrong type", data: map[string]interface{}{"enabled": "true"}, key: "enabled", want: false},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := getBool(tc.data, tc.key); got != tc.want {
					t.Fatalf("expected %v, got %v", tc.want, got)
				}
			})
		}
	})

	t.Run("getFloat", func(t *testing.T) {
		cases := []struct {
			name string
			data map[string]interface{}
			key  string
			want float64
		}{
			{name: "float64 value", data: map[string]interface{}{"usage": 1.5}, key: "usage", want: 1.5},
			{name: "int value", data: map[string]interface{}{"usage": 2}, key: "usage", want: 2},
			{name: "missing key", data: map[string]interface{}{}, key: "usage", want: 0},
			{name: "wrong type", data: map[string]interface{}{"usage": "2"}, key: "usage", want: 0},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := getFloat(tc.data, tc.key); got != tc.want {
					t.Fatalf("expected %v, got %v", tc.want, got)
				}
			})
		}
	})
}
