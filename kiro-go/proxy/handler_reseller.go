package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// handleResellerAPI 处理 /user/api/reseller/* 请求。
//
// 鉴权：
//  1. resolveUserKey 拿 ApiKeyInfo（沿用现有 Bearer key 逻辑）
//  2. keyInfo.IsReseller == true，否则 403
//  3. 操作子 key 时强制 child.ParentKeyID == keyInfo.ID
func (h *Handler) handleResellerAPI(w http.ResponseWriter, r *http.Request) {
	keyInfo := h.resolveUserKey(r)
	if keyInfo == nil {
		writeJSON(w, 401, map[string]string{"error": "invalid or missing api key"})
		return
	}
	if !keyInfo.IsReseller {
		writeJSON(w, 403, map[string]string{"error": "reseller permission required"})
		return
	}

	path := r.URL.Path
	switch {
	case path == "/user/api/reseller/summary" && r.Method == "GET":
		h.apiResellerSummary(w, keyInfo)
	case path == "/user/api/reseller/keys" && r.Method == "GET":
		h.apiListChildKeys(w, keyInfo)
	case path == "/user/api/reseller/keys" && r.Method == "POST":
		h.apiCreateChildKey(w, r, keyInfo)
	case strings.HasPrefix(path, "/user/api/reseller/keys/") && r.Method == "PATCH":
		id := strings.TrimPrefix(path, "/user/api/reseller/keys/")
		h.apiPatchChildKey(w, r, keyInfo, id)
	case strings.HasPrefix(path, "/user/api/reseller/keys/") && r.Method == "DELETE":
		id := strings.TrimPrefix(path, "/user/api/reseller/keys/")
		h.apiDeleteChildKey(w, r, keyInfo, id)
	case path == "/user/api/reseller/transfer" && r.Method == "POST":
		h.apiResellerTransfer(w, r, keyInfo)
	case path == "/user/api/reseller/transfers" && r.Method == "GET":
		h.apiResellerTransferHistory(w, r, keyInfo)
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

// GET /user/api/reseller/summary
//
// 返回 reseller 当前余额、子 key 数量、累计已售、利润估算等汇总。
func (h *Handler) apiResellerSummary(w http.ResponseWriter, parent *config.ApiKeyInfo) {
	children := config.GetChildKeys(parent.ID)
	var childTotalBalance, childTotalCredits float64
	var childTotalRequests int64
	for _, c := range children {
		childTotalBalance += c.Balance + c.GiftBalance
		childTotalCredits += c.Credits
		childTotalRequests += c.Requests
	}
	// 已实现利润 = SoldToChildren × (1 - discount)
	// 即：每卖出 $1，进价是 $discount，利润是 $(1-discount)；只在转给子 key 后产生利润。
	// 未卖出的余额仍是"库存"（按进价持有），不算亏损。
	profit := parent.SoldToChildren
	if parent.ResellerDiscount > 0 && parent.ResellerDiscount < 1 {
		profit = parent.SoldToChildren * (1 - parent.ResellerDiscount)
	}
	writeJSON(w, 200, map[string]interface{}{
		"balance":            parent.Balance,
		"giftBalance":        parent.GiftBalance,
		"totalBalance":       parent.Balance + parent.GiftBalance,
		"totalRecharged":     parent.TotalRecharged,
		"soldToChildren":     parent.SoldToChildren,
		"resellerDiscount":   parent.ResellerDiscount,
		"profitEstimateUSD":  profit,
		"maxChildKeys":       parent.MaxChildKeys,
		"childCount":         len(children),
		"childTotalBalance":  childTotalBalance,
		"childTotalCredits":  childTotalCredits,
		"childTotalRequests": childTotalRequests,
	})
}

// GET /user/api/reseller/keys
//
// 返回当前 reseller 的所有子 key（脱敏：key 字段只显示前后 4 位）。
func (h *Handler) apiListChildKeys(w http.ResponseWriter, parent *config.ApiKeyInfo) {
	children := config.GetChildKeys(parent.ID)
	out := make([]map[string]interface{}, 0, len(children))
	for _, c := range children {
		recent7d := recentCallCount(c.ID, 7)
		errType, _ := config.ValidateKeyAccess(&c)
		status := "active"
		if errType != "" {
			status = errType
		}
		out = append(out, map[string]interface{}{
			"id":             c.ID,
			"keyMasked":      maskKey(c.Key),
			"keyFull":        c.Key, // 列表里返回全 key 让 reseller 复制（自己分发的）
			"note":           c.Note,
			"plan":           c.Plan,
			"balance":        c.Balance,
			"giftBalance":    c.GiftBalance,
			"totalBalance":   c.Balance + c.GiftBalance,
			"totalRecharged": c.TotalRecharged,
			"requests":       c.Requests,
			"credits":        c.Credits,
			"recentCalls7d":  recent7d,
			"lastUsed":       c.LastUsed,
			"createdAt":      c.CreatedAt,
			"enabled":        c.Enabled,
			"status":         status,
		})
	}
	writeJSON(w, 200, map[string]interface{}{
		"keys":  out,
		"total": len(out),
	})
}

// POST /user/api/reseller/keys
//
// 创建子 key。可选初始划账 InitialBalanceUSD（同步从 reseller 余额转入子 key）。
func (h *Handler) apiCreateChildKey(w http.ResponseWriter, r *http.Request, parent *config.ApiKeyInfo) {
	var req struct {
		Note              string  `json:"note"`
		InitialBalanceUSD float64 `json:"initialBalanceUSD,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	// 上限校验
	if parent.MaxChildKeys > 0 {
		existing := config.GetChildKeys(parent.ID)
		if len(existing) >= parent.MaxChildKeys {
			writeJSON(w, 400, map[string]string{"error": fmt.Sprintf("child key limit reached (%d)", parent.MaxChildKeys)})
			return
		}
	}
	// 余额校验
	if req.InitialBalanceUSD > 0 && req.InitialBalanceUSD > parent.Balance {
		writeJSON(w, 400, map[string]string{"error": "insufficient balance for initial top-up"})
		return
	}
	// 创建子 key（强制 IsReseller=false 防套娃）
	childKey := config.ApiKeyInfo{
		ID:          config.GenerateMachineId(),
		Key:         config.GenerateApiKeyString(),
		Plan:        "credit",
		Enabled:     true,
		Note:        req.Note,
		ParentKeyID: parent.ID,
		IsReseller:  false,
		CreatedAt:   time.Now().Unix(),
	}
	if err := config.AddApiKey(childKey); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// 初始划账（如果有）
	if req.InitialBalanceUSD > 0 {
		if err := config.TransferBalance(parent.ID, childKey.ID, req.InitialBalanceUSD); err != nil {
			// 回滚：删掉刚创建的子 key
			_ = config.DeleteApiKey(childKey.ID)
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		appendRechargeRecord(buildResellerRechargeRecord(parent, &childKey, req.InitialBalanceUSD, "initial top-up by reseller"))
	}

	writeJSON(w, 200, map[string]interface{}{
		"id":      childKey.ID,
		"key":     childKey.Key,
		"balance": req.InitialBalanceUSD,
		"note":    childKey.Note,
	})
}

// PATCH /user/api/reseller/keys/:id
//
// 更新子 key 的部分字段（启用/禁用、改 note）。仅这两个字段可改。
func (h *Handler) apiPatchChildKey(w http.ResponseWriter, r *http.Request, parent *config.ApiKeyInfo, childID string) {
	child := config.FindApiKeyByID(childID)
	if child == nil || child.ParentKeyID != parent.ID {
		writeJSON(w, 404, map[string]string{"error": "child key not found"})
		return
	}
	var req struct {
		Enabled *bool   `json:"enabled,omitempty"`
		Note    *string `json:"note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	updated := *child
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if req.Note != nil {
		updated.Note = *req.Note
	}
	if err := config.UpdateApiKey(child.ID, updated); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"success": true})
}

// DELETE /user/api/reseller/keys/:id
//
// 删除子 key，自动把它剩余的 Balance + GiftBalance 退给 reseller.Balance。
func (h *Handler) apiDeleteChildKey(w http.ResponseWriter, _ *http.Request, parent *config.ApiKeyInfo, childID string) {
	child := config.FindApiKeyByID(childID)
	if child == nil || child.ParentKeyID != parent.ID {
		writeJSON(w, 404, map[string]string{"error": "child key not found"})
		return
	}
	refund, err := config.RefundChildBalance(childID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if err := config.DeleteApiKey(childID); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// 写一条退款流水（给 reseller 看）
	if refund > 0 {
		appendRechargeRecord(RechargeRecord{
			Time:      time.Now().In(cstZone()).Format("01-02 15:04:05"),
			Timestamp: time.Now().Unix(),
			KeyID:     parent.ID,
			KeyNote:   parent.Note,
			Type:      "reseller_refund",
			AmountUSD: refund,
			AmountCNY: refund * config.CNYPerUSDFace,
			Operator:  "reseller:" + parent.ID[:8],
			Note:      fmt.Sprintf("refund from deleted child %s", child.Note),
		})
	}
	writeJSON(w, 200, map[string]interface{}{
		"success":     true,
		"refundedUSD": refund,
	})
}

// POST /user/api/reseller/transfer
//
// reseller→child 转账。校验 child 必须是当前 reseller 的子 key。
func (h *Handler) apiResellerTransfer(w http.ResponseWriter, r *http.Request, parent *config.ApiKeyInfo) {
	var req struct {
		ToKeyID   string  `json:"toKeyId"`
		AmountUSD float64 `json:"amountUSD"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	if req.ToKeyID == "" || req.AmountUSD <= 0 {
		writeJSON(w, 400, map[string]string{"error": "toKeyId and amountUSD required"})
		return
	}
	child := config.FindApiKeyByID(req.ToKeyID)
	if child == nil || child.ParentKeyID != parent.ID {
		writeJSON(w, 404, map[string]string{"error": "child key not found"})
		return
	}
	if err := config.TransferBalance(parent.ID, req.ToKeyID, req.AmountUSD); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	appendRechargeRecord(buildResellerRechargeRecord(parent, child, req.AmountUSD, "transfer by reseller"))
	writeJSON(w, 200, map[string]interface{}{"success": true})
}

// GET /user/api/reseller/transfers
//
// 转账历史（reseller 视角）：从 recharge_records.jsonl 过滤出当前 reseller 涉及的 reseller_transfer_in/out + reseller_refund。
func (h *Handler) apiResellerTransferHistory(w http.ResponseWriter, r *http.Request, parent *config.ApiKeyInfo) {
	page, limit := parsePageLimit(r, 50, 500)

	// 拉所有子 key 的流水 + reseller 自己的退款流水
	children := config.GetChildKeys(parent.ID)
	keyIDs := make(map[string]string) // keyID → note
	keyIDs[parent.ID] = parent.Note
	for _, c := range children {
		keyIDs[c.ID] = c.Note
	}

	// 暴力扫所有 records，按 keyID 过滤；性能上 reseller 的子 key 不会很多
	var matched []RechargeRecord
	for kid := range keyIDs {
		recs, _ := readRechargeRecords(kid, 1, 10000)
		for _, r := range recs {
			t := r.Type
			if t == "reseller_transfer_in" || t == "reseller_transfer_out" || t == "reseller_refund" {
				matched = append(matched, r)
			}
		}
	}
	// 按 timestamp 倒序
	sortRechargesByTimestamp(matched)

	total := len(matched)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	out := make([]map[string]interface{}, 0, end-start)
	for _, rec := range matched[start:end] {
		out = append(out, map[string]interface{}{
			"time":      rec.Time,
			"timestamp": rec.Timestamp,
			"type":      rec.Type,
			"keyID":     rec.KeyID,
			"keyNote":   keyIDs[rec.KeyID],
			"amountUSD": rec.AmountUSD,
			"amountCNY": rec.AmountCNY,
			"note":      rec.Note,
		})
	}
	writeJSON(w, 200, map[string]interface{}{
		"records": out,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// ==================== Helpers ====================

// maskKey 显示前 6 + 后 4 位，中间用 ... 替换（key 通常是 sk-xxxx 长度足够）
func maskKey(k string) string {
	if len(k) <= 12 {
		return k
	}
	return k[:6] + "..." + k[len(k)-4:]
}

// cstZone 返回 CST (UTC+8)
func cstZone() *time.Location {
	return time.FixedZone("CST", 8*3600)
}

// buildResellerRechargeRecord 给子 key 写一条 reseller_transfer_in 流水（子 key 视角）。
// reseller 视角的 reseller_transfer_out 由调用方再写一次（这里只负责子 key 那条）。
func buildResellerRechargeRecord(parent *config.ApiKeyInfo, child *config.ApiKeyInfo, amountUSD float64, note string) RechargeRecord {
	return RechargeRecord{
		Time:      time.Now().In(cstZone()).Format("01-02 15:04:05"),
		Timestamp: time.Now().Unix(),
		KeyID:     child.ID,
		KeyNote:   child.Note,
		Type:      "reseller_transfer_in",
		AmountUSD: amountUSD,
		AmountCNY: amountUSD * config.CNYPerUSDFace,
		Operator:  "reseller:" + parent.ID[:8],
		Note:      note,
	}
}

// parsePageLimit 从 query 解析 page/limit
func parsePageLimit(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	page := 1
	limit := defaultLimit
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := parseIntPositive(p); err == nil {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := parseIntPositive(l); err == nil && v <= maxLimit {
			limit = v
		}
	}
	return page, limit
}

func parseIntPositive(s string) (int, error) {
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a positive int")
		}
		v = v*10 + int(c-'0')
	}
	if v <= 0 {
		return 0, fmt.Errorf("must be > 0")
	}
	return v, nil
}

// sortRechargesByTimestamp in-place 倒序
func sortRechargesByTimestamp(recs []RechargeRecord) {
	for i := 0; i < len(recs); i++ {
		for j := i + 1; j < len(recs); j++ {
			if recs[i].Timestamp < recs[j].Timestamp {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}
