package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"time"
)

// POST /user/api/redeem - redeem activation code
func (h *Handler) handleUserRedeem(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
	// 子 key 不允许兑激活码：钱由所属 reseller 转账下发，
	// 否则 child 兑的钱会在 reseller 删 child 时回流（参见 RefundChildBalance），造成资金错位。
	if info != nil && info.ParentKeyID != "" {
		writeJSON(w, 403, map[string]string{"error": "子 Key 不能兑换激活码，请联系您的服务商充值"})
		return
	}

	// IP rate limiting for brute force prevention（只信 RemoteAddr，不读 XFF）
	ip := requestIP(r)
	if allowed, reason := CheckRedeemRateLimit(ip); !allowed {
		writeJSON(w, 429, map[string]string{"error": reason})
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Code == "" {
		writeJSON(w, 400, map[string]string{"error": "code is required"})
		return
	}

	// Capture before state for receipt
	balanceBefore := info.Balance
	giftBefore := info.GiftBalance
	expiresAtBefore := info.ExpiresAt

	// 在兑换前先记下激活码金额（兑换后激活码会被删除）
	var codeAmountInput float64  // 兑换码原始 amount（balance 类型为 ¥CNY，days 类型为天数）
	var codeSalePriceCNY float64 // 仅 days/time 类型：admin 设的销售价格（¥），写入流水作 revenue 来源
	{
		codes := config.GetActivationCodes()
		for _, ac := range codes {
			if ac.Code == req.Code {
				codeAmountInput = ac.Amount
				codeSalePriceCNY = ac.SalePriceCNY
				break
			}
		}
	}

	codeType, err := config.RedeemActivationCode(req.Code, info.ID)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	// Fetch updated key info
	updated := config.FindApiKeyByID(info.ID)
	if updated == nil {
		writeJSON(w, 500, map[string]string{"error": "failed to fetch updated info"})
		return
	}

	fmt.Printf("[Redeem] key=%s code=%s type=%s balance=¥%.2f expiresAt=%d\n",
		info.ID[:8], req.Code, codeType, updated.Balance, updated.ExpiresAt)

	// 写充值流水（金额关键，立即落盘）
	{
		now := time.Now()
		cst := time.FixedZone("CST", 8*3600)
		recType := "code_redeem"
		var amountUSD, amountCNY float64
		switch codeType {
		case "balance":
			// codeAmountInput 是 CNY，转 virtual$
			amountCNY = codeAmountInput
			amountUSD = config.VirtualUSDFromCNY(codeAmountInput)
		case "days", "time":
			recType = "code_redeem_days"
			// 天卡兑换收入 = ac.SalePriceCNY（admin 创建天卡时填的售价）。
			// 老天卡 / 白送 → 0，不计入 revenue（向前兼容）。
			amountCNY = codeSalePriceCNY
			amountUSD = config.VirtualUSDFromCNY(amountCNY)
		}
		ip := requestIP(r)
		appendRechargeRecord(RechargeRecord{
			Time:          now.In(cst).Format("01-02 15:04:05"),
			Timestamp:     now.Unix(),
			KeyID:         info.ID,
			KeyNote:       updated.Note,
			Type:          recType,
			Code:          req.Code,
			AmountUSD:     amountUSD,
			AmountCNY:     amountCNY,
			BalanceBefore: balanceBefore,
			BalanceAfter:  updated.Balance,
			GiftBefore:    giftBefore,
			GiftAfter:     updated.GiftBalance,
			Operator:      "user",
			Note:          fmt.Sprintf("self-redeem %s", codeType),
			IP:            ip,
		})
	}

	// Find the code amount for receipt (convert CNY → virtual$)
	var amount float64
	switch codeType {
	case "balance":
		amount = config.VirtualUSDFromCNY(codeAmountInput) // ¥ → virtual$
	case "days", "time":
		amount = codeAmountInput // days: keep as-is
	}

	writeJSON(w, 200, map[string]interface{}{
		"type":            codeType,
		"amount":          amount,
		"balance":         updated.Balance,
		"balanceBefore":   balanceBefore,
		"balanceAfter":    updated.Balance,
		"expiresAt":       updated.ExpiresAt,
		"expiresAtBefore": expiresAtBefore,
	})
}
