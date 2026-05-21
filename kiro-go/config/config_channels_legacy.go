package config

// ==================== Channels (v3) ====================

// GetChannels 返回已配置渠道的线程安全副本（v4 深拷贝所有 map 字段，避免 lock 外被改）。
func GetChannels() []ChannelConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return deepCopyChannelsLocked(cfg.Channels)
}

// UpdateChannels 替换渠道列表（admin 接口用）。
func UpdateChannels(channels []ChannelConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Channels = channels
	return Save()
}

// GetSeries 返回当前系列列表的线程安全深拷贝（v4）。
func GetSeries() []Series {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return deepCopySeriesLocked(cfg.Series)
}

// UpdateSeries 替换系列列表（admin 接口用，v4）。
func UpdateSeries(series []Series) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Series = series
	return Save()
}

// GetRoutingConfig 原子地返回 (series, channels) 的深拷贝快照。
// admin 同时改动两者时用 UpdateRoutingConfig 保证一致。
func GetRoutingConfig() ([]Series, []ChannelConfig) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return deepCopySeriesLocked(cfg.Series), deepCopyChannelsLocked(cfg.Channels)
}

// UpdateRoutingConfig 原子地替换系列+渠道（admin 跨字段编辑用，v4）。
func UpdateRoutingConfig(series []Series, channels []ChannelConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Series = series
	cfg.Channels = channels
	return Save()
}

