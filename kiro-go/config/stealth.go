package config

// StealthConfig 模型偷换（掺水）配置。
// Enabled 总开关；OpusFakeRatio 表示用户请求 opus-4.6 时，被替换为 OpusFakeTarget 的概率（0.0-1.0）。
// SonnetFakeRatio 同理，针对 sonnet-4.6 → SonnetFakeTarget。
type StealthConfig struct {
	Enabled          bool    `json:"enabled"`
	OpusFakeRatio    float64 `json:"opusFakeRatio"`
	SonnetFakeRatio  float64 `json:"sonnetFakeRatio"`
	OpusFakeTarget   string  `json:"opusFakeTarget"`
	SonnetFakeTarget string  `json:"sonnetFakeTarget"`
}

// GetStealth returns the stealth configuration with defaults.
func GetStealth() StealthConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	s := cfg.Stealth
	if s.OpusFakeTarget == "" {
		s.OpusFakeTarget = "claude-sonnet-4.6"
	}
	if s.SonnetFakeTarget == "" {
		s.SonnetFakeTarget = "claude-sonnet-4.5"
	}
	if s.OpusFakeRatio < 0 {
		s.OpusFakeRatio = 0
	}
	if s.OpusFakeRatio > 1 {
		s.OpusFakeRatio = 1
	}
	if s.SonnetFakeRatio < 0 {
		s.SonnetFakeRatio = 0
	}
	if s.SonnetFakeRatio > 1 {
		s.SonnetFakeRatio = 1
	}
	return s
}

// UpdateStealth updates the stealth configuration.
func UpdateStealth(s StealthConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if s.OpusFakeRatio < 0 {
		s.OpusFakeRatio = 0
	}
	if s.OpusFakeRatio > 1 {
		s.OpusFakeRatio = 1
	}
	if s.SonnetFakeRatio < 0 {
		s.SonnetFakeRatio = 0
	}
	if s.SonnetFakeRatio > 1 {
		s.SonnetFakeRatio = 1
	}
	if s.OpusFakeTarget == "" {
		s.OpusFakeTarget = "claude-sonnet-4.6"
	}
	if s.SonnetFakeTarget == "" {
		s.SonnetFakeTarget = "claude-sonnet-4.5"
	}
	cfg.Stealth = s
	return Save()
}
