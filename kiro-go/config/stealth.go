package config

import "strings"

// StealthRule 单条掺水规则：用户原始请求模型名匹配 SourcePattern 时，
// 按 Ratio 概率替换为 Target。
//
// SourcePattern 用小写子串匹配（含 `-/.` 互换兼容）。
// 例如 SourcePattern="opus-4.7" 会同时匹配 "claude-opus-4.7" / "claude-opus-4-7" / "opus 4.7" 等变体。
type StealthRule struct {
	SourcePattern string  `json:"sourcePattern"` // 小写子串，如 "opus-4.6" / "opus-4.7" / "sonnet-4.6"
	Target        string  `json:"target"`        // 目标模型，如 "claude-sonnet-4.6"
	Ratio         float64 `json:"ratio"`         // 0.0-1.0
	Note          string  `json:"note,omitempty"`
}

// StealthConfig 模型偷换（掺水）配置。
//
// 推荐使用 Rules 列表（通用，任意条数）；旧字段 OpusFakeRatio/SonnetFakeRatio 仅向后兼容，
// 启动时会自动迁移到 Rules（GetStealth 内做透明转换）。
type StealthConfig struct {
	Enabled bool          `json:"enabled"`
	Rules   []StealthRule `json:"rules,omitempty"`

	// === Deprecated: 旧字段（启动后自动迁移到 Rules，写回时会清除）===
	OpusFakeRatio    float64 `json:"opusFakeRatio,omitempty"`
	SonnetFakeRatio  float64 `json:"sonnetFakeRatio,omitempty"`
	OpusFakeTarget   string  `json:"opusFakeTarget,omitempty"`
	SonnetFakeTarget string  `json:"sonnetFakeTarget,omitempty"`
}

// migrateLegacyStealth 把旧的 Opus/Sonnet 字段转成 Rules（仅当 Rules 为空时）。
func migrateLegacyStealth(s *StealthConfig) {
	if len(s.Rules) > 0 {
		return
	}
	if s.OpusFakeRatio > 0 || s.OpusFakeTarget != "" {
		target := s.OpusFakeTarget
		if target == "" {
			target = "claude-sonnet-4.6"
		}
		s.Rules = append(s.Rules, StealthRule{
			SourcePattern: "opus-4.6",
			Target:        target,
			Ratio:         clampRatio(s.OpusFakeRatio),
			Note:          "(legacy opus rule)",
		})
	}
	if s.SonnetFakeRatio > 0 || s.SonnetFakeTarget != "" {
		target := s.SonnetFakeTarget
		if target == "" {
			target = "claude-sonnet-4.5"
		}
		s.Rules = append(s.Rules, StealthRule{
			SourcePattern: "sonnet-4.6",
			Target:        target,
			Ratio:         clampRatio(s.SonnetFakeRatio),
			Note:          "(legacy sonnet rule)",
		})
	}
}

func clampRatio(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// GetStealth returns the stealth configuration with defaults + legacy migration.
func GetStealth() StealthConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	s := cfg.Stealth
	migrateLegacyStealth(&s)
	for i := range s.Rules {
		s.Rules[i].Ratio = clampRatio(s.Rules[i].Ratio)
		s.Rules[i].SourcePattern = strings.ToLower(strings.TrimSpace(s.Rules[i].SourcePattern))
	}
	return s
}

// UpdateStealth updates the stealth configuration.
// 写入时把旧字段清掉（已迁移到 Rules），保持 config.json 整洁。
func UpdateStealth(s StealthConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	migrateLegacyStealth(&s)
	for i := range s.Rules {
		s.Rules[i].Ratio = clampRatio(s.Rules[i].Ratio)
		s.Rules[i].SourcePattern = strings.ToLower(strings.TrimSpace(s.Rules[i].SourcePattern))
		if s.Rules[i].Target == "" {
			s.Rules[i].Target = "claude-sonnet-4.6"
		}
	}
	// 清掉 legacy 字段
	s.OpusFakeRatio = 0
	s.SonnetFakeRatio = 0
	s.OpusFakeTarget = ""
	s.SonnetFakeTarget = ""
	cfg.Stealth = s
	return Save()
}
