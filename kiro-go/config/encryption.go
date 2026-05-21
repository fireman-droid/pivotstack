package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/hkdf"
)

const (
	secretCipherPrefix = "v1:gcm:"
	secretHKDFInfo     = "pivotstack/newapi/secrets/v1"
	secretKeyLen       = 32
)

var secretFallbackWarnOnce sync.Once

// deriveSecretKey 用 HKDF-SHA256 派生 AES-256-GCM key。
// 生产应配置 PIVOTSTACK_ENCRYPTION_KEY；admin password hash fallback 只用于本地兼容。
func deriveSecretKey() []byte {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	return deriveSecretKeyLocked()
}

// deriveSecretKeyLocked 供已持 cfgLock 的调用方（如 MigrateConfigToV6）使用，避免重入死锁。
func deriveSecretKeyLocked() []byte {
	material := os.Getenv("PIVOTSTACK_ENCRYPTION_KEY")
	if material == "" {
		if cfg != nil {
			material = cfg.Password
		}
		secretFallbackWarnOnce.Do(func() {
			fmt.Println("[config] WARN: PIVOTSTACK_ENCRYPTION_KEY not set; deriving secret key from admin password hash (dev/test fallback only)")
		})
	}
	salt := ensureSecretKeySaltLocked()
	return deriveSecretKeyFromMaterial(material, salt)
}

// RequireProductionEncryptionKey 在 PIVOTSTACK_ENV=production 时强制要求
// PIVOTSTACK_ENCRYPTION_KEY，否则启动失败。
// 防御 P0：config.json 泄露 + admin password hash 泄露 → 上游密钥可解。
// dev/test 不强制，保留 fallback 让本地起服务方便。
func RequireProductionEncryptionKey() error {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("PIVOTSTACK_ENV")), "production") {
		return nil
	}
	if strings.TrimSpace(os.Getenv("PIVOTSTACK_ENCRYPTION_KEY")) == "" {
		return errors.New("PIVOTSTACK_ENCRYPTION_KEY is required when PIVOTSTACK_ENV=production")
	}
	return nil
}

func deriveSecretKeyFromMaterial(material, salt string) []byte {
	reader := hkdf.New(sha256.New, []byte(material), []byte(salt), []byte(secretHKDFInfo))
	key := make([]byte, secretKeyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		// hkdf.Reader 不应失败；保留防御分支，避免未来实现变化导致 panic。
		return make([]byte, secretKeyLen)
	}
	return key
}

// EncryptSecret exposes secret encryption to packages that persist provider credentials.
func EncryptSecret(plain string) (string, error) {
	return encryptSecret(plain)
}

// DecryptSecret exposes secret decryption to packages that need to call upstream providers.
func DecryptSecret(cipherText string) (string, error) {
	return decryptSecret(cipherText)
}

// DecryptSecretLocked 供已持 cfgLock 的调用方使用（避免重入死锁）。
// 仅 config 包内 migration / 锁内 helper 应使用；外部一律走 DecryptSecret。
func DecryptSecretLocked(cipherText string) (string, error) {
	return decryptSecretLocked(cipherText)
}

// encryptSecret 加密上游 secret，返回带版本前缀的密文。
func encryptSecret(plain string) (string, error) {
	block, err := aes.NewCipher(deriveSecretKey())
	if err != nil {
		return "", fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create secret gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return secretCipherPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

// decryptSecret 解密 v1:gcm 密文；未知版本、损坏密文、篡改密文都返回 error。
func decryptSecret(cipherText string) (string, error) {
	return decryptSecretWithKey(cipherText, deriveSecretKey())
}

// decryptSecretLocked 同 decryptSecret，但跳过 cfgLock 重入（调用方已持锁）。
// 用于 MigrateConfigToV6 / 锁内 helper；外部调用一律走 decryptSecret。
func decryptSecretLocked(cipherText string) (string, error) {
	return decryptSecretWithKey(cipherText, deriveSecretKeyLocked())
}

func decryptSecretWithKey(cipherText string, key []byte) (string, error) {
	if !strings.HasPrefix(cipherText, secretCipherPrefix) {
		return "", fmt.Errorf("unsupported secret format")
	}
	payload := strings.TrimPrefix(cipherText, secretCipherPrefix)
	raw, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decode secret payload: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create secret gcm: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("secret payload too short")
	}
	nonce := raw[:gcm.NonceSize()]
	cipherBytes := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}

// IsSecretFormatValid 只检查持久化格式版本，不尝试解密。
func IsSecretFormatValid(s string) bool {
	return strings.HasPrefix(s, secretCipherPrefix)
}
