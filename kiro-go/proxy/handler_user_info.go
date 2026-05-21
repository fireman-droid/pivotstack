package proxy

import (
	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
	"net/http"
	"time"
)

// GET /user/api/me
func (h *Handler) handleUserMe(w http.ResponseWriter, info *config.ApiKeyInfo) {
	resp := map[string]interface{}{
		"id":             info.ID,
		"tier":           info.Tier,
		"plan":           info.Plan,
		"balance":        info.Balance,
		"giftBalance":    info.GiftBalance,
		"totalBalance":   info.Balance + info.GiftBalance,
		"totalRecharged": info.TotalRecharged,
		"totalGifted":    info.TotalGifted,
		"credits":        info.Credits,
		"expiresAt":      info.ExpiresAt,
		"enabled":        info.Enabled,
		"requests":       info.Requests,
		"tokens":         info.Tokens,
		"models":         info.Models,
		"createdAt":      info.CreatedAt,
		"lastUsed":       info.LastUsed,
		"note":           info.Note,
		// v7: 暴露虚拟单位换算给 user 端 — 前端 useSystemUnit 据此把 virtual$ 转回 ¥ 显示。
		// 这是 read-only 公开数值，不含敏感字段。
		"pivotStackDollarsPerYuan": config.GetPivotStackDollarsPerYuan(),
	}

	// v6: 反查 User 实体；前端用 userId 是否为空判断是否需要"绑定账号"升级
	for _, u := range users.Default().ListUsers() {
		for _, kid := range u.ApiKeyIDs {
			if kid == info.ID {
				resp["userId"] = u.ID
				resp["email"] = u.Email
				if u.Username != "" {
					resp["username"] = u.Username
				}
				break
			}
		}
		if _, ok := resp["userId"]; ok {
			break
		}
	}

	// 代理身份：仅 reseller 自己能看到（让前端导航 v-if 显示"代理管理"菜单）。
	// 注意：永远不返回 parentKeyId 具体值 —— 子 key 不应知道自己被哪个 reseller 代理。
	if info.IsReseller {
		resp["isReseller"] = true
		resp["maxChildKeys"] = info.MaxChildKeys
		resp["resellerDiscount"] = info.ResellerDiscount
		resp["soldToChildren"] = info.SoldToChildren
	}
	// 子 key 标记：让前端隐藏充值/活动 UI，并显示"请联系服务商"提示
	if info.ParentKeyID != "" {
		resp["isChildKey"] = true
	}

	// 天卡速率上限：仅当 key 处于"按时长收费"活跃期时返回，让用户面板能展示。
	// 过期 / 纯 credit 用户不返回此字段（避免误导：以为还有限速）。
	if isTimedActive(info) {
		resp["rateLimitPerMin"] = getEffectiveRPM(info)
	}

	// Check access validity
	errType, err := config.ValidateKeyAccess(info)
	if err != nil {
		resp["status"] = errType
		resp["statusMessage"] = err.Error()
	} else {
		resp["status"] = "active"
	}

	// Time remaining for timed/hybrid plans
	if info.ExpiresAt > 0 {
		remaining := info.ExpiresAt - time.Now().Unix()
		if remaining > 0 {
			resp["daysRemaining"] = remaining / 86400
		} else {
			resp["daysRemaining"] = 0
		}
	}

	writeJSON(w, 200, resp)
}

// GET /user/api/usage - usage stats grouped by model AND by channel (v7).
// 返回 models（按 model 聚合，保留兼容）+ byChannel（v7 新增，按渠道聚合）。
func (h *Handler) handleUserUsage(w http.ResponseWriter, info *config.ApiKeyInfo) {
	h.callLogsMu.RLock()
	defer h.callLogsMu.RUnlock()

	modelStats := make(map[string]map[string]interface{})
	channelStats := make(map[string]map[string]interface{})
	var totalInput, totalOutput int
	var totalCredits float64
	count := 0

	for _, log := range h.callLogs {
		if log.ApiKeyID != info.ID || log.Status == "error" {
			continue
		}
		count++
		totalInput += log.InputTokens
		totalOutput += log.OutputTokens
		// v3 token 模式 Credits=0 但 UpstreamCredits>0；用 UpstreamCredits 兜底，
		// 否则 user dashboard 显示 0 credits 但实际已扣费
		if log.Credits > 0 {
			totalCredits += log.Credits
		} else if log.UpstreamCredits > 0 {
			totalCredits += log.UpstreamCredits
		}
		credPerCall := log.Credits
		if credPerCall == 0 && log.UpstreamCredits > 0 {
			credPerCall = log.UpstreamCredits
		}

		// 按 model 聚合（兼容旧前端）
		model := log.OriginalModel
		if _, ok := modelStats[model]; !ok {
			modelStats[model] = map[string]interface{}{
				"requests": 0, "inputTokens": 0, "outputTokens": 0, "credits": 0.0,
			}
		}
		ms := modelStats[model]
		ms["requests"] = ms["requests"].(int) + 1
		ms["inputTokens"] = ms["inputTokens"].(int) + log.InputTokens
		ms["outputTokens"] = ms["outputTokens"].(int) + log.OutputTokens
		ms["credits"] = ms["credits"].(float64) + credPerCall

		// 按 channel 聚合（v7 新增）—— 空 ChannelID（legacy 日志）归到 "legacy"
		channelID := log.ChannelID
		if channelID == "" {
			channelID = "legacy"
		}
		if _, ok := channelStats[channelID]; !ok {
			channelStats[channelID] = map[string]interface{}{
				"id": channelID, "alias": lookupChannelAlias(channelID),
				"requests": 0, "inputTokens": 0, "outputTokens": 0, "credits": 0.0, "costUsd": 0.0,
				"models": map[string]int{},
			}
		}
		cs := channelStats[channelID]
		cs["requests"] = cs["requests"].(int) + 1
		cs["inputTokens"] = cs["inputTokens"].(int) + log.InputTokens
		cs["outputTokens"] = cs["outputTokens"].(int) + log.OutputTokens
		cs["credits"] = cs["credits"].(float64) + credPerCall
		// v7：用真实计费金额（virtual $）累计 cost；credits 在 token 模式 fallback 到 UpstreamCredits
		// 会得到上游 quota 数（量级远大于 $），前端不能用 credits 直接显示成 $。
		cs["costUsd"] = cs["costUsd"].(float64) + log.CostUSD
		modelsMap := cs["models"].(map[string]int)
		modelsMap[model] = modelsMap[model] + 1
	}

	writeJSON(w, 200, map[string]interface{}{
		"totalRequests":     count,
		"totalInputTokens":  totalInput,
		"totalOutputTokens": totalOutput,
		"totalCredits":      totalCredits,
		"models":            modelStats,
		"byChannel":         channelStats,
	})
}

// lookupChannelAlias 优先 NewAPIChannel.Alias > DirectChannel.Name > raw ID。
// 对运维不暴露 upstream token / API key 等敏感字段。
func lookupChannelAlias(channelID string) string {
	if channelID == "" || channelID == "legacy" {
		return "Legacy"
	}
	// NewAPI: apijing:tok-961 等
	for _, nc := range config.GetNewAPIChannels() {
		if nc.DeletedAt == 0 && nc.ID == channelID {
			if alias := nc.Alias; alias != "" {
				return alias
			}
			return nc.UpstreamTokenName
		}
	}
	// Direct: "direct:<id>"
	if len(channelID) > 7 && channelID[:7] == "direct:" {
		for _, dc := range config.GetDirectChannels() {
			if dc.DeletedAt == 0 && "direct:"+dc.ID == channelID {
				if dc.Alias != "" {
					return dc.Alias
				}
				return dc.ID
			}
		}
	}
	return channelID
}
