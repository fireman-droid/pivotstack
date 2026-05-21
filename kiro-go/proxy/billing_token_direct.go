package proxy

import (
	"strings"

	"kiro-api-proxy/config"
)

// resolveSellPriceForChannel 是 TokenCostForChannel / PreAuthorizeTokensForChannel
// 的共用价格解析点。channelID "direct:<id>" 走 DirectChannel.SellPrice；
// 其它（v4 ChannelConfig / 全局兜底）走 config.GetSellPriceForChannel。
//
// 这条路径在 Stage 7b 才出现：runtime channel ID 加 "direct:" 前缀避免与 v4 撞表，
// 但 billing 层之前只识别 v4 形状，导致 DirectChannel 业务请求会 fail-closed
// （ErrSellPriceMissing）。这里把 DirectChannel.SellPrice 桥接成 ModelSellPrice。
func resolveSellPriceForChannel(channelID, model string) (config.ModelSellPrice, bool) {
	if price, ok := directChannelSellPrice(channelID, model); ok {
		return price, true
	}
	return config.GetSellPriceForChannel(channelID, model)
}

func directChannelSellPrice(channelID, model string) (config.ModelSellPrice, bool) {
	if !strings.HasPrefix(channelID, "direct:") {
		return config.ModelSellPrice{}, false
	}
	id := strings.TrimSpace(strings.TrimPrefix(channelID, "direct:"))
	if id == "" {
		return config.ModelSellPrice{}, false
	}
	ch, ok := config.GetDirectChannel(id)
	if !ok || !ch.Enabled || ch.DeletedAt > 0 {
		return config.ModelSellPrice{}, false
	}
	row, ok := directSellPriceRowForModel(ch.SellPrice, model)
	if !ok {
		return config.ModelSellPrice{}, false
	}
	return config.ModelSellPrice{
		InputPerM:      row.InputPerM,
		OutputPerM:     row.OutputPerM,
		CostInputPerM:  row.CostInputPerM,
		CostOutputPerM: row.CostOutputPerM,
	}, true
}

// directSellPriceRowForModel 在 DirectChannel.SellPrice 里找 model 对应行：
// 优先 per-model 覆盖（用 normalizeChannelModelKey 归一化匹配），
// 没有命中或行未配置时回退到 Default；Default 也全 0 时返回 false。
func directSellPriceRowForModel(price config.DirectSellPrice, model string) (config.DirectSellPriceRow, bool) {
	target := normalizeChannelModelKey(model)
	for k, row := range price.Models {
		if normalizeChannelModelKey(k) == target && directSellPriceRowConfigured(row) {
			return row, true
		}
	}
	if directSellPriceRowConfigured(price.Default) {
		return price.Default, true
	}
	return config.DirectSellPriceRow{}, false
}

func directSellPriceRowConfigured(row config.DirectSellPriceRow) bool {
	return row.InputPerM != 0 || row.OutputPerM != 0
}
