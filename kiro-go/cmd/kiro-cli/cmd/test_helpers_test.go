package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/fatih/color"
	"github.com/spf13/viper"
)

func resetCLIState(t *testing.T) {
	t.Helper()

	viper.Reset()
	cfgFile = ""
	apiURL = ""
	password = ""
	output = ""
}

func setTestHome(t *testing.T, dir string) {
	t.Helper()

	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	oldColorOutput := color.Output
	oldNoColor := color.NoColor

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	defer reader.Close()

	outputCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		outputCh <- buf.String()
	}()

	os.Stdout = writer
	color.Output = writer
	color.NoColor = true
	defer func() {
		os.Stdout = oldStdout
		color.Output = oldColorOutput
		color.NoColor = oldNoColor
	}()

	fn()

	_ = writer.Close()
	return <-outputCh
}

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin

	file, err := os.CreateTemp("", "kiro-cli-stdin-*")
	if err != nil {
		t.Fatalf("failed to create temp stdin file: %v", err)
	}
	if _, err := file.WriteString(input); err != nil {
		t.Fatalf("failed to write stdin content: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("failed to reset stdin cursor: %v", err)
	}

	os.Stdin = file
	defer func() {
		os.Stdin = oldStdin
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()

	fn()
}

func runHelperProcess(t *testing.T, helperCase string) (string, int) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestCLIHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"CLI_HELPER_CASE="+helperCase,
	)

	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}

	t.Fatalf("failed to run helper process: %v", err)
	return "", -1
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	resetCLIState(t)
	color.NoColor = true

	switch os.Getenv("CLI_HELPER_CASE") {
	case "account-list-missing-password":
		viper.Set("api_url", "http://127.0.0.1:65535")
		viper.Set("output", "table")
		accountListCmd.Run(accountListCmd, nil)
	case "account-refresh-api-error":
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("refresh failed"))
		}))
		defer server.Close()

		viper.Set("api_url", server.URL)
		viper.Set("password", "secret")
		accountRefreshCmd.Run(accountRefreshCmd, []string{"acc-1"})
	case "config-set-invalid-key":
		configSetCmd.Run(configSetCmd, []string{"unknown_key", "value"})
	case "status-missing-password":
		viper.Set("api_url", "http://127.0.0.1:65535")
		viper.Set("output", "table")
		statusCmd.Run(statusCmd, nil)
	case "status-api-error":
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		}))
		defer server.Close()

		viper.Set("api_url", server.URL)
		viper.Set("password", "secret")
		viper.Set("output", "table")
		statusCmd.Run(statusCmd, nil)
	default:
		os.Exit(2)
	}

	os.Exit(0)
}
