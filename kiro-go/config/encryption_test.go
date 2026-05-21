package config

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	resetTestConfig(t, &Config{Password: "fallback-hash"})
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "roundtrip-env-key")

	cases := []string{
		"",
		"plain ascii",
		"中文 secret",
		string([]byte{0x00, 0x01, 0x02, 0xff, 0x7f}),
	}
	for _, tc := range cases {
		cipherText, err := encryptSecret(tc)
		if err != nil {
			t.Fatalf("encryptSecret(%q): %v", tc, err)
		}
		if !IsSecretFormatValid(cipherText) {
			t.Fatalf("cipher text format invalid: %q", cipherText)
		}
		got, err := decryptSecret(cipherText)
		if err != nil {
			t.Fatalf("decryptSecret(%q): %v", cipherText, err)
		}
		if got != tc {
			t.Fatalf("roundtrip mismatch: got %q, want %q", got, tc)
		}
	}
}

func TestDecryptRejectsUnknownVersion(t *testing.T) {
	resetTestConfig(t, &Config{Password: "fallback-hash"})
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "env-key")
	if _, err := decryptSecret("v2:gcm:abcd"); err == nil {
		t.Fatal("expected error for unknown secret version")
	}
}

func TestDecryptRejectsTampered(t *testing.T) {
	resetTestConfig(t, &Config{Password: "fallback-hash"})
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "env-key")
	cipherText, err := encryptSecret("secret")
	if err != nil {
		t.Fatalf("encryptSecret: %v", err)
	}
	tampered := []byte(cipherText)
	if tampered[len(tampered)-1] == 'A' {
		tampered[len(tampered)-1] = 'B'
	} else {
		tampered[len(tampered)-1] = 'A'
	}
	if _, err := decryptSecret(string(tampered)); err == nil {
		t.Fatal("expected tampered ciphertext to fail")
	}
}

func TestDeriveSecretKeyFallsBackToAdminPasswordWhenEnvMissing(t *testing.T) {
	resetTestConfig(t, &Config{Password: "fallback-admin-hash"})
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "")
	key1 := deriveSecretKey()
	key2 := deriveSecretKey()
	if len(key1) != 32 {
		t.Fatalf("key length = %d, want 32", len(key1))
	}
	if !bytes.Equal(key1, key2) {
		t.Fatal("fallback-derived key changed across calls")
	}
}

func TestDeriveSecretKeyPrefersEnv(t *testing.T) {
	resetTestConfig(t, &Config{Password: "fallback-admin-hash"})
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "env-key-1")
	key1 := deriveSecretKey()
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "env-key-2")
	key2 := deriveSecretKey()
	if bytes.Equal(key1, key2) {
		t.Fatal("deriveSecretKey did not prefer env material")
	}
}

func TestSecretKeySaltPersists(t *testing.T) {
	resetTestConfig(t, &Config{Password: "fallback-admin-hash"})
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "env-key")

	key1 := deriveSecretKey()
	cfgLock.RLock()
	salt1 := cfg.SecretKeySalt
	cfgLock.RUnlock()
	if salt1 == "" {
		t.Fatal("SecretKeySalt was not initialized")
	}

	key2 := deriveSecretKey()
	cfgLock.RLock()
	salt2 := cfg.SecretKeySalt
	cfgLock.RUnlock()
	if salt1 != salt2 {
		t.Fatalf("SecretKeySalt changed: %q -> %q", salt1, salt2)
	}
	if !bytes.Equal(key1, key2) {
		t.Fatal("same env material and persisted salt produced different keys")
	}
}
