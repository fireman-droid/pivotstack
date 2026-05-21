package config

import (
	"fmt"
	"strings"
	"unicode"
)

// 常见弱口令 denylist。攻击者最先尝试的几十个 → 拒绝注册/改密时直接挡掉。
var weakPasswordDict = map[string]struct{}{
	"123456": {}, "123456789": {}, "12345678": {}, "password": {},
	"qwerty": {}, "abc123": {}, "111111": {}, "123123": {},
	"admin": {}, "admin123": {}, "admin123456": {}, "changeme": {},
	"password1": {}, "password123": {}, "letmein": {}, "welcome": {},
	"iloveyou": {}, "monkey": {}, "dragon": {}, "football": {},
	"baseball": {}, "princess": {}, "sunshine": {}, "master": {},
	"login": {}, "passw0rd": {}, "p@ssw0rd": {}, "qwerty123": {},
	"1q2w3e4r": {}, "zaq12wsx": {}, "trustno1": {}, "secret": {},
	"root": {}, "toor": {}, "test": {}, "test123": {},
	"user": {}, "user123": {}, "guest": {}, "guest123": {},
	"000000": {}, "666666": {}, "888888": {}, "1234567": {},
	"1234567890": {}, "password!": {}, "administrator": {}, "pivotstack": {},
}

// ValidateAdminPasswordStrength admin 新密码：≥12 / 弱口令 denylist / 至少 2 类字符。
// 旧密码兼容：不会强制现有密码立即失效，仅对新建/改密做校验。
func ValidateAdminPasswordStrength(password string) error {
	return ValidateStrongPassword(password, 12)
}

// ValidateUserPasswordStrength user 注册/改密：≥8 / 同上。
func ValidateUserPasswordStrength(password string) error {
	return ValidateStrongPassword(password, 8)
}

// ValidateStrongPassword 通用密码策略：
//   - 长度 [minLen, 256]
//   - 不在弱口令 denylist
//   - 不是纯顺序键盘（abcdef / qwerty / 12345）
//   - 至少 2 类字符（lower / upper / digit / symbol）
func ValidateStrongPassword(password string, minLen int) error {
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	if len(password) < minLen {
		return fmt.Errorf("password too short (min %d chars)", minLen)
	}
	if len(password) > 256 {
		return fmt.Errorf("password too long")
	}
	normalized := strings.ToLower(strings.TrimSpace(password))
	if _, weak := weakPasswordDict[normalized]; weak {
		return fmt.Errorf("password is too common")
	}
	if hasTrivialSequence(normalized) {
		return fmt.Errorf("password is too predictable")
	}
	if passwordClassCount(password) < 2 {
		return fmt.Errorf("password must include at least two character classes (lower/upper/digit/symbol)")
	}
	return nil
}

func passwordClassCount(password string) int {
	var lower, upper, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			symbol = true
		}
	}
	count := 0
	for _, ok := range []bool{lower, upper, digit, symbol} {
		if ok {
			count++
		}
	}
	return count
}

func hasTrivialSequence(s string) bool {
	if len(s) < 6 {
		return false
	}
	sequences := []string{
		"abcdefghijklmnopqrstuvwxyz",
		"zyxwvutsrqponmlkjihgfedcba",
		"0123456789",
		"9876543210",
		"qwertyuiop",
		"poiuytrewq",
		"asdfghjkl",
		"lkjhgfdsa",
	}
	for _, seq := range sequences {
		for i := 0; i+6 <= len(seq); i++ {
			if strings.Contains(s, seq[i:i+6]) {
				return true
			}
		}
	}
	return false
}
