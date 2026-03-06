package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestShowMenuDisplaysCoreCommands(t *testing.T) {
	out := captureStdout(t, func() {
		showMenu()
	})

	checks := []string{
		"kiro-cli status",
		"kiro-cli account list",
		"kiro-cli config set <key> <value>",
		"--output json",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("expected menu output to contain %q, got: %s", want, out)
		}
	}
}

func TestGetClient(t *testing.T) {
	t.Run("missing password returns error", func(t *testing.T) {
		resetCLIState(t)
		viper.Set("api_url", "http://localhost:8088")

		cli, err := getClient()
		if err == nil {
			t.Fatalf("expected missing password error, got nil (client=%v)", cli)
		}
		if !strings.Contains(err.Error(), "密码未设置") {
			t.Fatalf("expected password hint in error, got: %v", err)
		}
	})

	t.Run("returns client when configured", func(t *testing.T) {
		resetCLIState(t)
		viper.Set("api_url", "http://example.test")
		viper.Set("password", "secret")

		cli, err := getClient()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if cli.BaseURL != "http://example.test" {
			t.Fatalf("expected base url http://example.test, got %s", cli.BaseURL)
		}
		if cli.Password != "secret" {
			t.Fatalf("expected password secret, got %s", cli.Password)
		}
	})
}
