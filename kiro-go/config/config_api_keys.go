package config

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"
)

// ==================== API Key CRUD ====================

// FindApiKey 用 constant-time 比较防御时序侧信道。
// key 不存在时所有循环走完，避免按存在/不存在产生可观测时间差。
func FindApiKey(key string) *ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	provided := []byte(key)
	for _, k := range cfg.ApiKeys {
		stored := []byte(k.Key)
		if len(stored) == len(provided) && subtle.ConstantTimeCompare(stored, provided) == 1 {
			c := copyApiKeyInfoLocked(k)
			return &c
		}
	}
	return nil
}

func GetAllApiKeys() []ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	keys := make([]ApiKeyInfo, len(cfg.ApiKeys))
	for i, k := range cfg.ApiKeys {
		keys[i] = copyApiKeyInfoLocked(k)
	}
	return keys
}

func AddApiKey(key ApiKeyInfo) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ApiKeys = append(cfg.ApiKeys, key)
	return Save()
}

func DeleteApiKey(id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == id {
			cfg.ApiKeys = append(cfg.ApiKeys[:i], cfg.ApiKeys[i+1:]...)
			return Save()
		}
	}
	return nil
}

func UpdateApiKey(id string, key ApiKeyInfo) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == id {
			cfg.ApiKeys[i] = key
			return Save()
		}
	}
	return nil
}

func UpdateApiKeyStatsNoSave(id string, lastUsed, requests, errors, tokens int64, credits float64, models map[string]int64) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == id {
			cfg.ApiKeys[i].LastUsed = lastUsed
			cfg.ApiKeys[i].Requests = requests
			cfg.ApiKeys[i].Errors = errors
			cfg.ApiKeys[i].Tokens = tokens
			cfg.ApiKeys[i].Credits = credits
			if models != nil {
				cfg.ApiKeys[i].Models = copyModelCounts(models)
			}
			return
		}
	}
}

func GenerateApiKeyString() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

func copyModelCounts(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyApiKeyInfoLocked 集中处理 ApiKeyInfo 的深拷贝（包含 Models + SeriesPreferences + ChannelPreferences）。
// 凡是从 cfg.ApiKeys 取出返回给锁外的 ApiKeyInfo，都应走这里。
func copyApiKeyInfoLocked(src ApiKeyInfo) ApiKeyInfo {
	cp := src
	cp.Models = copyModelCounts(src.Models)
	cp.SeriesPreferences = copyStringMap(src.SeriesPreferences)
	cp.ChannelPreferences = copyStringMap(src.ChannelPreferences)
	return cp
}

// FindApiKeyByID returns a pointer to ApiKeyInfo by ID.
func FindApiKeyByID(id string) *ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	for _, k := range cfg.ApiKeys {
		if k.ID == id {
			c := copyApiKeyInfoLocked(k)
			return &c
		}
	}
	return nil
}

// DeductKeyBalance atomically deducts amount from an API key's balance.
// It prioritizes burning `Balance` (paid) first. If insufficient, it burns `GiftBalance`.
// Returns (success, remainingTotalBalance, paidAmountDeducted, giftedAmountDeducted).
func DeductKeyBalance(keyID string, amount float64) (bool, float64, float64, float64) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			totalBalance := cfg.ApiKeys[i].Balance + cfg.ApiKeys[i].GiftBalance
			if totalBalance < amount {
				return false, totalBalance, 0, 0
			}

			var paidDeducted, giftedDeducted float64

			// 1. Deduct from true Paid Balance first
			if cfg.ApiKeys[i].Balance >= amount {
				cfg.ApiKeys[i].Balance -= amount
				paidDeducted = amount
			} else {
				// Paid balance completely exhausted by this deduction
				paidDeducted = cfg.ApiKeys[i].Balance
				remainingAmount := amount - paidDeducted
				cfg.ApiKeys[i].Balance = 0

				// 2. Fallback to GiftBalance
				cfg.ApiKeys[i].GiftBalance -= remainingAmount
				giftedDeducted = remainingAmount
			}

			remainingTotal := cfg.ApiKeys[i].Balance + cfg.ApiKeys[i].GiftBalance
			Save()
			return true, remainingTotal, paidDeducted, giftedDeducted
		}
	}
	return false, 0, 0, 0
}

// AddKeyBalance adds paid balance to an API key. Reverses a deduction (used by RefundPreAuth).
func AddKeyBalance(keyID string, paidAmount, giftAmount float64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			if paidAmount != 0 {
				cfg.ApiKeys[i].Balance += paidAmount
			}
			if giftAmount != 0 {
				cfg.ApiKeys[i].GiftBalance += giftAmount
			}
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// AccumulateDebtUSD atomically adds unpaid NewAPI reconcile debt to an API key.
// 用于 Phase 4b 异步对账：扣不动余额时把欠款累积到 ApiKeyInfo.DebtUSD，
// admin 报表能直接看 underpaid 总额，下次有充值时再扣回。
func AccumulateDebtUSD(keyID string, deltaUSD float64) error {
	if deltaUSD <= 0 {
		return nil
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			cfg.ApiKeys[i].DebtUSD += deltaUSD
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// SetKeyBalances specifically sets both balance fields (used by admin panel).
func SetKeyBalances(keyID string, paidBalance float64, giftBalance float64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			// Track cumulative gifted amount (only increases)
			if giftBalance > cfg.ApiKeys[i].GiftBalance {
				cfg.ApiKeys[i].TotalGifted += giftBalance - cfg.ApiKeys[i].GiftBalance
			}
			cfg.ApiKeys[i].Balance = paidBalance
			cfg.ApiKeys[i].GiftBalance = giftBalance
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// ClearKeyWalletFields zeros legacy wallet fields after they have been migrated to a bound user.
// Used by users/store.go migrateLocked() v2→v3.
func ClearKeyWalletFields(keyID string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			cfg.ApiKeys[i].Balance = 0
			cfg.ApiKeys[i].GiftBalance = 0
			cfg.ApiKeys[i].TotalRecharged = 0
			cfg.ApiKeys[i].TotalGifted = 0
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// ExtendKeyExpiry extends expiration by N days. If current expiry is past, extends from now.
func ExtendKeyExpiry(keyID string, days int) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			base := cfg.ApiKeys[i].ExpiresAt
			now := time.Now().Unix()
			if base < now {
				base = now
			}
			cfg.ApiKeys[i].ExpiresAt = base + int64(days)*86400
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// ValidateKeyAccess checks if an API key has any active plan or balance.
// This is the initial gate check — model-level access is checked by ValidateKeyAccessForModel.
func ValidateKeyAccess(info *ApiKeyInfo) (string, error) {
	if !info.Enabled {
		return "key_disabled", fmt.Errorf("api key is disabled")
	}
	now := time.Now().Unix()
	hasDayCard := (info.Plan == "timed" || info.Plan == "hybrid") && (info.ExpiresAt == 0 || now <= info.ExpiresAt)
	hasBalance := info.Balance > 0 || info.GiftBalance > 0
	hasCreditPlan := info.Plan == "credit"

	// 如果没有 Plan 但有余额（赠送或付费），则视为隐式 credit 计划
	// 用户不需要激活码即可使用管理员赠送的余额
	if info.Plan == "" && hasBalance {
		hasCreditPlan = true
	}

	if !hasDayCard && !hasBalance && !hasCreditPlan {
		if info.Plan == "" {
			return "not_activated", fmt.Errorf("api key not activated, please redeem an activation code")
		}
		return "key_expired", fmt.Errorf("api key expired and insufficient balance")
	}
	return "", nil
}

// ValidateKeyAccessForModel checks if a key can access a model in the given pool.
// Returns action: "free" (no charge), "deduct" (charge balance), or error.
//
// v3.5 简化（2026-05-09）：取消"free 天卡 vs pro 天卡"区分。
//   - 任何天卡（plan=timed/hybrid 且未过期）覆盖**所有**模型（free + pro 池），不扣费
//   - 没天卡但有余额 → deduct
//   - 都没有 → 错误
//
// 历史背景：早期 ApiKeyInfo.Tier ("free"/"pro") 用于限制 free 天卡只能调 sonnet-4.5，
// 防止低价天卡用户调高成本 PRO 模型。但 admin UI 后来就不暴露 tier 选择了，
// 字段成了悬空逻辑（旧 key 残留 tier="free" 反而把用户卡住）。
// 想限制成本走 RateLimitPerMin（速率限制），不走 tier 区分。
// 字段保留兼容（不读不写）。
func ValidateKeyAccessForModel(info *ApiKeyInfo, modelPool string) (string, error) {
	if info == nil || !info.Enabled {
		return "", fmt.Errorf("api key disabled")
	}
	now := time.Now().Unix()
	hasDayCard := (info.Plan == "timed" || info.Plan == "hybrid") && (info.ExpiresAt == 0 || now <= info.ExpiresAt)
	hasBalance := info.Balance > 0 || info.GiftBalance > 0

	// 任何天卡覆盖所有模型，不扣费
	if hasDayCard {
		return "free", nil
	}
	// 余额按需扣（free / pro 池都从同一个 balance 扣）
	if hasBalance {
		return "deduct", nil
	}
	// modelPool 参数保留是为了不破坏调用方签名（未来如要区分计费规则可重新启用）
	_ = modelPool
	return "", fmt.Errorf("api key has no active day-card and no balance")
}

// IsResellerKey 判断是不是开通了代理的 key
func (i *ApiKeyInfo) IsResellerKey() bool {
	return i != nil && i.IsReseller
}

// IsChildKey 判断是不是某 reseller 的子 key
func (i *ApiKeyInfo) IsChildKey() bool {
	return i != nil && i.ParentKeyID != ""
}

// GetChildKeys 返回某 reseller 的所有子 key（深拷贝）
func GetChildKeys(parentKeyID string) []ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	var children []ApiKeyInfo
	for _, k := range cfg.ApiKeys {
		if k.ParentKeyID == parentKeyID {
			c := copyApiKeyInfoLocked(k)
			children = append(children, c)
		}
	}
	return children
}

// TransferBalance 原子操作：reseller→child 转账
//   - 校验 to.ParentKeyID == fromKeyID（防横向越权）
//   - 校验 from.Balance >= amountUSD
//   - 一次写盘
// TransferBalance 在 reseller(parent) 与 child key 之间转账。
//
// amountUSD 语义（双向）：
//   - amountUSD > 0: parent → child（充入；同步 parent.SoldToChildren += amount, child.TotalRecharged += amount）
//   - amountUSD < 0: child → parent（扣回；同步 parent.SoldToChildren -= |amount|, child.TotalRecharged -= |amount|，最少为 0 不到负）
//   - amountUSD == 0: 拒绝
//
// fromKeyID 始终是 reseller(parent) ID（不是资金来源方向）；toKeyID 始终是 child ID。
// 负数方向由后端语义决定，调用方不需要倒置参数。
func TransferBalance(fromKeyID, toKeyID string, amountUSD float64) error {
	if amountUSD == 0 {
		return fmt.Errorf("amount must be non-zero")
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	var fromIdx, toIdx int = -1, -1
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID == fromKeyID {
			fromIdx = i
		}
		if cfg.ApiKeys[i].ID == toKeyID {
			toIdx = i
		}
	}
	if fromIdx < 0 {
		return fmt.Errorf("source key not found")
	}
	if toIdx < 0 {
		return fmt.Errorf("target key not found")
	}
	if cfg.ApiKeys[toIdx].ParentKeyID != fromKeyID {
		return fmt.Errorf("not your child key") // 横向越权拦截
	}

	if amountUSD > 0 {
		// 充入：parent → child
		if cfg.ApiKeys[fromIdx].Balance < amountUSD {
			return fmt.Errorf("insufficient balance")
		}
		cfg.ApiKeys[fromIdx].Balance -= amountUSD
		cfg.ApiKeys[fromIdx].SoldToChildren += amountUSD
		cfg.ApiKeys[toIdx].Balance += amountUSD
		cfg.ApiKeys[toIdx].TotalRecharged += amountUSD
	} else {
		// 扣回：child → parent；amount = -amountUSD（正数）
		recall := -amountUSD
		if cfg.ApiKeys[toIdx].Balance < recall {
			return fmt.Errorf("child balance insufficient for recall")
		}
		cfg.ApiKeys[toIdx].Balance -= recall
		cfg.ApiKeys[fromIdx].Balance += recall
		// 修正历史统计（不到负）
		if cfg.ApiKeys[fromIdx].SoldToChildren >= recall {
			cfg.ApiKeys[fromIdx].SoldToChildren -= recall
		} else {
			cfg.ApiKeys[fromIdx].SoldToChildren = 0
		}
		if cfg.ApiKeys[toIdx].TotalRecharged >= recall {
			cfg.ApiKeys[toIdx].TotalRecharged -= recall
		} else {
			cfg.ApiKeys[toIdx].TotalRecharged = 0
		}
	}
	return Save()
}

// RefundChildBalance 删除子 key 时把它剩余余额（Balance + GiftBalance）退回 reseller 的 Balance。
// 返回退还的总金额（USD）。
func RefundChildBalance(childKeyID string) (float64, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	var childIdx, parentIdx int = -1, -1
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID == childKeyID {
			childIdx = i
		}
	}
	if childIdx < 0 {
		return 0, fmt.Errorf("child not found")
	}
	parentID := cfg.ApiKeys[childIdx].ParentKeyID
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID == parentID {
			parentIdx = i
		}
	}
	refund := cfg.ApiKeys[childIdx].Balance + cfg.ApiKeys[childIdx].GiftBalance
	if parentIdx >= 0 && refund > 0 {
		cfg.ApiKeys[parentIdx].Balance += refund
		// 修正"已销售"统计（避免负数）
		cfg.ApiKeys[parentIdx].SoldToChildren -= refund
		if cfg.ApiKeys[parentIdx].SoldToChildren < 0 {
			cfg.ApiKeys[parentIdx].SoldToChildren = 0
		}
	}
	cfg.ApiKeys[childIdx].Balance = 0
	cfg.ApiKeys[childIdx].GiftBalance = 0
	return refund, Save()
}

// ClearAllGiftBalances zeros GiftBalance on every key (does NOT touch Balance or TotalGifted).
// Returns (count, totalCleared).
func ClearAllGiftBalances() (int, float64) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	count := 0
	var total float64
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].GiftBalance > 0 {
			total += cfg.ApiKeys[i].GiftBalance
			cfg.ApiKeys[i].GiftBalance = 0
			count++
		}
	}
	if count > 0 {
		_ = Save()
	}
	return count, total
}
