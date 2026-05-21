package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

// unitConfigHistory 一行历史变更（pivotstack_unit_change action）。
type unitConfigHistory struct {
	Time     int64   `json:"time"`
	OldValue float64 `json:"oldValue"`
	NewValue float64 `json:"newValue"`
	Actor    string  `json:"actor"`
}

// readUnitConfigHistory 扫 data/audit.log 提取所有 pivotstack_unit_change 记录，最新在前，最多 50 条。
// audit.log 行格式：[2026-05-20 10:51:18] action=pivotstack_unit_change operator=<x> old=<v> new=<v> ...
func readUnitConfigHistory() []unitConfigHistory {
	dir := config.GetDataDir()
	if dir == "" {
		dir = "data"
	}
	f, err := os.Open(filepath.Join(dir, "audit.log"))
	if err != nil {
		return nil
	}
	defer f.Close()

	re := regexp.MustCompile(`^\[(.+?)\] action=pivotstack_unit_change operator=(\S+) old=([0-9.]+) new=([0-9.]+)`)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	cst := time.FixedZone("CST", 8*3600)
	out := make([]unitConfigHistory, 0)
	for scanner.Scan() {
		m := re.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		ts, _ := time.ParseInLocation("2006-01-02 15:04:05", m[1], cst)
		oldV, _ := strconv.ParseFloat(m[3], 64)
		newV, _ := strconv.ParseFloat(m[4], 64)
		out = append(out, unitConfigHistory{
			Time: ts.Unix(), OldValue: oldV, NewValue: newV, Actor: m[2],
		})
	}
	// 反序：最新在最上
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

// unitConfigResponse 是 GET /admin/api/system/unit-config 的响应。
// usersTotalPaid/Gift 是当前所有 ApiKey 的余额总和（虚拟$），供 admin 预估改值影响。
type unitConfigResponse struct {
	PivotStackDollarsPerYuan float64             `json:"pivotStackDollarsPerYuan"`
	DefaultValue             float64             `json:"defaultValue"`
	UsersCount               int                 `json:"usersCount"`
	UsersTotalPaid           float64             `json:"usersTotalPaid"`
	UsersTotalGift           float64             `json:"usersTotalGift"`
	History                  []unitConfigHistory `json:"history"`
}

// unitConfigChangeRequest 是 POST /admin/api/system/unit-config 的请求。
// 必须二次输入 admin 密码（敏感操作；改值会影响所有用户的虚拟$余额显示，
// 若 rebalance=false 则用户名义余额数字不动但购买力翻倍/减半）。
type unitConfigChangeRequest struct {
	NewValue              float64 `json:"newValue"`
	RebalanceUserBalances bool    `json:"rebalanceUserBalances"`
	AdminPassword         string  `json:"adminPassword"`
}

// unitConfigChangeResponse 包含改值前后的 stats，admin UI 用来显示「已影响 N 用户，余额差 ¥X」。
type unitConfigChangeResponse struct {
	OldValue        float64 `json:"oldValue"`
	NewValue        float64 `json:"newValue"`
	Rebalanced      bool    `json:"rebalanced"`
	UsersAffected   int     `json:"usersAffected"`
	PaidBalanceDiff float64 `json:"paidBalanceDiff"`
	GiftBalanceDiff float64 `json:"giftBalanceDiff"`
}

// GET /admin/api/system/unit-config — admin 查看当前值 + 用户余额总数（预估改值影响用）。
func (h *Handler) apiGetSystemUnitConfig(w http.ResponseWriter, _ *http.Request) {
	keys := config.GetAllApiKeys()
	var totalPaid, totalGift float64
	for _, k := range keys {
		totalPaid += k.Balance
		totalGift += k.GiftBalance
	}
	writeAdminJSON(w, http.StatusOK, unitConfigResponse{
		PivotStackDollarsPerYuan: config.GetPivotStackDollarsPerYuan(),
		DefaultValue:             config.DefaultPivotStackDollarsPerYuan,
		UsersCount:               len(keys),
		UsersTotalPaid:           totalPaid,
		UsersTotalGift:           totalGift,
		History:                  readUnitConfigHistory(),
	})
}

// POST /admin/api/system/unit-config — 改全局虚拟单位换算，要求二次输入 admin 密码。
func (h *Handler) apiPostSystemUnitConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var req unitConfigChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.NewValue <= 0 {
		writeAdminJSONError(w, http.StatusBadRequest, "newValue must be > 0")
		return
	}
	if req.AdminPassword == "" {
		writeAdminJSONError(w, http.StatusUnauthorized, "admin password required for sensitive operation")
		return
	}
	if !config.VerifyAdminPassword(req.AdminPassword) {
		writeAdminJSONError(w, http.StatusUnauthorized, "admin password mismatch")
		return
	}

	stats, err := config.UpdatePivotStackDollarsPerYuanWithStats(req.NewValue, req.RebalanceUserBalances)
	if err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	// v8: rebalance 还要同步 User 钱包，否则换算改了但 user.Balance 跟不上 → 显示与扣费撕裂。
	if req.RebalanceUserBalances && stats.OldValue > 0 {
		factor := stats.NewValue / stats.OldValue
		if usersAffected, paidDiff, giftDiff, werr := users.RebalanceWallets(factor); werr != nil {
			AuditLog("pivotstack_unit_change_wallet_rebalance_failed", adminAuditActor(r), werr.Error())
		} else {
			stats.UsersAffected += usersAffected
			stats.PaidBalanceDiff += paidDiff
			stats.GiftBalanceDiff += giftDiff
		}
	}
	// 改值不影响路由（PivotStackDollarsPerYuan 是 reservation snapshot，已在飞行的请求不受影响）。
	// 但 admin 后续创建的 reservation 用新值，UI 余额显示也用新值；call_log 估算成本会按新值算。
	AuditLog("pivotstack_unit_change", adminAuditActor(r),
		fmt.Sprintf("old=%.6f new=%.6f rebalance=%v users=%d", stats.OldValue, stats.NewValue, stats.Rebalanced, stats.UsersAffected))

	writeAdminJSON(w, http.StatusOK, unitConfigChangeResponse{
		OldValue:        stats.OldValue,
		NewValue:        stats.NewValue,
		Rebalanced:      stats.Rebalanced,
		UsersAffected:   stats.UsersAffected,
		PaidBalanceDiff: stats.PaidBalanceDiff,
		GiftBalanceDiff: stats.GiftBalanceDiff,
	})
}
