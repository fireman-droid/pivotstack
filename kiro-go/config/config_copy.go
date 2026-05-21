package config

// deepCopySeriesLocked 调用方必须持锁。
func deepCopySeriesLocked(in []Series) []Series {
	if len(in) == 0 {
		return nil
	}
	out := make([]Series, len(in))
	for i, s := range in {
		cp := s
		if len(s.ModelPatterns) > 0 {
			cp.ModelPatterns = append([]string{}, s.ModelPatterns...)
		}
		out[i] = cp
	}
	return out
}

// deepCopyChannelsLocked 调用方必须持锁。深拷贝 Models / ModelPrices / ModelAliases / ExtraHeaders。
func deepCopyChannelsLocked(in []ChannelConfig) []ChannelConfig {
	out := make([]ChannelConfig, len(in))
	for i, c := range in {
		cp := c
		if len(c.Models) > 0 {
			cp.Models = append([]string{}, c.Models...)
		}
		if len(c.ModelPrices) > 0 {
			cp.ModelPrices = make(map[string]ModelSellPrice, len(c.ModelPrices))
			for k, v := range c.ModelPrices {
				cp.ModelPrices[k] = v
			}
		}
		if len(c.ModelAliases) > 0 {
			cp.ModelAliases = make(map[string]string, len(c.ModelAliases))
			for k, v := range c.ModelAliases {
				cp.ModelAliases[k] = v
			}
		}
		if len(c.ExtraHeaders) > 0 {
			cp.ExtraHeaders = make(map[string]string, len(c.ExtraHeaders))
			for k, v := range c.ExtraHeaders {
				cp.ExtraHeaders[k] = v
			}
		}
		out[i] = cp
	}
	return out
}

// deepCopyNewAPIProvidersLocked 调用方必须持锁。
func deepCopyNewAPIProvidersLocked(in []NewAPIProvider) []NewAPIProvider {
	if len(in) == 0 {
		return nil
	}
	out := make([]NewAPIProvider, len(in))
	copy(out, in)
	return out
}

// deepCopyNewAPIChannelsLocked 调用方必须持锁。
func deepCopyNewAPIChannelsLocked(in []NewAPIChannel) []NewAPIChannel {
	if len(in) == 0 {
		return nil
	}
	out := make([]NewAPIChannel, len(in))
	for i, c := range in {
		cp := c
		if len(c.Models) > 0 {
			cp.Models = append([]string{}, c.Models...)
		}
		out[i] = cp
	}
	return out
}

// deepCopyChannelGroupsLocked 调用方必须持锁。深拷贝 ModelPatterns 和 Channels。
func deepCopyChannelGroupsLocked(in []ChannelGroup) []ChannelGroup {
	if len(in) == 0 {
		return nil
	}
	out := make([]ChannelGroup, len(in))
	for i, g := range in {
		cp := g
		if len(g.ModelPatterns) > 0 {
			cp.ModelPatterns = append([]string{}, g.ModelPatterns...)
		}
		if len(g.Channels) > 0 {
			cp.Channels = append([]ChannelGroupChannelRef{}, g.Channels...)
		}
		out[i] = cp
	}
	return out
}
