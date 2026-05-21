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
	// 不再计算系统层 profit —— 让利由 admin 出激活码时手算面值，
	// reseller 卖给真实客户的现金流由 reseller 自己记账。
	writeJSON(w, 200, map[string]interface{}{
		"balance":            parent.Balance,
		"giftBalance":        parent.GiftBalance,
		"totalBalance":       parent.Balance + parent.GiftBalance,
		"totalRecharged":     parent.TotalRecharged,
		"soldToChildren":     parent.SoldToChildren,
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
// 全功能编辑（仿 admin Key 编辑）：enabled/note/balance/expiresAt 都可改。
//
// balance 改动：自动算 delta = newBalance - oldBalance，调 TransferBalance 双向转账（正负皆可）。
//   - 增加余额：从 reseller.Balance 扣 delta 给子 key（reseller 余额不足则 400）
//   - 减少余额：从子 key.Balance 扣回 |delta| 给 reseller
// 每次成功的 balance 改动写一条 RechargeRecord（type=reseller_transfer 或 reseller_recall）。
//
// 不允许修改 GiftBalance（赠送余额是 admin 直接给的，不属于 reseller 资金链）。
func (h *Handler) apiPatchChildKey(w http.ResponseWriter, r *http.Request, parent *config.ApiKeyInfo, childID string) {
	child := config.FindApiKeyByID(childID)
	if child == nil || child.ParentKeyID != parent.ID {
		writeJSON(w, 404, map[string]string{"error": "child key not found"})
		return
	}
	var req struct {
		Enabled   *bool    `json:"enabled,omitempty"`
		Note      *string  `json:"note,omitempty"`
		Balance   *float64 `json:"balance,omitempty"`   // USD face，新值（绝对量）
		ExpiresAt *int64   `json:"expiresAt,omitempty"` // unix seconds, 0 = 永不过期
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}

	// 1. 余额改动：先做（涉及 reseller 钱包，要原子性 + 校验）
	if req.Balance != nil {
		oldBalance := child.Balance
		newBalance := *req.Balance
		if newBalance < 0 {
			writeJSON(w, 400, map[string]string{"error": "balance must be >= 0"})
			return
		}
		delta := newBalance - oldBalance
		if delta != 0 {
			if err := config.TransferBalance(parent.ID, child.ID, delta); err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			// 写流水（让 reseller 看历史）
			recType := "reseller_transfer"
			if delta < 0 {
				recType = "reseller_recall"
			}
			absUSD := delta
			if absUSD < 0 {
				absUSD = -absUSD
			}
			appendRechargeRecord(RechargeRecord{
				Time:      time.Now().In(cstZone()).Format("01-02 15:04:05"),
				Timestamp: time.Now().Unix(),
				KeyID:     parent.ID,
				KeyNote:   parent.Note,
				Type:      recType,
				AmountUSD: absUSD,
				AmountCNY: config.CNYFromVirtualUSD(absUSD),
				Operator:  "reseller:" + parent.ID[:8],
				Note:      fmt.Sprintf("adjust child %s balance: $%.4f → $%.4f", child.Note, oldBalance, newBalance),
			})
			// 重新加载 child（TransferBalance 已经持久化）
			if reloaded := config.FindApiKeyByID(child.ID); reloaded != nil {
				*child = *reloaded
			}
		}
	}

	// 2. 其它字段（note / enabled / expiresAt）— 不涉及钱
	if req.Enabled != nil || req.Note != nil || req.ExpiresAt != nil {
		updated := *child
		if req.Enabled != nil {
			updated.Enabled = *req.Enabled
		}
		if req.Note != nil {
			updated.Note = *req.Note
		}
		if req.ExpiresAt != nil {
			updated.ExpiresAt = *req.ExpiresAt
		}
		if err := config.UpdateApiKey(child.ID, updated); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
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
			AmountCNY: config.CNYFromVirtualUSD(refund),
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
// reseller↔child 双向转账。amountUSD 正负都接受：
//   - amountUSD > 0: parent → child（充入）
//   - amountUSD < 0: child → parent（扣回）
//   - amountUSD = 0: 拒绝
//
// 大部分调整余额场景推荐走 PATCH /user/api/reseller/keys/:id（绝对量编辑），
// 这个 transfer 接口保留作快捷增量入口。
func (h *Handler) apiResellerTransfer(w http.ResponseWriter, r *http.Request, parent *config.ApiKeyInfo) {
	var req struct {
		ToKeyID   string  `json:"toKeyId"`
		AmountUSD float64 `json:"amountUSD"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	if req.ToKeyID == "" || req.AmountUSD == 0 {
		writeJSON(w, 400, map[string]string{"error": "toKeyId and non-zero amountUSD required"})
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
	// 写流水：正向=transfer，负向=recall
	recType := "reseller_transfer"
	note := "transfer by reseller"
	absUSD := req.AmountUSD
	if absUSD < 0 {
		recType = "reseller_recall"
		note = "recall by reseller"
		absUSD = -absUSD
	}
	appendRechargeRecord(RechargeRecord{
		Time:      time.Now().In(cstZone()).Format("01-02 15:04:05"),
		Timestamp: time.Now().Unix(),
		KeyID:     parent.ID,
		KeyNote:   parent.Note,
		Type:      recType,
		AmountUSD: absUSD,
		AmountCNY: config.CNYFromVirtualUSD(absUSD),
		Operator:  "reseller:" + parent.ID[:8],
		Note:      fmt.Sprintf("%s, child=%s", note, child.Note),
	})
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
		AmountCNY: config.CNYFromVirtualUSD(amountUSD),
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
