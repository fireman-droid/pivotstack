package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

// ==================== API Key 管理 ====================

// adminApiKeyRow 嵌入 ApiKeyInfo 并补 ¥ 单位字段，给前端列表/排行使用。
type adminApiKeyRow struct {
	config.ApiKeyInfo
	BalanceCNY      *float64 `json:"balanceCNY,omitempty"`
	GiftBalanceCNY  *float64 `json:"giftBalanceCNY,omitempty"`
	TotalBalanceCNY *float64 `json:"totalBalanceCNY,omitempty"`
}

func wrapAdminApiKeyRows(keys []config.ApiKeyInfo) []adminApiKeyRow {
	psdpy := config.GetPivotStackDollarsPerYuan()
	if psdpy <= 0 {
		psdpy = config.DefaultPivotStackDollarsPerYuan
	}
	out := make([]adminApiKeyRow, len(keys))
	for i, k := range keys {
		bal := k.Balance / psdpy
		gift := k.GiftBalance / psdpy
		total := (k.Balance + k.GiftBalance) / psdpy
		out[i] = adminApiKeyRow{
			ApiKeyInfo:      k,
			BalanceCNY:      &bal,
			GiftBalanceCNY:  &gift,
			TotalBalanceCNY: &total,
		}
	}
	return out
}

func (h *Handler) apiGetApiKeys(w http.ResponseWriter, _ *http.Request) {
	keys := config.GetAllApiKeys()
	// 合并内存中的实时统计
	h.apiKeyStatsMu.RLock()
	for i, k := range keys {
		if stats, ok := h.apiKeyStats[k.ID]; ok {
			keys[i].LastUsed = stats.LastUsed
			keys[i].Requests = stats.Requests
			keys[i].Errors = stats.Errors
			keys[i].Tokens = stats.Tokens
			keys[i].Credits = stats.Credits
			if stats.Models != nil {
				keys[i].Models = make(map[string]int64, len(stats.Models))
				for m, c := range stats.Models {
					keys[i].Models[m] = c
				}
			}
		}
	}
	h.apiKeyStatsMu.RUnlock()
	json.NewEncoder(w).Encode(wrapAdminApiKeyRows(keys))
}

func (h *Handler) apiCreateApiKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	key := config.ApiKeyInfo{
		ID:        config.GenerateMachineId(),
		Key:       config.GenerateApiKeyString(),
		Enabled:   true,
		Note:      req.Note,
		CreatedAt: time.Now().Unix(),
	}
	if err := config.AddApiKey(key); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(key)
}

func (h *Handler) apiUpdateApiKey(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Plan             *string  `json:"plan"`
		ExpiresAt        *int64   `json:"expiresAt"`
		Enabled          *bool    `json:"enabled"`
		Balance          *float64 `json:"balance"`
		GiftBalance      *float64 `json:"giftBalance"`
		Note             *string  `json:"note"`
		// 代理设置（不再有 ResellerDiscount —— 杠杆由 admin 出卡时手动定面值）
		IsReseller   *bool `json:"isReseller"`
		MaxChildKeys *int  `json:"maxChildKeys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	keys := config.GetAllApiKeys()
	var existing *config.ApiKeyInfo
	for i := range keys {
		if keys[i].ID == id {
			existing = &keys[i]
			break
		}
	}
	if existing == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "API key not found"})
		return
	}
	// v8: bound key（绑了 user 且非子卡）的钱包在 user 上 — admin 调账要写 user wallet，
	// "before" 也读 user wallet 才能记准流水。
	ownerUser, isBound := users.Default().FindByApiKeyID(id)
	isBound = isBound && existing.ParentKeyID == ""

	beforeBalance := existing.Balance
	beforeGift := existing.GiftBalance
	if isBound {
		beforeBalance = ownerUser.Balance
		beforeGift = ownerUser.GiftBalance
	}
	beforeExpiresAt := existing.ExpiresAt

	if req.Plan != nil {
		existing.Plan = *req.Plan
	}
	if req.ExpiresAt != nil {
		existing.ExpiresAt = *req.ExpiresAt
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if isBound {
		// bound 走 user wallet；existing.Balance/GiftBalance 保持 0
		if req.Balance != nil || req.GiftBalance != nil {
			newPaid := beforeBalance
			newGift := beforeGift
			if req.Balance != nil {
				newPaid = *req.Balance
			}
			if req.GiftBalance != nil {
				newGift = *req.GiftBalance
			}
			if _, werr := users.SetWalletBalances(id, newPaid, newGift); werr != nil {
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(map[string]string{"error": werr.Error()})
				return
			}
		}
	} else {
		// 孤儿 / 子卡走 key 路径
		if req.Balance != nil {
			existing.Balance = *req.Balance
		}
		if req.GiftBalance != nil {
			existing.GiftBalance = *req.GiftBalance
		}
	}
	if req.Note != nil {
		existing.Note = *req.Note
	}
	// 代理设置：子 key 不允许开代理（防套娃）
	if req.IsReseller != nil {
		if existing.ParentKeyID != "" && *req.IsReseller {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "child key cannot become reseller"})
			return
		}
		existing.IsReseller = *req.IsReseller
		// 关闭代理时清空相关字段（保留 SoldToChildren 作为历史统计）
		if !existing.IsReseller {
			existing.MaxChildKeys = 0
			existing.ResellerDiscount = 0 // 历史字段：清零，新版不再使用
		}
	}
	if req.MaxChildKeys != nil && existing.IsReseller {
		if *req.MaxChildKeys < 0 {
			existing.MaxChildKeys = 0
		} else {
			existing.MaxChildKeys = *req.MaxChildKeys
		}
	}
	if err := config.UpdateApiKey(id, *existing); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 审计 + 充值流水（如果 balance/gift/expiresAt 有变化）
	operator := operatorFromRequest(r)
	now := time.Now()
	cst := time.FixedZone("CST", 8*3600)

	// v8: 流水里"after" 读 wallet（bound 时 existing.Balance=0 不真实，要读 user.Balance）
	afterBalance := existing.Balance
	afterGift := existing.GiftBalance
	if isBound {
		if w, _ := users.GetWalletTotals(id); true {
			afterBalance = w.Balance
			afterGift = w.GiftBalance
		}
	}

	// balance 变化 → 写流水
	if req.Balance != nil && afterBalance != beforeBalance {
		delta := afterBalance - beforeBalance
		appendRechargeRecord(RechargeRecord{
			Time: now.In(cst).Format("01-02 15:04:05"), Timestamp: now.Unix(),
			KeyID: existing.ID, KeyNote: existing.Note,
			Type: "admin_adjust", AmountUSD: delta, AmountCNY: config.CNYFromVirtualUSD(delta),
			BalanceBefore: beforeBalance, BalanceAfter: afterBalance,
			GiftBefore: beforeGift, GiftAfter: afterGift,
			Operator: operator, Note: "admin balance adjust",
		})
		AuditLog("apikey_balance_adjust", operator,
			fmt.Sprintf("keyID=%s before=$%.4f after=$%.4f delta=$%.4f", existing.ID, beforeBalance, afterBalance, delta))
	}
	// gift 变化 → 写流水
	if req.GiftBalance != nil && afterGift != beforeGift {
		delta := afterGift - beforeGift
		appendRechargeRecord(RechargeRecord{
			Time: now.In(cst).Format("01-02 15:04:05"), Timestamp: now.Unix(),
			KeyID: existing.ID, KeyNote: existing.Note,
			Type: "admin_gift", AmountUSD: delta, AmountCNY: 0, // 赠送不算 CNY 充值
			BalanceBefore: beforeBalance, BalanceAfter: afterBalance,
			GiftBefore: beforeGift, GiftAfter: afterGift,
			Operator: operator, Note: "admin gift adjust",
		})
		AuditLog("apikey_gift_adjust", operator,
			fmt.Sprintf("keyID=%s before=$%.4f after=$%.4f delta=$%.4f", existing.ID, beforeGift, afterGift, delta))
	}
	// ExpiresAt 变化 → audit（不写充值流水，但留痕方便排查"天卡消失"）
	if req.ExpiresAt != nil && existing.ExpiresAt != beforeExpiresAt {
		AuditLog("apikey_expires_change", operator,
			fmt.Sprintf("keyID=%s before=%d after=%d delta=%d", existing.ID, beforeExpiresAt, existing.ExpiresAt, existing.ExpiresAt-beforeExpiresAt))
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiDeleteApiKey(w http.ResponseWriter, r *http.Request, id string) {
	existing := config.FindApiKeyByID(id)
	// v8: 先从 user.ApiKeyIDs 移除（如绑定），不影响 user wallet。
	_ = users.DetachKeyFromUsers(id)
	if err := config.DeleteApiKey(id); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if existing != nil {
		AuditLog("apikey_delete", operatorFromRequest(r),
			fmt.Sprintf("keyID=%s note=%q balance=$%.4f gift=$%.4f",
				existing.ID, existing.Note, existing.Balance, existing.GiftBalance))
	}
	h.apiKeyStatsMu.Lock()
	delete(h.apiKeyStats, id)
	h.apiKeyStatsMu.Unlock()
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetApiKeyLogs(w http.ResponseWriter, _ *http.Request, keyID string) {
	h.callLogsMu.RLock()
	var filtered []CallLog
	for _, log := range h.callLogs {
		if log.ApiKeyID == keyID {
			filtered = append(filtered, log)
		}
	}
	h.callLogsMu.RUnlock()
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"logs": filtered})
}
