package users

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kiro-api-proxy/config"

	"golang.org/x/crypto/bcrypt"
)

// migrationLogMu 串行化 migration_log.jsonl 的 append，避免并发 bind 写入交错。
var migrationLogMu sync.Mutex

// appendMigrationLog 追加一行到 data/migration_log.jsonl。
// 失败不抛错（避免阻塞 bind 流程），但 stderr 打 warning 供运维查日志。
// schema 字段顺序与 plan v7-refactor.md 约定一致：ts/keyID/keyNote/oldBalance/userID/username/email/status/err
func appendMigrationLog(ts int64, keyID, keyNote string, oldBalance float64, userID, username, email, status, errMsg string) {
	rec := map[string]any{
		"ts":         ts,
		"keyID":      keyID,
		"keyNote":    keyNote,
		"oldBalance": oldBalance,
		"userID":     userID,
		"username":   username,
		"email":      email,
		"status":     status,
		"err":        errMsg,
	}
	line, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[users.migration] marshal failed: %v\n", err)
		return
	}
	path := filepath.Join(config.GetDataDir(), "migration_log.jsonl")
	migrationLogMu.Lock()
	defer migrationLogMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[users.migration] mkdir failed: %v\n", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[users.migration] open failed: %v\n", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "[users.migration] write failed: %v\n", err)
	}
}

// 全局开关（v1 hardcode；v2 admin UI 可调）
var (
	AllowSelfRegister     = true  // 是否允许注册接口；admin 后续可关
	RequireActivationCode = false // 注册时是否强制激活码（v7 起复用 config.ActivationCodes，注册即消费 + 入账）
)

var (
	// ErrKeyAlreadyBound 当前 ApiKey 已经关联到某个 User
	ErrKeyAlreadyBound = errors.New("key already bound to user")
	// ErrEmailAlreadyRegistered 邮箱已被注册
	ErrEmailAlreadyRegistered = errors.New("email already registered")
)

func newUserID(now int64) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("usr_%d_%s", now, hex.EncodeToString(b[:]))
}

// LoginByUsernameOrEmail 校验登录凭据（v2 主登录路径）。
// identifier 含 '@' → 走 email 路径（向后兼容 v1 的纯邮箱登录）；否则 → 走 username 路径。
//
// 如果 user 没绑定 ApiKey，会自动创建一张 default key（plan=credit，余额 0）。
func LoginByUsernameOrEmail(identifier, password string) (User, config.ApiKeyInfo, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return User{}, config.ApiKeyInfo{}, errors.New("invalid credentials")
	}
	var u User
	var ok bool
	if strings.Contains(identifier, "@") {
		u, ok = Default().FindByEmail(identifier)
	} else {
		u, ok = Default().FindByUsername(identifier)
	}
	if !ok {
		return User{}, config.ApiKeyInfo{}, errors.New("invalid credentials")
	}
	if u.Disabled {
		return User{}, config.ApiKeyInfo{}, errors.New("account disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return User{}, config.ApiKeyInfo{}, errors.New("invalid credentials")
	}

	key, err := ensureUserApiKey(&u)
	if err != nil {
		return User{}, config.ApiKeyInfo{}, err
	}

	now := time.Now().Unix()
	_ = Default().UpdateUser(u.ID, func(uu *User) {
		uu.LastLoginAt = now
		// 把可能新建的 key 也写回
		uu.ApiKeyIDs = u.ApiKeyIDs
		uu.DefaultKeyID = u.DefaultKeyID
	})
	return u, key, nil
}

// LoginByPassword 是 LoginByUsernameOrEmail 的向后兼容别名（v1 调用方继续用 email）。
func LoginByPassword(email, password string) (User, config.ApiKeyInfo, error) {
	return LoginByUsernameOrEmail(email, password)
}

// ensureUserApiKey 返回 user 的 default ApiKey；没有时新建一张并写回 user 索引。
//
// 注意：会就地修改 u 的 ApiKeyIDs / DefaultKeyID（调用方需要把改动写回 store）。
func ensureUserApiKey(u *User) (config.ApiKeyInfo, error) {
	// 优先 default
	if u.DefaultKeyID != "" {
		if k := config.FindApiKeyByID(u.DefaultKeyID); k != nil {
			return *k, nil
		}
	}
	for _, id := range u.ApiKeyIDs {
		if k := config.FindApiKeyByID(id); k != nil {
			u.DefaultKeyID = id
			return *k, nil
		}
	}
	// 没有任何 key → 新建
	now := time.Now().Unix()
	key := config.ApiKeyInfo{
		ID:        newUserID(now), // 复用 short id；config 层不强制特定前缀
		Key:       generateOpaqueKey(),
		Plan:      "credit",
		Enabled:   true,
		Note:      u.Email,
		CreatedAt: now,
	}
	if err := config.AddApiKey(key); err != nil {
		return config.ApiKeyInfo{}, fmt.Errorf("create api key: %w", err)
	}
	u.ApiKeyIDs = append(u.ApiKeyIDs, key.ID)
	u.DefaultKeyID = key.ID
	return key, nil
}

func generateOpaqueKey() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return "sk-pivot-" + hex.EncodeToString(b[:])
}

// RegisterInput 是注册接口入参。
// v2 起 Username 字段忽略（自动从 email 前缀派生 + 冲突 suffix）；保留字段是为了向后兼容旧客户端。
// v7 起 InviteCode 改名 ActivationCode，对应 config.ActivationCodes 中的兑换码。
type RegisterInput struct {
	Email          string `json:"email"`
	Username       string `json:"username,omitempty"` // v2: 入参被忽略，仅保留以兼容老客户端
	Password       string `json:"password"`
	ActivationCode string `json:"activationCode,omitempty"`
}

// validateActivationCodeForRegistration preflight 校验：码必须存在、未过期、未消费。
// 真正的 redeem 与扣减由 config.RedeemActivationCode 在 user 落库后执行。
func validateActivationCodeForRegistration(code string, now int64) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("activation code required")
	}
	for _, ac := range config.GetActivationCodes() {
		if ac.Code != code {
			continue
		}
		if ac.Used {
			return errors.New("activation code already used")
		}
		if ac.CodeExpiresAt > 0 && now > ac.CodeExpiresAt {
			return errors.New("activation code has expired")
		}
		return nil
	}
	return errors.New("activation code not found")
}

// Register 自助注册。返回新建的 User + ApiKey。
// v2 username 行为：自动从 email 前缀派生（NormalizeUsername），冲突时加 -2/-3 后缀。
func Register(in RegisterInput) (User, config.ApiKeyInfo, error) {
	if !AllowSelfRegister {
		return User{}, config.ApiKeyInfo{}, errors.New("self-register disabled")
	}
	if err := ValidateEmail(in.Email); err != nil {
		return User{}, config.ApiKeyInfo{}, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return User{}, config.ApiKeyInfo{}, err
	}
	now := time.Now().Unix()
	email := NormalizeEmail(in.Email)
	if _, exists := Default().FindByEmail(email); exists {
		return User{}, config.ApiKeyInfo{}, ErrEmailAlreadyRegistered
	}

	activationCode := strings.TrimSpace(in.ActivationCode)
	if RequireActivationCode {
		if err := validateActivationCodeForRegistration(activationCode, now); err != nil {
			return User{}, config.ApiKeyInfo{}, err
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, config.ApiKeyInfo{}, fmt.Errorf("hash password: %w", err)
	}

	// v2: 从 email 前缀派生 username
	base := email
	if at := strings.IndexByte(email, '@'); at > 0 {
		base = email[:at]
	}
	username := Default().NextAvailableUsername(base)

	u := User{
		ID:            newUserID(now),
		Email:         email,
		Username:      username,
		PasswordHash:  string(hash),
		ApiKeyIDs:     []string{},
		InvitedBy:     activationCode, // v7: 字段名沿用 invitedBy，语义改为激活码
		InviterUserID: "",             // 激活码注册无邀请人语义
		CreatedAt:     now,
	}
	key, err := ensureUserApiKey(&u)
	if err != nil {
		return User{}, config.ApiKeyInfo{}, err
	}
	if err := Default().AddUser(u); err != nil {
		// AddUser 失败 → 回滚刚创建的 key，避免孤儿 key
		_ = config.DeleteApiKey(key.ID)
		return User{}, config.ApiKeyInfo{}, err
	}
	if RequireActivationCode {
		if _, err := config.RedeemActivationCode(activationCode, key.ID); err != nil {
			// redeem 失败 → 回滚 user + key，避免脏数据
			_ = Default().DeleteUser(u.ID)
			_ = config.DeleteApiKey(key.ID)
			return User{}, config.ApiKeyInfo{}, fmt.Errorf("redeem activation code: %w", err)
		}
		updated := config.FindApiKeyByID(key.ID)
		if updated == nil {
			// Redeem 返回成功但 key 在并发删除窗口里消失 → 视为失败回滚
			_ = Default().DeleteUser(u.ID)
			return User{}, config.ApiKeyInfo{}, errors.New("api key missing after activation code redeem")
		}
		key = *updated
	}
	return u, key, nil
}

// BindKeyToNewUser 把现有 key 关联到新建 User（v6 老 key 升级 → v7 双登录路径）。
//
// 要求：
//  1. key 存在且未绑过 User
//  2. email 全局唯一
//  3. ValidateEmail + ValidatePassword
//  4. 创建 User，apiKeyIds=[key.id]，defaultKeyId=key.id
//
// v7 行为：
//  - usernameOverride 非空 → 经 NormalizeUsername 后使用（用户在弹窗里改了的情况）；
//  - usernameOverride 空 → 从 key.Note 派生（NormalizeUsername）；Note 也空 → 从 email 前缀派生
//  - username 冲突时自动加 -2/-3 suffix
//  - 整个过程在 bcrypt 之外串行：bcrypt 在锁外完成
//  - balance 来源 = key.Balance + key.GiftBalance（不迁移到 user，仍挂在 key 上 — 双登录架构 key 是 balance 载体）
//  - 失败时 AddUser 错误 → 不影响 key（key.OwnerUserID 暂未引入，保持兼容）
//  - 成功/失败都 append 一行到 data/migration_log.jsonl 供审计
func BindKeyToNewUser(keyID, email, password, usernameOverride string) (User, error) {
	if keyID == "" {
		return User{}, errors.New("key id is required")
	}
	keyInfo := config.FindApiKeyByID(keyID)
	if keyInfo == nil {
		return User{}, errors.New("api key not found")
	}
	if err := ValidateEmail(email); err != nil {
		return User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}

	email = NormalizeEmail(email)
	for _, existing := range Default().ListUsers() {
		if existing.Email == email {
			return User{}, ErrEmailAlreadyRegistered
		}
		if existing.DefaultKeyID == keyID {
			return User{}, ErrKeyAlreadyBound
		}
		for _, id := range existing.ApiKeyIDs {
			if id == keyID {
				return User{}, ErrKeyAlreadyBound
			}
		}
	}

	// 派生 username：override > key.Note > email-prefix
	usernameBase := strings.TrimSpace(usernameOverride)
	if usernameBase == "" {
		usernameBase = strings.TrimSpace(keyInfo.Note)
	}
	if usernameBase == "" {
		if at := strings.IndexByte(email, '@'); at > 0 {
			usernameBase = email[:at]
		} else {
			usernameBase = "user"
		}
	}
	username := Default().NextAvailableUsername(usernameBase)

	// bcrypt 在锁外完成（10ms+ 别拖累整个 store）
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	oldBalance := keyInfo.Balance + keyInfo.GiftBalance
	now := time.Now().Unix()
	u := User{
		ID:           newUserID(now),
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		ApiKeyIDs:    []string{keyID},
		DefaultKeyID: keyID,
		CreatedAt:    now,
	}
	if err := Default().AddUser(u); err != nil {
		appendMigrationLog(now, keyID, keyInfo.Note, oldBalance, "", username, email, "failed", err.Error())
		if err.Error() == ErrEmailAlreadyRegistered.Error() {
			return User{}, ErrEmailAlreadyRegistered
		}
		return User{}, err
	}

	// v8: 搬钱 — 把 key 的 4 个钱字段累加到 user.Balance/Gift/TotalRecharged/TotalGifted。
	// 失败则回滚整个绑定（钱仍在 key 上，user 不持久化）。
	if keyInfo.ParentKeyID == "" &&
		(keyInfo.Balance != 0 || keyInfo.GiftBalance != 0 ||
			keyInfo.TotalRecharged != 0 || keyInfo.TotalGifted != 0) {
		moveErr := Default().UpdateUser(u.ID, func(uu *User) {
			uu.Balance += keyInfo.Balance
			uu.GiftBalance += keyInfo.GiftBalance
			uu.TotalRecharged += keyInfo.TotalRecharged
			uu.TotalGifted += keyInfo.TotalGifted
		})
		if moveErr != nil {
			_ = Default().DeleteUser(u.ID)
			appendMigrationLog(now, keyID, keyInfo.Note, oldBalance, "", username, email, "failed_move_wallet", moveErr.Error())
			return User{}, fmt.Errorf("move wallet: %w", moveErr)
		}
		// 钱已落 user → 清零 key legacy 字段（清零失败不回滚，user 已是事实源）。
		if clearErr := config.ClearKeyWalletFields(keyID); clearErr != nil {
			appendMigrationLog(now, keyID, keyInfo.Note, oldBalance, u.ID, username, email, "success_legacy_clear_failed", clearErr.Error())
		} else {
			appendMigrationLog(now, keyID, keyInfo.Note, oldBalance, u.ID, username, email, "success_wallet_migrated", "")
		}
	} else {
		appendMigrationLog(now, keyID, keyInfo.Note, oldBalance, u.ID, username, email, "success", "")
	}
	return u, nil
}

// ChangePassword 旧密码校验通过后更新密码。
func ChangePassword(userID, oldPassword, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	u, ok := Default().FindByID(userID)
	if !ok {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("old password incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}
	return Default().UpdateUser(userID, func(uu *User) {
		uu.PasswordHash = string(hash)
	})
}

