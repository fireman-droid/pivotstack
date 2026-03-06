package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file %s: %v", path, err)
	}
}

func TestConfigSetCommandArgsValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "missing all args", args: []string{}, wantErr: true},
		{name: "missing value", args: []string{"api_url"}, wantErr: true},
		{name: "exact args", args: []string{"api_url", "http://localhost:8088"}, wantErr: false},
		{name: "too many args", args: []string{"api_url", "http://localhost:8088", "extra"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := configSetCmd.Args(configSetCmd, tc.args)
			if tc.wantErr && err == nil {
				t.Fatal("expected args validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected args validation to pass, got %v", err)
			}
		})
	}
}

func TestConfigSetCommandWritesConfigFile(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
	}{
		{name: "set api_url", key: "api_url", value: "http://127.0.0.1:8088"},
		{name: "set password", key: "password", value: "secret"},
		{name: "set output", key: "output", value: "json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			setTestHome(t, home)
			t.Setenv("API_URL", "")
			t.Setenv("PASSWORD", "")
			t.Setenv("OUTPUT", "")
			resetCLIState(t)

			out := captureStdout(t, func() {
				configSetCmd.Run(configSetCmd, []string{tc.key, tc.value})
			})

			if !strings.Contains(out, "配置已保存") {
				t.Fatalf("expected save success message, got: %s", out)
			}

			configPath := filepath.Join(home, ".kiro-cli.yaml")
			loaded := viper.New()
			loaded.SetConfigFile(configPath)
			if err := loaded.ReadInConfig(); err != nil {
				t.Fatalf("failed to read generated config file: %v", err)
			}

			if got := loaded.GetString(tc.key); got != tc.value {
				t.Fatalf("expected config %s=%q, got %q", tc.key, tc.value, got)
			}

			if runtime.GOOS != "windows" {
				info, err := os.Stat(configPath)
				if err != nil {
					t.Fatalf("failed to stat config file: %v", err)
				}
				if info.Mode().Perm() != 0o600 {
					t.Fatalf("expected permissions 0600, got %o", info.Mode().Perm())
				}
			}
		})
	}
}

func TestConfigShowCommandWithConfigFile(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("API_URL", "")
	t.Setenv("PASSWORD", "")
	t.Setenv("OUTPUT", "")
	resetCLIState(t)

	configPath := filepath.Join(home, ".kiro-cli.yaml")
	writeConfigFile(t, configPath, "api_url: http://from-config:8088\noutput: json\npassword: super-secret\n")

	initConfig()

	out := captureStdout(t, func() {
		configShowCmd.Run(configShowCmd, nil)
	})

	checks := []string{
		"API 地址: http://from-config:8088",
		"输出格式: json",
		"密码: ********",
		"配置文件:",
		".kiro-cli.yaml",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got: %s", want, out)
		}
	}
	if strings.Contains(out, "super-secret") {
		t.Fatalf("expected masked password, but plain password was printed: %s", out)
	}
}

func TestConfigShowCommandDefaultsWhenConfigMissing(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("API_URL", "")
	t.Setenv("PASSWORD", "")
	t.Setenv("OUTPUT", "")
	resetCLIState(t)

	initConfig()

	out := captureStdout(t, func() {
		configShowCmd.Run(configShowCmd, nil)
	})

	checks := []string{
		"API 地址: http://localhost:8088",
		"输出格式: table",
		"密码: 未设置",
		"配置文件: 未找到",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got: %s", want, out)
		}
	}
}

func TestInitConfigEnvOverridesConfigFile(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("API_URL", "http://from-env:9090")
	t.Setenv("PASSWORD", "")
	t.Setenv("OUTPUT", "")
	resetCLIState(t)

	configPath := filepath.Join(home, ".kiro-cli.yaml")
	writeConfigFile(t, configPath, "api_url: http://from-config:8088\noutput: json\npassword: from-file\n")

	initConfig()

	if got := viper.GetString("api_url"); got != "http://from-env:9090" {
		t.Fatalf("expected env to override api_url, got %q", got)
	}
	if got := viper.GetString("output"); got != "json" {
		t.Fatalf("expected output from config file, got %q", got)
	}
	if got := viper.GetString("password"); got != "from-file" {
		t.Fatalf("expected password from config file, got %q", got)
	}
}

func TestInitConfigUsesExplicitConfigFile(t *testing.T) {
	t.Setenv("API_URL", "")
	t.Setenv("PASSWORD", "")
	t.Setenv("OUTPUT", "")
	resetCLIState(t)

	configPath := filepath.Join(t.TempDir(), "custom-config.yaml")
	writeConfigFile(t, configPath, "api_url: http://explicit:9000\noutput: json\npassword: explicit-secret\n")
	cfgFile = configPath

	initConfig()

	if got := viper.GetString("api_url"); got != "http://explicit:9000" {
		t.Fatalf("expected explicit config api_url, got %q", got)
	}
	if got := viper.GetString("output"); got != "json" {
		t.Fatalf("expected explicit config output, got %q", got)
	}
	if got := viper.GetString("password"); got != "explicit-secret" {
		t.Fatalf("expected explicit config password, got %q", got)
	}
}

func TestConfigSetCommandInvalidKeyShowsError(t *testing.T) {
	out, exitCode := runHelperProcess(t, "config-set-invalid-key")

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d (output=%s)", exitCode, out)
	}
	if !strings.Contains(out, "无效的配置项") {
		t.Fatalf("expected invalid key error message, got: %s", out)
	}
	if !strings.Contains(out, "可用的配置项: api_url, password, output") {
		t.Fatalf("expected available keys hint, got: %s", out)
	}
}
