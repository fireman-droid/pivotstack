package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestStatusCommandOutputFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/admin/api/status" {
			t.Fatalf("expected /admin/api/status, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Admin-Password") != "secret" {
			t.Fatalf("expected password header to be set")
		}

		_, _ = io.WriteString(w, `{
			"totalRequests": 12,
			"activeAccounts": 4,
			"totalAccounts": 8,
			"uptime": "2h15m"
		}`)
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

				var payload map[string]interface{}
				if err := json.Unmarshal([]byte(out), &payload); err != nil {
					t.Fatalf("expected valid json output, got error: %v", err)
				}
				if payload["totalRequests"] != float64(12) {
					t.Fatalf("expected totalRequests=12, got %v", payload["totalRequests"])
				}
				if payload["uptime"] != "2h15m" {
					t.Fatalf("expected uptime=2h15m, got %v", payload["uptime"])
				}
			},
		},
		{
			name:   "table output",
			output: "table",
			verify: func(t *testing.T, out string) {
				t.Helper()

				checks := []string{
					"Kiro-Stack 系统状态",
					"总请求数: 12",
					"活跃账号: 4",
					"总账号数: 8",
					"运行时间: 2h15m",
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
				statusCmd.Run(statusCmd, nil)
			})

			tc.verify(t, out)
		})
	}
}

func TestStatusCommandTableOutputSkipsUptimeWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"totalRequests": 1,
			"activeAccounts": 1,
			"totalAccounts": 1
		}`)
	}))
	defer server.Close()

	resetCLIState(t)
	viper.Set("api_url", server.URL)
	viper.Set("password", "secret")
	viper.Set("output", "table")

	out := captureStdout(t, func() {
		statusCmd.Run(statusCmd, nil)
	})

	if strings.Contains(out, "运行时间:") {
		t.Fatalf("did not expect uptime line when field is missing, got: %s", out)
	}
}

func TestStatusCommandMissingPasswordShowsError(t *testing.T) {
	out, exitCode := runHelperProcess(t, "status-missing-password")

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d (output=%s)", exitCode, out)
	}
	if !strings.Contains(out, "错误:") {
		t.Fatalf("expected error prefix, got: %s", out)
	}
	if !strings.Contains(out, "密码未设置") {
		t.Fatalf("expected missing password hint, got: %s", out)
	}
}

func TestStatusCommandAPIFailureShowsError(t *testing.T) {
	out, exitCode := runHelperProcess(t, "status-api-error")

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d (output=%s)", exitCode, out)
	}
	if !strings.Contains(out, "获取状态失败") {
		t.Fatalf("expected status failure message, got: %s", out)
	}
	if !strings.Contains(out, "HTTP 500") {
		t.Fatalf("expected HTTP 500 detail, got: %s", out)
	}
}
