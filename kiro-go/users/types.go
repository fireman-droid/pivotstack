// Package users 实现 PivotStack 用户名/密码账户体系（v6 新增）。
//
// 设计：
//   - User 是「人」的身份，可绑定 0..N 个 ApiKey；登录后返回其中一个 default ApiKey
//   - 凭证存 bcrypt hash，不存明文密码
//   - 激活码注册（v7+）：自助开户 → 立即把激活码面值注入新建 ApiKey
//   - 与现有 ApiKey 体系并存：API Key 登录走旧路径不变，新增 user login = email/password
//
// 持久化：独立 data/users.json；激活码由 config.json 的 activationCodes 管理。
package users

import (
	"errors"
	"strings"

	"kiro-api-proxy/config"
)

// User 一个登录身份。
type User struct {
	ID            string   `json:"id"`         // usr_<unix>_<short>
	Email         string   `json:"email"`      // 小写归一，全局唯一
	Username      string   `json:"username"`   // 显示名，可选
	PasswordHash  string   `json:"passwordHash"` // bcrypt
	ApiKeyIDs     []string `json:"apiKeyIds"`  // 绑定的 ApiKey.ID 列表
	DefaultKeyID  string   `json:"defaultKeyId"` // 登录返回此 key；空则 ApiKeyIDs[0]
	InvitedBy     string   `json:"invitedBy,omitempty"`     // v7+: 激活码 code；字段名保留以兼容旧 users.json
	InviterUserID string   `json:"inviterUserId,omitempty"` // legacy invite-only；激活码注册留空
	CreatedAt     int64    `json:"createdAt"`
	LastLoginAt   int64    `json:"lastLoginAt,omitempty"`
	Disabled      bool     `json:"disabled,omitempty"`

	// v8+ user wallet — bound non-child keys deposit/spend here via wallet helpers.
	// Orphan keys and reseller children still use the legacy fields on ApiKeyInfo.
	Balance        float64 `json:"balance,omitempty"`
	GiftBalance    float64 `json:"giftBalance,omitempty"`
	TotalRecharged float64 `json:"totalRecharged,omitempty"`
	TotalGifted    float64 `json:"totalGifted,omitempty"`
}

// InviteCode 邀请码（admin 生成；user 注册时消费）。
//
// Deprecated: v7+ self-register 改用 config.ActivationCode；本类型保留仅为兼容旧 users.json 中的 inviteCodes 字段。
type InviteCode struct {
	Code      string `json:"code"`      // 短码，唯一
	MaxUses   int    `json:"maxUses"`   // 0 = 无限
	UsedCount int    `json:"usedCount"`
	ExpiresAt int64  `json:"expiresAt,omitempty"` // 0 = 永久
	CreatedBy string `json:"createdBy,omitempty"`
	CreatedAt int64  `json:"createdAt"`
	Note      string `json:"note,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
}

// UsersFile 落盘结构。
type UsersFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	Users         []User       `json:"users"`
	InviteCodes   []InviteCode `json:"inviteCodes"` // Deprecated: legacy read-only; v7+ 不再读写
	UpdatedAt     int64        `json:"updatedAt,omitempty"`
}

const CurrentSchemaVersion = 3

// NormalizeEmail 邮箱归一（小写 + trim）。
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NormalizeUsername 归一化用户名：
//   - 小写
//   - 仅保留 ASCII 字母 / 数字 / '-' / '_'
//   - 非法字符（含中文 / 空格 / 标点）替换成 '_'
//   - 连续 '_' 合并为单个
//   - 首尾的 '_' / '-' 去掉
//   - 全空时返回 "user"（兜底，调用方需要处理冲突加 suffix）
//
// 不引入拼音库，避免运行时依赖膨胀；中文用户可在迁移弹窗手动调整。
func NormalizeUsername(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	lastUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
			lastUnderscore = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '_':
			if !lastUnderscore {
				b.WriteByte('_')
			}
			lastUnderscore = true
		default:
			if !lastUnderscore {
				b.WriteByte('_')
			}
			lastUnderscore = true
		}
	}
	v := strings.Trim(b.String(), "_-")
	if v == "" {
		return "user"
	}
	return v
}

// ValidateUsername 基础校验（在归一化后调用，不重复归一）。
// 长度 1-64；首字符必须是字母或数字（防 -foo 这种 CLI 误用）。
func ValidateUsername(s string) error {
	if s == "" {
		return errors.New("username is required")
	}
	if len(s) > 64 {
		return errors.New("username too long")
	}
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		return errors.New("username must start with letter or digit")
	}
	return nil
}

// ValidateEmail 基础校验（不做正则，留给后续 v2）。
func ValidateEmail(s string) error {
	s = NormalizeEmail(s)
	if s == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(s, "@") || !strings.Contains(s, ".") {
		return errors.New("invalid email")
	}
	if len(s) > 256 {
		return errors.New("email too long")
	}
	return nil
}

// ValidatePassword 用户密码强度校验：≥8 + 弱口令字典 + 复杂度（≥2 类字符）。
// 旧用户已存的 bcrypt hash 不强制 rehash，仅对新注册/改密做校验。
func ValidatePassword(s string) error {
	if err := config.ValidateUserPasswordStrength(s); err != nil {
		return errors.New(err.Error())
	}
	return nil
}
