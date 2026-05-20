package config

import (
	"os"
	"testing"
)

func TestGetStorageBackend_DefaultJSON(t *testing.T) {
	ResetStorageBackendForTest()
	t.Setenv("STORAGE_BACKEND", "")
	if got := GetStorageBackend(); got != StorageBackendJSON {
		t.Fatalf("default = %s, want json", got)
	}
}

func TestGetStorageBackend_PGAlias(t *testing.T) {
	cases := []string{"pg", "PG", "Postgres", "postgresql"}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			ResetStorageBackendForTest()
			t.Setenv("STORAGE_BACKEND", raw)
			if got := GetStorageBackend(); got != StorageBackendPG {
				t.Fatalf("STORAGE_BACKEND=%s → %s, want pg", raw, got)
			}
		})
	}
}

func TestStorageBackend_UnknownFallsBackJSON(t *testing.T) {
	ResetStorageBackendForTest()
	t.Setenv("STORAGE_BACKEND", "mongo")
	if got := GetStorageBackend(); got != StorageBackendJSON {
		t.Fatalf("unknown → %s, want json fallback", got)
	}
}

func TestStorageBackend_BoolEnvs(t *testing.T) {
	t.Setenv("STORAGE_JSON_FALLBACK_READ", "true")
	t.Setenv("STORAGE_DUAL_WRITE_JSON", "yes")
	t.Setenv("STORAGE_ROLLBACK_READY", "1")
	if !IsJSONFallbackReadEnabled() {
		t.Fatal("fallback read should be true")
	}
	if !IsDualWriteJSONEnabled() {
		t.Fatal("dual write should be true")
	}
	if !IsRollbackReady() {
		t.Fatal("rollback ready should be true")
	}
	t.Setenv("STORAGE_JSON_FALLBACK_READ", "no")
	if IsJSONFallbackReadEnabled() {
		t.Fatal("fallback read should be false for 'no'")
	}
}

func TestDatabaseURL_Trimmed(t *testing.T) {
	t.Setenv("DATABASE_URL", "  postgres://x   ")
	if got := DatabaseURL(); got != "postgres://x" {
		t.Fatalf("DatabaseURL = %q", got)
	}
}

// 保留入参隔离：tests that don't touch os.Setenv (e.g., for parseBoolEnv default)
func TestParseBoolEnv_DefaultPath(t *testing.T) {
	_ = os.Unsetenv("PIVOTSTACK_TEST_NONEXISTENT_BOOL")
	if got := parseBoolEnv("PIVOTSTACK_TEST_NONEXISTENT_BOOL", true); !got {
		t.Fatal("default true ignored")
	}
	if got := parseBoolEnv("PIVOTSTACK_TEST_NONEXISTENT_BOOL", false); got {
		t.Fatal("default false ignored")
	}
}
