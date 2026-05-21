package config

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// argon2id parameters for admin password hashing.
// 64 MiB / 3 iterations / 2 lanes — OWASP 2026 recommended baseline.
const (
	adminPasswordArgonMemory      uint32 = 64 * 1024 // KiB → 64 MiB
	adminPasswordArgonTime        uint32 = 3
	adminPasswordArgonParallelism uint8  = 2
	adminPasswordSaltLen                 = 16
	adminPasswordKeyLen                  = 32
)

// passwordEnvOverride 标记当前内存中的 cfg.Password 是否来自 ADMIN_PASSWORD 环境变量。
// 为真时，UI 上的改密接口会直接拒绝（防止 admin 改完发现重启后又被 env 覆盖）。
var passwordEnvOverride bool

// ErrInvalidOldPassword 用于 ChangeAdminPassword：调用方据此区分
// "旧密码错"（401 给前端）vs hash/写盘等服务端错（500）。
var ErrInvalidOldPassword = fmt.Errorf("invalid old password")

// SetPassword 接受明文密码（典型来自 ADMIN_PASSWORD 环境变量），hash 后写入内存。
// 不写盘（避免 env 覆盖回写到 config.json 造成混淆）。同时标记 envOverride=true，
// 之后 UI 的改密接口会返回 409 拒绝。
//
// 向后兼容：ENV 启动路径走宽松校验（≥8 + 弱口令字典）；UI 改密走 ChangeAdminPassword 的 ≥12 严格策略。
func SetPassword(password string) error {
	if err := ValidateStrongPassword(password, 8); err != nil {
		return err
	}
	hash, err := HashAdminPassword(password)
	if err != nil {
		return err
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Password = hash
	passwordEnvOverride = true
	return nil
}

// HashAdminPassword 用 argon2id 生成 PHC 格式的密码 hash。
func HashAdminPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	salt := make([]byte, adminPasswordSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey(
		[]byte(password), salt,
		adminPasswordArgonTime, adminPasswordArgonMemory, adminPasswordArgonParallelism,
		adminPasswordKeyLen,
	)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		adminPasswordArgonMemory, adminPasswordArgonTime, adminPasswordArgonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyAdminPassword 验证明文密码与存储 hash 是否匹配。
// 支持 argon2id（推荐）/ bcrypt（兼容）/ 明文（迁移期最后兜底）。
func VerifyAdminPassword(password string) bool {
	cfgLock.RLock()
	stored := cfg.Password
	cfgLock.RUnlock()
	return verifyAdminPasswordHash(password, stored)
}

// ChangeAdminPassword 校验旧密码后写入新 hash。
// 调用前会检查 ADMIN_PASSWORD env override，若启用则直接拒绝。
func ChangeAdminPassword(oldPassword, newPassword string) error {
	if err := ValidateAdminPasswordStrength(newPassword); err != nil {
		return err
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if passwordEnvOverride {
		return fmt.Errorf("password managed by ADMIN_PASSWORD env")
	}
	if !verifyAdminPasswordHash(oldPassword, cfg.Password) {
		return ErrInvalidOldPassword
	}
	hash, err := HashAdminPassword(newPassword)
	if err != nil {
		return err
	}
	cfg.Password = hash
	return Save()
}

// IsSupportedPasswordHash 检测一个字符串是否是已知的 hash 格式。
func IsSupportedPasswordHash(s string) bool {
	return strings.HasPrefix(s, "$argon2id$") ||
		strings.HasPrefix(s, "$2a$") ||
		strings.HasPrefix(s, "$2b$") ||
		strings.HasPrefix(s, "$2y$")
}

// IsPasswordEnvOverride 报告当前密码是否被 ADMIN_PASSWORD env 覆盖。
func IsPasswordEnvOverride() bool {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return passwordEnvOverride
}

func verifyAdminPasswordHash(password, stored string) bool {
	if stored == "" {
		return false
	}
	switch {
	case strings.HasPrefix(stored, "$argon2id$"):
		return verifyArgon2IDPassword(password, stored)
	case strings.HasPrefix(stored, "$2a$"),
		strings.HasPrefix(stored, "$2b$"),
		strings.HasPrefix(stored, "$2y$"):
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	default:
		// 明文兜底（迁移期；migrateAdminPasswordLocked 失败时仍允许登录修复）
		return subtle.ConstantTimeCompare([]byte(password), []byte(stored)) == 1
	}
}

func verifyArgon2IDPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, iterations, parallelism uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	if memory == 0 || iterations == 0 || parallelism == 0 || parallelism > 255 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

// migrateAdminPasswordLocked 把明文 cfg.Password 升级为 argon2id hash。
// 调用方必须已经持有 cfgLock.Lock()。
// 迁移前自动备份 config.json 到 config.json.bak_admin_password_<timestamp>。
// 迁移失败时回滚内存到原值（避免锁死自己）。
func migrateAdminPasswordLocked() error {
	if cfg == nil || cfg.Password == "" || IsSupportedPasswordHash(cfg.Password) {
		return nil
	}
	backupPath := fmt.Sprintf("%s.bak_admin_password_%s", cfgPath, time.Now().Format("20060102_150405"))
	if data, err := os.ReadFile(cfgPath); err == nil {
		if werr := os.WriteFile(backupPath, data, 0600); werr != nil {
			return fmt.Errorf("backup admin password config: %w", werr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read config for admin password backup: %w", err)
	}
	original := cfg.Password
	hash, err := HashAdminPassword(original)
	if err != nil {
		return err
	}
	cfg.Password = hash
	if err := Save(); err != nil {
		cfg.Password = original
		return fmt.Errorf("save migrated admin password failed (backup=%s): %w", backupPath, err)
	}
	fmt.Printf("[config] admin password migrated to argon2id (backup=%s)\n", backupPath)
	return nil
}
